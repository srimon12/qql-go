"""
Mini Auth Server — self-contained identity provider for the qql-go gateway demo.

4 organizations, 12+ teams, 30+ users.
Each user has: org, team (department), role.

Endpoints:
    GET  /.well-known/jwks.json   → public keys (gateway polls this)
    POST /auth/login              → authenticate, get JWT with custom claims.
    GET  /health                  → health check
    GET  /users                   → list all users (for UI)
"""

from __future__ import annotations

import argparse
import base64
import time
import uuid

import jwt
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.primitives import serialization
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

# ---------------------------------------------------------------------------
# RSA key pair
# ---------------------------------------------------------------------------

_private_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
_public_key = _private_key.public_key()
KID = uuid.uuid4().hex[:8]

_PRIVATE_PEM = _private_key.private_bytes(
    encoding=serialization.Encoding.PEM,
    format=serialization.PrivateFormat.PKCS8,
    encryption_algorithm=serialization.NoEncryption(),
)

_pub_numbers = _public_key.public_numbers()
_n_bytes = _pub_numbers.n.to_bytes((_pub_numbers.n.bit_length() + 7) // 8, "big")
_e_bytes = _pub_numbers.e.to_bytes((_pub_numbers.e.bit_length() + 7) // 8, "big")


def _b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


JWKS = {
    "keys": [{
        "kty": "RSA", "alg": "RS256", "use": "sig", "kid": KID,
        "n": _b64url(_n_bytes), "e": _b64url(_e_bytes),
    }]
}

# ---------------------------------------------------------------------------
# User store — 4 orgs, 12+ teams, 30+ users
# ---------------------------------------------------------------------------

USERS = {
    # ── Superadmin (god mode, no tenant scoping) ─────────────────────
    "admin@qql-go.io": {"user_id": "superadmin", "name": "QQL Admin", "role": "platform_admin", "org_id": "system", "org_name": "System", "department": "all", "password": "admin123"},

    # ── ACME Corp (Cloud Infrastructure) ─────────────────────────────
    "alice@acme.com":   {"user_id": "acme_alice",   "name": "Alice Chen",     "role": "reader",  "org_id": "acme", "org_name": "ACME Corp",     "department": "engineering",  "password": "alice123"},
    "bob@acme.com":     {"user_id": "acme_bob",     "name": "Bob Smith",      "role": "admin",   "org_id": "acme", "org_name": "ACME Corp",     "department": "engineering",  "password": "bob123"},
    "dave@acme.com":    {"user_id": "acme_dave",    "name": "Dave Patel",     "role": "reader",  "org_id": "acme", "org_name": "ACME Corp",     "department": "finance",      "password": "dave123"},
    "frank@acme.com":   {"user_id": "acme_frank",   "name": "Frank Miller",   "role": "reader",  "org_id": "acme", "org_name": "ACME Corp",     "department": "engineering",  "password": "frank123"},
    "grace@acme.com":   {"user_id": "acme_grace",   "name": "Grace Lee",      "role": "reader",  "org_id": "acme", "org_name": "ACME Corp",     "department": "security",     "password": "grace123"},
    "helen@acme.com":   {"user_id": "acme_helen",   "name": "Helen Park",     "role": "reader",  "org_id": "acme", "org_name": "ACME Corp",     "department": "finance",      "password": "helen123"},
    "ivan@acme.com":    {"user_id": "acme_ivan",    "name": "Ivan Torres",    "role": "reader",  "org_id": "acme", "org_name": "ACME Corp",     "department": "security",     "password": "ivan123"},

    # ── Globex Corp (Digital Banking) ────────────────────────────────
    "carol@globex.com": {"user_id": "globex_carol", "name": "Carol Rivera",   "role": "reader",  "org_id": "globex", "org_name": "Globex Corp",   "department": "finance",      "password": "carol123"},
    "eve@globex.com":   {"user_id": "globex_eve",   "name": "Eve Nakamura",   "role": "manager", "org_id": "globex", "org_name": "Globex Corp",   "department": "engineering",  "password": "eve123"},
    "mike@globex.com":  {"user_id": "globex_mike",  "name": "Mike Johnson",   "role": "reader",  "org_id": "globex", "org_name": "Globex Corp",   "department": "engineering",  "password": "mike123"},
    "nina@globex.com":  {"user_id": "globex_nina",  "name": "Nina Williams",  "role": "reader",  "org_id": "globex", "org_name": "Globex Corp",   "department": "compliance",   "password": "nina123"},
    "pat@globex.com":   {"user_id": "globex_pat",   "name": "Pat Garcia",     "role": "reader",  "org_id": "globex", "org_name": "Globex Corp",   "department": "finance",      "password": "pat123"},
    "uma@globex.com":   {"user_id": "globex_uma",   "name": "Uma Sharma",     "role": "admin",   "org_id": "globex", "org_name": "Globex Corp",   "department": "engineering",  "password": "uma123"},

    # ── Initech Inc (Healthcare Devices) ─────────────────────────────
    "finn@initech.com":   {"user_id": "initech_finn",   "name": "Finn O'Brien",    "role": "admin",   "org_id": "initech", "org_name": "Initech Inc",  "department": "engineering",  "password": "finn123"},
    "victor@initech.com": {"user_id": "initech_victor", "name": "Victor Hugo",     "role": "reader",  "org_id": "initech", "org_name": "Initech Inc",  "department": "engineering",  "password": "victor123"},
    "wendy@initech.com":  {"user_id": "initech_wendy",  "name": "Wendy Tanaka",    "role": "reader",  "org_id": "initech", "org_name": "Initech Inc",  "department": "clinical",     "password": "wendy123"},
    "xavier@initech.com": {"user_id": "initech_xavier", "name": "Xavier Dubois",   "role": "reader",  "org_id": "initech", "org_name": "Initech Inc",  "department": "compliance",   "password": "xavier123"},
    "yuki@initech.com":   {"user_id": "initech_yuki",   "name": "Yuki Tanaka",     "role": "reader",  "org_id": "initech", "org_name": "Initech Inc",  "department": "rd",           "password": "yuki123"},
    "zara@initech.com":   {"user_id": "initech_zara",   "name": "Zara Ahmed",      "role": "reader",  "org_id": "initech", "org_name": "Initech Inc",  "department": "clinical",     "password": "zara123"},

    # ── Umbrella Corp (Industrial IoT) ───────────────────────────────
    "quinn@umbrella.com": {"user_id": "umbrella_quinn", "name": "Quinn Adams",    "role": "admin",   "org_id": "umbrella", "org_name": "Umbrella Corp", "department": "operations",   "password": "quinn123"},
    "glenn@umbrella.com": {"user_id": "umbrella_glenn", "name": "Glenn Rossi",    "role": "reader",  "org_id": "umbrella", "org_name": "Umbrella Corp", "department": "engineering",  "password": "glenn123"},
    "holly@umbrella.com": {"user_id": "umbrella_holly", "name": "Holly Chen",     "role": "reader",  "org_id": "umbrella", "org_name": "Umbrella Corp", "department": "supply_chain", "password": "holly123"},
    "kira@umbrella.com":  {"user_id": "umbrella_kira",  "name": "Kira Volkov",    "role": "reader",  "org_id": "umbrella", "org_name": "Umbrella Corp", "department": "quality",      "password": "kira123"},
    "noah@umbrella.com":  {"user_id": "umbrella_noah",  "name": "Noah Fischer",   "role": "reader",  "org_id": "umbrella", "org_name": "Umbrella Corp", "department": "quality",      "password": "noah123"},
    "olivia@umbrella.com":{"user_id": "umbrella_olivia","name": "Olivia Santos",  "role": "reader",  "org_id": "umbrella", "org_name": "Umbrella Corp", "department": "operations",   "password": "olivia123"},
}

ISSUER = "qql-demo-auth"
TOKEN_TTL = 3600


def mint_token(claims: dict) -> str:
    now = int(time.time())
    payload = {"iss": ISSUER, "iat": now, "exp": now + TOKEN_TTL, "jti": uuid.uuid4().hex}
    payload.update(claims)
    return jwt.encode(payload, _PRIVATE_PEM, algorithm="RS256", headers={"kid": KID})


# ---------------------------------------------------------------------------
# FastAPI app
# ---------------------------------------------------------------------------

app = FastAPI(title="qql-demo auth server", version="0.2.0")


class LoginRequest(BaseModel):
    email: str
    password: str


class TokenRequest(BaseModel):
    sub: str | None = None
    role: str | None = None
    org_id: str | None = None
    org_name: str | None = None
    department: str | None = None
    token_type: str | None = None


@app.get("/.well-known/jwks.json")
def jwks():
    return JWKS


@app.post("/auth/login")
def login(req: LoginRequest):
    user = USERS.get(req.email)
    if not user or user["password"] != req.password:
        raise HTTPException(status_code=401, detail="invalid credentials")
    token = mint_token({
        "sub": user["user_id"], "email": req.email, "name": user["name"],
        "role": user["role"], "org_id": user["org_id"], "org_name": user["org_name"],
        "department": user["department"],
    })
    return {"token": token, "user": {k: v for k, v in user.items() if k != "password"}}


@app.get("/health")
def health():
    return {"ok": True, "users": len(USERS), "kid": KID}


@app.get("/users")
def list_users():
    return [
        {"email": email, **{k: v for k, v in u.items() if k != "password"}}
        for email, u in USERS.items()
    ]


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="qql-demo auth server")
    parser.add_argument("--port", type=int, default=8081)
    parser.add_argument("--host", default="127.0.0.1")
    args = parser.parse_args()

    import uvicorn
    print(f"auth server on http://{args.host}:{args.port} ({len(USERS)} users, 4 orgs)")
    uvicorn.run(app, host=args.host, port=args.port, log_level="warning")
