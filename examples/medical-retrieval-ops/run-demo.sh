#!/bin/bash

set -euo pipefail

if ! command -v qql-go >/dev/null 2>&1; then
    echo "Error: qql-go must be installed and available on PATH" >&2
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "Error: medical-retrieval-ops requires jq" >&2
    exit 1
fi

if ! command -v uv >/dev/null 2>&1; then
    echo "Error: medical-retrieval-ops requires uv" >&2
    exit 1
fi

DEMO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARTIFACTS="${MEDICAL_RAG_ARTIFACTS:-$DEMO_ROOT/artifacts}"
GENERATED_DIR="${MEDICAL_RAG_GENERATED_DIR:-$DEMO_ROOT/generated}"
COLLECTION="medical_retrieval_ops"

rm -rf "$ARTIFACTS" "$GENERATED_DIR"
mkdir -p "$ARTIFACTS" "$GENERATED_DIR"

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

echo "Building full medical benchmark corpus..."
MEDICAL_RAG_GENERATED_DIR="$GENERATED_DIR" MEDICAL_RAG_MAX_ROWS="${MEDICAL_RAG_MAX_ROWS:-all}" uv run "$DEMO_ROOT/build-medical-corpus.py" > "$ARTIFACTS/00-build.json"

MAIN_QUERY="$(jq -r '.queries.main.question' "$GENERATED_DIR/eval.json" | sed "s/'/\\\\'/g")"
MAIN_ID="$(jq -r '.queries.main.id' "$GENERATED_DIR/eval.json")"
MAIN_SPECIALTY="$(jq -r '.queries.main.specialty' "$GENERATED_DIR/eval.json" | sed "s/'/\\\\'/g")"
MAIN_TENANT="$(jq -r '.queries.main.tenant_id' "$GENERATED_DIR/eval.json" | sed "s/'/\\\\'/g")"
MAIN_PRIORITY="$(jq -r '.queries.main.case_priority' "$GENERATED_DIR/eval.json" | sed "s/'/\\\\'/g")"
MAIN_STATUS="$(jq -r '.queries.main.case_status' "$GENERATED_DIR/eval.json" | sed "s/'/\\\\'/g")"
RELATED_ID="$(jq -r '.queries.related.id' "$GENERATED_DIR/eval.json")"

echo "Running medical retrieval operations..."

qql-go doctor --quiet --json > "$ARTIFACTS/01-doctor.json"
qql-go exec --quiet --json "DROP COLLECTION $COLLECTION" > /dev/null 2>&1 || true
qql-go execute --quiet --json "$DEMO_ROOT/01-provision.qql" > "$ARTIFACTS/02-provision.json"
qql-go execute --quiet --json "$GENERATED_DIR/02-seed.qql" > "$ARTIFACTS/03-seed.json"

run_step "04-inspect" "exec" "SHOW COLLECTION $COLLECTION"
run_step "05-explain-hybrid-rrf" "explain" "SEARCH $COLLECTION SIMILAR TO '$MAIN_QUERY' LIMIT 5 USING HYBRID"
run_step "06-search-dense" "exec" "SEARCH $COLLECTION SIMILAR TO '$MAIN_QUERY' LIMIT 5"
run_step "07-search-sparse" "exec" "SEARCH $COLLECTION SIMILAR TO '$MAIN_QUERY' LIMIT 5 USING SPARSE"
run_step "08-search-hybrid-rrf" "exec" "SEARCH $COLLECTION SIMILAR TO '$MAIN_QUERY' LIMIT 5 USING HYBRID"
run_step "09-search-hybrid-dbsf" "exec" "SEARCH $COLLECTION SIMILAR TO '$MAIN_QUERY' LIMIT 5 USING HYBRID FUSION 'dbsf'"
run_step "10-search-exact" "exec" "SEARCH $COLLECTION SIMILAR TO '$MAIN_QUERY' LIMIT 5 EXACT"
run_step "11-search-filtered-tenant" "exec" "SEARCH $COLLECTION SIMILAR TO '$MAIN_QUERY' LIMIT 5 WHERE tenant_id = '$MAIN_TENANT' AND case_status = '$MAIN_STATUS' AND case_priority = '$MAIN_PRIORITY' WITH { acorn: true }"
run_step "12-search-score-threshold" "exec" "SEARCH $COLLECTION SIMILAR TO '$MAIN_QUERY' LIMIT 5 SCORE THRESHOLD 0.0 USING HYBRID"
run_step "13-search-offset-window" "exec" "SEARCH $COLLECTION SIMILAR TO '$MAIN_QUERY' LIMIT 5 OFFSET 1 USING HYBRID"
run_step "14-search-grouped-specialty" "exec" "SEARCH $COLLECTION SIMILAR TO '$MAIN_QUERY' LIMIT 6 SCORE THRESHOLD 0.0 USING HYBRID GROUP BY specialty GROUP_SIZE 2"
run_step "15-search-dense-mmr" "exec" "SEARCH $COLLECTION SIMILAR TO '$MAIN_QUERY' LIMIT 5 WITH { mmr_diversity: 0.5, mmr_candidates: 20 }"
run_step "16-select-main" "exec" "SELECT * FROM $COLLECTION WHERE id = $MAIN_ID"
run_step "17-recommend-related" "exec" "RECOMMEND FROM $COLLECTION POSITIVE IDS ($RELATED_ID) LIMIT 5"
run_step "18-scroll-tenant" "exec" "SCROLL FROM $COLLECTION WHERE tenant_id = '$MAIN_TENANT' LIMIT 5"
qql-go dump --quiet --json "$COLLECTION" "$ARTIFACTS/backup.qql" > "$ARTIFACTS/19-dump.json"
uv run "$DEMO_ROOT/run-benchmark.py" "$GENERATED_DIR/benchmark-questions.json" > "$ARTIFACTS/20-benchmark.json"

bash "$DEMO_ROOT/validate-artifacts.sh" "$GENERATED_DIR/eval.json" "$ARTIFACTS"

echo "Workflow complete. Artifacts saved to: $ARTIFACTS"
