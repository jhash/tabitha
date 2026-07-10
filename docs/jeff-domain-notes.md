# Domain notes from Jeff (2026-07-10)

Raw notes from Jake's in-person meeting with Jeff, cleaned up for
reference. This is source material for future parser/editor/data-model
work — not a task list. See `todos.md` for what's actually scheduled.
The canonical chord-notation legend Jeff references is saved verbatim at
`music/template-song.txt` (the doc he copies to start every new song).

## Transpose workflow

Jeff's current process, done by hand in Google Docs:

1. Look up the new key in Chord Changer.
2. Put the new key's version at the **top** of the doc.
3. A blank page separates the two versions.
4. The original key's version follows (usually — he said "usually," not
   always).

**Implication:** a single Google Doc can contain more than one full
transcription (different keys), concatenated with a page break as the
delimiter. Our schema already has a `key` and `capo` column per
`transcription_versions` row, and supports multiple versions per song — so
the data model already fits. What's still open: digestion needs to detect
the page-break delimiter and split one doc into N version rows, and decide
which one is `is_current` (probably whichever is first/top, i.e. the
newest key, but confirm with Jake before assuming). Relevant to Task 23
(real digestion) — no action needed until then.

## Table of contents conventions

- New songs Jeff wants added go at the **bottom** of the TOC.
- Jeff starts a new song by copying the template doc (`music/template-song.txt`).
- **Cell/row color carries meaning the free-text `status` column doesn't
  capture on its own:**
  - Blue title cell = nothing written yet.
  - Yellow title cell = needs adjustments.
  - Light green row = marked "Quality Check" (Jeff estimates roughly half
    of these are actually already good to go).
  - Darker green = Disney.
  - Orange = one of Jeff's cruise gigs, played in an unfinished state.
- Categorization/genre-adjacent grouping also happens via Google Drive
  **folder structure**, separate from the sheet itself.

**Implication:** our current `toc_sync` job only reads the sheet's
unauthenticated CSV export, which carries cell *values* but not
formatting (color) or the source doc's Drive folder. Real color/folder
data needs the authenticated Sheets/Drive API — same OAuth dependency as
`digest_song` and the doc-ID hyperlink extraction. Worth capturing a
`color`/`category` signal on `songs` once that's wired up (Task 23+), not
now.

## Chord notation conventions

- Reference metadata (tuning, key, capo) often gets copy-pasted in from
  Ultimate Guitar or Chord Changer sources — matches our existing `key`/
  `capo` columns.
- Repeated progressions across several lines get written as `x2`, `x3`,
  etc. rather than spelled out. The parser doesn't currently special-case
  this.
- `(CHORUS)` in parentheses means "repeat exactly what it did before."
  Some sheets leave the repeat implicit (blank) and assume the reader
  knows to repeat between verses — sometimes the repeat is even slightly
  different from the original. (Our parser's existing repeat-reference
  handling was built against this kind of thing already, per the original
  Satisfaction sample — good sign it generalizes, but expect more variety
  once we see a real range of docs.)
- Prefers `4` over `sus` to save space, and will drop the root letter
  entirely when it's unambiguous from context (e.g. "last chord was a 4").
- `C(m)` = "C major or minor, context-dependent" (not C-major-over-C-minor
  as a slash chord). Jake flagged this is genuinely ambiguous notation and
  was thinking out loud about whether a `+` or other character would be
  clearer than reusing `(m)` — **open question, not a decision**, don't
  design around a specific replacement yet.
- Full legend: see `music/template-song.txt`.

## Live performance / UX priorities

- Jeff's current goal for any given sheet is a **one-pager** — easiest to
  read at a glance. Printable output still matters even once this is a
  web app, not just on-screen.
- Wants shortened/wrapped lines for phone-sized screens; a wider layout
  (matching 8.5"x11" print) is his current focus for iPad.
- Prefers **swiping between pages** over scrolling (Google Docs' current
  scroll behavior is explicitly called out as not what he wants).
- A live-playthrough UI matters a lot for actual on-stage/cruise-ship use,
  ideally auto-detecting phone vs. iPad and adjusting layout accordingly.
- Jeff currently uses **OnSong** during cruise gigs — worth a look as
  prior art for the live-performance UI specifically (not the editing
  side).

This is a real, distinct UI mode (performance/reading mode vs. editing
mode) — bigger than a styling tweak, but squarely post-editor work.

## Data model / feature ideas surfaced

- **Genre** needs to be many-to-many (a song can have multiple genres) —
  its own table + join table, not a column on `songs`.
- Search should index all TOC metadata, not just title/artist.
- Sheet-music PDFs exist for some songs (Jeff has them separately) — idea
  is to show them per-song, possibly as a popup scoped to a specific
  riff rather than the whole PDF inline.
- Crowd-sourced quality checks, with a superadmin "seal of approval" as a
  final step — implies a review/approval state beyond our current
  `user`/`superadmin` role enum, and possibly a status field on
  `transcription_versions` itself. Not designed yet.
- Live MIDI recording, or some other live-ingestion path, as another way
  to get a transcription in besides typing/scraping.
- Notation preference as a **user setting**: e.g. show `∆` vs `maj7` vs
  `M` for major-seventh depending on what the viewer prefers, rather than
  picking one convention for everyone.

All of the above: future features, not scheduled. Logged here so they
don't get lost, and so schema/parser decisions made before then don't
accidentally foreclose them (e.g. don't hardcode single-genre-per-song
anywhere).

## Open research question (Jake's own note, not yet investigated)

Jeff asked whether it'd be legal to have AI listen to a recording and
transcribe the exact notes played. Not researched yet — flagging it here
since it's directly relevant to the already-deferred "audio-upload
transcription" and "LLM-based auto-transcription" future features. Ask
Jake if/when he wants this actually researched.
