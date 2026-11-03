// Aurora is a single-page app. The worker only satisfies installability;
// network responses stay live so host telemetry is never served stale.
self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => event.waitUntil(self.clients.claim()));
self.addEventListener("fetch", () => {});
