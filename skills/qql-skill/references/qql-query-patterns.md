# QQL Query Patterns

Use these patterns as templates. Keep them short and adapt only what matters.

## Inference note

- `cloud` mode (default) uses Qdrant Cloud inference.
- `local` / `external` mode generates dense and sparse vectors client-side via an OpenAI-compatible embeddings API.
- Rerank is **cloud-only**.

## Dense search

```sql
QUERY 'vector database performance tuning' FROM articles LIMIT 5
```

## MMR diversity

```sql
QUERY 'vector database performance tuning' FROM articles LIMIT 5
WITH { mmr_diversity: 0.5, mmr_candidates: 25 }

QUERY 'vector database performance tuning' FROM articles LIMIT 5
USING HYBRID WITH { mmr_diversity: 0.5, mmr_candidates: 25 }
```

## Dense search with filter

Create indexes first:

```sql
CREATE INDEX ON COLLECTION articles FOR category TYPE keyword
CREATE INDEX ON COLLECTION articles FOR year TYPE integer
```

Then filter:

```sql
QUERY 'transformer inference' FROM articles LIMIT 10
WHERE category = 'ml' AND year >= 2024
```

## Search pagination and score threshold

```sql
QUERY 'refund policy' FROM articles LIMIT 10
OFFSET 20
SCORE THRESHOLD 0.8
```

`OFFSET` is for flat search results. Do not combine it with `GROUP BY`.

## Cross-collection lookup

```sql
QUERY 'personalized refund policy' FROM articles LIMIT 5
LOOKUP FROM user_profiles VECTOR 'preferences'
USING HYBRID
```

## Hybrid search

```sql
QUERY 'out of memory hnsw_ef acorn' FROM incidents LIMIT 10
USING HYBRID
```

Default fusion for `USING HYBRID` is `RRF`.

## Hybrid search with DBSF fusion

```sql
QUERY 'out of memory hnsw_ef acorn' FROM incidents LIMIT 10
USING HYBRID FUSION DBSF
```

## Parameterized RRF

```sql
QUERY 'vector search' FROM docs LIMIT 10
USING HYBRID
WITH { rrf_k: 30, rrf_weights: [0.7, 0.3] }
```

## Manual prefetch DAGs

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

## Sparse-only search

```sql
QUERY 'out of memory hnsw_ef acorn' FROM incidents LIMIT 10
USING SPARSE
```

## Sparse-only search plus rerank (cloud only)

```sql
QUERY 'out of memory hnsw_ef acorn' FROM incidents LIMIT 10
USING SPARSE
RERANK
```

## Hybrid search with filter

```sql
CREATE INDEX ON COLLECTION medical_records FOR specialty TYPE keyword
CREATE INDEX ON COLLECTION medical_records FOR priority TYPE keyword
```

```sql
QUERY 'acute abdominal pain pancreatitis elevated lipase' FROM medical_records
LIMIT 5
USING HYBRID
WHERE specialty = 'gastroenterology' AND priority = 'high'
```

## Grouped search

```sql
QUERY 'vector database' FROM articles LIMIT 5
GROUP BY category
GROUP_SIZE 2
```

Grouped search can use `SCORE THRESHOLD`, but not `OFFSET`.

## Grouped hybrid search with query-time params

```sql
QUERY 'hnsw recall regression' FROM incidents LIMIT 4
USING HYBRID
WITH { hnsw_ef: 128, acorn: true }
GROUP BY team
GROUP_SIZE 2
```

## Exact baseline

```sql
QUERY 'attention mechanism' FROM articles LIMIT 10 EXACT
```

## Query-time tuning

```sql
QUERY 'transformer inference' FROM articles LIMIT 10
WITH { hnsw_ef: 256 }
```

## Tenant-aware indexing and payload HNSW

```sql
CREATE COLLECTION tenant_docs HYBRID WITH HNSW { payload_m: 16 }
CREATE INDEX ON COLLECTION tenant_docs FOR tenant_id TYPE keyword WITH { is_tenant: true, on_disk: true }
```

## Full-text index tuning

```sql
CREATE INDEX ON COLLECTION tenant_docs FOR title TYPE text
WITH { tokenizer: 'word', min_token_len: 2, max_token_len: 20, lowercase: true, phrase_matching: true }
```

## Rerank

```sql
QUERY 'late interaction retrieval' FROM papers LIMIT 5 RERANK
```

## Hybrid plus rerank (cloud only)

```sql
QUERY 'cross encoder rerank retrieval' FROM docs LIMIT 8
USING HYBRID
RERANK
```

## Recommend

```sql
QUERY RECOMMEND POSITIVE IDS ('uuid-1', 'uuid-2') FROM articles LIMIT 5
```

With negative examples and strategy:

```sql
QUERY RECOMMEND POSITIVE IDS ('uuid-1', 'uuid-2') NEGATIVE IDS ('uuid-3')
STRATEGY 'average_vector'
FROM articles LIMIT 5
```

With pagination and score threshold:

```sql
QUERY RECOMMEND POSITIVE IDS ('uuid-1') FROM articles LIMIT 10
OFFSET 5
SCORE THRESHOLD 0.5
```

With search params (exact search baseline):

```sql
QUERY RECOMMEND POSITIVE IDS ('uuid-1') FROM articles LIMIT 5
WITH { exact: true }
```

Cross-collection recommend (lookup IDs from another collection):

```sql
QUERY RECOMMEND POSITIVE IDS ('uuid-1') FROM target_collection
  LOOKUP FROM source_collection VECTOR 'dense'
  USING 'sparse'
  LIMIT 5
```

With filter:

```sql
QUERY RECOMMEND POSITIVE IDS ('uuid-1') FROM articles LIMIT 5
WHERE year >= 2024
```

## Context search

```sql
QUERY CONTEXT PAIRS (('uuid-1', 'uuid-2'), ('uuid-3', 'uuid-4')) FROM docs LIMIT 10
```

## Discover search

```sql
QUERY DISCOVER TARGET 'uuid-1' CONTEXT PAIRS (('uuid-2', 'uuid-3')) FROM docs LIMIT 10
```

## Insert

Dense-only:

```sql
INSERT INTO COLLECTION notes VALUES {
  'text': 'Qdrant uses HNSW for approximate nearest neighbor search',
  'topic': 'retrieval',
  'year': 2026
}
```

Hybrid:

```sql
INSERT INTO COLLECTION notes VALUES {
  'text': 'ACORN improves filtered ANN recall',
  'topic': 'retrieval'
} USING HYBRID
```

With explicit ID:

```sql
INSERT INTO COLLECTION notes VALUES {
  'id': '123e4567-e89b-12d3-a456-426614174000',
  'text': 'Explicit ID document',
  'topic': 'retrieval'
}
```

## Bulk insert

```sql
INSERT BULK INTO COLLECTION notes VALUES [
  {'text': 'First document', 'topic': 'ml'},
  {'text': 'Second document', 'topic': 'search'}
] USING HYBRID
```

## Collection operations

```sql
CREATE COLLECTION notes
CREATE COLLECTION notes HYBRID
CREATE COLLECTION notes HYBRID WITH HNSW { payload_m: 16 }
CREATE COLLECTION notes USING MODEL 'sentence-transformers/all-MiniLM-L6-v2'
CREATE COLLECTION notes QUANTIZE SCALAR
CREATE COLLECTION notes QUANTIZE SCALAR QUANTILE 0.95 ALWAYS RAM
CREATE COLLECTION notes HYBRID QUANTIZE BINARY
CREATE COLLECTION notes HYBRID QUANTIZE TURBO BITS 2 ALWAYS RAM
CREATE COLLECTION notes (my_dense VECTOR(384, COSINE), my_sparse SPARSE)
SHOW COLLECTIONS
SHOW COLLECTION notes
DROP COLLECTION old_notes
ALTER COLLECTION notes WITH VECTORS { on_disk: true }
ALTER COLLECTION notes QUANTIZE DISABLED
```

## Select by ID

```sql
SELECT * FROM notes WHERE id = '123e4567-e89b-12d3-a456-426614174000'
SELECT * FROM notes WHERE id = 42
```

## Scroll through points

```sql
SCROLL FROM notes LIMIT 25
SCROLL FROM notes WHERE category = 'retrieval' LIMIT 25
SCROLL FROM notes AFTER '123e4567-e89b-12d3-a456-426614174000' LIMIT 25
```

## Delete

```sql
DELETE FROM notes WHERE id = '123e4567-e89b-12d3-a456-426614174000'
DELETE FROM notes WHERE category = 'archived'
```

## Update vector

```sql
UPDATE notes SET VECTOR WHERE id = 42 [0.1, 0.2, 0.3]
```

## Update payload

```sql
UPDATE notes SET PAYLOAD WHERE id = 42 {'topic': 'retrieval', 'status': 'reviewed'}
UPDATE notes SET PAYLOAD WHERE category = 'draft' {'status': 'published'}
```

## Explain

```powershell
qql-go explain "QUERY 'vector db' FROM docs LIMIT 5 USING HYBRID WHERE year = 2024"
```

## Agent-safe CLI calls

```powershell
qql-go exec --quiet --json "SHOW COLLECTIONS"
qql-go exec --quiet --json "SHOW COLLECTION docs"
qql-go explain --quiet --json "QUERY 'vector db' FROM docs LIMIT 5 USING HYBRID"
qql-go doctor --quiet --json
qql-go connect --quiet --json --url https://<cluster>.qdrant.io --secret <api-key>
qql-go disconnect --quiet --json
qql-go version --quiet --json
qql-go execute --quiet --json script.qql
qql-go execute --stop-on-error --quiet --json script.qql
qql-go dump --quiet --json notes backup.qql
qql-go dump --quiet --json --batch-size 200 notes backup.qql
qql-go repl
```

Use these forms for scripts and agents so output is structured and compact.

### Script File Format

Script files (`.qql`) use **newline-delimited statements WITHOUT semicolons**:

```sql
-- QQL script example
CREATE COLLECTION my_collection
CREATE INDEX ON COLLECTION my_collection FOR category TYPE keyword
INSERT INTO COLLECTION my_collection VALUES {'text': 'hello world', 'category': 'greeting'}
QUERY 'hello' FROM my_collection LIMIT 5
DROP COLLECTION my_collection
```

## Self-hosted/local mode setup

Connect with local inference:

```powershell
qql-go connect `
  --url http://localhost:6334 `
  --inference-mode local `
  --embedding-endpoint http://127.0.0.1:1234/v1/embeddings `
  --embedding-key <embedding-api-key> `
  --embedding-model text-embedding-all-minilm-l6-v2-embedding `
  --embedding-dimension 384
```

Then use all text operations normally:

```sql
CREATE COLLECTION docs HYBRID
INSERT INTO COLLECTION docs VALUES {'text': 'hello world'} USING HYBRID
QUERY 'hello world' FROM docs LIMIT 5 USING HYBRID
```

## Intent Mapping

- semantic similarity -> `QUERY '<text>' FROM <collection>`
- exact terms also matter -> `USING HYBRID`
- hybrid retrieval with default fusion -> `USING HYBRID` (`RRF`)
- hybrid retrieval with explicit DBSF fusion -> `USING HYBRID FUSION DBSF`
- hybrid retrieval with tuned RRF -> `USING HYBRID WITH { rrf_k: ..., rrf_weights: [...] }`
- multi-stage retrieval -> `PREFETCH (...) FUSION RRF`
- keyword-only retrieval -> `USING SPARSE`
- query by point ID -> `QUERY <id> FROM <collection>`
- recommendation by example -> `QUERY RECOMMEND POSITIVE IDS (...)`
- context-aware search -> `QUERY CONTEXT PAIRS (...)`
- exploration search -> `QUERY DISCOVER TARGET ... CONTEXT PAIRS (...)`
- recall debugging -> `EXACT`
- query-time recall tuning -> `WITH { hnsw_ef: ... }`
- filtered recall concern -> `WITH { acorn: true }`
- semantically diverse dense or hybrid results -> `WITH { mmr_diversity: ..., mmr_candidates: ... }`
- right docs, wrong order -> `RERANK` (cloud only)
- broader retrieval plus better ordering -> `USING HYBRID RERANK` (cloud only)
- sparse retrieval plus better ordering -> `USING SPARSE RERANK` (cloud only)
- grouped top results by field -> `GROUP BY <field> [GROUP_SIZE <n>]`
- exact point lookup -> `SELECT`
- browse points page by page -> `SCROLL`
- batch ingest -> `INSERT BULK`
- patch a stored payload -> `UPDATE ... SET PAYLOAD ...`
- replace a stored vector -> `UPDATE ... SET VECTOR ...`
- script round-trip -> `qql-go execute` / `qql-go dump [--batch-size N]`
- interactive shell -> `qql-go repl`
- formula, score boosting, relevance feedback, ORDER BY -> outside current QQL
