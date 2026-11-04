# Redoc

- Upstream: https://github.com/Redocly/redoc
- Package: `redoc@3.0.1`
- File: `bundles/redoc.standalone.js`
- License: MIT (see `LICENSE`)

The bundle is vendored so Aurora never loads a script from a CDN and keeps its
`script-src 'self'` Content-Security-Policy. The Redocly attribution URLs are
rewritten to local copies of the Aurora mark, and FlexSearch's `Function()`
worker is disabled because it would require `unsafe-eval`. After replacing the
bundle with a newer `redoc` release, run `python3 scripts/patch-redoc.py`.

The reference is read-only: `init.js` hides the try-it panel and only renders
the OpenAPI document from `/api/openapi.json`.
