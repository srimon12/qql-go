#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------
# test_server.sh — Start the QQL server and test the Convert
# endpoint against every payload in all_payloads.json.
# Also sends each converted statement to Explain to verify parsing.
# ---------------------------------------------------------------

PORT=15005
ADDR="http://localhost:${PORT}"
PAYLOADS="all_payloads.json"
SERVER_PID=""

cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
    echo ""
    echo "Stopping server (PID $SERVER_PID)..."
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -f ./qql-go-test
}
trap cleanup EXIT

# ── 1. Build the binary ────────────────────────────────────────
echo "Building qql-go..."
go build -o ./qql-go-test ./cmd/qql-go/main.go
echo ""

# ── 2. Start the server (no Qdrant needed — Convert is offline) ─
echo "Starting server on :${PORT}..."
./qql-go-test serve \
  --listen ":${PORT}" \
  --qdrant-url "http://localhost:6334" &
SERVER_PID=$!

echo "Waiting for server..."
for i in $(seq 1 30); do
  if curl -sf "${ADDR}/health" >/dev/null 2>&1; then
    echo "Server is up (PID $SERVER_PID)"
    break
  fi
  sleep 0.5
done

if ! curl -sf "${ADDR}/health" >/dev/null 2>&1; then
  echo "ERROR: server did not start"
  exit 1
fi
echo ""

# ── 3. Run all tests via Python ────────────────────────────────
python3 << 'PYEOF'
import json, sys, urllib.request, urllib.error

ADDR = "http://localhost:15005"
PAYLOADS = "all_payloads.json"

def post_json(path, body):
    data = json.dumps(body).encode()
    req = urllib.request.Request(
        f"{ADDR}{path}",
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())

with open(PAYLOADS) as f:
    payloads = json.load(f)

total = len(payloads)
print(f"Testing {total} payloads against {ADDR}/qql.QQL/Convert")
print("=" * 64)

convert_pass = 0
convert_fail = 0
explain_pass = 0
explain_fail = 0

for i, p in enumerate(payloads):
    label = p["label"]
    json_payload = p["json"]

    # ── Convert ──
    try:
        resp = post_json("/qql.QQL/Convert", {"jsonPayload": json_payload})
    except Exception as e:
        print(f"CONVERT ✗ [{label}] — {e}")
        convert_fail += 1
        continue

    if resp.get("ok"):
        stmts = resp.get("statements", [])
        print(f"CONVERT ✓ [{label}] — {len(stmts)} statement(s)")
        convert_pass += 1

        # ── Explain each statement ──
        for stmt in stmts:
            try:
                er = post_json("/qql.QQL/Explain", {"query": stmt})
                if er.get("ok"):
                    print(f"  EXPLAIN ✓ — {stmt}")
                    explain_pass += 1
                else:
                    print(f"  EXPLAIN ✗ — {stmt}")
                    explain_fail += 1
            except Exception as e:
                print(f"  EXPLAIN ✗ — {stmt} ({e})")
                explain_fail += 1
    else:
        err_msg = resp.get("error", "unknown")
        print(f"CONVERT ✗ [{label}] — {err_msg}")
        convert_fail += 1

print()
print("=" * 64)
print(f"  RESULTS:  {total} payloads tested")
print("=" * 64)
print(f"  Convert : {convert_pass} passed, {convert_fail} failed")
print(f"  Explain : {explain_pass} passed, {explain_fail} failed")
print("=" * 64)

if convert_fail > 0 or explain_fail > 0:
    sys.exit(1)
PYEOF
