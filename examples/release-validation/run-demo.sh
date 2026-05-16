#!/bin/bash

set -euo pipefail

if ! command -v qql-go >/dev/null 2>&1; then
    echo "Error: qql-go must be installed and available on PATH" >&2
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "Error: release-validation requires jq" >&2
    exit 1
fi

DEMO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARTIFACTS="${QQL_REGRESSION_ARTIFACTS:-$DEMO_ROOT/artifacts}"
SUITE_PATH="${QQL_REGRESSION_SUITE:-$DEMO_ROOT/regression-suite.json}"

rm -rf "$ARTIFACTS"
mkdir -p "$ARTIFACTS"

connect_if_requested() {
    if [ -z "${QDRANT_URL:-}" ]; then
        return 0
    fi

    local args=(connect --quiet --json --url "$QDRANT_URL")
    if [ -n "${QDRANT_API_KEY:-}" ]; then
        args+=(--secret "$QDRANT_API_KEY")
    fi
    if [ -n "${EMBEDDING_ENDPOINT:-}" ]; then
        args+=(--inference-mode external --embedding-endpoint "$EMBEDDING_ENDPOINT")
        if [ -n "${EMBEDDING_API_KEY:-}" ]; then
            args+=(--embedding-key "$EMBEDDING_API_KEY")
        fi
        if [ -z "${EMBEDDING_MODEL:-}" ]; then
            echo "Error: EMBEDDING_MODEL is required when EMBEDDING_ENDPOINT is set" >&2
            exit 1
        fi
        args+=(--embedding-model "$EMBEDDING_MODEL")
    fi

    qql-go "${args[@]}" > "$ARTIFACTS/00-connect.json"
}

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

if [ ! -f "$SUITE_PATH" ]; then
    echo "Error: Could not find regression suite at $SUITE_PATH" >&2
    exit 1
fi

echo "Running retrieval regression validation..."

connect_if_requested
qql-go doctor --quiet --json > "$ARTIFACTS/01-doctor.json"

COLLECTION="$(jq -r '.collection' "$SUITE_PATH")"
if [ -z "$COLLECTION" ] || [ "$COLLECTION" = "null" ]; then
    echo "Error: suite must define a collection" >&2
    exit 1
fi

run_step "02-inspect" "exec" "SHOW COLLECTION $COLLECTION"

while IFS= read -r check; do
    id="$(echo "$check" | jq -r '.id')"
    command="$(echo "$check" | jq -r '.command // "exec"')"
    statement="$(echo "$check" | jq -r '.statement')"
    run_step "$id" "$command" "$statement"
done < <(jq -c '.checks[]' "$SUITE_PATH")

bash "$DEMO_ROOT/validate-artifacts.sh" "$SUITE_PATH" "$ARTIFACTS"

echo "Workflow complete. Artifacts saved to: $ARTIFACTS"
