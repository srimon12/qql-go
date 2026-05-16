#!/bin/bash

set -euo pipefail

SUITE_PATH="${1:?suite path required}"
ARTIFACTS="${2:?artifact dir required}"

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

collection_expect="$(jq -c '.collection_expect // {}' "$SUITE_PATH")"

topology="$(echo "$collection_expect" | jq -r '.topology // empty')"
if [ -n "$topology" ]; then
    assert_jq "$ARTIFACTS/02-inspect.json" ".ok == true and .data.topology == \"$topology\"" "collection topology should stay $topology"
fi

min_points="$(echo "$collection_expect" | jq -r '.min_points // empty')"
if [ -n "$min_points" ]; then
    assert_jq "$ARTIFACTS/02-inspect.json" ".ok == true and (.data.points_count // 0) >= $min_points" "collection should have at least $min_points points"
fi

while IFS= read -r field; do
    assert_jq "$ARTIFACTS/02-inspect.json" ".data.payload_schema[\"$field\"] != null" "payload index '$field' should exist"
done < <(echo "$collection_expect" | jq -r '.payload_indexes[]?')

while IFS= read -r check; do
    id="$(echo "$check" | jq -r '.id')"
    artifact="$ARTIFACTS/$id.json"

    assert_jq "$artifact" '.ok == true' "step '$id' should succeed"

    while IFS= read -r snippet; do
        assert_jq "$artifact" ".plan | contains(\"$snippet\")" "step '$id' plan should contain '$snippet'"
    done < <(echo "$check" | jq -r '.expect.plan_contains[]?')

    min_results="$(echo "$check" | jq -r '.expect.min_results // empty')"
    if [ -n "$min_results" ]; then
        assert_jq "$artifact" "(.data.count // 0) >= $min_results" "step '$id' should return at least $min_results results"
    fi

    hybrid="$(echo "$check" | jq -r '.expect.hybrid // empty')"
    if [ -n "$hybrid" ]; then
        assert_jq "$artifact" ".data.hybrid == $hybrid" "step '$id' hybrid flag should be $hybrid"
    fi

    group_by="$(echo "$check" | jq -r '.expect.group_by // empty')"
    if [ -n "$group_by" ]; then
        assert_jq "$artifact" ".data.group_by == \"$group_by\"" "step '$id' should stay grouped by '$group_by'"
    fi

    first_group="$(echo "$check" | jq -r '.expect.first_group // empty')"
    if [ -n "$first_group" ]; then
        assert_jq "$artifact" ".data.groups[0].group_id == \"$first_group\"" "step '$id' should keep group '$first_group' first"
    fi

    while IFS= read -r pair; do
        idx="${pair%%:*}"
        expected="${pair#*:}"
        assert_jq "$artifact" ".data.results[$idx].id == \"$expected\"" "step '$id' result[$idx] should be '$expected'"
    done < <(echo "$check" | jq -r '.expect.top_ids // [] | to_entries[] | "\(.key):\(.value)"')
done < <(jq -c '.checks[]' "$SUITE_PATH")

echo "Validated retrieval regression artifacts in $ARTIFACTS"
