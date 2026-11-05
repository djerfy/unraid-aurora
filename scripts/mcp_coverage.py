#!/usr/bin/env python3
"""Keep the MCP tool catalog aligned with the HTTP API surface.

Every route registered in backend/internal/api/server.go must appear in
docs/mcp-coverage.json with either the MCP tools that expose it or a reason why
it is intentionally excluded. The script also checks that every MCP tool is
referenced by at least one route.

Run after adding, renaming or removing an API route or an MCP tool:

    python3 scripts/mcp_coverage.py
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SERVER = ROOT / "backend/internal/api/server.go"
MCP = ROOT / "backend/internal/mcpserver/server.go"
MANIFEST = ROOT / "docs/mcp-coverage.json"

ROUTE_RE = re.compile(r'HandleFunc\("([A-Z]+)\s+(/api/v1/[^"]+)"')
TOOL_RE = re.compile(r'mcp\.Tool\{Name:\s*"([a-z0-9_]+)"')
ACTION_RE = re.compile(r'\{"([a-z0-9_]+)",\s*"[^"]*",\s*provider\.Action')


def fail(message: str) -> None:
    print(f"error: {message}", file=sys.stderr)


def main() -> int:
    routes = {(method, path) for method, path in ROUTE_RE.findall(SERVER.read_text())}
    tools = set(TOOL_RE.findall(MCP.read_text())) | set(ACTION_RE.findall(MCP.read_text()))
    if not routes:
        fail("no API routes found")
        return 1
    if not tools:
        fail("no MCP tools found")
        return 1

    try:
        manifest = json.loads(MANIFEST.read_text())
    except FileNotFoundError:
        fail(f"{MANIFEST.relative_to(ROOT)} is missing")
        return 1
    except json.JSONDecodeError as error:
        fail(f"{MANIFEST.relative_to(ROOT)} is not valid JSON: {error}")
        return 1

    if manifest.get("version") != 1 or not isinstance(manifest.get("routes"), dict):
        fail(f"{MANIFEST.relative_to(ROOT)} must contain version 1 and a routes object")
        return 1

    listed = manifest["routes"]
    errors: list[str] = []

    missing = sorted(f"{method} {path}" for method, path in routes if f"{method} {path}" not in listed)
    stale = sorted(key for key in listed if key not in {f"{method} {path}" for method, path in routes})
    for key in missing:
        errors.append(f"API route not covered: {key}. Add it to docs/mcp-coverage.json with tools or an exclusion reason.")
    for key in stale:
        errors.append(f"stale manifest entry: {key}. The route no longer exists; remove it.")

    referenced: set[str] = set()
    for key, entry in listed.items():
        if not isinstance(entry, dict):
            errors.append(f"{key}: entry must be an object")
            continue
        has_tools = "tools" in entry
        has_excluded = "excluded" in entry
        if has_tools == has_excluded:
            errors.append(f"{key}: use exactly one of 'tools' or 'excluded'")
            continue
        if has_excluded:
            if not isinstance(entry["excluded"], str) or not entry["excluded"].strip():
                errors.append(f"{key}: 'excluded' must explain why the route is not exposed")
            continue
        route_tools = entry["tools"]
        if not isinstance(route_tools, list) or not route_tools or not all(isinstance(item, str) and item for item in route_tools):
            errors.append(f"{key}: 'tools' must be a non-empty list of tool names")
            continue
        for tool in route_tools:
            referenced.add(tool)
            if tool not in tools:
                errors.append(f"{key}: unknown MCP tool {tool!r}")

    orphaned = sorted(tools - referenced)
    for tool in orphaned:
        errors.append(f"MCP tool not mapped to an API route: {tool}. Map it in docs/mcp-coverage.json.")

    if errors:
        for message in errors:
            fail(message)
        return 1

    print(f"MCP coverage OK: {len(routes)} API routes, {len(tools)} tools, {sum(1 for e in listed.values() if 'tools' in e)} exposed routes")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
