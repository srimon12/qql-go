# QQL Query Examples

Use these examples for complex, advanced retrieval patterns. For basic syntax, see the grammar in `SKILL.md`.

## Manual Prefetch DAGs

Multi-stage retrieval with per-prefetch filters, limits, and score thresholds:

```sql
QUERY 'search' FROM docs LIMIT 10
  PREFETCH (
    QUERY 'search' USING 'dense' LIMIT 100 WHERE category = 'tech' SCORE THRESHOLD 0.8,
    QUERY 'search' USING 'sparse' LIMIT 100 WITH { exact: true }
  )
  FUSION RRF
```

With parameterized RRF tuning:

```sql
QUERY 'search' FROM docs LIMIT 10
  PREFETCH (
    QUERY 'search' USING 'dense' LIMIT 100,
    QUERY 'search' USING 'sparse' LIMIT 100
  )
  FUSION RRF WITH { rrf_k: 10, rrf_weights: [0.7, 0.3] }
```

Nested prefetches:

```sql
QUERY 'search' FROM docs LIMIT 10
  PREFETCH (
    PREFETCH (
      QUERY 123 USING 'dense',
      QUERY 'text' USING 'sparse'
    ),
    QUERY 'fallback' USING 'dense' LIMIT 50
  )
  FUSION RRF
```

## Hybrid Search with Parameterized RRF

```sql
QUERY 'vector search' FROM docs LIMIT 10
USING HYBRID
WITH { rrf_k: 30, rrf_weights: [0.7, 0.3] }
```

## MMR Diversity

```sql
QUERY 'vector database performance tuning' FROM articles LIMIT 5
WITH { mmr_diversity: 0.5, mmr_candidates: 25 }

QUERY 'vector database performance tuning' FROM articles LIMIT 5
USING HYBRID WITH { mmr_diversity: 0.5, mmr_candidates: 25 }
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
QUERY RECOMMEND POSITIVE IDS ('uuid-1') FROM target_collection
  LOOKUP FROM source_collection VECTOR 'dense'
  USING 'sparse'
  LIMIT 5
```

## Grouped Hybrid Search with Query-Time Params

```sql
QUERY 'hnsw recall regression' FROM incidents LIMIT 4
USING HYBRID
WITH { hnsw_ef: 128, acorn: true }
GROUP BY team
GROUP_SIZE 2
```
