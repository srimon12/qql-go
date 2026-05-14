# QQL Query Patterns

Use these patterns as templates. Keep them short and adapt only what matters.

## Inference note

- `cloud` mode (default) uses Qdrant Cloud inference.
- `local` / `external` mode generates dense and sparse vectors client-side via an OpenAI-compatible embeddings API.
- Rerank is **cloud-only**.

## Dense search

```sql
SEARCH articles SIMILAR TO 'vector database performance tuning' LIMIT 5
```

## Dense search with filter

Create indexes first:

```sql
CREATE INDEX ON COLLECTION articles FOR category TYPE keyword
CREATE INDEX ON COLLECTION articles FOR year TYPE integer
```

Then filter:

```sql
SEARCH articles SIMILAR TO 'transformer inference' LIMIT 10
WHERE category = 'ml' AND year >= 2024
```

## Hybrid search

```sql
SEARCH incidents SIMILAR TO 'out of memory hnsw_ef acorn' LIMIT 10
USING HYBRID
```

Default fusion for `USING HYBRID` is `RRF`.

## Hybrid search with DBSF fusion

```sql
SEARCH incidents SIMILAR TO 'out of memory hnsw_ef acorn' LIMIT 10
USING HYBRID FUSION 'dbsf'
```

## Sparse-only search

```sql
SEARCH incidents SIMILAR TO 'out of memory hnsw_ef acorn' LIMIT 10
USING SPARSE
```

## Sparse-only search plus rerank (cloud only)

```sql
SEARCH incidents SIMILAR TO 'out of memory hnsw_ef acorn' LIMIT 10
USING SPARSE
RERANK
```

## Hybrid search with filter

```sql
CREATE INDEX ON COLLECTION medical_records FOR specialty TYPE keyword
CREATE INDEX ON COLLECTION medical_records FOR priority TYPE keyword
```

```sql
SEARCH medical_records
SIMILAR TO 'acute abdominal pain pancreatitis elevated lipase'
LIMIT 5
USING HYBRID
WHERE specialty = 'gastroenterology' AND priority = 'high'
```

## Exact baseline

```sql
SEARCH articles SIMILAR TO 'attention mechanism' LIMIT 10 EXACT
```

## Query-time tuning

```sql
SEARCH articles SIMILAR TO 'transformer inference' LIMIT 10
WITH { hnsw_ef: 256 }
```

## Rerank

```sql
SEARCH papers SIMILAR TO 'late interaction retrieval' LIMIT 5 RERANK
```

## Hybrid plus rerank (cloud only)

```sql
SEARCH docs SIMILAR TO 'cross encoder rerank retrieval' LIMIT 8
USING HYBRID
RERANK
```

## Recommend

```sql
RECOMMEND FROM articles POSITIVE IDS ('uuid-1', 'uuid-2') LIMIT 5
```

With negative examples and strategy:

```sql
RECOMMEND FROM articles
POSITIVE IDS ('uuid-1', 'uuid-2')
NEGATIVE IDS ('uuid-3')
STRATEGY 'average_vector'
LIMIT 5
```

With pagination and score threshold:

```sql
RECOMMEND FROM articles
POSITIVE IDS ('uuid-1')
LIMIT 10
OFFSET 5
SCORE THRESHOLD 0.5
```

With search params (exact search baseline):

```sql
RECOMMEND FROM articles
POSITIVE IDS ('uuid-1')
LIMIT 5
WITH { exact: true }
```

Cross-collection recommend (lookup IDs from another collection):

```sql
RECOMMEND FROM target_collection
  POSITIVE IDS ('uuid-1')
  LOOKUP FROM source_collection VECTOR 'dense'
  USING 'sparse'
  LIMIT 5
```

With filter:

```sql
RECOMMEND FROM articles
POSITIVE IDS ('uuid-1')
LIMIT 5
WHERE year >= 2024
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
CREATE COLLECTION notes USING MODEL 'sentence-transformers/all-MiniLM-L6-v2'
CREATE COLLECTION notes QUANTIZE SCALAR
CREATE COLLECTION notes QUANTIZE SCALAR QUANTILE 0.95 ALWAYS RAM
CREATE COLLECTION notes HYBRID QUANTIZE BINARY
CREATE COLLECTION notes HYBRID QUANTIZE TURBO BITS 2 ALWAYS RAM
SHOW COLLECTIONS
DROP COLLECTION old_notes
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

## Explain

```powershell
qql-go explain "SEARCH articles SIMILAR TO 'query' LIMIT 5 USING HYBRID WHERE year = 2024"
```

## Agent-safe CLI calls

```powershell
qql-go exec --quiet --json "SHOW COLLECTIONS"
qql-go explain --quiet --json "SEARCH docs SIMILAR TO 'vector db' LIMIT 5 USING HYBRID"
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
SEARCH my_collection SIMILAR TO 'hello' LIMIT 5
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
SEARCH docs SIMILAR TO 'hello world' LIMIT 5 USING HYBRID
```

## Intent Mapping

- semantic similarity -> dense
- exact terms also matter -> `USING HYBRID`
- hybrid retrieval with default fusion -> `USING HYBRID` (`RRF`)
- hybrid retrieval with explicit DBSF fusion -> `USING HYBRID FUSION 'dbsf'`
- keyword-only retrieval -> `USING SPARSE`
- recall debugging -> `EXACT`
- query-time recall tuning -> `WITH { hnsw_ef: ... }`
- filtered recall concern -> `WITH { acorn: true }`
- right docs, wrong order -> `RERANK` (cloud only)
- broader retrieval plus better ordering -> `USING HYBRID RERANK` (cloud only)
- sparse retrieval plus better ordering -> `USING SPARSE RERANK` (cloud only)
- exact point lookup -> `SELECT`
- browse points page by page -> `SCROLL`
- find similar items by example IDs -> `RECOMMEND`
- batch ingest -> `INSERT BULK`
- script round-trip -> `qql-go execute` / `qql-go dump [--batch-size N]`
- interactive shell -> `qql-go repl`
- MMR, feedback, score boosting, discovery -> outside current QQL
