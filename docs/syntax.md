# QQL Syntax Reference

Complete statement reference for the QQL query language.

## Collection Management

```sql
-- Create
CREATE COLLECTION <name>
CREATE COLLECTION <name> HYBRID
CREATE COLLECTION <name> HYBRID RERANK
CREATE COLLECTION <name> USING MODEL '<model>'
CREATE COLLECTION <name> (name VECTOR(size, DISTANCE), ...)

-- Config
CREATE COLLECTION <name> WITH HNSW (m = 32, ef_construct = 100)
CREATE COLLECTION <name> WITH QUANTIZATION (type = 'scalar', quantile = 0.95)
CREATE COLLECTION <name> WITH QUANTIZATION (type = 'turbo', bits = 2, always_ram = true)
CREATE COLLECTION <name> WITH QUANTIZATION (type = 'binary', always_ram = true)
CREATE COLLECTION <name> WITH QUANTIZATION (type = 'product')

-- Per-vector config
CREATE COLLECTION docs (
  dense VECTOR(384, COSINE) WITH QUANTIZATION (type = 'scalar'),
  sparse VECTOR(768, DOT)
)

-- Alter
ALTER COLLECTION <name> WITH VECTORS (on_disk = true)
ALTER COLLECTION <name> WITH HNSW (m = 32)
ALTER COLLECTION <name> WITH OPTIMIZERS (max_segment_size = 500000)
ALTER COLLECTION <name> WITH PARAMS (replication_factor = 3)
ALTER COLLECTION <name> WITH QUANTIZATION (type = 'scalar')
ALTER COLLECTION <name> WITH QUANTIZATION (disabled = true)

-- Drop / Show
DROP COLLECTION <name>
SHOW COLLECTIONS
SHOW COLLECTION <name>
```

## Indexes

```sql
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE keyword
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE integer
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE float
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE bool
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE uuid
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE datetime
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE geo

-- With options
CREATE INDEX ON docs FOR tags TYPE keyword WITH (is_tenant = true, on_disk = true, enable_hnsw = false)
CREATE INDEX ON docs FOR content TYPE text WITH (tokenizer = 'word', min_token_len = 2, max_token_len = 20, lowercase = true, phrase_matching = true, stopwords = ['en'])
```

## Insert

```sql
INSERT INTO <name> VALUES {'id': 1, 'text': 'hello', 'field': 'value'}
INSERT INTO <name> VALUES {'id': 1, 'text': 'hello'}, {'id': 2, 'text': 'world'}
INSERT INTO <name> VALUES {'id': 1, 'text': 'hello'} USING HYBRID
INSERT INTO <name> VALUES {'id': 1, 'text': 'hello'} USING MODEL '<model>'
INSERT INTO <name> VALUES {'id': 1, 'text': 'hello'} USING HYBRID DENSE MODEL '<m1>' SPARSE MODEL '<m2>'
```

The `id` field is required. It must be an unsigned integer or UUID string.

The `text` field is required for auto-vectorization.

## QUERY

The unified query statement with multiple modes:

### NEAREST (default)

```sql
QUERY '<text>' FROM <collection> LIMIT <n>
QUERY '<text>' FROM <collection> LIMIT <n> OFFSET <n>
QUERY '<text>' FROM <collection> LIMIT <n> SCORE THRESHOLD 0.5
QUERY '<text>' FROM <collection> LIMIT <n> LOOKUP FROM <other> [VECTOR '<name>']
QUERY '<text>' FROM <collection> LIMIT <n> USING MODEL '<model>'
```

### Hybrid

```sql
QUERY '<text>' FROM <collection> LIMIT <n> USING HYBRID
QUERY '<text>' FROM <collection> LIMIT <n> USING HYBRID FUSION DBSF
QUERY '<text>' FROM <collection> LIMIT <n> USING HYBRID WITH (rrf_k = 30, rrf_weights = [0.7, 0.3])
```

### Sparse

```sql
QUERY '<text>' FROM <collection> LIMIT <n> USING SPARSE
QUERY '<text>' FROM <collection> LIMIT <n> USING DENSE
```

### Recommend

```sql
QUERY RECOMMEND WITH (positive = ('id-1', 'id-2')) FROM <collection> LIMIT <n>
QUERY RECOMMEND WITH (positive = ('id-1'), negative = ('id-3')) FROM <collection> LIMIT <n>
QUERY RECOMMEND WITH (positive = ('id-1')) STRATEGY 'best_score' FROM <collection> LIMIT <n>
```

Strategies: `average_vector`, `best_score`, `sum_scores`.

### Context and Discover

```sql
QUERY CONTEXT PAIRS (('id-1', 'id-2'), ('id-3', 'id-4')) FROM <collection> LIMIT <n>
QUERY DISCOVER TARGET 'id-1' CONTEXT PAIRS (('id-2', 'id-3')) FROM <collection> LIMIT <n>
```

### ORDER BY

```sql
QUERY ORDER BY <field> ASC FROM <collection> LIMIT <n>
QUERY ORDER BY <field> DESC FROM <collection> LIMIT <n>
```

### SAMPLE

```sql
QUERY SAMPLE FROM <collection> LIMIT <n>
QUERY SAMPLE FROM <collection> LIMIT <n> WHERE <filter>
```

### CTEs and Prefetch DAGs

```sql
WITH
  dense AS (QUERY 'search' USING dense LIMIT 200 WHERE category = 'tech'),
  sparse AS (QUERY 'search' USING sparse LIMIT 300)
QUERY 'search' FROM <collection> LIMIT 10
  PREFETCH (
    dense WHERE priority = 'high' SCORE THRESHOLD 0.6,
    sparse SCORE THRESHOLD 0.3
  )
  FUSION RRF WITH (rrf_k = 20, rrf_weights = [0.6, 0.4])
```

CTEs can reference previously defined CTEs for nested prefetch DAGs.

### Clauses (apply to all modes)

| Clause | Description |
|---|---|
| `LIMIT <n>` | Max results (default 10) |
| `OFFSET <n>` | Skip first N results |
| `SCORE THRESHOLD <float>` | Minimum score filter |
| `LOOKUP FROM <col> [VECTOR '<name>']` | Cross-collection vector lookup |
| `WHERE <filter>` | Payload filter |
| `USING HYBRID` | Dense + sparse fusion |
| `USING SPARSE` | Sparse-only |
| `USING DENSE` | Dense-only |
| `USING MODEL '<model>'` | Pin dense model |
| `EXACT` | Exact KNN (no HNSW) |
| `RERANK [MODEL '<model>']` | Cloud reranking |
| `GROUP BY '<field>' [GROUP_SIZE <n>]` | Grouped results |
| `WITH LOOKUP FROM <col>` | Cross-collection group ID lookup |
| `STRATEGY '<strategy>'` | Recommend strategy |
| `BOOST (<expr>)` | Score shaping |
| `DEFAULTS (key = value)` | Formula variable defaults |

### WITH params

```sql
WITH (hnsw_ef = 256)
WITH (exact = true)
WITH (acorn = true)
WITH (indexed_only = true)
WITH (mmr_diversity = 0.5, mmr_candidates = 20)
WITH (rrf_k = 30, rrf_weights = [0.7, 0.3])
WITH PAYLOAD (include = ['title', 'body'])
WITH PAYLOAD (exclude = ['embedding'])
WITH PAYLOAD false
WITH VECTORS ('dense', 'sparse')
WITH VECTORS true
WITH MODEL '<model>'
WITH LOOKUP FROM <collection>
```

Multiple `WITH` clauses merge: `WITH MODEL 'x' WITH PAYLOAD (include = ['a']) WITH (exact = true)`

## BOOST (Score Shaping)

```sql
QUERY '<text>' FROM <collection> LIMIT <n>
  BOOST (<expression>)
  DEFAULTS (key = value, ...)
```

### Expression syntax

```sql
-- Arithmetic
$score * 2 + 1
a + b - c * d / e

-- Math functions
ABS(x)
SQRT(x)
LOG(x)
LN(x)
EXP(x)
POW(base, exponent)

-- Geo distance
GEO_DISTANCE({lat: 40.7, lon: -74.0}, location_field)

-- Decay functions
GAUSS_DECAY(field, target: value, scale: 30d)
EXP_DECAY(field, target: value, scale: 30d)
LIN_DECAY(field, target: value, scale: 30d)
GAUSS_DECAY(field, target: datetime('2026-01-01'), scale: 30d, midpoint: 0.5)

-- Datetime
datetime('2026-01-01T00:00:00Z')
datetime_key('published_at')

-- Conditional
CASE WHEN <filter> THEN <expr> ELSE <expr> END

-- Variables
$score          -- current relevance score
field_name      -- payload field value
```

### Operator precedence

1. `*`, `/` (highest)
2. `+`, `-`
3. Comparison operators
4. `NOT`
5. `AND`
6. `OR` (lowest)

Parentheses override precedence.

## SELECT / SCROLL

```sql
SELECT * FROM <collection> WHERE id = '<uuid>'
SELECT * FROM <collection> WHERE id = <integer>

SCROLL FROM <collection> LIMIT <n>
SCROLL FROM <collection> WHERE <filter> LIMIT <n>
SCROLL FROM <collection> AFTER '<point_id>' LIMIT <n>
SCROLL FROM <collection> WHERE <filter> AFTER <point_id> LIMIT <n>
```

## UPDATE

```sql
UPDATE <collection> SET PAYLOAD = {'field': 'value'} WHERE id = '<uuid>'
UPDATE <collection> SET PAYLOAD = {'field': 'value'} WHERE <filter>
UPDATE <collection> SET VECTOR = [0.1, 0.2, 0.3] WHERE id = '<uuid>'
```

## DELETE

```sql
DELETE FROM <collection> WHERE id = '<uuid>'
DELETE FROM <collection> WHERE id = <integer>
DELETE FROM <collection> WHERE <field> = '<value>'
DELETE FROM <collection> WHERE <filter>
```

## EXPLAIN

```bash
qql-go explain "QUERY 'search' FROM docs LIMIT 5 USING HYBRID"
qql-go explain --json "QUERY 'search' FROM docs LIMIT 5"
```

## Scripts

```bash
qql-go execute <script.qql>
qql-go execute --stop-on-error <script.qql>
qql-go dump <collection> <output.qql>
qql-go dump --batch-size 100 <collection> <output.qql>
```

Scripts are multi-statement QQL files. Statements are separated by newlines. Comments start with `--`.
