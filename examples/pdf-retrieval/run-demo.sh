#!/usr/bin/env bash
set -euo pipefail
# pdf-retrieval/run-demo.sh
# Demonstrates scalable PDF retrieval with ColBERT/ColPali multivectors.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ARTIFACTS="$SCRIPT_DIR/artifacts"
mkdir -p "$ARTIFACTS"

COLLECTION="pdf_retrieval_demo"

echo "=== PDF Retrieval with Multivectors ==="
echo ""

# ── Cleanup ─────────────────────────────────────────────────────
qql-go exec "DROP COLLECTION $COLLECTION" 2>/dev/null || true

# ── 1. Create collection ───────────────────────────────────────
echo "[1] Creating collection with 3 multivector vectors..."
qql-go exec "CREATE COLLECTION $COLLECTION (
    original VECTOR(128, COSINE) WITH MULTIVECTOR (comparator = 'max_sim') WITH HNSW (m = 0),
    mean_pooling_columns VECTOR(128, COSINE) WITH MULTIVECTOR (comparator = 'max_sim'),
    mean_pooling_rows VECTOR(128, COSINE) WITH MULTIVECTOR (comparator = 'max_sim')
)"
echo ""

# ── 2. Create indexes ──────────────────────────────────────────
echo "[2] Creating payload indexes..."
qql-go exec "CREATE INDEX ON COLLECTION $COLLECTION FOR title TYPE text WITH (tokenizer = 'word', min_token_len = 2, lowercase = true)"
qql-go exec "CREATE INDEX ON COLLECTION $COLLECTION FOR page_number TYPE integer"
echo ""

# ── 3. Insert pages with named vectors ─────────────────────────
echo "[3] Inserting PDF pages with multivector representations..."
for i in 1 2 3 4 5; do
    qql-go exec "INSERT INTO $COLLECTION VALUES {
        'id': $i,
        'title': 'PDF Page $i',
        'page_number': $i,
        'vector': {
            'original': [[0.$i, 0.$((i+1)), 0.$((i+2))], [0.$((i+3)), 0.$((i+4)), 0.$((i+5))]],
            'mean_pooling_columns': [[0.$i, 0.$((i+1)), 0.$((i+2))]],
            'mean_pooling_rows': [[0.$((i+3)), 0.$((i+4)), 0.$((i+5))]]
        }
    }"
done
echo ""

# ── 4. Two-stage retrieval ─────────────────────────────────────
echo "[4] Two-stage retrieval: prefetch with mean-pooled, rerank with original..."
PLAN=$(qql-go explain "WITH
    _pf0 AS (QUERY [0.1, 0.2, 0.3] USING 'mean_pooling_columns' LIMIT 100),
    _pf1 AS (QUERY [0.1, 0.2, 0.3] USING 'mean_pooling_rows' LIMIT 100)
QUERY [0.1, 0.2, 0.3] FROM $COLLECTION USING 'original' LIMIT 5 PREFETCH (_pf0, _pf1)")
echo "$PLAN"
echo "$PLAN" > "$ARTIFACTS/explain-pdf-retrieval.txt"
echo ""

# ── 5. Filtered retrieval ──────────────────────────────────────
echo "[5] Filtered retrieval (page >= 3)..."
PLAN=$(qql-go explain "WITH
    _pf0 AS (QUERY [0.1, 0.2, 0.3] USING 'mean_pooling_columns' LIMIT 50 WHERE page_number >= 3)
QUERY [0.1, 0.2, 0.3] FROM $COLLECTION USING 'original' LIMIT 3 PREFETCH (_pf0)")
echo "$PLAN"
echo "$PLAN" > "$ARTIFACTS/explain-filtered-retrieval.txt"
echo ""

# ── 6. Show collection ─────────────────────────────────────────
echo "[6] Collection details..."
qql-go exec "SHOW COLLECTION $COLLECTION" > "$ARTIFACTS/collection-info.json"
qql-go exec --quiet --json "SHOW COLLECTION $COLLECTION" | python3 -m json.tool 2>/dev/null || qql-go exec "SHOW COLLECTION $COLLECTION"
echo ""

# ── 7. Convert example ─────────────────────────────────────────
echo "[7] REST JSON → QQL conversion..."
echo '{"collection_name":"'$COLLECTION'","vectors":{"original":{"size":128,"distance":"Cosine","multivector_config":{"comparator":"max_sim"},"hnsw_config":{"m":0}},"mean_pooling_columns":{"size":128,"distance":"Cosine","multivector_config":{"comparator":"max_sim"}}}}' | qql-go convert --quiet
echo ""

# ── Cleanup ─────────────────────────────────────────────────────
echo "[cleanup] Dropping collection..."
qql-go exec "DROP COLLECTION $COLLECTION"
echo ""
echo "=== Done. Artifacts in $ARTIFACTS ==="
