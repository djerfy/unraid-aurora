#!/usr/bin/env python3
"""Fail when total Go statement coverage drops below a threshold."""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

THRESHOLD = float(sys.argv[2]) if len(sys.argv) > 2 else 45.0
PROFILE = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("coverage.out")
if not PROFILE.exists():
    raise SystemExit(f"coverage profile not found: {PROFILE}")
output = subprocess.check_output(["go", "tool", "cover", "-func", str(PROFILE)], text=True)
match = re.search(r"total:\s+\(statements\)\s+([0-9.]+)%", output)
if not match:
    raise SystemExit("could not parse coverage output")
coverage = float(match.group(1))
print(f"total coverage: {coverage:.1f}% (threshold {THRESHOLD:.1f}%)")
if coverage < THRESHOLD:
    raise SystemExit(1)
