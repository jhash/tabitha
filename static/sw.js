// tabitha's offline service worker.
//
// Two layers of offline support:
//   1. Any page actually visited while online gets cached as-is (below,
//      handlePageRequest) — the classic "revisit what you saw" PWA cache.
//   2. A song that was *never* visited can still work offline, because
//      static/js/offline-sync.js has already copied a background-built
//      SQLite export of every digested song's pre-rendered HTML (see
//      internal/web/offline_snapshot.go) into IndexedDB. sql.js (vendored,
//      static/js/vendor/sqljs/) is loaded lazily, right here, only once
//      that fallback path is actually needed — it's never fetched on a
//      normal page load.
importScripts("/static/js/offline-db.js");

var CACHE_NAME = "tabitha-shell-v1";
var SQLJS_DIR = "/static/js/vendor/sqljs/";

// The minimum needed to render *something* for any page offline: shared
// chrome assets, not any particular page.
var APP_SHELL = [
  "/static/css/reset.css",
  "/static/css/style.css",
  "/static/js/htmx.min.js",
  "/static/fonts/Lora-Variable.woff2",
  "/static/fonts/Lora-Italic-Variable.woff2",
  "/static/manifest.webmanifest",
  "/static/icons/icon-192.png",
  "/static/icons/icon-512.png",
];

self.addEventListener("install", function (event) {
  event.waitUntil(
    caches
      .open(CACHE_NAME)
      .then(function (cache) {
        return cache.addAll(APP_SHELL);
      })
      .then(function () {
        return self.skipWaiting();
      })
  );
});

self.addEventListener("activate", function (event) {
  event.waitUntil(
    caches
      .keys()
      .then(function (keys) {
        return Promise.all(
          keys
            .filter(function (key) {
              return key !== CACHE_NAME;
            })
            .map(function (key) {
              return caches.delete(key);
            })
        );
      })
      .then(function () {
        return self.clients.claim();
      })
  );
});

self.addEventListener("fetch", function (event) {
  var req = event.request;
  if (req.method !== "GET") {
    return;
  }

  var url = new URL(req.url);
  if (url.origin !== self.location.origin) {
    return;
  }

  if (url.pathname.indexOf("/static/") === 0) {
    event.respondWith(cacheFirst(req));
    return;
  }

  if (isPageRequest(req)) {
    event.respondWith(handlePageRequest(req));
  }
});

// isPageRequest recognizes both a real browser navigation and an
// htmx-boosted fetch (hx-boost is on site-wide — see internal/web/layout.go
// — so most in-app link clicks are the latter, not the former) as "this
// wants a rendered song/home page back", the two cases that need an
// offline fallback.
function isPageRequest(req) {
  if (req.mode === "navigate") {
    return true;
  }
  if (req.headers.get("HX-Request") !== "true") {
    return false;
  }
  var accept = req.headers.get("Accept") || "";
  return accept.indexOf("application/json") === -1;
}

function cacheFirst(req) {
  return caches.match(req).then(function (cached) {
    if (cached) {
      return cached;
    }
    return fetch(req).then(function (res) {
      var copy = res.clone();
      caches.open(CACHE_NAME).then(function (cache) {
        cache.put(req, copy);
      });
      return res;
    });
  });
}

function handlePageRequest(req) {
  return fetch(req)
    .then(function (res) {
      if (res.ok) {
        var copy = res.clone();
        caches.open(CACHE_NAME).then(function (cache) {
          cache.put(req, copy);
        });
      }
      return res;
    })
    .catch(function () {
      return caches.match(req).then(function (cached) {
        return cached || serveFromOfflineSnapshot(req);
      });
    });
}

function serveFromOfflineSnapshot(req) {
  var path = new URL(req.url).pathname;
  var match = /^\/songs\/([^/]+)\/?$/.exec(path);
  if (!match) {
    // Not a song URL (e.g. the home page, or a numeric-ID redirect target)
    // — nothing in the snapshot to look it up by.
    return offlineFallbackResponse();
  }
  var slug = match[1];

  return offlineDBGet("data")
    .then(function (data) {
      if (!data) {
        return offlineFallbackResponse();
      }
      return querySnapshotForSlug(data, slug).then(function (html) {
        if (!html) {
          return offlineFallbackResponse();
        }
        return new Response(html, {
          headers: { "Content-Type": "text/html; charset=utf-8" },
        });
      });
    })
    .catch(function () {
      return offlineFallbackResponse();
    });
}

var sqlJsPromise = null;
function loadSqlJs() {
  if (!sqlJsPromise) {
    importScripts(SQLJS_DIR + "sql-wasm.js");
    sqlJsPromise = initSqlJs({
      locateFile: function (file) {
        return SQLJS_DIR + file;
      },
    });
  }
  return sqlJsPromise;
}

function querySnapshotForSlug(data, slug) {
  return loadSqlJs().then(function (SQL) {
    var db = new SQL.Database(new Uint8Array(data));
    try {
      var stmt = db.prepare("SELECT html FROM songs WHERE slug = ?");
      stmt.bind([slug]);
      var html = stmt.step() ? stmt.getAsObject().html : null;
      stmt.free();
      return html;
    } finally {
      db.close();
    }
  });
}

function offlineFallbackResponse() {
  return new Response(
    "<!doctype html><meta charset=utf-8><meta name=viewport content=\"width=device-width, initial-scale=1\">" +
      "<title>Offline · tabitha</title>" +
      "<p style=\"font:1.125rem system-ui,sans-serif;max-width:32rem;margin:3rem auto;padding:0 1rem;\">" +
      "You’re offline, and this page hasn’t been saved for offline viewing yet. " +
      "Open it once while online and it’ll be available offline from then on.</p>",
    { status: 503, headers: { "Content-Type": "text/html; charset=utf-8" } }
  );
}
