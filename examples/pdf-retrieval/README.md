# PDF Retrieval with Multivectors

Scalable PDF retrieval using ColBERT/ColPali-style multivector representations with Qdrant.

## Problem

ColPali generates ~1,000 vectors per PDF page. ColQwen generates ~700. Indexing all of them with HNSW is expensive:

- High RAM usage
- Slow insert times
- Wasted compute for reranking workloads

## Solution

Two-stage retrieval:

1. **First stage**: Retrieve with mean-pooled vectors (32 vectors per page, HNSW indexed)
2. **Second stage**: Rerank with original multivectors (no HNSW, used only for MaxSim scoring)

## QQL Syntax

```sql
-- Create collection: indexed mean-pooled + unindexed original
CREATE COLLECTION pdf_retrieval (
    original VECTOR(128, COSINE) WITH MULTIVECTOR (comparator = 'max_sim') WITH HNSW (m = 0),
    mean_pooling_columns VECTOR(128, COSINE) WITH MULTIVECTOR (comparator = 'max_sim'),
    mean_pooling_rows VECTOR(128, COSINE) WITH MULTIVECTOR (comparator = 'max_sim')
)

-- Insert with named vectors
INSERT INTO pdf_retrieval VALUES {
    'id': 1,
    'title': 'PDF Page 1',
    'vector': {
        'original': [[0.1, 0.2, 0.3], [0.4, 0.5, 0.6]],
        'mean_pooling_columns': [[0.1, 0.2, 0.3]],
        'mean_pooling_rows': [[0.4, 0.5, 0.6]]
    }
}

-- Two-stage retrieval
WITH
    _pf0 AS (QUERY [0.1, 0.2, 0.3] USING 'mean_pooling_columns' LIMIT 100),
    _pf1 AS (QUERY [0.1, 0.2, 0.3] USING 'mean_pooling_rows' LIMIT 100)
QUERY [0.1, 0.2, 0.3] FROM pdf_retrieval USING 'original' LIMIT 10
    PREFETCH (_pf0, _pf1)
```

## Run

```bash
bash examples/pdf-retrieval/run-demo.sh
```

## Key Concepts

| Concept | QQL |
|---------|-----|
| Disable HNSW for reranking | `WITH HNSW (m = 0)` |
| Multivector comparator | `WITH MULTIVECTOR (comparator = 'max_sim')` |
| Named vector in query | `USING 'vector_name'` |
| Prefetch with different vectors | CTE with `USING 'vector_name'` per prefetch |
| Insert named vectors | `'vector': {'name': [...]}` in VALUES |

## When To Use

- ColBERT / ColPali / ColQwen embeddings
- PDF page retrieval at scale
- Any workload where token-level vectors are expensive to index
