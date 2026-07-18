// Shared between static/js/offline-sync.js (runs on the main thread) and
// static/sw.js (runs in the service worker, via importScripts) so both
// agree on the IndexedDB layout without duplicating it. Plain top-level
// function/var declarations rather than an ES module, since a classic
// script works unchanged in both a <script> tag and importScripts().
//
// Two object stores:
//   - OFFLINE_META_STORE: a plain key/value store. Currently just
//     "syncedVersion" — the catalog manifest version
//     static/js/offline-sync.js last finished downloading every song for
//     (see reconcileCatalog there). Unset or stale means some songs might
//     still be missing.
//   - OFFLINE_SONGS_STORE: one record per digested song, keyed by slug
//     (its inline keyPath) — a direct get(slug) is all static/sw.js ever
//     needs to serve a song page that was never visited online.

var OFFLINE_DB_NAME = "tabitha-offline";
var OFFLINE_DB_VERSION = 1;
var OFFLINE_META_STORE = "meta";
var OFFLINE_SONGS_STORE = "songs";

function openOfflineDB() {
  return new Promise(function (resolve, reject) {
    var req = indexedDB.open(OFFLINE_DB_NAME, OFFLINE_DB_VERSION);
    req.onupgradeneeded = function () {
      var db = req.result;
      if (!db.objectStoreNames.contains(OFFLINE_META_STORE)) {
        db.createObjectStore(OFFLINE_META_STORE);
      }
      if (!db.objectStoreNames.contains(OFFLINE_SONGS_STORE)) {
        db.createObjectStore(OFFLINE_SONGS_STORE, { keyPath: "slug" });
      }
    };
    req.onsuccess = function () {
      resolve(req.result);
    };
    req.onerror = function () {
      reject(req.error);
    };
  });
}

function offlineDBGetMeta(key) {
  return openOfflineDB().then(function (db) {
    return new Promise(function (resolve, reject) {
      var req = db.transaction(OFFLINE_META_STORE, "readonly").objectStore(OFFLINE_META_STORE).get(key);
      req.onsuccess = function () {
        resolve(req.result);
      };
      req.onerror = function () {
        reject(req.error);
      };
    });
  });
}

function offlineDBSetMeta(key, value) {
  return openOfflineDB().then(function (db) {
    return new Promise(function (resolve, reject) {
      var tx = db.transaction(OFFLINE_META_STORE, "readwrite");
      tx.objectStore(OFFLINE_META_STORE).put(value, key);
      tx.oncomplete = function () {
        resolve();
      };
      tx.onerror = function () {
        reject(tx.error);
      };
    });
  });
}

// offlineDBGetSong looks up one song's stored page by slug — used by
// static/sw.js to serve a song offline that was never actually visited.
function offlineDBGetSong(slug) {
  return openOfflineDB().then(function (db) {
    return new Promise(function (resolve, reject) {
      var req = db.transaction(OFFLINE_SONGS_STORE, "readonly").objectStore(OFFLINE_SONGS_STORE).get(slug);
      req.onsuccess = function () {
        resolve(req.result);
      };
      req.onerror = function () {
        reject(req.error);
      };
    });
  });
}

// offlineDBGetAllSongs reads every stored song — used to diff against the
// catalog manifest (see static/js/offline-sync.js) and figure out which
// slugs are missing or stale.
function offlineDBGetAllSongs() {
  return openOfflineDB().then(function (db) {
    return new Promise(function (resolve, reject) {
      var req = db.transaction(OFFLINE_SONGS_STORE, "readonly").objectStore(OFFLINE_SONGS_STORE).getAll();
      req.onsuccess = function () {
        resolve(req.result || []);
      };
      req.onerror = function () {
        reject(req.error);
      };
    });
  });
}

function offlineDBPutSong(song) {
  return openOfflineDB().then(function (db) {
    return new Promise(function (resolve, reject) {
      var tx = db.transaction(OFFLINE_SONGS_STORE, "readwrite");
      tx.objectStore(OFFLINE_SONGS_STORE).put(song);
      tx.oncomplete = function () {
        resolve();
      };
      tx.onerror = function () {
        reject(tx.error);
      };
    });
  });
}

function offlineDBDeleteSongs(slugs) {
  if (!slugs.length) {
    return Promise.resolve();
  }
  return openOfflineDB().then(function (db) {
    return new Promise(function (resolve, reject) {
      var tx = db.transaction(OFFLINE_SONGS_STORE, "readwrite");
      var store = tx.objectStore(OFFLINE_SONGS_STORE);
      for (var i = 0; i < slugs.length; i++) {
        store.delete(slugs[i]);
      }
      tx.oncomplete = function () {
        resolve();
      };
      tx.onerror = function () {
        reject(tx.error);
      };
    });
  });
}
