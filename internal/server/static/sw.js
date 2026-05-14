// PWA service worker. Cache the shell (HTML/CSS/JS); never cache panel data.
// Panel responses live under /panels/* and /api/* and must always hit the
// network so redaction state and freshness are correct.
const SHELL_CACHE = "shell-v1";
const SHELL_ASSETS = [
  "/",
  "/static/app.css",
  "/static/htmx.min.js",
  "/static/manifest.webmanifest",
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(SHELL_CACHE).then((cache) => cache.addAll(SHELL_ASSETS)),
  );
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== SHELL_CACHE).map((k) => caches.delete(k))),
    ),
  );
  self.clients.claim();
});

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  // Never cache panel data or API calls. Always network.
  if (url.pathname.startsWith("/panels/") || url.pathname.startsWith("/api/")) {
    return;
  }
  // Cache-first for shell assets.
  if (SHELL_ASSETS.some((a) => url.pathname === a || url.pathname.startsWith(a + "/"))) {
    event.respondWith(
      caches.match(event.request).then((hit) => hit || fetch(event.request)),
    );
  }
});
