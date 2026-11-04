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
            {"name": "method", "in": "query", "schema": {"type": "string", "enum": ["GET", "POST", "PUT", "DELETE"]}, "description": "Filter by HTTP method."},
            {"name": "statusClass", "in": "query", "schema": {"type": "integer", "enum": [2, 4, 5]}, "description": "Filter by status class."},
            {"name": "user", "in": "query", "schema": {"type": "string"}, "description": "Filter by user or token name."},
        ],
        "/api/v1/metrics/history": [
            {"name": "range", "in": "query", "schema": {"type": "string", "enum": ["1h", "24h", "7d", "30d"], "default": "1h"}, "description": "History span."},
        ],
        "/api/v1/docker/containers/{id}/logs": [
            {"name": "lines", "in": "query", "schema": {"type": "integer", "minimum": 1, "maximum": 1000, "default": 200}, "description": "Number of lines."},
        ],
    }
    params.extend(query.get(path, []))
    if path == "/api/v1/bootstrap":
        params.append({"name": "light", "in": "query", "schema": {"type": "string", "enum": ["1"]}, "description": "Skip plugin and notification scans."})
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
    if path.endswith("/usb/attach") or path.endswith("/usb/detach"):
        return {"required": True, "content": {"application/json": {"schema": {"type": "object", "required": ["vendor", "product"], "properties": {"vendor": {"type": "string", "pattern": "^[0-9a-fA-F]{4}$"}, "product": {"type": "string", "pattern": "^[0-9a-fA-F]{4}$"}}}}}}
    if path.endswith("/autostart"):
        return {"required": True, "content": {"application/json": {"schema": {"type": "object", "required": ["enabled"], "properties": {"enabled": {"type": "boolean"}}}}}}
    if path.endswith("/snapshots") and method == "POST":
        return {"required": False, "content": {"application/json": {"schema": {"type": "object", "properties": {"name": {"type": "string"}, "description": {"type": "string"}}}}}}
    return {"required": True, "content": {"application/json": {"schema": {"type": "object", "additionalProperties": True}}}}


def array_of(name: str) -> dict:
    return {"type": "array", "items": ref(name)}


RESPONSE_SCHEMAS = {
    ("GET", "/api/v1/bootstrap"): ref("Snapshot"),
    ("GET", "/api/v1/system"): ref("System"),
    ("GET", "/api/v1/array"): ref("Array"),
    ("GET", "/api/v1/disks"): array_of("Disk"),
    ("GET", "/api/v1/shares"): array_of("Share"),
    ("GET", "/api/v1/shares/{name}/config"): ref("ShareConfig"),
    ("GET", "/api/v1/pools"): array_of("Pool"),
    ("GET", "/api/v1/parity/history"): array_of("ParityCheck"),
    ("GET", "/api/v1/docker/containers"): array_of("Container"),
    ("GET", "/api/v1/docker/containers/{id}/config"): ref("ContainerConfig"),
    ("GET", "/api/v1/docker/security"): ref("SecurityOverview"),
    ("GET", "/api/v1/docker/containers/{id}/security"): ref("SecurityDetail"),
    ("GET", "/api/v1/vms"): array_of("VM"),
    ("GET", "/api/v1/vms/host-devices"): array_of("VMHostDevice"),
    ("GET", "/api/v1/vms/{id}/advanced"): ref("VMAdvanced"),
    ("GET", "/api/v1/vms/{id}/snapshots"): array_of("VMSnapshot"),
    ("GET", "/api/v1/network"): ref("Network"),
    ("GET", "/api/v1/network/config"): ref("NetworkConfig"),
    ("GET", "/api/v1/plugins"): array_of("Plugin"),
    ("GET", "/api/v1/notifications"): array_of("Notification"),
    ("GET", "/api/v1/users"): array_of("User"),
    ("GET", "/api/v1/gpus"): array_of("GPU"),
    ("GET", "/api/v1/ups"): ref("UPS"),
    ("GET", "/api/v1/ups/settings"): ref("UPSSettings"),
    ("GET", "/api/v1/ups/nut"): ref("NUTSettings"),
    ("GET", "/api/v1/zfs/pools"): array_of("ZFSPool"),
    ("GET", "/api/v1/zfs/snapshots"): array_of("ZFSSnapshot"),
    ("GET", "/api/v1/config-history"): array_of("ConfigHistory"),
    ("GET", "/api/v1/config-history/{id}"): ref("ConfigHistory"),
    ("GET", "/api/v1/storage-layout"): ref("StorageLayout"),
    ("GET", "/api/v1/system/os"): ref("OSStatus"),
    ("GET", "/api/v1/unraid-api"): ref("UnraidAPIStatus"),
    ("GET", "/api/v1/trivy"): ref("TrivyStatus"),
    ("GET", "/api/v1/trivy/versions"): array_of("TrivyRelease"),
    ("GET", "/api/v1/apps"): ref("AppCatalog"),
    ("GET", "/api/v1/user-scripts"): array_of("UserScript"),
    ("GET", "/api/v1/user-scripts/{id}"): ref("UserScript"),
    ("GET", "/api/v1/mover"): ref("MoverStatus"),
    ("GET", "/api/v1/balance/status"): ref("BalanceJob"),
    ("GET", "/api/v1/balance/history"): array_of("BalanceJob"),
    ("GET", "/api/v1/audit"): array_of("AuditEntry"),
    ("GET", "/api/v1/tasks"): array_of("Task"),
    ("GET", "/api/v1/tokens"): array_of("APIToken"),
    ("POST", "/api/v1/tokens"): {"type": "object", "properties": {"token": ref("APIToken"), "secret": {"type": "string"}}},
    ("GET", "/api/v1/system/scheduler"): ref("SchedulerState"),
    ("POST", "/api/v1/system/scheduler/preview"): ref("SchedulerPreview"),
    ("GET", "/api/v1/health"): ref("Health"),
    ("GET", "/api/v1/auth/session"): ref("Session"),
    ("GET", "/api/v1/auth/sessions"): array_of("AuthSession"),
    ("GET", "/api/v1/settings"): ref("Settings"),
}


def response_schema_for(method: str, path: str) -> dict | None:
    return RESPONSE_SCHEMAS.get((method, path))


RESPONSE_EXAMPLES = {
    ("GET", "/api/v1/health"): {"status": "ok", "version": "0.1.0", "provider": "live", "uptime": 3600, "time": "2026-01-01T12:00:00Z"},
    ("GET", "/api/v1/auth/session"): {"authenticated": True, "user": "root"},
    ("POST", "/api/v1/auth/login"): {"authenticated": True, "user": "root"},
    ("POST", "/api/v1/docker/containers/bulk"): {"ok": True, "succeeded": ["plex", "sonarr"], "failed": []},
    ("POST", "/api/v1/system/scheduler/preview"): {
        "files": [{"path": "/boot/config/plugins/dynamix/mover.cron", "content": "30 3 * * * /usr/local/sbin/mover start |& logger -t move\n", "diff": "--- /dev/null\n+++ mover.cron\n@@ -0,0 +1 @@\n+30 3 * * * /usr/local/sbin/mover start |& logger -t move\n", "existed": False, "delete": False}],
        "warnings": [],
    },
    ("GET", "/api/v1/audit"): [{"time": "2026-01-01T12:00:00Z", "user": "root", "method": "POST", "path": "/api/v1/actions", "status": 200, "remote": "192.168.1.10"}],
    ("GET", "/api/v1/mover"): {"running": True, "progressEnabled": True, "progressSource": "mover.ini", "total": 1000000000, "remain": 400000000, "progress": 60, "throughput": 50000000, "eta": 8, "phase": "moving", "files": [], "history": []},
}


def response_example_for(method: str, path: str) -> dict | list | None:
    return RESPONSE_EXAMPLES.get((method, path))


REQUEST_EXAMPLES = {
    ("POST", "/api/v1/auth/login"): {"username": "root", "password": "secret"},
    ("POST", "/api/v1/docker/containers/bulk"): {"action": "restart", "targets": ["plex", "sonarr"]},
    ("POST", "/api/v1/system/scheduler/preview"): {"parity": {"mode": "disabled"}, "mover": {"mode": "daily", "hour": 3, "minute": 30}, "trim": {"mode": "disabled"}},
    ("PUT", "/api/v1/system/scheduler"): {"parity": {"mode": "disabled"}, "mover": {"mode": "daily", "hour": 3, "minute": 30}, "trim": {"mode": "disabled"}, "acknowledge": True},
    ("POST", "/api/v1/tokens"): {"name": "home-assistant", "role": "viewer"},
}


def request_example_for(method: str, path: str) -> dict | None:
    return REQUEST_EXAMPLES.get((method, path))

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
            example = request_example_for(method, path)
            if example:
                operation["requestBody"]["content"]["application/json"]["example"] = example
        schema = response_schema_for(method, path)
        if schema and "200" in operation["responses"] and "content" in operation["responses"]["200"]:
            operation["responses"]["200"]["content"]["application/json"]["schema"] = schema
        example = response_example_for(method, path)
        if example is not None and "200" in operation["responses"] and "content" in operation["responses"]["200"]:
            operation["responses"]["200"]["content"]["application/json"]["example"] = example
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
                "AuthSession": {"type": "object", "properties": {"nonce": {"type": "string"}, "user": {"type": "string"}, "ip": {"type": "string"}, "created": {"type": "string", "format": "date-time"}, "lastSeen": {"type": "string", "format": "date-time"}, "expires": {"type": "string", "format": "date-time"}, "current": {"type": "boolean"}}},
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
                "Task": {"type": "object", "properties": {"id": {"type": "string"}, "name": {"type": "string"}, "category": {"type": "string"}, "status": {"type": "string", "enum": ["running", "queued", "completed", "failed"]}, "pid": {"type": "integer"}, "description": {"type": "string"}, "detail": {"type": "string"}, "path": {"type": "string"}, "command": {"type": "string"}, "trigger": {"type": "string"}, "duration": {"type": "integer"}, "progress": {"type": "number"}, "startedAt": {"type": ["string", "null"], "format": "date-time"}, "updatedAt": {"type": ["string", "null"], "format": "date-time"}, "finishedAt": {"type": ["string", "null"], "format": "date-time"}, "error": {"type": "string"}}},
                "AuditEntry": {
                    "type": "object",
                    "properties": {"time": {"type": "string", "format": "date-time"}, "user": {"type": "string"}, "method": {"type": "string"}, "path": {"type": "string"}, "status": {"type": "integer"}, "remote": {"type": "string"}},
                },
                "APIToken": {
                    "type": "object",
                    "properties": {"id": {"type": "string"}, "name": {"type": "string"}, "role": {"type": "string", "enum": ["viewer", "operator", "admin"]}, "readOnly": {"type": "boolean"}, "created": {"type": "string", "format": "date-time"}, "lastUsed": {"type": "string", "format": "date-time"}, "expiresAt": {"type": ["string", "null"], "format": "date-time"}, "scopes": {"type": "array", "items": {"type": "string", "enum": ["docker", "vms", "storage", "network", "system", "ups", "apps", "automation", "access"]}}},
                },
                "TokenCreate": {
                    "type": "object",
                    "required": ["name", "role"],
                    "properties": {"name": {"type": "string"}, "role": {"type": "string", "enum": ["viewer", "operator", "admin"]}, "expiresInDays": {"type": "integer", "minimum": 0, "maximum": 3650, "default": 90, "description": "0 means never expires."}, "scopes": {"type": "array", "items": {"type": "string"}}}},
                "System": {
                    "type": "object",
                    "additionalProperties": True,
                    "properties": {
                        "hostname": {"type": "string"}, "name": {"type": "string"}, "version": {"type": "string"},
                        "kernel": {"type": "string"}, "board": {"type": "string"}, "uptime": {"type": "integer"},
                        "cpu": {"type": "object", "additionalProperties": True}, "memory": {"type": "object", "additionalProperties": True},
                        "load": {"type": "array", "items": {"type": "number"}}, "health": {"type": "object", "additionalProperties": True},
                        "temperatures": {"type": "array", "items": {"type": "object", "additionalProperties": True}},
                    },
                },
                "Array": {
                    "type": "object",
                    "additionalProperties": True,
                    "properties": {
                        "state": {"type": "string"}, "status": {"type": "string"}, "total": {"type": "integer"},
                        "used": {"type": "integer"}, "free": {"type": "integer"}, "usage": {"type": "number"},
                        "parity": {"type": "object", "additionalProperties": True},
                        "rebuild": {"type": "object", "additionalProperties": True},
                    },
                },
                "Disk": {
                    "type": "object",
                    "additionalProperties": True,
                    "properties": {
                        "id": {"type": "string"}, "name": {"type": "string"}, "device": {"type": "string"},
                        "type": {"type": "string"}, "size": {"type": "integer"}, "used": {"type": "integer"},
                        "temp": {"type": "integer"}, "status": {"type": "string"}, "smart": {"type": "string"},
                        "rotational": {"type": "boolean"}, "warnings": {"type": "array", "items": {"type": "string"}},
                    },
                },
                "Share": {
                    "type": "object",
                    "additionalProperties": True,
                    "properties": {
                        "name": {"type": "string"}, "comment": {"type": "string"}, "size": {"type": "integer"},
                        "free": {"type": "integer"}, "used": {"type": "integer"}, "usage": {"type": "number"},
                        "allocator": {"type": "string"}, "splitLevel": {"type": "string"},
                        "cacheMode": {"type": "string"}, "pool": {"type": "string"}, "smb": {"type": "boolean"}, "nfs": {"type": "boolean"}, "usageUpdated": {"type": ["string", "null"], "format": "date-time"},
                    },
                },
                "ShareConfig": {"type": "object", "additionalProperties": True},
                "Pool": {"type": "object", "additionalProperties": True, "properties": {"name": {"type": "string"}, "fsType": {"type": "string"}, "size": {"type": "integer"}, "used": {"type": "integer"}, "devices": {"type": "array", "items": {"type": "string"}}}},
                "ParityCheck": {"type": "object", "additionalProperties": True},
                "ContainerConfig": {"type": "object", "additionalProperties": True},
                "SecurityOverview": {"type": "object", "additionalProperties": True},
                "SecurityDetail": {"type": "object", "additionalProperties": True},
                "VMHostDevice": {"type": "object", "additionalProperties": True, "properties": {"type": {"type": "string"}, "address": {"type": "string"}, "vendor": {"type": "string"}, "product": {"type": "string"}, "name": {"type": "string"}, "driver": {"type": "string"}, "iommuGroup": {"type": "string"}}},
                "VMAdvanced": {"type": "object", "additionalProperties": True, "properties": {"vcpus": {"type": "integer"}, "memoryMB": {"type": "integer"}, "cpuSet": {"type": "string"}, "networks": {"type": "array", "items": {"type": "object", "additionalProperties": True}}, "soundModel": {"type": "string"}, "hostDevices": {"type": "array", "items": {"type": "object", "additionalProperties": True}}, "sharedFolders": {"type": "array", "items": {"type": "object", "properties": {"source": {"type": "string"}, "target": {"type": "string"}, "readOnly": {"type": "boolean"}}}}}},
                "VMSnapshot": {"type": "object", "additionalProperties": True, "properties": {"name": {"type": "string"}, "description": {"type": "string"}, "created": {"type": "string", "format": "date-time"}, "state": {"type": "string"}}},
                "Network": {"type": "object", "additionalProperties": True, "properties": {"hostname": {"type": "string"}, "interfaces": {"type": "array", "items": {"type": "object", "additionalProperties": True}}, "wireGuard": {"type": "object", "additionalProperties": True}, "services": {"type": "array", "items": {"type": "object", "additionalProperties": True}}}},
                "NetworkConfig": {"type": "object", "additionalProperties": True},
                "Plugin": {"type": "object", "additionalProperties": True, "properties": {"name": {"type": "string"}, "version": {"type": "string"}, "latest": {"type": "string"}, "updateAvailable": {"type": "boolean"}, "description": {"type": "string"}}},
                "Notification": {"type": "object", "additionalProperties": True, "properties": {"id": {"type": "string"}, "title": {"type": "string"}, "subject": {"type": "string"}, "description": {"type": "string"}, "importance": {"type": "string"}, "source": {"type": "string"}, "timestamp": {"type": "string", "format": "date-time"}, "read": {"type": "boolean"}, "archived": {"type": "boolean"}}},
                "User": {"type": "object", "additionalProperties": True, "properties": {"name": {"type": "string"}, "description": {"type": "string"}, "uid": {"type": "integer"}, "role": {"type": "string"}, "root": {"type": "boolean"}, "shares": {"type": "array", "items": {"type": "string"}}}},
                "GPU": {"type": "object", "additionalProperties": True, "properties": {"index": {"type": "integer"}, "name": {"type": "string"}, "vendor": {"type": "string"}, "memoryTotal": {"type": "integer"}, "memoryUsed": {"type": "integer"}, "utilization": {"type": "number"}, "temperature": {"type": "number"}, "power": {"type": "number"}, "processes": {"type": "array", "items": {"type": "object", "additionalProperties": True}}}},
                "UPS": {"type": "object", "additionalProperties": True, "properties": {"present": {"type": "boolean"}, "model": {"type": "string"}, "status": {"type": "string"}, "battery": {"type": "number"}, "load": {"type": "number"}, "runtime": {"type": "integer"}, "onBattery": {"type": "boolean"}}},
                "UPSSettings": {"type": "object", "additionalProperties": True},
                "NUTSettings": {"type": "object", "additionalProperties": True, "properties": {"available": {"type": "boolean"}, "service": {"type": "string"}, "mode": {"type": "string"}, "name": {"type": "string"}, "driver": {"type": "string"}, "port": {"type": "string"}, "ipAddr": {"type": "string"}, "batteryLevel": {"type": "integer"}, "runtimeValue": {"type": "integer"}, "timeout": {"type": "integer"}, "poll": {"type": "integer"}, "upsKill": {"type": "string"}, "passwordSet": {"type": "boolean"}, "monitorPassword": {"type": "string"}}},
                "ZFSPool": {"type": "object", "additionalProperties": True},
                "ZFSSnapshot": {"type": "object", "additionalProperties": True},
                "ConfigHistory": {"type": "object", "additionalProperties": True, "properties": {"id": {"type": "string"}, "created": {"type": "string", "format": "date-time"}, "action": {"type": "string"}, "description": {"type": "string"}, "files": {"type": "array", "items": {"type": "object", "additionalProperties": True}}}},
                "StorageLayout": {"type": "object", "additionalProperties": True},
                "OSStatus": {"type": "object", "additionalProperties": True},
                "UnraidAPIStatus": {"type": "object", "additionalProperties": True},
                "TrivyStatus": {"type": "object", "additionalProperties": True},
                "TrivyRelease": {"type": "object", "additionalProperties": True, "properties": {"version": {"type": "string"}, "name": {"type": "string"}, "publishedAt": {"type": "string", "format": "date-time"}, "prerelease": {"type": "boolean"}}},
                "AppCatalog": {"type": "object", "additionalProperties": True, "properties": {"entries": {"type": "array", "items": {"type": "object", "additionalProperties": True}}, "categories": {"type": "array", "items": {"type": "string"}}, "updated": {"type": "string", "format": "date-time"}}},
                "UserScript": {"type": "object", "additionalProperties": True, "properties": {"id": {"type": "string"}, "name": {"type": "string"}, "description": {"type": "string"}, "schedule": {"type": "string"}, "running": {"type": "boolean"}}},
                "BalanceJob": {"type": "object", "additionalProperties": True},
                "SchedulerState": {"type": "object", "additionalProperties": True, "properties": {"settings": ref("SchedulerSettings"), "system": {"type": "object", "additionalProperties": True}}},
                "Settings": {
                    "type": "object",
                    "additionalProperties": True,
                    "properties": {
                        "theme": {"type": "string"}, "accent": {"type": "string"}, "density": {"type": "string"},
                        "refreshInterval": {"type": "integer"}, "language": {"type": "string"},
                        "notifyMinImportance": {"type": "string"}, "notifyMutedSources": {"type": "array", "items": {"type": "string"}}, "notifyEvents": {"type": "object", "additionalProperties": {"type": "boolean"}},
                        "trivyEnabled": {"type": "boolean"}, "trivyVersion": {"type": "string"},
                    },
                },

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
