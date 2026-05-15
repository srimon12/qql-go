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
        "mode": "hybrid-dbsf",
        "when": "Use when you want hybrid retrieval with DBSF fusion instead of the default RRF.",
        "query": (
            "SEARCH incidents SIMILAR TO 'out of memory hnsw_ef acorn' "
            "LIMIT 10 USING HYBRID FUSION 'dbsf'"
        ),
        "requires_index": [],
    },
    {
        "mode": "sparse",
        "when": "Use when keyword or BM25 retrieval matters more than semantic similarity.",
        "query": (
            "SEARCH incidents SIMILAR TO 'out of memory hnsw_ef acorn' "
            "LIMIT 10 USING SPARSE"
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
        "mode": "grouped",
        "when": "Use when results should be grouped by a payload field instead of returned as one flat list.",
        "query": (
            "SEARCH incidents SIMILAR TO 'retrieval recall regression' "
            "LIMIT 5 GROUP BY team GROUP_SIZE 2"
        ),
        "requires_index": ["team"],
    },
    {
        "mode": "grouped-hybrid",
        "when": "Use when grouped results still need hybrid recall and query-time tuning.",
        "query": (
            "SEARCH incidents SIMILAR TO 'retrieval recall regression' "
            "LIMIT 4 USING HYBRID WITH { hnsw_ef: 128, acorn: true } "
            "GROUP BY team GROUP_SIZE 2"
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
        "mode": "sparse-rerank",
        "when": "Use when sparse recall is strong but the top ordering still needs rerank. Requires Qdrant Cloud and a rerank-capable collection.",
        "query": (
            "SEARCH docs SIMILAR TO 'cross encoder ms marco minimlm' "
            "LIMIT 8 USING SPARSE RERANK"
        ),
        "requires_index": [],
        "requires_cloud": True,
    },
    {
        "mode": "select-by-id",
        "when": "Use when you already know the exact point ID and want the stored payload.",
        "query": "SELECT * FROM articles WHERE id = 'pt-42'",
        "requires_index": [],
    },
    {
        "mode": "scroll",
        "when": "Use when you need to page through a collection or walk filtered points.",
        "query": (
            "SCROLL FROM articles WHERE category = 'ml' AFTER 'pt-42' LIMIT 25"
        ),
        "requires_index": ["category"],
    },
    {
        "mode": "delete-by-field",
        "when": "Delete points by field value instead of ID.",
        "query": ("DELETE FROM articles WHERE category = 'archived'"),
        "requires_index": ["category"],
    },
    {
        "mode": "update-payload",
        "when": "Patch stored metadata in place for one point or a filtered subset.",
        "query": (
            "UPDATE articles SET PAYLOAD WHERE category = 'draft' "
            "{'status': 'published'}"
        ),
        "requires_index": ["category"],
    },
    {
        "mode": "update-vector",
        "when": "Replace the stored dense vector for one exact point ID.",
        "query": "UPDATE articles SET VECTOR WHERE id = 42 [0.1, 0.2, 0.3]",
        "requires_index": [],
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
