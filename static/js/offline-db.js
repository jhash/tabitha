// Shared between static/js/offline-sync.js (runs on the main thread) and
// static/sw.js (runs in the service worker, via importScripts) so both
// agree on the IndexedDB layout without duplicating it. Plain top-level
// function/var declarations rather than an ES module, since a classic
// script works unchanged in both a <script> tag and importScripts().
//
// Two object stores:
//   - OFFLINE_META_STORE: a plain key/value store, currently just holding
//     the snapshot "version" string from GET /offline/meta.
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

function offlineDBGetVersion() {
  return openOfflineDB().then(function (db) {
    return new Promise(function (resolve, reject) {
      var req = db.transaction(OFFLINE_META_STORE, "readonly").objectStore(OFFLINE_META_STORE).get("version");
      req.onsuccess = function () {
        resolve(req.result);
      };
      req.onerror = function () {
        reject(req.error);
      };
    });
  });
}

// offlineDBGetSong looks up one song's stored page by slug.
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

// offlineDBReplaceSnapshot stores a freshly downloaded snapshot: the new
// version marker and every song row, replacing whatever was there before —
// all in one transaction, so a reader never observes a half-written
// snapshot or a version marker that doesn't match the rows it names.
function offlineDBReplaceSnapshot(version, songs) {
  return openOfflineDB().then(function (db) {
    return new Promise(function (resolve, reject) {
      var tx = db.transaction([OFFLINE_META_STORE, OFFLINE_SONGS_STORE], "readwrite");
      var metaStore = tx.objectStore(OFFLINE_META_STORE);
      var songsStore = tx.objectStore(OFFLINE_SONGS_STORE);
      metaStore.put(version, "version");
      songsStore.clear();
      for (var i = 0; i < songs.length; i++) {
        songsStore.put(songs[i]);
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
