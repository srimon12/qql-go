# QQL Query Examples

Use these examples for complex, advanced retrieval patterns. For basic syntax, see the grammar in `SKILL.md`.

## Multi-Stage Retrieval with CTEs

Define named sub-queries and reference them in the main query:

```sql
WITH
  dense_stage AS (QUERY 'search' USING dense LIMIT 100 WHERE category = 'tech' SCORE THRESHOLD 0.8),
  sparse_stage AS (QUERY 'search' USING sparse LIMIT 100 WITH (exact = true))
QUERY 'search' FROM docs LIMIT 10 PREFETCH (dense_stage, sparse_stage) FUSION RRF
```

Nested CTE (one CTE references another):

```sql
WITH
  inner AS (QUERY 'deep search' USING dense LIMIT 50),
  outer AS (QUERY 'fallback' USING sparse LIMIT 100 PREFETCH (inner))
QUERY 'search' FROM docs LIMIT 10 PREFETCH (outer) FUSION RRF
```

With RRF tuning:

```sql
WITH
  a AS (QUERY 'search' USING dense LIMIT 100),
  b AS (QUERY 'search' USING sparse LIMIT 100)
QUERY 'search' FROM docs LIMIT 10 PREFETCH (a, b) FUSION RRF WITH (rrf_k = 10, rrf_weights = [0.7, 0.3])
```

## Hybrid Search with Parametrized RRF

```sql
QUERY 'vector search' FROM docs LIMIT 10
USING HYBRID
WITH (rrf_k = 30, rrf_weights = [0.7, 0.3])
```

## MMR Diversity

```sql
QUERY 'vector database performance tuning' FROM articles LIMIT 5
WITH (mmr_diversity = 0.5, mmr_candidates = 25)

QUERY 'vector database performance tuning' FROM articles LIMIT 5
USING HYBRID WITH (mmr_diversity = 0.5, mmr_candidates = 25)
```

## Context Search

```sql
QUERY CONTEXT PAIRS (('uuid-1', 'uuid-2'), ('uuid-3', 'uuid-4')) FROM docs LIMIT 10
```

## Discover Search

```sql
QUERY DISCOVER TARGET 'uuid-1' CONTEXT PAIRS (('uuid-2', 'uuid-3')) FROM docs LIMIT 10
```

## Cross-collection Recommend (Lookup)

```sql
QUERY RECOMMEND WITH (positive = ('uuid-1')) FROM target_collection
  LOOKUP FROM source_collection VECTOR 'dense'
  USING sparse
  LIMIT 5
```

## Query-time Params

```sql
QUERY 'search' FROM docs LIMIT 10 WITH (hnsw_ef = 128, exact = true, acorn = true)
```

## Grouped Retrieval

```sql
QUERY 'search' FROM docs LIMIT 10 USING HYBRID GROUP BY category GROUP_SIZE 3
```

## Pagination without similarity score (ORDER BY)

```sql
-- Retrieve the 10 most recently published articles from the tech category,
-- sorted chronologically (newest first) rather than by semantic similarity.
-- Useful for standard list views where exact ordering is strictly required.
QUERY ORDER BY published_at DESC FROM articles 
  WHERE category = 'tech' AND status = 'published'
  LIMIT 10 OFFSET 20
```

## Field and Vector selection (WITH PAYLOAD / WITH VECTORS)

```sql
-- Execute a semantic search on medical records, but heavily restrict the returned
-- payload to only what's necessary for the UI (title, summary, url).
-- We explicitly exclude heavy fields to drastically reduce network transfer latency.
-- We also explicitly request the 'dense_v2' vector back for downstream local processing.
QUERY 'acute bronchitis treatment' FROM medical_records 
  USING HYBRID 
  WHERE specialty = 'pulmonology'
  LIMIT 10
  WITH PAYLOAD (include = ['title', 'summary', 'url'], exclude = ['raw_text', 'embedding'])
  WITH VECTORS ('dense_v2')
```
