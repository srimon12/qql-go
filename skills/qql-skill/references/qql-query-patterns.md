# QQL Query Patterns

Use these patterns as templates. Keep them short and adapt only what matters.

Inference note:

- In the current Go CLI, text `INSERT` and text `SEARCH ... SIMILAR TO ...` are cloud inference paths.
- Use a Qdrant Cloud connection for the retrieval patterns below.

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

## Hybrid plus rerank

```sql
SEARCH docs SIMILAR TO 'cross encoder rerank retrieval' LIMIT 8
USING HYBRID
RERANK
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

## Collection operations

```sql
CREATE COLLECTION notes
CREATE COLLECTION notes HYBRID
SHOW COLLECTIONS
DROP COLLECTION old_notes
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
```

Use these forms for scripts and agents so output is structured and compact.

## Self-hosted-safe command patterns (no text inference)

```sql
SHOW COLLECTIONS
CREATE COLLECTION docs
CREATE COLLECTION docs HYBRID
CREATE INDEX ON COLLECTION docs FOR category TYPE keyword
DELETE FROM docs WHERE category = 'archived'
DROP COLLECTION docs
```

## Intent Mapping

- semantic similarity -> dense
- exact terms also matter -> `USING HYBRID`
- recall debugging -> `EXACT`
- query-time recall tuning -> `WITH { hnsw_ef: ... }`
- filtered recall concern -> `WITH { acorn: true }`
- right docs, wrong order -> `RERANK`
- broader retrieval plus better ordering -> `USING HYBRID RERANK`
- recommend, MMR, feedback, score boosting -> outside current QQL
