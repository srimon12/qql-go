#!/bin/bash

set -euo pipefail

if ! command -v qql-go >/dev/null 2>&1; then
    echo "Error: qql-go must be installed and available on PATH" >&2
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "Error: retrieval-debug-runbook requires jq" >&2
    exit 1
fi

DEMO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARTIFACTS="${QQL_RUNBOOK_ARTIFACTS:-$DEMO_ROOT/artifacts}"
COLLECTION="retrieval_debug_runbook"

rm -rf "$ARTIFACTS"
mkdir -p "$ARTIFACTS"

run_step() {
    local id="$1"
    local command="$2"
    local statement="$3"
    local artifact="$ARTIFACTS/$id.json"

    qql-go "$command" --quiet --json "$statement" > "$artifact"
    if [ "$(jq -r '.ok // false' "$artifact")" != "true" ]; then
        echo "Step '$id' failed" >&2
        cat "$artifact" >&2
        exit 1
    fi
}

echo "Running retrieval debug runbook..."

qql-go doctor --quiet --json > "$ARTIFACTS/01-doctor.json"
qql-go exec --quiet --json "DROP COLLECTION $COLLECTION" > /dev/null 2>&1 || true
qql-go execute --quiet --json "$DEMO_ROOT/01-seed.qql" > "$ARTIFACTS/02-seed.json"

run_step "03-inspect" "exec" "SHOW COLLECTION $COLLECTION"
run_step "04-explain" "explain" "QUERY 'billing policy search regression after index removal' FROM $COLLECTION LIMIT 3 USING HYBRID"
run_step "05-search-hybrid" "exec" "QUERY 'billing policy search regression after index removal' FROM $COLLECTION LIMIT 3 USING HYBRID"
run_step "06-search-exact" "exec" "QUERY 'billing policy search regression after index removal' FROM $COLLECTION LIMIT 3 EXACT"
run_step "07-search-sparse" "exec" "QUERY 'billing policy search regression after index removal' FROM $COLLECTION LIMIT 3 USING SPARSE"
run_step "08-search-filtered" "exec" "QUERY 'billing policy search regression after index removal' FROM $COLLECTION LIMIT 3 USING HYBRID WHERE team = 'billing'"
run_step "08b-search-prefetch" "exec" "QUERY 'billing policy search regression' FROM $COLLECTION LIMIT 3 PREFETCH (QUERY 'billing policy search regression' USING 'dense' LIMIT 10, QUERY 'billing policy search regression' USING 'sparse' LIMIT 10) FUSION RRF"
run_step "09-select-doc" "exec" "SELECT * FROM $COLLECTION WHERE id = 4104"
run_step "10-scroll-runbooks" "exec" "SCROLL FROM $COLLECTION WHERE doc_type = 'runbook' LIMIT 10"

bash "$DEMO_ROOT/validate-artifacts.sh" "$ARTIFACTS"

echo "Workflow complete. Artifacts saved to: $ARTIFACTS"
