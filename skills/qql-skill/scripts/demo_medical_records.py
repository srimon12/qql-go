#!/usr/bin/env python3
from __future__ import annotations

import argparse

from _qql_cli import drop_collection_if_exists, execute_json, print_result

COLLECTION = "medical_records_demo"


BASE_STATEMENTS = [
    # Create a HYBRID collection with payload-aware HNSW and scalar quantization.
    (
        "create-collection",
        f"CREATE COLLECTION {COLLECTION} HYBRID WITH HNSW {{ payload_m: 16 }} QUANTIZE SCALAR QUANTILE 0.99 ALWAYS RAM",
    ),
    # Create payload indexes for filtering
    (
        "index-specialty",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR specialty TYPE keyword",
    ),
    (
        "index-patient",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR patient_id TYPE keyword WITH {{ is_tenant: true, on_disk: true }}",
    ),
    (
        "index-priority",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR priority TYPE keyword",
    ),
    (
        "index-diagnosis",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR diagnosis TYPE text WITH {{ tokenizer: 'word', min_token_len: 2, max_token_len: 20, lowercase: true, phrase_matching: true }}",
    ),
    (
        "index-status",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR status TYPE keyword",
    ),
    (
        "index-year",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR year TYPE integer",
    ),
    # Insert medical records using HYBRID (local or cloud dense + sparse vectorization)
    (
        "insert-stroke",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'id': 414,
  'text': 'Patient presents with sudden right-sided weakness and slurred speech. CT brain confirms left MCA infarct. Thrombolysis initiated within treatment window.',
  'patient_id': 'PT-00414',
  'specialty': 'neurology',
  'priority': 'high',
  'diagnosis': 'Acute ischemic stroke',
  'status': 'admitted',
  'year': 2026
}} USING HYBRID""",
    ),
    (
        "insert-pneumonia",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'id': 415,
  'text': 'Patient with high-grade fever, chills, and productive cough. Chest X-ray shows right lower lobe consolidation. Started on broad-spectrum antibiotics.',
  'patient_id': 'PT-00415',
  'specialty': 'pulmonology',
  'priority': 'medium',
  'diagnosis': 'Community-acquired pneumonia',
  'status': 'reviewed',
  'year': 2025
}} USING HYBRID""",
    ),
    (
        "insert-mi",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'id': 416,
  'text': 'Patient with crushing substernal chest pain radiating to jaw. ECG shows ST depression in leads V4-V6. Troponin I elevated at 2.4 ng/mL.',
  'patient_id': 'PT-00416',
  'specialty': 'cardiology',
  'priority': 'high',
  'diagnosis': 'Non-ST elevation myocardial infarction',
  'status': 'admitted',
  'year': 2026
}} USING HYBRID""",
    ),
    (
        "insert-appendicitis",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'id': 417,
  'text': 'RLQ pain with positive McBurney sign. WBC elevated at 14,500. CT confirms acute appendicitis with no perforation. Patient scheduled for laparoscopic appendectomy.',
  'patient_id': 'PT-00417',
  'specialty': 'surgery',
  'priority': 'medium',
  'diagnosis': 'Acute appendicitis',
  'status': 'preoperative',
  'year': 2024
}} USING HYBRID""",
    ),
    (
        "insert-migraine",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'id': 418,
  'text': 'Severe unilateral headache with photophobia and phonophobia. Previous similar episodes. Patient reports nausea. Migraine without aura.',
  'patient_id': 'PT-00418',
  'specialty': 'neurology',
  'priority': 'low',
  'diagnosis': 'Migraine',
  'status': 'discharged',
  'year': 2023
}} USING HYBRID""",
    ),
    # Basic HYBRID search
    (
        "search-hybrid",
        f"QUERY 'acute stroke weakness slurred speech' FROM {COLLECTION} LIMIT 3 USING HYBRID",
    ),
    # HYBRID search with DBSF fusion
    (
        "search-hybrid-dbsf",
        f"QUERY 'acute stroke weakness slurred speech' FROM {COLLECTION} LIMIT 3 USING HYBRID FUSION DBSF",
    ),
    # Sparse-only search
    (
        "search-sparse",
        f"QUERY 'acute stroke weakness slurred speech' FROM {COLLECTION} LIMIT 3 USING SPARSE",
    ),
    # Exact baseline for comparison
    (
        "search-exact",
        f"QUERY 'acute stroke weakness slurred speech' FROM {COLLECTION} LIMIT 3 EXACT",
    ),
    (
        "search-hybrid-mmr",
        f"QUERY 'acute neurological emergency triage' FROM {COLLECTION} LIMIT 3 USING HYBRID WITH {{ mmr_diversity: 0.5, mmr_candidates: 20 }}",
    ),
    # HYBRID with specialty filter
    (
        "search-neurology",
        f"QUERY 'headache neurological' FROM {COLLECTION} LIMIT 3 USING HYBRID WHERE specialty = 'neurology'",
    ),
    # HYBRID with priority filter (IN)
    (
        "search-high-priority",
        f"QUERY 'chest pain cardiac' FROM {COLLECTION} LIMIT 3 USING HYBRID WHERE priority IN ('high', 'medium')",
    ),
    # HYBRID with status filter
    (
        "search-admitted",
        f"QUERY 'pain' FROM {COLLECTION} LIMIT 3 USING HYBRID WHERE status = 'admitted'",
    ),
    # Combined filters
    (
        "search-combined",
        f"QUERY 'cardiac emergency chest' FROM {COLLECTION} LIMIT 3 USING HYBRID WHERE priority = 'high' AND status = 'admitted'",
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
    # Recommend from an explicit ID
    (
        "recommend-stroke",
        f"QUERY RECOMMEND POSITIVE IDS (414) FROM {COLLECTION} LIMIT 3",
    ),
    (
        "update-stroke-payload",
        f"UPDATE {COLLECTION} SET PAYLOAD WHERE id = 414 {{'status': 'reviewed', 'care_path': 'stroke-alert'}}",
    ),
    (
        "publish-discharged",
        f"UPDATE {COLLECTION} SET PAYLOAD WHERE status = 'discharged' {{'status': 'archived'}}",
    ),
    # Retrieve a point by exact ID
    (
        "select-stroke",
        f"SELECT * FROM {COLLECTION} WHERE id = 414",
    ),
    # Scroll through recent records
    (
        "scroll-recent",
        f"SCROLL FROM {COLLECTION} WHERE year >= 2024 AFTER 414 LIMIT 2",
    ),
    # Delete by filter
    (
        "delete-discharged",
        f"DELETE FROM {COLLECTION} WHERE status = 'discharged'",
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
        description="QQL Medical Records Demo - end-to-end showcase for hybrid retrieval, filtering, quantization, and recommend"
    )
    parser.add_argument(
        "--execute",
        action="store_true",
        help="Run the statements against Qdrant instead of only printing them.",
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
            "create-collection",
            f"CREATE COLLECTION {COLLECTION} HYBRID RERANK QUANTIZE SCALAR QUANTILE 0.99 ALWAYS RAM",
        )
        statements.insert(
            len(statements) - 1,
            (
                "search-hybrid-rerank",
                f"QUERY 'acute stroke weakness slurred speech' FROM {COLLECTION} LIMIT 3 USING HYBRID RERANK",
            ),
        )
        statements.insert(
            len(statements) - 1,
            (
                "search-sparse-rerank",
                f"QUERY 'acute stroke weakness slurred speech' FROM {COLLECTION} LIMIT 3 USING SPARSE RERANK",
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
                print_result(label, result)
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
