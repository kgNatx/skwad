// Minimal service worker for PWA installability.
// Skwad is a live-data app so we don't cache aggressively —
// just enough to satisfy the PWA install criteria.

const CACHE_NAME = 'skwad-v14';
const PRECACHE = [
  '/',
  '/style.css',
  '/app.js',
  '/jsqr.min.js',
  '/skwad-icon.svg',
  '/icon-192.png',
  '/icon-512.png',
  '/manifest.json',
];

self.addEventListener('install', function (e) {
  e.waitUntil(
    caches.open(CACHE_NAME).then(function (cache) {
      return cache.addAll(PRECACHE);
    })
  );
  self.skipWaiting();
});

self.addEventListener('activate', function (e) {
  e.waitUntil(
    caches.keys().then(function (names) {
      return Promise.all(
        names.filter(function (n) { return n !== CACHE_NAME; })
          .map(function (n) { return caches.delete(n); })
      );
    })
  );
  self.clients.claim();
});

// Network-first strategy: always try the network, fall back to cache.
// This ensures live session data is never stale.
self.addEventListener('fetch', function (e) {
  // Skip non-GET and API requests entirely — let them go straight to network.
  if (e.request.method !== 'GET' || e.request.url.includes('/api/')) {
    return;
  }

  e.respondWith(
    fetch(e.request, { cache: 'no-cache' }).then(function (response) {
      // Update cache with fresh response.
      var clone = response.clone();
      caches.open(CACHE_NAME).then(function (cache) {
        cache.put(e.request, clone);
      });
      return response;
    }).catch(function () {
      return caches.match(e.request);
    })
  );
});
