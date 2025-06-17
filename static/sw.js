const CACHE_NAME = 'hike-tracker-cache-v1';
const CORE_ASSETS = [
  '/index.html',
  '/favicon.ico',
  // Add other critical static assets here if they are externalized later
  // e.g., '/styles/main.css', '/js/app.js'
  // For now, index.html and favicon.ico are the main ones.
  // The waiver.txt could also be a candidate.
  '/waiver.txt'
];

// Install event: Cache core assets
self.addEventListener('install', event => {
  console.log('Service Worker: Installing...');
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then(cache => {
        console.log('Service Worker: Caching core assets...');
        return cache.addAll(CORE_ASSETS);
      })
      .then(() => {
        console.log('Service Worker: Core assets cached successfully.');
        return self.skipWaiting(); // Activate the new service worker immediately
      })
      .catch(error => {
        console.error('Service Worker: Failed to cache core assets:', error);
      })
  );
});

// Activate event: Clean up old caches
self.addEventListener('activate', event => {
  console.log('Service Worker: Activating...');
  event.waitUntil(
    caches.keys().then(cacheNames => {
      return Promise.all(
        cacheNames.map(cacheName => {
          if (cacheName !== CACHE_NAME) {
            console.log('Service Worker: Deleting old cache:', cacheName);
            return caches.delete(cacheName);
          }
        })
      );
    }).then(() => {
      console.log('Service Worker: Activated and old caches cleaned.');
      return self.clients.claim(); // Take control of uncontrolled clients
    })
  );
});

// fetch event: Serve cached assets (Cache-First for core, Network-First for API)
self.addEventListener('fetch', event => {
  const url = new URL(event.request.url);

  // Serve core assets from cache first
  if (CORE_ASSETS.includes(url.pathname)) {
    console.log('Service Worker: Serving core asset (cache-first):', event.request.url);
    event.respondWith(
      caches.match(event.request).then(cachedResponse => {
        if (cachedResponse) {
          return cachedResponse;
        }
        console.warn('Service Worker: Core asset not in cache (should have been pre-cached):', event.request.url);
        // Fallback to network for core assets if somehow missed by install.
        return fetch(event.request).then(response => {
          // Optionally re-cache if it's a successful response, but install should handle this.
          // if (response.ok) {
          //   const responseToCache = response.clone();
          //   caches.open(CACHE_NAME).then(cache => cache.put(event.request, responseToCache));
          // }
          return response;
        });
      })
    );
  }
  // Network-first strategy for API calls
  else if (url.pathname.startsWith('/api/')) {
    console.log('Service Worker: API call (network-first):', event.request.url);
    event.respondWith(
      fetch(event.request)
        .then(response => {
          // If the request is successful (response.ok), cache it and return it
          if (response && response.ok) {
            console.log('Service Worker: API call successful, caching response:', event.request.url);
            const responseToCache = response.clone();
            caches.open(CACHE_NAME).then(cache => {
              cache.put(event.request, responseToCache);
            });
            return response;
          }
          // If response is not ok (e.g. 404, 500), don't cache, just return it.
          console.log(`Service Worker: API call response not OK (status: ${response.status}) or expecting .catch for network failure:`, event.request.url);
          return response;
        })
        .catch(error => {
          // Network request failed (likely offline), try to serve from cache
          console.warn('Service Worker: Network fetch failed for API call, trying cache:', event.request.url, error);
          return caches.match(event.request).then(cachedResponse => {
            if (cachedResponse) {
              console.log('Service Worker: Serving API call from cache:', event.request.url);
              return cachedResponse;
            }
            // If not in cache, and network failed, then it's a genuine failure.
            console.error('Service Worker: API call not in cache and network failed:', event.request.url);
            if (event.request.method === 'GET') {
              return new Response(JSON.stringify({ error: 'Offline and data not available in cache' }), {
                headers: { 'Content-Type': 'application/json' },
                status: 503, // Service Unavailable
                statusText: 'Service Unavailable (Offline)'
              });
            }
            // For non-GET requests, re-throw the error. Client-side queuing should handle mutations.
            throw error;
          });
        })
    );
  }
  // For other requests (e.g. external CDN, or non-API, non-core local assets)
  else {
    console.log('Service Worker: Fetching from network (default/other):', event.request.url);
    event.respondWith(fetch(event.request));
  }
});

// Sync event listener
self.addEventListener('sync', event => {
  if (event.tag === 'hike-data-sync') {
    console.log('Service Worker: Sync event received for hike-data-sync.');
    // Normally, we would process the queue here.
    // For now, this just acknowledges the event.
    // Actual queue processing from localStorage would require client-to-SW communication
    // or moving the queue to IndexedDB.
    event.waitUntil(
      Promise.resolve().then(() => { // Ensure waitUntil is called with a promise
        console.log('Service Worker: Sync event for hike-data-sync processed (placeholder).');
      })
    );
  }
});
