// Registers the offline service worker and, in the background after the
// page has already loaded, works through the catalog one song at a time:
// diff the lightweight manifest (GET /offline/manifest.json — slugs and
// content hashes only, always fresh, never cached server-side — see
// internal/web/offline_snapshot.go) against what's already in IndexedDB,
// then fetch just the missing/stale songs' rendered pages
// (GET /offline/songs/{slug} — see internal/web/offline_snapshot.go), one
// request at a time so a bad connection degrades gracefully instead of
// racing a pile of parallel requests. Runs on every page load — home page
// or a song page — and if it's a song page, that song jumps to the front
// of the queue so it's available offline right away, not whenever its
// turn comes up in slug order.
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

  window.addEventListener("load", syncOfflineCatalog);

  // A song page's URL always starts /songs/<slug> (show, play, or edit) —
  // whichever song is actually on screen right now is what should finish
  // downloading first.
  function currentSongSlug() {
    var match = /^\/songs\/([^/]+)/.exec(location.pathname);
    return match ? match[1] : null;
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
            return; // every song in this manifest is already stored
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
  // downloads the rest one at a time (current-page song first). Only
  // marks the manifest version as fully synced if every song in the queue
  // actually landed — a failure partway through just leaves the remaining
  // slugs looking stale, so the next page load's diff naturally picks up
  // where this one left off.
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

      return offlineDBDeleteSongs(toDelete)
        .then(function () {
          return processQueue(toFetch);
        })
        .then(function (completed) {
          if (completed) {
            return offlineDBSetMeta("syncedVersion", manifest.version);
          }
        });
    });
  }

  // processQueue fetches one song at a time. Returns whether the whole
  // queue finished — false means it stopped early after a slug failed
  // even after retrying, most likely because the connection dropped
  // entirely; there's no point burning through the rest of a possibly
  // large catalog one slow timeout at a time once that's happened.
  function processQueue(slugs) {
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
        return offlineDBPutSong(song).then(next);
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
