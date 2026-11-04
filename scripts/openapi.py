#!/usr/bin/env python3
"""Generate docs/openapi.json from the registered routes and docs/API.md.

The API surface is declared in backend/internal/api/server.go. This script keeps
the OpenAPI document honest: every registered route must have a matching
description, otherwise generation fails. Run it after adding or renaming a
route:

    python3 scripts/openapi.py
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SERVER = ROOT / "backend/internal/api/server.go"
DOC = ROOT / "docs/API.md"
OUT = ROOT / "backend/internal/api/apidocs/openapi.json"
DOCS_OUT = ROOT / "docs/openapi.json"

ROUTE_RE = re.compile(r'HandleFunc\("([A-Z]+)\s+(/api/v1/[^"]+)"')
TABLE_RE = re.compile(r"^\|\s*([A-Z/]+)\s*\|\s*`([^`]+)`\s*\|\s*([^|]+?)\s*\|\s*(.+?)\s*\|$")

TAGS = {
    "auth": ("Authentication", "Sessions and token authentication."),
    "system": ("System", "Host identity, settings, logs and maintenance."),
    "storage": ("Storage", "Array, disks, shares, pools and parity."),
    "docker": ("Docker", "Containers, images, updates and network calls."),
    "vms": ("Virtual machines", "Libvirt inventory, lifecycle, hardware, snapshots and console."),
    "network": ("Network", "Interfaces, configuration and services."),
    "apps": ("Applications", "Community Applications catalog and templates."),
    "plugins": ("Plugins", "Installed plugins and the official plugin manager."),
    "scheduler": ("Scheduler", "Parity, mover and TRIM schedules plus display formats."),
    "mover": ("Mover", "Background cache/array mover."),
    "balance": ("Balance", "Disk evacuation and selective moves."),
    "zfs": ("ZFS", "Pools, scrub and snapshots."),
    "ups": ("UPS", "Battery telemetry and shutdown policy."),
    "gpus": ("GPUs", "NVIDIA and AMD telemetry."),
    "access": ("Access", "Users, API tokens, audit and notifications."),
    "config": ("Configuration", "Settings, backups, history and tools."),
    "trivy": ("Security scanning", "Managed Trivy binary, databases and scans."),
    "unraid": ("Unraid", "Unraid OS updates and the official API service."),
    "automation": ("Automation", "User scripts and their schedules."),
    "events": ("Events", "Server-sent events and metrics."),
}


def route_tag(path: str) -> str:
    if path.startswith("/api/v1/auth") or path == "/api/v1/health":
        return "auth"
    if path.startswith("/api/v1/system/scheduler") or path == "/api/v1/system/display":
        return "scheduler"
    if path.startswith("/api/v1/system/os") or path.startswith("/api/v1/unraid-api"):
        return "unraid"
    if path.startswith("/api/v1/system") or path in ("/api/v1/bootstrap", "/api/v1/settings", "/api/v1/logs", "/api/v1/terminal", "/api/v1/actions"):
        return "system"
    if path.startswith("/api/v1/array") or path.startswith("/api/v1/disks") or path.startswith("/api/v1/shares") or path.startswith("/api/v1/pools") or path.startswith("/api/v1/parity"):
        return "storage"
    if path.startswith("/api/v1/docker"):
        return "docker"
    if path.startswith("/api/v1/vms"):
        return "vms"
    if path.startswith("/api/v1/network"):
        return "network"
    if path.startswith("/api/v1/apps"):
        return "apps"
    if path.startswith("/api/v1/plugins"):
        return "plugins"
    if path.startswith("/api/v1/mover"):
        return "mover"
    if path.startswith("/api/v1/balance"):
        return "balance"
    if path.startswith("/api/v1/zfs"):
        return "zfs"
    if path.startswith("/api/v1/ups"):
        return "ups"
    if path.startswith("/api/v1/gpus"):
        return "gpus"
    if path.startswith("/api/v1/users") or path.startswith("/api/v1/tokens") or path.startswith("/api/v1/audit") or path.startswith("/api/v1/notifications"):
        return "access"
    if path.startswith("/api/v1/config") or path.startswith("/api/v1/tools") or path.startswith("/api/v1/storage-layout") or path.startswith("/api/v1/metrics"):
        return "config"
    if path.startswith("/api/v1/trivy"):
        return "trivy"
    if path.startswith("/api/v1/user-scripts"):
        return "automation"
    if path.startswith("/api/v1/events"):
        return "events"
    return "system"


def humanize(path: str) -> str:
    parts = [p for p in path.replace("/api/v1/", "").split("/") if p]
    words = []
    for part in parts:
        if part.startswith("{"):
            words.append(part.strip("{}"))
            continue
        words.extend(part.replace("-", " ").split())
    return " ".join(words).capitalize()


SUMMARY_OVERRIDES = {
    "GET /api/v1/health": "Liveness and version",
    "GET /api/v1/auth/session": "Current session state",
    "POST /api/v1/auth/login": "Log in with root credentials",
    "POST /api/v1/auth/logout": "Log out the current session",
    "POST /api/v1/auth/revoke-all": "Revoke every session",
    "GET /api/v1/bootstrap": "Load the full application snapshot",
    "GET /api/v1/events": "Stream live metrics",
    "POST /api/v1/actions": "Run an enumerated action",
    "POST /api/v1/docker/containers/bulk": "Bulk container action",
    "GET /api/v1/system/scheduler": "Read scheduler and display settings",
    "POST /api/v1/system/scheduler/preview": "Preview generated scheduler files",
    "PUT /api/v1/system/scheduler": "Apply scheduler settings",
    "GET /api/v1/tokens": "List API tokens",
    "POST /api/v1/tokens": "Create an API token",
    "DELETE /api/v1/tokens/{id}": "Revoke an API token",
    "GET /api/v1/audit": "Read the mutation audit log",
    "GET /api/v1/mover": "Read mover progress",
}


def parse_routes() -> list[tuple[str, str]]:
    routes = []
    for match in ROUTE_RE.finditer(SERVER.read_text()):
        routes.append((match.group(1), match.group(2)))
    return sorted(set(routes))


def parse_descriptions() -> dict[tuple[str, str], tuple[str, str, str]]:
    """Return {(method, path): (access, summary, description)}."""
    result: dict[tuple[str, str], tuple[str, str, str]] = {}
    for line in DOC.read_text().splitlines():
        match = TABLE_RE.match(line.strip())
        if not match:
            continue
        methods, path, access, description = match.groups()
        if not path.startswith("/"):
            continue
        methods = [m.strip() for m in methods.split("/") if m.strip()]
        if not all(m in {"GET", "POST", "PUT", "DELETE", "PATCH"} for m in methods):
            continue
        access = access.strip()
        description = description.strip()
        summary = description.split(".")[0][:120]
        for method in methods:
            result[(method, f"/api/v1{path.split('?')[0]}")] = (access, summary, description)
    # Rows that group several concrete token paths.
    for method in ("GET", "POST"):
        result[(method, "/api/v1/tokens")] = result.get((method, "/api/v1/tokens"), ("session", "API tokens", "List or create API tokens."))
    result[("DELETE", "/api/v1/tokens/{id}")] = ("session", "Revoke an API token", "Revoke a token by id.")
    return result


def security_for(access: str) -> tuple[list[dict], str]:
    access = access.lower()
    if access == "public":
        return [], "public"
    if access == "session":
        return [{"sessionCookie": []}], "session"
    if access.startswith("operator"):
        return [{"sessionCookie": []}, {"bearerAuth": []}], "operator"
    if access.startswith("admin"):
        return [{"sessionCookie": []}, {"bearerAuth": []}], "admin"
    return [{"sessionCookie": []}, {"bearerAuth": []}], "viewer"


def parameters_for(path: str, method: str) -> list[dict]:
    params = []
    for name in re.findall(r"\{([^}]+)\}", path):
        description = {
            "device": "Disk device name, for example `sda`.",
            "name": "Resource name.",
            "id": "Container or VM identifier.",
            "snapshot": "Snapshot name.",
            "action": "Action name.",
        }.get(name, f"`{name}` path parameter.")
        params.append({"name": name, "in": "path", "required": True, "schema": {"type": "string"}, "description": description})
    query = {
        "/api/v1/logs": [
            {"name": "source", "in": "query", "schema": {"type": "string", "enum": ["syslog", "docker", "nginx", "libvirt", "maillog", "parity"]}, "description": "Log source."},
            {"name": "lines", "in": "query", "schema": {"type": "integer", "minimum": 1, "maximum": 1000, "default": 200}, "description": "Number of lines."},
        ],
        "/api/v1/audit": [
            {"name": "limit", "in": "query", "schema": {"type": "integer", "minimum": 1, "maximum": 500, "default": 200}, "description": "Number of entries."},
        ],
        "/api/v1/metrics/history": [
            {"name": "range", "in": "query", "schema": {"type": "string", "enum": ["1h", "24h", "7d", "30d"], "default": "1h"}, "description": "History span."},
        ],
        "/api/v1/docker/containers/{id}/logs": [
            {"name": "lines", "in": "query", "schema": {"type": "integer", "minimum": 1, "maximum": 1000, "default": 200}, "description": "Number of lines."},
        ],
    }
    params.extend(query.get(path, []))
    return params


def ref(name: str) -> dict:
    return {"$ref": f"#/components/schemas/{name}"}


def json_response(description: str, schema: dict | None = None) -> dict:
    return {"description": description, "content": {"application/json": {"schema": schema or {"type": "object"}}}}


def default_responses(method: str, path: str, access: str) -> dict:
    responses = {"200": json_response("Successful response.")}
    if path == "/api/v1/events":
        responses = {"200": {"description": "Server-sent events stream.", "content": {"text/event-stream": {"schema": {"type": "string"}}}}}
    elif path == "/api/v1/terminal" or path.endswith("/console"):
        responses = {"101": {"description": "WebSocket upgrade."}}
    elif path.endswith("/update") or path.endswith("/install") or path.startswith("/api/v1/system/os/update") or path.startswith("/api/v1/plugins/"):
        responses = {"200": {"description": "NDJSON progress stream.", "content": {"application/x-ndjson": {"schema": {"type": "object"}}}}}
    elif method == "DELETE":
        responses = {"200": json_response("Resource removed.", {"type": "object", "properties": {"ok": {"type": "boolean"}}})}
    if access != "public":
        responses["401"] = json_response("Authentication required.", ref("Error"))
        responses["403"] = json_response("Role or read-only mode denied.", ref("Error"))
    return responses


def request_body_for(method: str, path: str) -> dict | None:
    if method in {"GET", "HEAD", "DELETE"}:
        return None
    bodies = {
        "/api/v1/auth/login": ("Login credentials.", {"type": "object", "required": ["username", "password"], "properties": {"username": {"type": "string"}, "password": {"type": "string", "format": "password"}}}),
        "/api/v1/actions": ("Enumerated mutation.", ref("Action")),
        "/api/v1/docker/containers/bulk": ("Bulk container action.", ref("ContainerBulkRequest")),
        "/api/v1/system/scheduler/preview": ("Scheduler settings to preview.", ref("SchedulerSettings")),
        "/api/v1/system/scheduler": ("Scheduler settings plus acknowledgement.", ref("SchedulerApply")),
        "/api/v1/system/display": ("Display formats plus acknowledgement.", {"type": "object", "properties": {"dateFormat": {"type": "string"}, "timeFormat": {"type": "string"}, "acknowledge": {"type": "boolean"}}}),
        "/api/v1/tokens": ("Token name and role.", ref("TokenCreate")),
        "/api/v1/settings": ("Full settings object.", ref("Settings")),
        "/api/v1/network/config": ("Network interface configuration.", {"type": "object", "additionalProperties": True}),
        "/api/v1/vms": ("VM creation options.", {"type": "object", "additionalProperties": True}),
        "/api/v1/shares": ("Share creation options.", {"type": "object", "additionalProperties": True}),
        "/api/v1/storage-layout": ("Visual chassis layout.", {"type": "object", "additionalProperties": True}),
    }
    if path in bodies:
        description, schema = bodies[path]
        return {"required": True, "description": description, "content": {"application/json": {"schema": schema}}}
    if path.endswith("/autostart"):
        return {"required": True, "content": {"application/json": {"schema": {"type": "object", "required": ["enabled"], "properties": {"enabled": {"type": "boolean"}}}}}}
    if path.endswith("/snapshots") and method == "POST":
        return {"required": False, "content": {"application/json": {"schema": {"type": "object", "properties": {"name": {"type": "string"}, "description": {"type": "string"}}}}}}
    return {"required": True, "content": {"application/json": {"schema": {"type": "object", "additionalProperties": True}}}}


def response_schema_for(method: str, path: str) -> dict | None:
    if path == "/api/v1/bootstrap":
        return ref("Snapshot")
    if path == "/api/v1/system/scheduler/preview":
        return ref("SchedulerPreview")
    if path == "/api/v1/system/scheduler":
        return json_response("Applied scheduler state.", {"type": "object", "properties": {"ok": {"type": "boolean"}, "state": {"type": "object", "additionalProperties": True}}}).get("content", {}).get("application/json", {}).get("schema")
    if path == "/api/v1/mover":
        return ref("MoverStatus")
    if path == "/api/v1/audit":
        return {"type": "array", "items": ref("AuditEntry")}
    if path == "/api/v1/tokens" and method == "GET":
        return {"type": "array", "items": ref("APIToken")}
    if path == "/api/v1/tokens" and method == "POST":
        return {"type": "object", "properties": {"token": ref("APIToken"), "secret": {"type": "string"}}}
    if path == "/api/v1/docker/containers":
        return {"type": "array", "items": ref("Container")}
    if path == "/api/v1/vms":
        return {"type": "array", "items": ref("VM")}
    if path == "/api/v1/docker/containers/bulk":
        return ref("ContainerBulkResult")
    if path == "/api/v1/health":
        return ref("Health")
    if path == "/api/v1/auth/session":
        return ref("Session")
    return None


def build_spec() -> dict:
    routes = parse_routes()
    descriptions = parse_descriptions()
    missing = [f"{m} {p}" for m, p in routes if (m, p) not in descriptions]
    if missing:
        print("Missing descriptions in docs/API.md for:", file=sys.stderr)
        for item in missing:
            print("  -", item, file=sys.stderr)
        raise SystemExit(1)

    paths: dict[str, dict] = {}
    for method, path in routes:
        access, summary, description = descriptions[(method, path)]
        summary = SUMMARY_OVERRIDES.get(f"{method} {path}", summary)
        security, role = security_for(access)
        operation = {
            "tags": [TAGS[route_tag(path)][0]],
            "summary": summary or humanize(path),
            "description": description,
            "operationId": re.sub(r"[^A-Za-z0-9]+", "_", f"{method.lower()}_{path.replace('/api/v1/', '')}").strip("_"),
            "security": security,
            "responses": default_responses(method, path, access),
        }
        if role not in {"public", "viewer"}:
            operation["x-aurora-role"] = role
        params = parameters_for(path, method)
        if params:
            operation["parameters"] = params
        body = request_body_for(method, path)
        if body:
            operation["requestBody"] = body
        schema = response_schema_for(method, path)
        if schema and "200" in operation["responses"] and "content" in operation["responses"]["200"]:
            operation["responses"]["200"]["content"]["application/json"]["schema"] = schema
        paths.setdefault(path, {})[method.lower()] = operation

    tags = [{"name": name, "description": desc} for name, desc in TAGS.values()]
    return {
        "openapi": "3.1.0",
        "info": {
            "title": "Aurora API",
            "version": "1.0.0",
            "description": (
                "Aurora is an independent control plane for Unraid. This reference is generated from "
                "the registered routes and docs/API.md. Redoc is read-only: it documents the API but "
                "does not execute requests. Authenticate with the `aurora_session` cookie or a Bearer token."
            ),
            "license": {"name": "AGPL-3.0-only"},
        },
        "servers": [{"url": "/", "description": "Current Aurora host"}],
        "tags": tags,
        "security": [{"sessionCookie": []}, {"bearerAuth": []}],
        "paths": paths,
        "components": {
            "securitySchemes": {
                "sessionCookie": {"type": "apiKey", "in": "cookie", "name": "aurora_session"},
                "bearerAuth": {"type": "http", "scheme": "bearer", "bearerFormat": "opaque"},
            },
            "schemas": {
                "Error": {
                    "type": "object",
                    "properties": {"error": {"type": "object", "properties": {"code": {"type": "string"}, "message": {"type": "string"}}}},
                },
                "Health": {
                    "type": "object",
                    "properties": {"status": {"type": "string"}, "version": {"type": "string"}, "provider": {"type": "string"}, "uptime": {"type": "integer"}, "time": {"type": "string", "format": "date-time"}},
                },
                "Session": {"type": "object", "properties": {"authenticated": {"type": "boolean"}, "user": {"type": "string"}}},
                "Action": {
                    "type": "object",
                    "required": ["kind"],
                    "properties": {"kind": {"type": "string"}, "target": {"type": "string"}, "value": {"type": "string"}},
                },
                "Container": {
                    "type": "object",
                    "additionalProperties": True,
                    "properties": {
                        "id": {"type": "string"}, "name": {"type": "string"}, "image": {"type": "string"},
                        "state": {"type": "string"}, "status": {"type": "string"}, "health": {"type": "string"},
                        "autostart": {"type": "boolean"}, "cpu": {"type": "number"}, "memory": {"type": "integer"},
                        "updateAvailable": {"type": "boolean"}, "security": {"type": "object", "additionalProperties": True},
                    },
                },
                "VM": {
                    "type": "object",
                    "additionalProperties": True,
                    "properties": {"id": {"type": "string"}, "name": {"type": "string"}, "state": {"type": "string"}, "vcpus": {"type": "integer"}, "memory": {"type": "integer"}, "autostart": {"type": "boolean"}},
                },
                "Snapshot": {
                    "type": "object",
                    "additionalProperties": True,
                    "properties": {
                        "system": {"type": "object", "additionalProperties": True},
                        "array": {"type": "object", "additionalProperties": True},
                        "disks": {"type": "array", "items": {"type": "object", "additionalProperties": True}},
                        "shares": {"type": "array", "items": {"type": "object", "additionalProperties": True}},
                        "pools": {"type": "array", "items": {"type": "object", "additionalProperties": True}},
                        "containers": {"type": "array", "items": ref("Container")},
                        "vms": {"type": "array", "items": ref("VM")},
                        "network": {"type": "object", "additionalProperties": True},
                        "metrics": {"type": "object", "additionalProperties": True},
                        "history": {"type": "array", "items": {"type": "object", "additionalProperties": True}},
                        "serverTime": {"type": "string", "format": "date-time"},
                    },
                },
                "MoverStatus": {
                    "type": "object",
                    "additionalProperties": True,
                    "properties": {
                        "running": {"type": "boolean"}, "progressEnabled": {"type": "boolean"},
                        "progressSource": {"type": "string"}, "total": {"type": "integer"}, "remain": {"type": "integer"},
                        "progress": {"type": "number"}, "throughput": {"type": "number"}, "eta": {"type": "integer"},
                        "file": {"type": "string"}, "action": {"type": "string"}, "phase": {"type": "string"},
                        "files": {"type": "array", "items": {"type": "string"}},
                        "history": {"type": "array", "items": {"type": "object", "additionalProperties": True}},
                    },
                },
                "SchedulerJob": {
                    "type": "object",
                    "properties": {
                        "mode": {"type": "string", "enum": ["disabled", "hourly", "daily", "weekly", "monthly", "yearly"]},
                        "hour": {"type": "integer", "minimum": 0, "maximum": 23},
                        "minute": {"type": "integer", "minimum": 0, "maximum": 59},
                        "day": {"type": "integer", "minimum": 0, "maximum": 6},
                        "dayOfMonth": {"type": "integer", "minimum": 1, "maximum": 31},
                        "month": {"type": "integer", "minimum": 1, "maximum": 12},
                        "intervalHours": {"type": "integer", "minimum": 1},
                    },
                },
                "SchedulerSettings": {
                    "type": "object",
                    "properties": {
                        "parity": {"allOf": [ref("SchedulerJob"), {"type": "object", "properties": {"writeCorrections": {"type": "boolean"}}}]},
                        "mover": {"allOf": [ref("SchedulerJob"), {"type": "object", "properties": {"logging": {"type": "boolean"}, "progress": {"type": "boolean"}}}]},
                        "trim": ref("SchedulerJob"),
                    },
                },
                "SchedulerApply": {"allOf": [ref("SchedulerSettings"), {"type": "object", "properties": {"acknowledge": {"type": "boolean"}}}]},
                "SchedulerPreview": {
                    "type": "object",
                    "properties": {
                        "files": {"type": "array", "items": {"type": "object", "properties": {"path": {"type": "string"}, "content": {"type": "string"}, "diff": {"type": "string"}, "existed": {"type": "boolean"}, "delete": {"type": "boolean"}}}},
                        "warnings": {"type": "array", "items": {"type": "string"}},
                    },
                },
                "ContainerBulkRequest": {
                    "type": "object",
                    "required": ["action", "targets"],
                    "properties": {"action": {"type": "string", "enum": ["start", "stop", "restart"]}, "targets": {"type": "array", "maxItems": 100, "items": {"type": "string"}}},
                },
                "ContainerBulkResult": {
                    "type": "object",
                    "properties": {"ok": {"type": "boolean"}, "succeeded": {"type": "array", "items": {"type": "string"}}, "failed": {"type": "array", "items": {"type": "object", "properties": {"target": {"type": "string"}, "error": {"type": "string"}}}}},
                },
                "AuditEntry": {
                    "type": "object",
                    "properties": {"time": {"type": "string", "format": "date-time"}, "user": {"type": "string"}, "method": {"type": "string"}, "path": {"type": "string"}, "status": {"type": "integer"}, "remote": {"type": "string"}},
                },
                "APIToken": {
                    "type": "object",
                    "properties": {"id": {"type": "string"}, "name": {"type": "string"}, "role": {"type": "string", "enum": ["viewer", "operator", "admin"]}, "readOnly": {"type": "boolean"}, "created": {"type": "string", "format": "date-time"}, "lastUsed": {"type": "string", "format": "date-time"}},
                },
                "TokenCreate": {
                    "type": "object",
                    "required": ["name", "role"],
                    "properties": {"name": {"type": "string"}, "role": {"type": "string", "enum": ["viewer", "operator", "admin"]}},
                },
                "Settings": {"type": "object", "additionalProperties": True},
            },
        },
    }


def main() -> None:
    spec = build_spec()
    document = json.dumps(spec, indent=2) + "\n"
    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(document)
    DOCS_OUT.write_text(document)
    print(f"wrote {OUT.relative_to(ROOT)} and {DOCS_OUT.relative_to(ROOT)} ({len(spec['paths'])} paths)")


if __name__ == "__main__":
    main()
