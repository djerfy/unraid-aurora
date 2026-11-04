#!/usr/bin/env python3
"""Fail when a user-facing key has no French translation.

English is the source-key language: components call t("English sentence") and
the French locale maps that key. Dynamic keys and server-only strings are out
of scope.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "frontend/src"
LOCALES = SRC / "i18n/locales/fr"

KEY_RE = re.compile(r'\bt\(\s*"((?:[^"\\]|\\.)*)"')
LOCALE_RE = re.compile(r'^\s*"((?:[^"\\]|\\.)*)"\s*:', re.MULTILINE)


def keys_in_sources() -> dict[str, set[str]]:
    keys: dict[str, set[str]] = {}
    for path in SRC.rglob("*.tsx"):
        if "i18n" in path.parts:
            continue
        for match in KEY_RE.finditer(path.read_text()):
            keys.setdefault(match.group(1), set()).add(str(path.relative_to(ROOT)))
    return keys


def keys_in_locale() -> set[str]:
    keys: set[str] = set()
    for path in LOCALES.glob("*.ts"):
        if path.name == "index.ts":
            continue
        keys.update(LOCALE_RE.findall(path.read_text()))
    return keys


def main() -> None:
    sources = keys_in_sources()
    translated = keys_in_locale()
    missing = {key: files for key, files in sources.items() if key not in translated}
    if missing:
        print(f"{len(missing)} missing French translation(s):", file=sys.stderr)
        for key in sorted(missing):
            files = ", ".join(sorted(missing[key]))
            print(f"  - {key!r} ({files})", file=sys.stderr)
        raise SystemExit(1)
    print(f"i18n OK: {len(sources)} keys covered by {len(translated)} French entries")


if __name__ == "__main__":
    main()
