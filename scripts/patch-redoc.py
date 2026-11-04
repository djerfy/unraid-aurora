#!/usr/bin/env python3
"""Rewrite Redoc's CDN attribution URLs to local assets.

Redoc always renders an "API docs by Redocly" logo. Aurora keeps
`img-src 'self'` instead of allowing the CDN, so the vendored bundle points at
local copies of the Aurora mark. Run this after replacing the bundle.
"""

from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BUNDLE = ROOT / "frontend/public/redoc/redoc.standalone.js"
MARK = ROOT / "frontend/public/aurora-mark.svg"

REPLACEMENTS = {
    "https://cdn.redoc.ly/redoc/logo-mini-full.svg": "/redoc/logo-mini-full.svg",
    "https://cdn.redoc.ly/redoc/logo-mini.svg": "/redoc/logo-mini.svg",
    # FlexSearch compiles a worker with Function(), which violates Aurora's
    # script-src 'self' CSP. Force the main-thread index instead.
    "function ZDe(){if(typeof window>`u`||typeof Worker>`u`)return!1;let e=globalThis;try{return e._factory??=Function(XDe),!0}catch{return!1}}": "function ZDe(){return!1}",
}


def main() -> None:
    text = BUNDLE.read_text()
    for old, new in REPLACEMENTS.items():
        if old not in text:
            raise SystemExit(f"missing Redoc URL: {old}")
        text = text.replace(old, new)
    BUNDLE.write_text(text)
    mark = MARK.read_text()
    for name in ("logo-mini-full.svg", "logo-mini.svg"):
        (BUNDLE.parent / name).write_text(mark)
    print("redoc bundle patched")


if __name__ == "__main__":
    main()
