// Shared between static/js/offline-sync.js (runs on the main thread) and
// static/sw.js (runs in the service worker, via importScripts) so both
// agree on the IndexedDB layout without duplicating it. Plain top-level
// function/var declarations rather than an ES module, since a classic
// script works unchanged in both a <script> tag and importScripts().

var OFFLINE_DB_NAME = "tabitha-offline";
var OFFLINE_DB_VERSION = 1;
var OFFLINE_STORE = "snapshot";

function openOfflineDB() {
  return new Promise(function (resolve, reject) {
    var req = indexedDB.open(OFFLINE_DB_NAME, OFFLINE_DB_VERSION);
    req.onupgradeneeded = function () {
      req.result.createObjectStore(OFFLINE_STORE);
    };
    req.onsuccess = function () {
      resolve(req.result);
    };
    req.onerror = function () {
      reject(req.error);
    };
  });
}

// offlineDBGet reads one key ("version" or "data") from the snapshot store.
function offlineDBGet(key) {
  return openOfflineDB().then(function (db) {
    return new Promise(function (resolve, reject) {
      var req = db.transaction(OFFLINE_STORE, "readonly").objectStore(OFFLINE_STORE).get(key);
      req.onsuccess = function () {
        resolve(req.result);
      };
      req.onerror = function () {
        reject(req.error);
      };
    });
  });
}

// offlineDBPutSnapshot stores a freshly downloaded snapshot's version marker
// and raw SQLite file bytes together, in one transaction, so a reader can
// never observe one updated without the other.
function offlineDBPutSnapshot(version, data) {
  return openOfflineDB().then(function (db) {
    return new Promise(function (resolve, reject) {
      var tx = db.transaction(OFFLINE_STORE, "readwrite");
      tx.objectStore(OFFLINE_STORE).put(version, "version");
      tx.objectStore(OFFLINE_STORE).put(data, "data");
      tx.oncomplete = function () {
        resolve();
      };
      tx.onerror = function () {
        reject(tx.error);
      };
    });
  });
}
