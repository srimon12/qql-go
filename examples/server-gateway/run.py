"""
run.py — qql-go gateway demo runner.

Starts the auth server and gateway, then runs scenarios that show
how the same query returns different results for different users.

Prerequisites:
    Qdrant running (default: http://localhost:6334 gRPC, :6333 REST)
    Embedding server running (default: http://127.0.0.1:1234)

Usage:
    uv run run.py
    uv run run.py --qdrant-url http://my-qdrant:6334
"""

from __future__ import annotations

import argparse
import base64
import json
import os
import signal
import subprocess
import sys
import tempfile
import time
import urllib.request
import urllib.error
from pathlib import Path

from rich.console import Console
from rich.panel import Panel
from rich.table import Table

console = Console()

SCRIPT_DIR = Path(__file__).parent
REPO_ROOT = SCRIPT_DIR.parent.parent
BINARY = REPO_ROOT / "qql-go"


# ---------------------------------------------------------------------------
# HTTP helpers
# ---------------------------------------------------------------------------

def get_token(auth_url: str, email: str, password: str) -> dict:
    body = json.dumps({"email": email, "password": password}).encode()
    req = urllib.request.Request(
        f"{auth_url}/auth/login",
        data=body,
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())


def custom_token(auth_url: str, claims: dict) -> str:
    body = json.dumps(claims).encode()
    req = urllib.request.Request(
        f"{auth_url}/auth/token",
        data=body,
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())["token"]


def qql_exec(gw_url: str, token: str, query: str) -> dict:
    body = json.dumps({"query": query}).encode()
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(f"{gw_url}/qql.QQL/Exec", data=body, headers=headers)
    try:
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        err_body = e.read().decode()
        try:
            return json.loads(err_body)
        except Exception:
            return {"error": err_body, "status": e.code}


def decode_data(raw):
    """Decode base64-encoded Connect RPC bytes into Python object."""
    if isinstance(raw, str) and raw:
        try:
            return json.loads(base64.b64decode(raw))
        except Exception:
            return raw
    return raw


def scroll_points(result: dict) -> list:
    """Extract point list from a SCROLL response."""
    data = decode_data(result.get("data", ""))
    if isinstance(data, dict):
        return data.get("points", [])
    if isinstance(data, list):
        return data
    return []


def wait_for(url: str, label: str, timeout: float = 15) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            urllib.request.urlopen(url, timeout=2)
            return True
        except Exception:
            time.sleep(0.5)
    console.print(f"[red]✗[/red] {label} not ready after {timeout}s")
    return False


# ---------------------------------------------------------------------------
# Display helpers
# ---------------------------------------------------------------------------

def ok(label: str, detail: str = ""):
    console.print(f"  [green]✓[/green] {label}")
    if detail:
        console.print(f"    [dim]{detail}[/dim]")


def denied(label: str, reason: str):
    console.print(f"  [red]✗[/red] {label}")
    console.print(f"    [red]DENIED:[/red] {reason}")


def failed(label: str, reason: str):
    console.print(f"  [red]✗[/red] {label}")
    console.print(f"    [red]FAILED:[/red] {reason}")


def show_docs(points: list):
    """Print a table of SCROLL results."""
    if not points:
        console.print("    [dim](no documents)[/dim]")
        return
    table = Table(show_header=True, header_style="bold", box=None, padding=(0, 1))
    table.add_column("Source", style="cyan", width=25)
    table.add_column("Tenant", width=12)
    table.add_column("Dept", width=12)
    table.add_column("Access", width=12)
    table.add_column("Text", max_width=45)
    for item in points:
        p = item.get("payload", {})
        table.add_row(
            p.get("source", "?"),
            p.get("tenant_id", "?"),
            p.get("department", "?"),
            p.get("access_level", "?"),
            (p.get("text", "")[:45] + "…") if len(p.get("text", "")) > 45 else p.get("text", ""),
        )
    console.print(table)


# ---------------------------------------------------------------------------
# Scenarios
# ---------------------------------------------------------------------------

def run_scenarios(gw_url: str, auth_url: str, audit_path: str) -> None:
    console.rule("[bold]qql-go Gateway Demo[/bold]")
    console.print()
    console.print("Same Qdrant cluster. Same query. Different users. Different results.")
    console.print()

    # ── Step 1: Get tokens ─────────────────────────────────────────────
    console.rule("Step 1: Authenticate users")
    console.print()

    users = {
        "alice": get_token(auth_url, "alice@acme.com", "alice123"),
        "bob": get_token(auth_url, "bob@acme.com", "bob123"),
        "carol": get_token(auth_url, "carol@globex.com", "carol123"),
        "dave": get_token(auth_url, "dave@acme.com", "dave123"),
        "eve": get_token(auth_url, "eve@globex.com", "eve123"),
    }

    for name, data in users.items():
        u = data["user"]
        console.print(f"  [bold]{name:6s}[/bold] {u['name']:15s}  "
                       f"role={u['role']:10s}  org={u['org_id']:12s}  dept={u.get('department', '-')}")
    console.print()

    tokens = {name: data["token"] for name, data in users.items()}

    # ── Step 2: Tenant isolation ───────────────────────────────────────
    console.rule("Step 2: Tenant isolation — same query, different tenants")
    console.print()
    console.print("  [dim]SCROLL FROM docs LIMIT 20  →  gateway injects WHERE tenant_id = <org_id>[/dim]")
    console.print()

    for name in ["alice", "carol"]:
        result = qql_exec(gw_url, tokens[name], "SCROLL FROM docs LIMIT 20")
        pts = scroll_points(result)
        u = users[name]["user"]
        label = f"{name} ({u['org_id']})"
        if result.get("ok"):
            ok(label, f"{len(pts)} documents returned")
            show_docs(pts)
        else:
            failed(label, result.get("message", "unknown error"))
    console.print()

    # ── Step 3: Admin sees all ─────────────────────────────────────────
    console.rule("Step 3: Admin bypasses tenant filter")
    console.print()
    console.print("  [dim]bob is admin — no tenant_id filter injected[/dim]")
    console.print()

    result = qql_exec(gw_url, tokens["bob"], "SCROLL FROM docs LIMIT 20")
    pts = scroll_points(result)
    if result.get("ok"):
        ok(f"bob (admin)", f"{len(pts)} documents — sees all tenants")
        show_docs(pts)
    else:
        failed("bob (admin)", result.get("message", "unknown error"))
    console.print()

    # ── Step 4: Destructive operations ─────────────────────────────────
    console.rule("Step 4: Operation control — DELETE blocked for readers")
    console.print()

    result = qql_exec(gw_url, tokens["alice"], "DELETE FROM docs WHERE id = 1")
    if not result.get("ok") and "not permitted" in result.get("message", "").lower():
        denied("alice (reader) → DELETE", result["message"])
    else:
        ok("alice (reader) → DELETE")

    result = qql_exec(gw_url, tokens["eve"], "INSERT INTO docs VALUES {'id': 999, 'text': 'test', 'tenant_id': 'globex-corp', 'department': 'all', 'access_level': 'internal', 'topic': 'test', 'source': 'test.md'} USING HYBRID")
    if result.get("ok"):
        ok("eve (manager) → INSERT", result.get("message", ""))
    else:
        failed("eve (manager) → INSERT", result.get("message", "unknown error"))
    console.print()

    # ── Step 5: LIMIT cap ──────────────────────────────────────────────
    console.rule("Step 5: LIMIT cap — policy enforces max_limit=50")
    console.print()
    console.print("  [dim]alice requests LIMIT 500, policy caps to 50[/dim]")
    console.print()

    result = qql_exec(gw_url, tokens["alice"], "SCROLL FROM docs LIMIT 500")
    pts = scroll_points(result)
    if result.get("ok"):
        ok("alice → SCROLL LIMIT 500", f"{len(pts)} documents returned (capped by policy)")
    else:
        failed("alice → SCROLL LIMIT 500", result.get("message", "unknown error"))
    console.print()

    # ── Step 6: Agent restrictions ─────────────────────────────────────
    console.rule("Step 6: Agent tokens — narrowest surface")
    console.print()

    agent = custom_token(auth_url, {"sub": "agent-bot", "token_type": "agent"})

    result = qql_exec(gw_url, agent, "SCROLL FROM docs LIMIT 10")
    pts = scroll_points(result)
    if result.get("ok"):
        ok("agent → SCROLL docs", f"{len(pts)} public documents")
        show_docs(pts)
    else:
        failed("agent → SCROLL docs", result.get("message", "unknown error"))

    result = qql_exec(gw_url, agent, "DELETE FROM docs WHERE id = 1")
    if not result.get("ok") and "not permitted" in result.get("message", "").lower():
        denied("agent → DELETE", result["message"])
    else:
        ok("agent → DELETE")
    console.print()

    # ── Step 7: Unauthenticated ────────────────────────────────────────
    console.rule("Step 7: Unauthenticated request rejected")
    console.print()

    result = qql_exec(gw_url, "", "SCROLL FROM docs LIMIT 1")
    if not result.get("ok"):
        denied("no token → SCROLL", result.get("message", "authentication required"))
    else:
        ok("no token → SCROLL")
    console.print()

    # ── Step 8: Audit log ──────────────────────────────────────────────
    console.rule("Step 8: Audit trail")
    console.print()

    if os.path.exists(audit_path):
        with open(audit_path) as f:
            lines = [l.strip() for l in f if l.strip()]
        entries = []
        for line in lines:
            try:
                entries.append(json.loads(line))
            except json.JSONDecodeError:
                pass

        if entries:
            table = Table(show_header=True, header_style="bold", box=None, padding=(0, 1))
            table.add_column("Operation", width=8)
            table.add_column("Subject", width=20)
            table.add_column("Tenant", width=12)
            table.add_column("Collection", width=10)
            table.add_column("Status", width=8)
            table.add_column("Denied Reason", width=35)
            table.add_column("ms", width=5, justify="right")
            for e in entries:
                table.add_row(
                    e.get("operation", "?"),
                    e.get("subject", "-") or "-",
                    e.get("tenant_id", "-") or "-",
                    e.get("collection", "-") or "-",
                    e.get("status", "?"),
                    e.get("denied_reason", "") or "",
                    str(e.get("latency_ms", "")),
                )
            console.print(table)
        else:
            console.print("  [dim](no audit entries parsed)[/dim]")
    else:
        console.print(f"  [dim](audit log not found at {audit_path})[/dim]")
    console.print()

    console.rule("[bold green]Demo complete[/bold green]")
    console.print()


# ---------------------------------------------------------------------------
# Main — start services, run scenarios, clean up.
# ---------------------------------------------------------------------------

def main() -> None:
    parser = argparse.ArgumentParser(description="qql-go gateway demo")
    parser.add_argument("--qdrant-url", default="http://localhost:6334",
                        help="Qdrant gRPC endpoint for the gateway (default: http://localhost:6334)")
    parser.add_argument("--embedding-url", default="http://127.0.0.1:1234/v1/embeddings",
                        help="Embedding server endpoint (default: http://127.0.0.1:1234/v1/embeddings)")
    parser.add_argument("--auth-port", type=int, default=8081)
    parser.add_argument("--gw-port", type=int, default=50051)
    parser.add_argument("--no-gateway", action="store_true",
                        help="Skip starting gateway (use if already running)")
    parser.add_argument("--no-auth", action="store_true",
                        help="Skip starting auth server (use if already running)")
    args = parser.parse_args()

    auth_url = f"http://127.0.0.1:{args.auth_port}"
    gw_url = f"http://127.0.0.1:{args.gw_port}"

    procs: list[subprocess.Popen] = []
    audit_file = tempfile.NamedTemporaryFile(mode="w", suffix=".jsonl", delete=False, prefix="qql_audit_")
    audit_path = audit_file.name
    audit_file.close()

    def cleanup():
        for p in procs:
            p.terminate()
            p.wait(timeout=5)

    try:
        # ── Check Qdrant ───────────────────────────────────────────────
        console.print(f"[cyan]qdrant:[/cyan] {args.qdrant_url}")
        qdrant_rest = args.qdrant_url.replace(":6334", ":6333")
        if not wait_for(f"{qdrant_rest}/healthz", "Qdrant"):
            console.print("[red]Qdrant is not running. Start it first.[/red]")
            sys.exit(1)
        console.print("[green]✓[/green] Qdrant ready")
        console.print()

        # ── Build binary ───────────────────────────────────────────────
        console.print("[cyan]building qql-go...[/cyan]")
        subprocess.run(
            ["go", "build", "-o", str(BINARY), "./cmd/qql-go/"],
            cwd=str(REPO_ROOT),
            check=True,
            capture_output=True,
        )
        console.print("[green]✓[/green] binary built")
        console.print()

        # ── Seed data ──────────────────────────────────────────────────
        seed_file = SCRIPT_DIR / "01-seed.qql"
        console.print(f"[cyan]seeding:[/cyan] {seed_file.name}")
        result = subprocess.run(
            [str(BINARY), "execute", str(seed_file)],
            cwd=str(REPO_ROOT),
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            console.print(f"[red]✗[/red] seed failed: {result.stderr.strip()}")
            sys.exit(1)
        # Count lines in output as a rough indicator.
        lines = [l for l in result.stdout.strip().split("\n") if l.strip()]
        console.print(f"[green]✓[/green] seeded ({len(lines)} statements executed)")
        console.print()

        # ── Start auth server ──────────────────────────────────────────
        if not args.no_auth:
            console.print(f"[cyan]auth server:[/cyan] starting on :{args.auth_port}")
            p = subprocess.Popen(
                [sys.executable, str(SCRIPT_DIR / "auth_server.py"), "--port", str(args.auth_port)],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            procs.append(p)
            if not wait_for(f"{auth_url}/health", "Auth server"):
                sys.exit(1)
            console.print(f"[green]✓[/green] auth server ready (pid {p.pid})")
        console.print()

        # ── Start gateway with audit log ───────────────────────────────
        if not args.no_gateway:
            policy_file = SCRIPT_DIR / "policies.yaml"
            console.print(f"[cyan]gateway:[/cyan] starting on :{args.gw_port}")
            console.print(f"[cyan]audit log:[/cyan] {audit_path}")
            p = subprocess.Popen(
                [
                    str(BINARY), "serve",
                    "--qdrant-url", args.qdrant_url,
                    "--listen", f":{args.gw_port}",
                    "--jwks-url", f"{auth_url}/.well-known/jwks.json",
                    "--jwt-issuer", "qql-demo-auth",
                    "--tenant-claim", "org_id",
                    "--role-claim", "role",
                    "--policy-file", str(policy_file),
                    "--audit",
                    "--audit-file", audit_path,
                    "--inference-mode", "local",
                    "--embedding-endpoint", args.embedding_url,
                    "--embedding-model", "text-embedding-all-minilm-l6-v2-embedding",
                    "--embedding-dimension", "384",
                ],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            procs.append(p)
            if not wait_for(f"{gw_url}/health", "Gateway"):
                sys.exit(1)
            console.print(f"[green]✓[/green] gateway ready (pid {p.pid})")
        console.print()

        # ── Run scenarios ──────────────────────────────────────────────
        run_scenarios(gw_url, auth_url, audit_path)

    except KeyboardInterrupt:
        console.print("\n[yellow]Interrupted[/yellow]")
    finally:
        cleanup()
        console.print("[dim]Services stopped.[/dim]")


if __name__ == "__main__":
    main()
