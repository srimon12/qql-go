#!/usr/bin/env python3
from __future__ import annotations

import argparse
from _qql_cli import drop_collection_if_exists, execute_json, print_result
COLLECTION = "qql_skill_demo_kitchen_sink"

STROKE_ID = "123e4567-e89b-12d3-a456-426614174001"
STEMI_ID = "123e4567-e89b-12d3-a456-426614174002"
PNEUMONIA_ID = "123e4567-e89b-12d3-a456-426614174003"
HEADACHE_ID = "123e4567-e89b-12d3-a456-426614174004"


BASE_STATEMENTS = [
    # Create collection with HYBRID vectors, payload-aware HNSW, and TURBO quantization.
    (
        "create-hybrid",
        f"CREATE COLLECTION {COLLECTION} HYBRID WITH HNSW {{ payload_m: 16 }} QUANTIZE TURBO BITS 2 ALWAYS RAM",
    ),
    # Create payload indexes for filtering and tenant-style grouping.
    (
        "index-specialty",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR specialty TYPE keyword",
    ),
    (
        "index-priority",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR priority TYPE keyword",
    ),
    (
        "index-status",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR status TYPE keyword",
    ),
    (
        "index-year",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR year TYPE integer",
    ),
    (
        "index-patient",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR patient_id TYPE keyword WITH {{ is_tenant: true, on_disk: true }}",
    ),
    (
        "index-diagnosis-text",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR diagnosis TYPE text WITH {{ tokenizer: 'word', min_token_len: 2, max_token_len: 20, lowercase: true, phrase_matching: true }}",
    ),
    # Insert medical records using HYBRID (auto dense + sparse vectorization)
    (
        "insert-stroke",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'id': '{STROKE_ID}',
  'text': 'Patient presents with sudden right-sided weakness and slurred speech. CT confirms left MCA infarct. Thrombolysis initiated within the treatment window.',
  'patient_id': 'PT-1001',
  'specialty': 'neurology',
  'priority': 'high',
  'diagnosis': 'Acute ischemic stroke',
  'status': 'admitted',
  'year': 2026,
  'score': 9.7
}} USING HYBRID""",
    ),
    (
        "insert-stemi",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'id': '{STEMI_ID}',
  'text': 'Patient with crushing chest pain radiating to the left arm. ECG shows ST elevation and troponin is elevated.',
  'patient_id': 'PT-1002',
  'specialty': 'cardiology',
  'priority': 'high',
  'diagnosis': 'STEMI',
  'status': 'admitted',
  'year': 2025,
  'score': 9.2
}} USING HYBRID""",
    ),
    (
        "insert-pneumonia",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'id': '{PNEUMONIA_ID}',
  'text': 'Patient has high-grade fever, productive cough, and right lower lobe consolidation on chest X-ray. Started on IV antibiotics.',
  'patient_id': 'PT-1003',
  'specialty': 'pulmonology',
  'priority': 'medium',
  'diagnosis': 'Community-acquired pneumonia',
  'status': 'reviewed',
  'year': 2024,
  'score': 7.1
}} USING HYBRID""",
    ),
    (
        "insert-headache",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'id': '{HEADACHE_ID}',
  'text': 'Mild tension headache improved with rest and hydration. No focal neurological deficits observed.',
  'patient_id': 'PT-1004',
  'specialty': 'general-medicine',
  'priority': 'low',
  'diagnosis': 'Tension headache',
  'status': 'draft',
  'year': 2023,
  'score': 4.5
}} USING HYBRID""",
    ),
    # Dense vector search (bypasses HNSW, scans exact)
    (
        "search-dense-exact",
        f"QUERY 'acute stroke weakness slurred speech' FROM {COLLECTION} LIMIT 3 EXACT",
    ),
    (
        "search-hybrid-mmr",
        f"QUERY 'acute neurological emergency triage' FROM {COLLECTION} LIMIT 3 USING HYBRID WITH {{ mmr_diversity: 0.5, mmr_candidates: 20 }}",
    ),
    # HYBRID search (dense + sparse fusion)
    (
        "search-hybrid",
        f"QUERY 'stroke thrombolysis ICU' FROM {COLLECTION} LIMIT 3 USING HYBRID",
    ),
    (
        "search-hybrid-dbsf",
        f"QUERY 'stroke thrombolysis ICU' FROM {COLLECTION} LIMIT 3 USING HYBRID FUSION DBSF",
    ),
    (
        "search-sparse",
        f"QUERY 'chest pain radiating arm troponin' FROM {COLLECTION} LIMIT 3 USING SPARSE",
    ),
    # HYBRID with WHERE filter (equality)
    (
        "filter-equality",
        f"QUERY 'stroke' FROM {COLLECTION} LIMIT 3 USING HYBRID WHERE specialty = 'neurology'",
    ),
    # HYBRID with WHERE filter (IN operator)
    (
        "filter-in",
        f"QUERY 'emergency cardiac chest pain' FROM {COLLECTION} LIMIT 3 USING HYBRID WHERE priority IN ('high', 'medium')",
    ),
    # HYBRID with WHERE filter (BETWEEN range)
    (
        "filter-range",
        f"QUERY 'medical' FROM {COLLECTION} LIMIT 3 USING HYBRID WHERE year BETWEEN 2024 AND 2026",
    ),
    # HYBRID with multiple filters (AND)
    (
        "filter-and",
        f"QUERY 'cardiac emergency' FROM {COLLECTION} LIMIT 3 USING HYBRID WHERE priority = 'high' AND status = 'admitted'",
    ),
    # HYBRID with compound filter (OR)
    (
        "filter-or",
        f"QUERY 'brain scan' FROM {COLLECTION} LIMIT 3 USING HYBRID WHERE specialty = 'neurology' OR specialty = 'radiology'",
    ),
    # HYBRID with NOT filter
    (
        "filter-not",
        f"QUERY 'infection' FROM {COLLECTION} LIMIT 3 USING HYBRID WHERE status NOT IN ('draft')",
    ),
    # Combined: HYBRID + WHERE + multiple conditions
    (
        "filter-combined",
        f"QUERY 'cardiac chest pain' FROM {COLLECTION} LIMIT 3 USING HYBRID WHERE priority IN ('high', 'medium') AND status = 'admitted' AND year >= 2024",
    ),
    (
        "group-by-specialty",
        f"QUERY 'acute neurological emergency' FROM {COLLECTION} LIMIT 3 USING HYBRID GROUP BY specialty GROUP_SIZE 2",
    ),
    (
        "grouped-hybrid-mmr",
        f"QUERY 'acute neurological emergency' FROM {COLLECTION} LIMIT 3 USING HYBRID WITH {{ mmr_diversity: 0.35, mmr_candidates: 20 }} GROUP BY specialty GROUP_SIZE 2",
    ),
    (
        "group-by-priority-with-params",
        f"QUERY 'critical care escalation' FROM {COLLECTION} LIMIT 3 USING HYBRID WITH {{ hnsw_ef: 128, acorn: true }} GROUP BY priority GROUP_SIZE 2",
    ),
    (
        "recommend-stroke",
        f"QUERY RECOMMEND POSITIVE IDS ('{STROKE_ID}') FROM {COLLECTION} LIMIT 3",
    ),
    (
        "update-stroke-payload",
        f"UPDATE {COLLECTION} SET PAYLOAD WHERE id = '{STROKE_ID}' {{'status': 'reviewed', 'care_path': 'stroke-alert'}}",
    ),
    (
        "publish-drafts",
        f"UPDATE {COLLECTION} SET PAYLOAD WHERE status = 'draft' {{'status': 'reviewed'}}",
    ),
    (
        "select-stemi",
        f"SELECT * FROM {COLLECTION} WHERE id = '{STEMI_ID}'",
    ),
    (
        "scroll-high-priority",
        f"SCROLL FROM {COLLECTION} WHERE priority = 'high' AFTER '{STROKE_ID}' LIMIT 2",
    ),
    # SHOW COLLECTIONS
    (
        "show-collections",
        "SHOW COLLECTIONS",
    ),
    (
        "show-collection",
        f"SHOW COLLECTION {COLLECTION}",
    ),
]


def main() -> None:
    parser = argparse.ArgumentParser(
        description="QQL Kitchen Sink Demo - showcases all QQL features"
    )
    parser.add_argument(
        "--execute",
        action="store_true",
        help="Run the statements against Qdrant instead of printing them.",
    )
    parser.add_argument(
        "--keep",
        action="store_true",
        help="Keep the demo collection instead of dropping it at the end.",
    )
    parser.add_argument(
        "--rerank",
        action="store_true",
        help="Include the rerank showcase with a rerank-capable collection.",
    )
    args = parser.parse_args()

    statements = list(BASE_STATEMENTS)
    if args.rerank:
        statements[0] = (
            "create-hybrid",
            f"CREATE COLLECTION {COLLECTION} HYBRID RERANK QUANTIZE TURBO BITS 2 ALWAYS RAM",
        )
        statements.insert(
            len(statements) - 1,
            (
                "search-hybrid-rerank",
                f"QUERY 'stroke thrombolysis ICU' FROM {COLLECTION} LIMIT 3 USING HYBRID RERANK",
            ),
        )
        statements.insert(
            len(statements) - 1,
            (
                "search-sparse-rerank",
                f"QUERY 'chest pain radiating arm troponin' FROM {COLLECTION} LIMIT 3 USING SPARSE RERANK",
            ),
        )

    try:
        if args.execute:
            drop_collection_if_exists(COLLECTION)

        for label, statement in statements:
            print(f"[{label}]")
            print(statement)
            print()

            if not args.execute:
                continue

            try:
                result = execute_json(statement)
                print_result(label, result, limit=3)
            except Exception as exc:
                print(f"  ERROR: {exc}")
                print()

    finally:
        if args.execute and not args.keep:
            try:
                result = execute_json(f"DROP COLLECTION {COLLECTION}")
                print(f"[cleanup]\n{result.message}\n")
            except Exception as exc:
                print(f"[cleanup]\ncleanup failed: {exc}\n")


if __name__ == "__main__":
    main()
