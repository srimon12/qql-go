#!/usr/bin/env bash
# start_ui.sh — Start all services and launch the Streamlit dashboard.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}▶${NC} $*"; }
ok()    { echo -e "${GREEN}✓${NC} $*"; }

cleanup() {
    info "Stopping services..."
    [ -n "${GW_PID:-}" ] && kill "$GW_PID" 2>/dev/null
    [ -n "${AUTH_PID:-}" ] && kill "$AUTH_PID" 2>/dev/null
    [ -n "${ST_PID:-}" ] && kill "$ST_PID" 2>/dev/null
    # Kill any leftover processes on our ports
    for port in 8501 50051 8081; do
        lsof -ti :"$port" 2>/dev/null | xargs kill 2>/dev/null
    done
}
trap cleanup EXIT

# ── Check Qdrant ────────────────────────────────────────────────────
info "Checking Qdrant..."
if ! curl -sf http://localhost:6333/healthz >/dev/null 2>&1; then
    echo -e "${RED}Qdrant is not running. Start it first.${NC}"
    echo "  docker run -d --name qdrant -p 6334:6333 qdrant/qdrant:latest"
    exit 1
fi
ok "Qdrant ready"

# ── Build binary ────────────────────────────────────────────────────
info "Building qql-go..."
go build -o "$REPO_ROOT/qql-go" "$REPO_ROOT/cmd/qql-go/"
ok "Binary built"

# ── Start auth server ───────────────────────────────────────────────
info "Starting auth server on :8081..."
uv run python3 "$SCRIPT_DIR/auth_server.py" --port 8081 &
AUTH_PID=$!
sleep 2
ok "Auth server ready (pid $AUTH_PID)"

# ── Start gateway ───────────────────────────────────────────────────
AUDIT_FILE="$SCRIPT_DIR/audit.jsonl"
: > "$AUDIT_FILE"  # truncate on each run
info "Starting gateway on :50051..."
info "Audit log: $AUDIT_FILE"

"$REPO_ROOT/qql-go" serve \
    --qdrant-url http://localhost:6334 \
    --listen :50051 \
    --jwks-url http://127.0.0.1:8081/.well-known/jwks.json \
    --jwt-issuer qql-demo-auth \
    --tenant-claim org_id \
    --role-claim role \
    --policy-file "$SCRIPT_DIR/policies.yaml" \
    --policy-reload \
    --templates "$SCRIPT_DIR/templates.yaml" \
    --audit \
    --audit-file "$AUDIT_FILE" \
    --inference-mode local \
    --embedding-endpoint http://127.0.0.1:1234/v1/embeddings \
    --embedding-model text-embedding-all-minilm-l6-v2-embedding \
    --embedding-dimension 384 &
GW_PID=$!
sleep 3
ok "Gateway ready (pid $GW_PID)"

# ── Seed data via gateway ───────────────────────────────────────────
info "Seeding data via gateway..."
uv run python3 "$SCRIPT_DIR/seed.py" --gateway http://localhost:50051 --auth http://127.0.0.1:8081
ok "Seeded"

# ── Start Streamlit ─────────────────────────────────────────────────
info "Starting Streamlit dashboard..."
export AUTH_URL="http://127.0.0.1:8081"
export GW_URL="http://127.0.0.1:50051"
export QQL_AUDIT_FILE="$AUDIT_FILE"

cd "$SCRIPT_DIR"
uv run streamlit run ui/app.py \
    --server.port 8501 \
    --server.headless true \
    --browser.gatherUsageStats false \
    --server.fileWatcherType none &
ST_PID=$!

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  Dashboard: http://localhost:8501${NC}"
echo -e "${GREEN}  Gateway:   http://localhost:50051${NC}"
echo -e "${GREEN}  Auth:      http://localhost:8081${NC}"
echo -e "${GREEN}  Audit:     $AUDIT_FILE${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "Press Ctrl+C to stop all services."

wait
