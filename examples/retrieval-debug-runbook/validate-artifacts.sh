#!/bin/bash

set -euo pipefail

ARTIFACTS="${1:?artifact dir required}"

if ! command -v jq >/dev/null 2>&1; then
    echo "validate-artifacts.sh requires jq" >&2
    exit 1
fi

assert_jq() {
    local file="$1"
    local expr="$2"
    local message="$3"
    if ! jq -e "$expr" "$file" >/dev/null; then
        echo "Assertion failed: $message" >&2
        echo "  file: $file" >&2
        exit 1
    fi
}

assert_jq "$ARTIFACTS/01-doctor.json" '.ok == true and .healthy == true' "doctor should report a healthy connection"
assert_jq "$ARTIFACTS/03-inspect.json" '.ok == true and .data.topology == "hybrid"' "collection should be hybrid"
assert_jq "$ARTIFACTS/03-inspect.json" '.data.payload_schema.team.type == "keyword" and .data.payload_schema.title.type == "text"' "payload indexes should include team and title"
assert_jq "$ARTIFACTS/04-explain.json" '.ok == true and (.plan | contains("QUERY NEAREST"))' "explain plan should use QUERY NEAREST syntax"
assert_jq "$ARTIFACTS/05-search-hybrid.json" '.ok == true and .data[0].id != null' "hybrid search should return results"
assert_jq "$ARTIFACTS/06-search-exact.json" '.ok == true and .data[0].id != null' "exact search should return results"
assert_jq "$ARTIFACTS/07-search-sparse.json" '.ok == true and .data[0].id != null' "sparse search should return results"
assert_jq "$ARTIFACTS/08-search-filtered.json" '.ok == true and .data[0].id != null' "filtered billing search should return results"
assert_jq "$ARTIFACTS/09-select-doc.json" '.ok == true' "expected document should be fetchable by ID"
assert_jq "$ARTIFACTS/10-scroll-runbooks.json" '.ok == true and (.data.points | length) >= 1' "runbook slice should be scrollable"

echo "Validated retrieval debug runbook artifacts in $ARTIFACTS"
