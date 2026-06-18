"""
Mini Auth Server — a self-contained identity provider for the qql-go gateway demo.

This is a FastAPI app that acts as a minimal IdP:
- Stores users in-memory (swap for a real DB in production)
- Issues RS256 JWTs with configurable claims
- Exposes JWKS at /.well-known/jwks.json so the gateway can validate tokens
- Supports login with email/password

Endpoints:
    GET  /.well-known/jwks.json   → public keys (gateway polls this)
    POST /auth/login              → authenticate, get JWT
    POST /auth/token              → get JWT with custom claims (for demo)
    GET  /health                  → health check

Usage:
    uv run auth_server.py
    uv run auth_server.py --port 8081
"""

from __future__ import annotations

import argparse
import base64
import time
import uuid
from datetime import datetime, timezone
from typing import Optional

import jwt
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.primitives import serialization
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

# ---------------------------------------------------------------------------
# RSA key pair — generated once at startup.
# In production, persist and rotate these. For a demo, ephemeral is fine.
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
    "keys": [
        {
            "kty": "RSA",
            "alg": "RS256",
            "use": "sig",
            "kid": KID,
            "n": _b64url(_n_bytes),
            "e": _b64url(_e_bytes),
        }
    ]
}

# ---------------------------------------------------------------------------
# User store — in-memory. In production, this is your database.
# ---------------------------------------------------------------------------

USERS: dict[str, dict] = {
    "alice@acme.com": {
        "user_id": "usr_alice",
        "email": "alice@acme.com",
        "password": "alice123",
        "name": "Alice Chen",
        "role": "reader",
        "org_id": "acme-corp",
        "org_name": "ACME Corp",
        "department": "engineering",
    },
    "bob@acme.com": {
        "user_id": "usr_bob",
        "email": "bob@acme.com",
        "password": "bob123",
        "name": "Bob Smith",
        "role": "admin",
        "org_id": "acme-corp",
        "org_name": "ACME Corp",
        "department": "engineering",
    },
    "carol@globex.com": {
        "user_id": "usr_carol",
        "email": "carol@globex.com",
        "password": "carol123",
        "name": "Carol Rivera",
        "role": "reader",
        "org_id": "globex-corp",
        "org_name": "Globex Corp",
        "department": "finance",
    },
    "dave@acme.com": {
        "user_id": "usr_dave",
        "email": "dave@acme.com",
        "password": "dave123",
        "name": "Dave Patel",
        "role": "reader",
        "org_id": "acme-corp",
        "org_name": "ACME Corp",
        "department": "finance",
    },
    "eve@globex.com": {
        "user_id": "usr_eve",
        "email": "eve@globex.com",
        "password": "eve123",
        "name": "Eve Nakamura",
        "role": "manager",
        "org_id": "globex-corp",
        "org_name": "Globex Corp",
        "department": "engineering",
    },
}

# ---------------------------------------------------------------------------
# JWT minting
# ---------------------------------------------------------------------------

ISSUER = "qql-demo-auth"
TOKEN_TTL = 3600  # 1 hour


def mint_token(claims: dict) -> str:
    now = int(time.time())
    payload = {
        "iss": ISSUER,
        "iat": now,
        "exp": now + TOKEN_TTL,
        "jti": uuid.uuid4().hex,
    }
    payload.update(claims)
    return jwt.encode(payload, _PRIVATE_PEM, algorithm="RS256", headers={"kid": KID})


# ---------------------------------------------------------------------------
# FastAPI app
# ---------------------------------------------------------------------------

app = FastAPI(title="qql-demo auth server", version="0.1.0")


class LoginRequest(BaseModel):
    email: str
    password: str


class TokenRequest(BaseModel):
    """Request a token with custom claims. For demo/testing only."""
    sub: Optional[str] = None
    role: Optional[str] = None
    org_id: Optional[str] = None
    org_name: Optional[str] = None
    department: Optional[str] = None
    token_type: Optional[str] = None


@app.get("/.well-known/jwks.json")
def jwks():
    return JWKS


@app.post("/auth/login")
def login(req: LoginRequest):
    user = USERS.get(req.email)
    if not user or user["password"] != req.password:
        raise HTTPException(status_code=401, detail="invalid credentials")

    token = mint_token({
        "sub": user["user_id"],
        "email": user["email"],
        "name": user["name"],
        "role": user["role"],
        "org_id": user["org_id"],
        "org_name": user["org_name"],
        "department": user["department"],
    })
    return {
        "token": token,
        "user": {
            "user_id": user["user_id"],
            "email": user["email"],
            "name": user["name"],
            "role": user["role"],
            "org_id": user["org_id"],
            "department": user["department"],
        },
    }


@app.post("/auth/token")
def custom_token(req: TokenRequest):
    """Issue a token with arbitrary claims. Used by the demo script."""
    claims = {}
    if req.sub:
        claims["sub"] = req.sub
    if req.role:
        claims["role"] = req.role
    if req.org_id:
        claims["org_id"] = req.org_id
    if req.org_name:
        claims["org_name"] = req.org_name
    if req.department:
        claims["department"] = req.department
    if req.token_type:
        claims["token_type"] = req.token_type
    token = mint_token(claims)
    return {"token": token, "claims": claims}


@app.get("/health")
def health():
    return {"ok": True, "users": len(USERS), "kid": KID}


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="qql-demo auth server")
    parser.add_argument("--port", type=int, default=8081)
    parser.add_argument("--host", default="127.0.0.1")
    args = parser.parse_args()

    import uvicorn
    print(f"auth server listening on http://{args.host}:{args.port}")
    print(f"  JWKS:  http://{args.host}:{args.port}/.well-known/jwks.json")
    print(f"  Login: POST http://{args.host}:{args.port}/auth/login")
    uvicorn.run(app, host=args.host, port=args.port, log_level="warning")
