// On-the-fly chord transposition: shifts every ".chord" span in the
// current page's transcription by whole semitones. Pure string rewriting
// over DOM nodes that are already rendered (see
// internal/web/transcription_render.go's chordLineNode, which gives each
// chord its own <span class="chord">) — no network round-trip, no
// server re-render, so this works identically online or from the
// service-worker/IndexedDB offline cache (see static/js/offline-db.js).
//
// Deliberately a classic script, not a module: it's embedded inside
// boosted content (see song_show.go/song_play.go) and needs to re-run on
// every htmx-boosted navigation between songs. ES modules are deduped by
// the browser per URL and never re-execute on a re-inserted <script
// type="module">, which is exactly why editor.js/play.js instead opt
// their pages out of hx-boost (see TestSongShowEditLinkOptsOutOfHtmxBoost).
// A classic script has no such dedup, so it can live directly in the
// swapped content and just re-initialize each time.
(function () {
  var NOTE_TO_PC = {
    C: 0, "B#": 0,
    "C#": 1, Db: 1,
    D: 2,
    "D#": 3, Eb: 3,
    E: 4, Fb: 4,
    F: 5, "E#": 5,
    "F#": 6, Gb: 6,
    G: 7,
    "G#": 8, Ab: 8,
    A: 9,
    "A#": 10, Bb: 10,
    B: 11, Cb: 11,
  };
  var SHARP_NAMES = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"];
  var FLAT_NAMES = ["C", "Db", "D", "Eb", "E", "F", "Gb", "G", "Ab", "A", "Bb", "B"];

  // Tonic pitch classes conventionally spelled with flats (the rest,
  // including "no key known", default to sharps) — the standard circle-
  // of-fifths flat side: F Bb Eb Ab Db Gb (major), Dm Gm Cm Fm Bbm Ebm
  // (minor).
  var FLAT_MAJOR_PCS = { 1: true, 3: true, 5: true, 6: true, 8: true, 10: true };
  var FLAT_MINOR_PCS = { 0: true, 2: true, 3: true, 5: true, 7: true, 10: true };

  // Matches a bare chord: root, optional accidental, optional
  // quality/extension text, optional /bass — the same shape as
  // internal/transcription/parser.go's chordTokenRe, minus the
  // x-count/bar/paren alternatives (handled separately below).
  var CHORD_RE = /^([A-Ga-g])(#|b)?((?:maj|min|dim|aug|sus2|sus4|sus|add2|add4|add6|add9|m|∆|Δ)?[0-9]{0,2})(?:\/([A-Ga-g])(#|b)?)?$/i;

  // A bare note inside a parenthesized chord group, e.g. the "/F" and "G"
  // in "(/F /F /F#  G)" — optionally slash-prefixed, no quality.
  var PAREN_NOTE_RE = /^(\/?)([A-Ga-g])(#|b)?$/i;

  function parseNote(letter, accidental) {
    return NOTE_TO_PC[letter.toUpperCase() + (accidental || "")];
  }

  function spellNote(pc, useFlats, caseLike) {
    var names = useFlats ? FLAT_NAMES : SHARP_NAMES;
    var name = names[((pc % 12) + 12) % 12];
    return caseLike === caseLike.toLowerCase() ? name.toLowerCase() : name;
  }

  function transposeNote(letter, accidental, semitones, useFlats) {
    var pc = parseNote(letter, accidental);
    if (pc === undefined) return letter + (accidental || "");
    return spellNote(pc + semitones, useFlats, letter);
  }

  function transposeParenGroup(token, semitones, useFlats) {
    var words = token
      .slice(1, -1)
      .split(" ")
      .map(function (w) {
        var m = PAREN_NOTE_RE.exec(w);
        return m ? m[1] + transposeNote(m[2], m[3], semitones, useFlats) : w;
      });
    return "(" + words.join(" ") + ")";
  }

  // transposeChordToken shifts one chord-row token by `semitones`,
  // preserving repeat markers ("x4"), bar delimiters ("|"), and
  // non-chord parenthetical annotations ("(drums)") untouched.
  function transposeChordToken(token, semitones, useFlats) {
    if (token === "" || token === "|" || /^x[0-9]+$/i.test(token)) return token;
    if (token.charAt(0) === "(" && token.charAt(token.length - 1) === ")") {
      return transposeParenGroup(token, semitones, useFlats);
    }
    var m = CHORD_RE.exec(token);
    if (!m) return token;
    var root = transposeNote(m[1], m[2], semitones, useFlats);
    var bass = m[4] ? "/" + transposeNote(m[4], m[5], semitones, useFlats) : "";
    return root + (m[3] || "") + bass;
  }

  // parseKey reads a plain key string like "Bb" or "F#m" into a tonic
  // {pc, isMinor} — used only to pick sharp-vs-flat spelling. Falls back
  // to C major for empty or unrecognized text so a missing/odd Key line
  // never breaks transposition, just its spelling preference.
  function parseKey(text) {
    var m = /^([A-Ga-g])(#|b)?(m)?$/i.exec((text || "").trim());
    if (!m) return { pc: 0, isMinor: false };
    var pc = parseNote(m[1], m[2]);
    return { pc: pc === undefined ? 0 : pc, isMinor: !!m[3] };
  }

  function keyUsesFlats(key) {
    var pcs = key.isMinor ? FLAT_MINOR_PCS : FLAT_MAJOR_PCS;
    return !!pcs[((key.pc % 12) + 12) % 12];
  }

  function spellKey(key) {
    var name = (keyUsesFlats(key) ? FLAT_NAMES : SHARP_NAMES)[((key.pc % 12) + 12) % 12];
    return key.isMinor ? name + "m" : name;
  }

  function initTransposeControls(root) {
    var controls = root.querySelector(".transpose-controls");
    if (!controls) return;

    var chordEls = controls.dataset.scope
      ? root.querySelectorAll(controls.dataset.scope + " .chord")
      : root.querySelectorAll(".chord");
    var keyEl = controls.querySelector(".transpose-key");
    var downBtn = controls.querySelector(".transpose-down");
    var upBtn = controls.querySelector(".transpose-up");
    var baseKey = parseKey(controls.dataset.key);
    var knownKey = !!controls.dataset.key;
    var semitones = 0;

    // At net-zero semitones, restore each chord's exact original text
    // rather than round-tripping it through transposeChordToken/spellKey
    // — those always re-spell per the sharp/flat convention, which can
    // silently swap an original flat spelling (e.g. "Eb") for its sharp
    // equivalent ("D#") even though nothing was actually transposed.
    function apply() {
      chordEls.forEach(function (el) {
        if (el.dataset.original === undefined) {
          el.dataset.original = el.textContent;
        }
      });
      if (semitones === 0) {
        chordEls.forEach(function (el) {
          el.textContent = el.dataset.original;
        });
        if (keyEl) keyEl.textContent = knownKey ? controls.dataset.key : "0";
        return;
      }
      var current = { pc: baseKey.pc + semitones, isMinor: baseKey.isMinor };
      var useFlats = keyUsesFlats(current);
      chordEls.forEach(function (el) {
        el.textContent = transposeChordToken(el.dataset.original, semitones, useFlats);
      });
      if (keyEl) {
        keyEl.textContent = knownKey ? spellKey(current) : (semitones > 0 ? "+" : "") + semitones;
      }
    }

    downBtn.addEventListener("click", function () {
      semitones -= 1;
      apply();
    });
    upBtn.addEventListener("click", function () {
      semitones += 1;
      apply();
    });
  }

  initTransposeControls(document);
})();
