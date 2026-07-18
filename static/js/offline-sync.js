// Registers the offline service worker and, in the background after the
// page has already loaded, copies the latest offline data snapshot (every
// digested song, pre-rendered — see internal/web/offline_snapshot.go) into
// IndexedDB. Deliberately tiny and dependency-free: this never delays
// first paint, and does no rendering itself — static/sw.js does the actual
// offline serving, reading what this script stored.
(function () {
  "use strict";

  if (!("serviceWorker" in navigator) || !("indexedDB" in window)) {
    return;
  }

  navigator.serviceWorker.register("/sw.js").catch(function () {
    // Offline support just isn't available this session — the site works
    // fine online either way, so there's nothing to surface to the user.
  });

  window.addEventListener("load", syncOfflineSnapshot);

  function syncOfflineSnapshot() {
    fetch("/offline/meta", { cache: "no-store" })
      .then(function (res) {
        return res.ok ? res.json() : null;
      })
      .then(function (meta) {
        if (!meta || !meta.version) {
          return;
        }
        return offlineDBGet("version").then(function (storedVersion) {
          if (storedVersion === meta.version) {
            return; // already have the current catalog offline
          }
          return fetch("/offline/snapshot.sqlite", { cache: "no-store" })
            .then(function (res) {
              return res.ok ? res.arrayBuffer() : null;
            })
            .then(function (data) {
              if (data) {
                return offlineDBPutSnapshot(meta.version, data);
              }
            });
        });
      })
      .catch(function () {
        // No network, or IndexedDB unavailable — whatever offline copy is
        // already stored (if any) stays as-is until this succeeds later.
      });
  }
})();
