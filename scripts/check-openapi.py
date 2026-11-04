#!/usr/bin/env python3
"""Lightweight OpenAPI 3.1 consistency checks for CI.

The generator already rejects missing route descriptions. This script catches
the remaining structural mistakes without adding a Node or Python dependency.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SPEC = ROOT / "docs/openapi.json"


def fail(message: str) -> None:
    print(f"openapi: {message}", file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    spec = json.loads(SPEC.read_text())
    if not str(spec.get("openapi", "")).startswith("3."):
        fail("openapi must be 3.x")
    tags = {tag["name"] for tag in spec.get("tags", [])}
    schemes = set(spec.get("components", {}).get("securitySchemes", {}))
    schemas = set(spec.get("components", {}).get("schemas", {}))
    seen_operations: set[str] = set()
    refs: set[str] = set()

    def walk(value: object) -> None:
        if isinstance(value, dict):
            if "$ref" in value:
                refs.add(str(value["$ref"]))
            for item in value.values():
                walk(item)
        elif isinstance(value, list):
            for item in value:
                walk(item)

    for path, methods in spec.get("paths", {}).items():
        for method, operation in methods.items():
            if method not in {"get", "post", "put", "delete", "patch", "head", "options"}:
                continue
            where = f"{method.upper()} {path}"
            if not operation.get("summary"):
                fail(f"{where} is missing a summary")
            operation_id = operation.get("operationId")
            if not operation_id or operation_id in seen_operations:
                fail(f"{where} has a missing or duplicate operationId")
            seen_operations.add(operation_id)
            if not operation.get("tags") or not set(operation["tags"]).issubset(tags):
                fail(f"{where} references an unknown tag")
            if not operation.get("responses"):
                fail(f"{where} has no responses")
            declared = {parameter["name"] for parameter in operation.get("parameters", []) if parameter.get("in") == "path"}
            expected = {part.split("}", 1)[0] for part in path.split("{")[1:]}
            if declared != expected:
                fail(f"{where} path parameters are not declared")

    walk(spec)
    for reference in refs:
        if reference.startswith("#/components/schemas/") and reference.rsplit("/", 1)[-1] not in schemas:
            fail(f"unresolved schema reference {reference}")
    for scheme in schemes:
        if scheme not in {"sessionCookie", "bearerAuth"}:
            fail(f"unexpected security scheme {scheme}")
    print(f"openapi OK: {len(spec['paths'])} paths, {len(schemas)} schemas, {len(seen_operations)} operations")


if __name__ == "__main__":
    main()
