#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json


EXAMPLES = [
    {
        "mode": "dense",
        "when": "Use when semantic similarity matters more than exact term matching.",
        "query": "SEARCH articles SIMILAR TO 'vector database performance tuning' LIMIT 5",
        "requires_index": [],
    },
    {
        "mode": "hybrid",
        "when": "Use when exact terms, acronyms, model names, or error strings matter.",
        "query": (
            "SEARCH incidents SIMILAR TO 'out of memory hnsw_ef acorn' "
            "LIMIT 10 USING HYBRID"
        ),
        "requires_index": [],
    },
    {
        "mode": "exact",
        "when": "Use when debugging recall and you need an exact KNN baseline.",
        "query": "SEARCH articles SIMILAR TO 'attention mechanism' LIMIT 10 EXACT",
        "requires_index": [],
    },
    {
        "mode": "with-hnsw-ef",
        "when": "Use when you want query-time recall tuning without changing collection config.",
        "query": (
            "SEARCH articles SIMILAR TO 'transformer inference' "
            "LIMIT 10 WITH { hnsw_ef: 256 }"
        ),
        "requires_index": [],
    },
    {
        "mode": "with-filter",
        "when": "Use when metadata constraints should narrow the search. Requires CREATE INDEX first.",
        "query": (
            "CREATE INDEX ON COLLECTION articles FOR category TYPE keyword;\n"
            "SEARCH articles SIMILAR TO 'transformer inference' "
            "LIMIT 10 WHERE category = 'ml'"
        ),
        "requires_index": ["category"],
    },
    {
        "mode": "with-acorn",
        "when": "Use when filtered-query recall is the focus and ACORN should be tested.",
        "query": (
            "SEARCH incidents SIMILAR TO 'retrieval recall regression' "
            "LIMIT 10 WHERE team = 'search' WITH { acorn: true }"
        ),
        "requires_index": ["team"],
    },
    {
        "mode": "rerank",
        "when": "Use when recall is likely good but top-result ordering needs help. Requires Qdrant Cloud and a rerank-capable collection.",
        "query": (
            "SEARCH papers SIMILAR TO 'late interaction retrieval' LIMIT 5 RERANK"
        ),
        "requires_index": [],
        "requires_cloud": True,
    },
    {
        "mode": "hybrid-rerank",
        "when": "Use when both keyword recall and top-rank precision matter. Requires Qdrant Cloud and a rerank-capable collection.",
        "query": (
            "SEARCH docs SIMILAR TO 'cross encoder ms marco minimlm' "
            "LIMIT 8 USING HYBRID RERANK"
        ),
        "requires_index": [],
        "requires_cloud": True,
    },
    {
        "mode": "delete-by-field",
        "when": "Delete points by field value instead of ID.",
        "query": ("DELETE FROM articles WHERE category = 'archived'"),
        "requires_index": ["category"],
    },
]


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Print compact QQL retrieval examples."
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit the examples as JSON.",
    )
    parser.add_argument(
        "--execute",
        action="store_true",
        help="Reserved for parity with other demos; this script prints queries only.",
    )
    args = parser.parse_args()

    if args.json:
        print(json.dumps(EXAMPLES, indent=2))
        return

    for example in EXAMPLES:
        print(f"[{example['mode']}]")
        print(example["when"])
        if example.get("requires_index"):
            print(f"  Note: Requires index on {example['requires_index']}")
        if example.get("requires_cloud"):
            print(f"  Note: Requires Qdrant Cloud and a rerank-capable collection")
        print(example["query"])
        print()


if __name__ == "__main__":
    main()
