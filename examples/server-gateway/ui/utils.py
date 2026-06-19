"""Shared utilities for the qql-go gateway dashboard."""

from __future__ import annotations

import base64
import json
import os
from pathlib import Path

import httpx

AUTH_URL = os.getenv("AUTH_URL", "http://127.0.0.1:8081")
GW_URL = os.getenv("GW_URL", "http://127.0.0.1:50051")
QDRANT_URL = os.getenv("QDRANT_URL", "http://127.0.0.1:6333")
EMBEDDING_URL = os.getenv("EMBEDDING_URL", "http://127.0.0.1:1234")
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


def embed_text(text: str, model: str = "text-embedding-all-minilm-l6-v2-embedding") -> list[float] | None:
    """Get embedding vector from local embedding service."""
    try:
        resp = httpx.post(
            f"{EMBEDDING_URL}/v1/embeddings",
            json={"model": model, "input": text},
            timeout=10
        )
        data = resp.json()
        return data.get("data", [{}])[0].get("embedding")
    except Exception:
        return None


DEMO_USERS = {
    # Superadmin
    "admin@qql-go.io":  {"password": "admin123",   "label": "QQL Admin · system · platform_admin"},
    # ACME Corp
    "alice@acme.com":   {"password": "alice123",   "label": "Alice Chen · acme · engineering · reader"},
    "bob@acme.com":     {"password": "bob123",     "label": "Bob Smith · acme · engineering · admin"},
    "dave@acme.com":    {"password": "dave123",    "label": "Dave Patel · acme · finance · reader"},
    "frank@acme.com":   {"password": "frank123",   "label": "Frank Miller · acme · engineering · reader"},
    "grace@acme.com":   {"password": "grace123",   "label": "Grace Lee · acme · security · reader"},
    # Globex Corp
    "carol@globex.com": {"password": "carol123",   "label": "Carol Rivera · globex · finance · reader"},
    "eve@globex.com":   {"password": "eve123",     "label": "Eve Nakamura · globex · engineering · manager"},
    "mike@globex.com":  {"password": "mike123",    "label": "Mike Johnson · globex · engineering · reader"},
    "nina@globex.com":  {"password": "nina123",    "label": "Nina Williams · globex · compliance · reader"},
    "uma@globex.com":   {"password": "uma123",     "label": "Uma Sharma · globex · engineering · admin"},
    # Initech Inc
    "finn@initech.com":   {"password": "finn123",   "label": "Finn O'Brien · initech · engineering · admin"},
    "victor@initech.com": {"password": "victor123", "label": "Victor Hugo · initech · engineering · reader"},
    "wendy@initech.com":  {"password": "wendy123",  "label": "Wendy Tanaka · initech · clinical · reader"},
    "yuki@initech.com":   {"password": "yuki123",   "label": "Yuki Tanaka · initech · rd · reader"},
    # Umbrella Corp
    "quinn@umbrella.com": {"password": "quinn123",  "label": "Quinn Adams · umbrella · operations · admin"},
    "glenn@umbrella.com": {"password": "glenn123",  "label": "Glenn Rossi · umbrella · engineering · reader"},
    "holly@umbrella.com": {"password": "holly123",  "label": "Holly Chen · umbrella · supply_chain · reader"},
    "kira@umbrella.com":  {"password": "kira123",   "label": "Kira Volkov · umbrella · quality · reader"},
}
