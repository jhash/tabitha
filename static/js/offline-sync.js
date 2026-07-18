// Registers the offline service worker (needed for PWA installability
// itself, and for the lighter "cache pages I actually visit" layer — see
// static/sw.js — regardless of context) and, only when actually running
// as an installed PWA or inside the Capacitor native wrapper (mobile/),
// works through the catalog one song at a time in the background: diff
// the lightweight manifest (GET /offline/manifest.json — slugs and
// content hashes only, always fresh, never cached server-side — see
// internal/web/offline_snapshot.go) against what's already in IndexedDB,
// then fetch just the missing/stale songs' rendered pages
// (GET /offline/songs/{slug}), one request at a time so a bad connection
// degrades gracefully instead of racing a pile of parallel requests. Runs
// on every page load — home page or a song page — and if it's a song
// page, that song jumps to the front of the queue so it's available
// offline right away, not whenever its turn comes up in slug order.
//
// The catalog download is real bandwidth and device storage — worth
// spending for something that's actually going to be used offline, not
// for an ordinary browser tab a visitor happens to have open. Gating on
// "installed" rather than "supports service workers" is what that check
// is for.
//
// Deliberately hand-rolled rather than a job-queue library: IndexedDB is
// already the durable "what's done" record, so the pending queue is just
// recomputed from a diff each run — there's no separate queue state to
// persist, and nothing here needs to survive the page closing (Safari has
// no Background Sync API to do that with anyway, so the queue's real
// resilience story is simply "try again next time a page loads").
(function () {
  "use strict";

  if (!("serviceWorker" in navigator) || !("indexedDB" in window)) {
    return;
  }

  navigator.serviceWorker.register("/sw.js").catch(function () {
    // Offline support just isn't available this session — the site works
    // fine online either way, so there's nothing to surface to the user.
  });

  if (!isInstalledApp()) {
    return;
  }

  window.addEventListener("load", syncOfflineCatalog);

  // isInstalledApp recognizes the two ways this page can be running as an
  // installed app rather than a regular browser tab: launched standalone
  // (the PWA install path, on any browser that supports the display-mode
  // media feature, plus the older iOS Safari-specific flag) or loaded
  // inside Capacitor's native WebView (mobile/ — Capacitor injects
  // window.Capacitor into whatever it loads, remote URL or not).
  function isInstalledApp() {
    if (window.matchMedia && window.matchMedia("(display-mode: standalone)").matches) {
      return true;
    }
    if (navigator.standalone === true) {
      return true;
    }
    if (window.Capacitor && typeof window.Capacitor.isNativePlatform === "function" && window.Capacitor.isNativePlatform()) {
      return true;
    }
    return false;
  }

  // A song page's URL always starts /songs/<slug> (show, play, or edit) —
  // whichever song is actually on screen right now is what should finish
  // downloading first.
  function currentSongSlug() {
    var match = /^\/songs\/([^/]+)/.exec(location.pathname);
    return match ? match[1] : null;
  }

  // setStatus updates the shared header's "x/total downloaded" indicator
  // (see internal/web/layout.go) — a no-op if it's not on this page (it
  // isn't, on the chrome-less Play mode reader).
  function setStatus(current, total, syncing) {
    var el = document.getElementById("offline-status");
    if (!el) {
      return;
    }
    el.textContent = current + "/" + total + " downloaded";
    el.hidden = false;
    el.classList.toggle("is-syncing", !!syncing);
  }

  function syncOfflineCatalog() {
    fetch("/offline/manifest.json", { cache: "no-store" })
      .then(function (res) {
        return res.ok ? res.json() : null;
      })
      .then(function (manifest) {
        if (!manifest || !manifest.version) {
          return;
        }
        return offlineDBGetMeta("syncedVersion").then(function (syncedVersion) {
          if (syncedVersion === manifest.version) {
            // Every song in this manifest is already stored.
            setStatus(manifest.songs.length, manifest.songs.length, false);
            return;
          }
          return reconcileCatalog(manifest);
        });
      })
      .catch(function () {
        // No network, or IndexedDB unavailable — whatever's already
        // stored stays as-is until this succeeds on some future page load.
      });
  }

  // reconcileCatalog diffs the manifest against IndexedDB's current
  // contents, deletes any stored song no longer in the catalog, and
  // downloads the rest one at a time (current-page song first), updating
  // the header status as it goes. Only marks the manifest version as
  // fully synced if every song in the queue actually landed — a failure
  // partway through just leaves the remaining slugs looking stale, so the
  // next page load's diff naturally picks up where this one left off.
  function reconcileCatalog(manifest) {
    return offlineDBGetAllSongs().then(function (stored) {
      var storedHashes = {};
      stored.forEach(function (song) {
        storedHashes[song.slug] = song.contentHash;
      });

      var manifestSlugs = {};
      var toFetch = [];
      manifest.songs.forEach(function (entry) {
        manifestSlugs[entry.slug] = true;
        if (storedHashes[entry.slug] !== entry.contentHash) {
          toFetch.push(entry.slug);
        }
      });

      var toDelete = Object.keys(storedHashes).filter(function (slug) {
        return !manifestSlugs[slug];
      });

      var priority = currentSongSlug();
      if (priority) {
        var idx = toFetch.indexOf(priority);
        if (idx > 0) {
          toFetch.splice(idx, 1);
          toFetch.unshift(priority);
        }
      }

      var total = manifest.songs.length;
      var done = total - toFetch.length;
      setStatus(done, total, toFetch.length > 0);

      return offlineDBDeleteSongs(toDelete)
        .then(function () {
          return processQueue(toFetch, function () {
            done++;
            setStatus(done, total, done < total);
          });
        })
        .then(function (completed) {
          if (completed) {
            setStatus(total, total, false);
            return offlineDBSetMeta("syncedVersion", manifest.version);
          }
        });
    });
  }

  // processQueue fetches one song at a time, calling onProgress after each
  // successful save. Returns whether the whole queue finished — false
  // means it stopped early after a slug failed even after retrying, most
  // likely because the connection dropped entirely; there's no point
  // burning through the rest of a possibly large catalog one slow timeout
  // at a time once that's happened.
  function processQueue(slugs, onProgress) {
    var i = 0;
    function next() {
      if (i >= slugs.length) {
        return Promise.resolve(true);
      }
      var slug = slugs[i++];
      return fetchSongWithRetry(slug, 0).then(function (song) {
        if (!song) {
          return false;
        }
        return offlineDBPutSong(song).then(function () {
          onProgress();
          return next();
        });
      });
    }
    return next();
  }

  // fetchSongWithRetry gives a single flaky request a couple of quick
  // retries (500ms, then 1000ms) before giving up on this slug for now.
  function fetchSongWithRetry(slug, attempt) {
    return fetch("/offline/songs/" + encodeURIComponent(slug), { cache: "no-store" })
      .then(function (res) {
        return res.ok ? res.json() : null;
      })
      .catch(function () {
        return null;
      })
      .then(function (song) {
        if (song || attempt >= 2) {
          return song;
        }
        return new Promise(function (resolve) {
          setTimeout(resolve, 500 * (attempt + 1));
        }).then(function () {
          return fetchSongWithRetry(slug, attempt + 1);
        });
      });
  }
})();
