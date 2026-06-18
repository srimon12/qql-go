"""Shared utilities for the qql-go gateway dashboard."""

from __future__ import annotations

import base64
import json
import os
from pathlib import Path

import httpx

AUTH_URL = os.getenv("AUTH_URL", "http://127.0.0.1:8081")
GW_URL = os.getenv("GW_URL", "http://127.0.0.1:50051")
SCRIPT_DIR = Path(__file__).parent
EXAMPLE_DIR = SCRIPT_DIR.parent
REPO_ROOT = EXAMPLE_DIR.parent.parent
POLICY_FILE = EXAMPLE_DIR / "policies.yaml"
TEMPLATE_FILE = EXAMPLE_DIR / "templates.yaml"
PAYLOADS_FILE = REPO_ROOT / "all_payloads.json"


def api_post(url: str, data: dict, token: str | None = None) -> dict:
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    try:
        resp = httpx.post(url, json=data, headers=headers, timeout=30)
        return resp.json()
    except httpx.HTTPStatusError as e:
        try:
            return e.response.json()
        except Exception:
            return {"error": str(e), "status": e.response.status_code}
    except Exception as e:
        return {"error": str(e)}


def api_get(url: str, token: str | None = None) -> dict:
    headers = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    try:
        resp = httpx.get(url, headers=headers, timeout=10)
        return resp.json()
    except Exception as e:
        return {"error": str(e)}


def login(email: str, password: str) -> dict | None:
    return api_post(f"{AUTH_URL}/auth/login", {"email": email, "password": password})


def decode_data(raw):
    if isinstance(raw, str) and raw:
        try:
            return json.loads(base64.b64decode(raw))
        except Exception:
            return raw
    return raw


def scroll_points(result: dict) -> list:
    data = decode_data(result.get("data", ""))
    if isinstance(data, dict):
        return data.get("points", [])
    if isinstance(data, list):
        return data
    return []


def load_payload_examples() -> dict[str, str]:
    if PAYLOADS_FILE.exists():
        with open(PAYLOADS_FILE) as f:
            return {p["label"]: p["json"] for p in json.load(f)}
    return {}


DEMO_USERS = {
    "alice@acme.com": {"password": "alice123", "label": "Alice (reader, acme-corp, engineering)"},
    "bob@acme.com": {"password": "bob123", "label": "Bob (admin, acme-corp)"},
    "carol@globex.com": {"password": "carol123", "label": "Carol (reader, globex-corp, finance)"},
    "dave@acme.com": {"password": "dave123", "label": "Dave (reader, acme-corp, finance)"},
    "eve@globex.com": {"password": "eve123", "label": "Eve (manager, globex-corp, engineering)"},
}
