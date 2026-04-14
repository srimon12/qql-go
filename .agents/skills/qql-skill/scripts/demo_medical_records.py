#!/usr/bin/env python3
from __future__ import annotations

import argparse

from _qql_cli import execute_json, print_result

COLLECTION = "medical_records_demo"


BASE_STATEMENTS = [
    # Create collection with HYBRID vectors (dense + sparse).
    (
        "create-collection",
        f"CREATE COLLECTION {COLLECTION} HYBRID",
    ),
    # Create payload indexes for filtering
    (
        "index-specialty",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR specialty TYPE keyword",
    ),
    (
        "index-patient",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR patient_id TYPE keyword",
    ),
    (
        "index-priority",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR priority TYPE keyword",
    ),
    (
        "index-diagnosis",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR diagnosis TYPE keyword",
    ),
    (
        "index-status",
        f"CREATE INDEX ON COLLECTION {COLLECTION} FOR status TYPE keyword",
    ),
    # Insert medical records using HYBRID (server-side inference for dense + sparse)
    (
        "insert-stroke",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'text': 'Patient presents with sudden right-sided weakness and slurred speech. CT brain confirms left MCA infarct. Thrombolysis initiated within treatment window.',
  'patient_id': 'PT-00414',
  'specialty': 'neurology',
  'priority': 'high',
  'diagnosis': 'Acute ischemic stroke',
  'status': 'admitted'
}} USING HYBRID""",
    ),
    (
        "insert-pneumonia",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'text': 'Patient with high-grade fever, chills, and productive cough. Chest X-ray shows right lower lobe consolidation. Started on broad-spectrum antibiotics.',
  'patient_id': 'PT-00415',
  'specialty': 'pulmonology',
  'priority': 'medium',
  'diagnosis': 'Community-acquired pneumonia',
  'status': 'reviewed'
}} USING HYBRID""",
    ),
    (
        "insert-mi",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'text': 'Patient with crushing substernal chest pain radiating to jaw. ECG shows ST depression in leads V4-V6. Troponin I elevated at 2.4 ng/mL.',
  'patient_id': 'PT-00416',
  'specialty': 'cardiology',
  'priority': 'high',
  'diagnosis': 'Non-ST elevation myocardial infarction',
  'status': 'admitted'
}} USING HYBRID""",
    ),
    (
        "insert-appendicitis",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'text': 'RLQ pain with positive McBurney sign. WBC elevated at 14,500. CT confirms acute appendicitis with no perforation. Patient scheduled for laparoscopic appendectomy.',
  'patient_id': 'PT-00417',
  'specialty': 'surgery',
  'priority': 'medium',
  'diagnosis': 'Acute appendicitis',
  'status': 'preoperative'
}} USING HYBRID""",
    ),
    (
        "insert-migraine",
        f"""INSERT INTO COLLECTION {COLLECTION} VALUES {{
  'text': 'Severe unilateral headache with photophobia and phonophobia. Previous similar episodes. Patient reports nausea. Migraine without aura.',
  'patient_id': 'PT-00418',
  'specialty': 'neurology',
  'priority': 'low',
  'diagnosis': 'Migraine',
  'status': 'discharged'
}} USING HYBRID""",
    ),
    # Basic HYBRID search
    (
        "search-hybrid",
        f"SEARCH {COLLECTION} SIMILAR TO 'acute stroke weakness slurred speech' LIMIT 3 USING HYBRID",
    ),
    # HYBRID with specialty filter
    (
        "search-neurology",
        f"SEARCH {COLLECTION} SIMILAR TO 'headache neurological' LIMIT 3 WHERE specialty = 'neurology'",
    ),
    # HYBRID with priority filter (IN)
    (
        "search-high-priority",
        f"SEARCH {COLLECTION} SIMILAR TO 'chest pain cardiac' LIMIT 3 WHERE priority IN ('high', 'medium')",
    ),
    # HYBRID with status filter
    (
        "search-admitted",
        f"SEARCH {COLLECTION} SIMILAR TO 'pain' LIMIT 3 WHERE status = 'admitted'",
    ),
    # Combined filters
    (
        "search-combined",
        f"SEARCH {COLLECTION} SIMILAR TO 'cardiac emergency chest' LIMIT 3 WHERE priority = 'high' AND status = 'admitted'",
    ),
    # SHOW COLLECTIONS
    (
        "show-collections",
        "SHOW COLLECTIONS",
    ),
]


def main() -> None:
    parser = argparse.ArgumentParser(
        description="QQL Medical Records Demo - end-to-end showcase with filtering"
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
            f"CREATE COLLECTION {COLLECTION} HYBRID RERANK",
        )
        statements.insert(
            len(statements) - 1,
            (
                "search-hybrid-rerank",
                f"SEARCH {COLLECTION} SIMILAR TO 'acute stroke weakness slurred speech' LIMIT 3 USING HYBRID RERANK",
            ),
        )

    try:
        if args.execute:
            try:
                execute_json(f"DROP COLLECTION {COLLECTION}")
            except Exception:
                pass

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
