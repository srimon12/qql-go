#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json


EXAMPLES = [
    {
        "mode": "dense",
        "when": "Use when semantic similarity matters more than exact term matching.",
        "query": "QUERY 'vector database performance tuning' FROM articles LIMIT 5",
        "setup": [],
        "requires_index": [],
    },
    {
        "mode": "hybrid",
        "when": "Use when exact terms, acronyms, model names, or error strings matter.",
        "query": (
            "QUERY 'out of memory hnsw_ef acorn' FROM incidents "
            "LIMIT 10 USING HYBRID"
        ),
        "setup": [],
        "requires_index": [],
    },
    {
        "mode": "hybrid-dbsf",
        "when": "Use when you want hybrid retrieval with DBSF fusion instead of the default RRF.",
        "query": (
            "QUERY 'out of memory hnsw_ef acorn' FROM incidents "
            "LIMIT 10 USING HYBRID FUSION DBSF"
        ),
        "setup": [],
        "requires_index": [],
    },
    {
        "mode": "sparse",
        "when": "Use when keyword or BM25 retrieval matters more than semantic similarity.",
        "query": (
            "QUERY 'out of memory hnsw_ef acorn' FROM incidents "
            "LIMIT 10 USING SPARSE"
        ),
        "setup": [],
        "requires_index": [],
    },
    {
        "mode": "exact",
        "when": "Use when debugging recall and you need an exact KNN baseline.",
        "query": "QUERY 'attention mechanism' FROM articles LIMIT 10 EXACT",
        "setup": [],
        "requires_index": [],
    },
    {
        "mode": "with-hnsw-ef",
        "when": "Use when you want query-time recall tuning without changing collection config.",
        "query": (
            "QUERY 'transformer inference' FROM articles "
            "LIMIT 10 WITH { hnsw_ef: 256 }"
        ),
        "setup": [],
        "requires_index": [],
    },
    {
        "mode": "hybrid-mmr",
        "when": "Use when hybrid search results are too redundant and you want semantic diversity on the dense leg before fusion.",
        "query": (
            "QUERY 'vector database performance tuning' FROM articles "
            "LIMIT 10 USING HYBRID WITH { mmr_diversity: 0.5, mmr_candidates: 25 }"
        ),
        "setup": [],
        "requires_index": [],
    },
    {
        "mode": "with-filter",
        "when": "Use when metadata constraints should narrow the search. Requires CREATE INDEX first.",
        "setup": [
            "CREATE INDEX ON COLLECTION articles FOR category TYPE keyword",
        ],
        "query": (
            "QUERY 'transformer inference' FROM articles "
            "LIMIT 10 WHERE category = 'ml'"
        ),
        "requires_index": ["category"],
    },
    {
        "mode": "with-acorn",
        "when": "Use when filtered-query recall is the focus and ACORN should be tested.",
        "query": (
            "QUERY 'retrieval recall regression' FROM incidents "
            "LIMIT 10 WHERE team = 'search' WITH { acorn: true }"
        ),
        "setup": [
            "CREATE INDEX ON COLLECTION incidents FOR team TYPE keyword",
        ],
        "requires_index": ["team"],
    },
    {
        "mode": "tenant-aware-indexing",
        "when": "Use when a filter field acts like a tenant boundary and Qdrant should optimize for that grouping.",
        "query": (
            "QUERY 'stroke discharge summary' FROM tenant_docs "
            "LIMIT 5 WHERE tenant_id = 'tenant-a'"
        ),
        "setup": [
            "CREATE COLLECTION tenant_docs HYBRID WITH HNSW { payload_m: 16 }",
            "CREATE INDEX ON COLLECTION tenant_docs FOR tenant_id TYPE keyword WITH { is_tenant: true, on_disk: true }",
        ],
        "requires_index": ["tenant_id"],
    },
    {
        "mode": "text-index-tuning",
        "when": "Use when a text payload field needs explicit tokenization controls before phrase or keyword-heavy filtering.",
        "query": (
            "CREATE INDEX ON COLLECTION tenant_docs FOR title TYPE text "
            "WITH { tokenizer: 'word', min_token_len: 2, max_token_len: 20, lowercase: true, phrase_matching: true }"
        ),
        "setup": [],
        "requires_index": [],
    },
    {
        "mode": "grouped",
        "when": "Use when results should be grouped by a payload field instead of returned as one flat list.",
        "query": (
            "QUERY 'retrieval recall regression' FROM incidents "
            "LIMIT 5 GROUP BY team GROUP_SIZE 2"
        ),
        "setup": [
            "CREATE INDEX ON COLLECTION incidents FOR team TYPE keyword",
        ],
        "requires_index": ["team"],
    },
    {
        "mode": "grouped-hybrid",
        "when": "Use when grouped results still need hybrid recall and query-time tuning.",
        "query": (
            "QUERY 'retrieval recall regression' FROM incidents "
            "LIMIT 4 USING HYBRID WITH { hnsw_ef: 128, acorn: true } "
            "GROUP BY team GROUP_SIZE 2"
        ),
        "setup": [
            "CREATE INDEX ON COLLECTION incidents FOR team TYPE keyword",
        ],
        "requires_index": ["team"],
    },
    {
        "mode": "rerank",
        "when": "Use when recall is likely good but top-result ordering needs help. Requires Qdrant Cloud and a rerank-capable collection.",
        "query": (
            "QUERY 'late interaction retrieval' FROM papers LIMIT 5 RERANK"
        ),
        "setup": [],
        "requires_index": [],
        "requires_cloud": True,
    },
    {
        "mode": "hybrid-rerank",
        "when": "Use when both keyword recall and top-rank precision matter. Requires Qdrant Cloud and a rerank-capable collection.",
        "query": (
            "QUERY 'cross encoder ms marco minimlm' FROM docs "
            "LIMIT 8 USING HYBRID RERANK"
        ),
        "setup": [],
        "requires_index": [],
        "requires_cloud": True,
    },
    {
        "mode": "sparse-rerank",
        "when": "Use when sparse recall is strong but the top ordering still needs rerank. Requires Qdrant Cloud and a rerank-capable collection.",
        "query": (
            "QUERY 'cross encoder ms marco minimlm' FROM docs "
            "LIMIT 8 USING SPARSE RERANK"
        ),
        "setup": [],
        "requires_index": [],
        "requires_cloud": True,
    },
    {
        "mode": "select-by-id",
        "when": "Use when you already know the exact point ID and want the stored payload.",
        "query": "SELECT * FROM articles WHERE id = 'pt-42'",
        "setup": [],
        "requires_index": [],
    },
    {
        "mode": "scroll",
        "when": "Use when you need to page through a collection or walk filtered points.",
        "query": (
            "SCROLL FROM articles WHERE category = 'ml' AFTER 'pt-42' LIMIT 25"
        ),
        "setup": [
            "CREATE INDEX ON COLLECTION articles FOR category TYPE keyword",
        ],
        "requires_index": ["category"],
    },
    {
        "mode": "delete-by-field",
        "when": "Delete points by field value instead of ID.",
        "query": ("DELETE FROM articles WHERE category = 'archived'"),
        "setup": [
            "CREATE INDEX ON COLLECTION articles FOR category TYPE keyword",
        ],
        "requires_index": ["category"],
    },
    {
        "mode": "update-payload",
        "when": "Patch stored metadata in place for one point or a filtered subset.",
        "query": (
            "UPDATE articles SET PAYLOAD WHERE category = 'draft' "
            "{'status': 'published'}"
        ),
        "setup": [
            "CREATE INDEX ON COLLECTION articles FOR category TYPE keyword",
        ],
        "requires_index": ["category"],
    },
    {
        "mode": "update-vector",
        "when": "Replace the stored dense vector for one exact point ID.",
        "query": "UPDATE articles SET VECTOR WHERE id = 42 [0.1, 0.2, 0.3]",
        "setup": [],
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
        for setup_stmt in example.get("setup", []):
            print(f"  Setup: {setup_stmt}")
        if example.get("requires_index"):
            print(f"  Note: Requires index on {example['requires_index']}")
        if example.get("requires_cloud"):
            print(f"  Note: Requires Qdrant Cloud and a rerank-capable collection")
        print(example["query"])
        print()


if __name__ == "__main__":
    main()
