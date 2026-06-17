---
name: qql-skill
description: "Use QQL to manage collections, insert documents, search, filter, rerank, recommend, and more. Use when Codex needs to write or review QQL statements for the Go CLI."
---

# QQL Skill

Use this skill to turn retrieval intent into valid QQL for the current Go implementation.
Treat QQL as a query language and execution surface, not as a retrieval strategy engine.

## Reference Wiki

Read these reference documents **ONLY** when you need details on their specific topics:
- [references/qql-install.md](references/qql-install.md) — Read if `qql-go` is not installed or for `local`/`external` mode setup.
- [references/qql-gaps.md](references/qql-gaps.md) — Read if a user asks for unsupported features (ReadConsistency, Timeout, ShardKeySelector).
- [references/qql-examples.md](references/qql-examples.md) — Read for advanced examples (CTEs, MMR, Context patterns).

For runnable demo scripts, see `scripts/demo_retrieval_modes.py`, `scripts/demo_medical_records.py`, and `scripts/demo_kitchen_sink.py`.

## Intent Mapping
Translate user intent directly into QQL syntax:
- Semantic similarity -> `QUERY '<text>' FROM <collection>`
- Exact terms also matter -> add `USING HYBRID`
- Hybrid retrieval with DBSF fusion -> `USING HYBRID FUSION DBSF`
- Hybrid retrieval with tuned RRF -> `USING HYBRID WITH (rrf_k = ..., rrf_weights = [...])`
- Multi-stage retrieval -> `WITH <name> AS (...), ... QUERY ... PREFETCH (name1, name2) FUSION RRF`
- Keyword-only retrieval -> `USING SPARSE`
- Query by point ID -> `QUERY <id> FROM <collection>`
- Recommendation by example -> `QUERY RECOMMEND WITH (positive = (...), negative = (...))`
- Context-aware search -> `QUERY CONTEXT PAIRS (...)`
- Exploration search -> `QUERY DISCOVER TARGET <id> CONTEXT PAIRS (...)`
- Random sampling -> `QUERY SAMPLE FROM <collection> LIMIT <n>`
- Browse by field -> `QUERY ORDER BY <field> [ASC|DESC] FROM <collection>`
- Score boosting -> `BOOST ($score + 0.3 * popularity)` or `BOOST (CASE WHEN ... THEN ... ELSE ... END)`
- Recall debugging -> add `EXACT`
- Query-time recall tuning -> add `WITH (hnsw_ef = ...)`
- Filtered recall concern -> add `WITH (acorn = true)`
- Diverse dense/hybrid results -> add `WITH (mmr_diversity = ..., mmr_candidates = ...)`
- Better ordering (Cloud Only) -> add `RERANK`
- Grouped top results by field -> add `GROUP BY <field> [GROUP_SIZE <n>]`
- Cross-collection group lookup -> add `WITH LOOKUP FROM <collection>` on grouped queries
- Exact point lookup -> `SELECT * FROM <collection> WHERE id = <id>`
- Browse points -> `SCROLL FROM <collection> [AFTER <id>] LIMIT <n>`
- Batch ingest -> `INSERT INTO <collection> VALUES {...}, {...}`

## QQL Capabilities & Grammar

Use the following bracketed syntax. Elements in `[]` are optional. Elements separated by `|` are choices.

### Collection Management
```sql
CREATE COLLECTION <name> [HYBRID [RERANK]]
  [WITH HNSW (m = <n>, ef_construct = <n>, ...)]
  [WITH OPTIMIZERS (deleted_threshold = <f>, ...)]
  [WITH PARAMS (replication_factor = <n>, ...)]
  [WITH QUANTIZATION (type = 'scalar'|'binary'|'product'|'turbo', ...)]
  [USING MODEL '<model>' | USING HYBRID [DENSE MODEL '<model>']]

ALTER COLLECTION <name> ... -- Supports WITH HNSW, WITH OPTIMIZERS, WITH PARAMS, WITH QUANTIZATION (disabled = true)
SHOW COLLECTIONS
SHOW COLLECTION <name>
DROP COLLECTION <name>
```

### Payload Indexes
Always index fields before using them in `WHERE` filters.
```sql
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE <keyword|integer|float|bool|uuid|text>
  [WITH (
    is_tenant = bool, on_disk = bool, enable_hnsw = bool,
    tokenizer = 'word|whitespace|prefix|multilingual', min_token_len = <n>, max_token_len = <n>,
    lowercase = bool, ascii_folding = bool, phrase_matching = bool, stopwords = ['en', ...]
  )]
```

### Insert & Update
```sql
INSERT INTO <name> VALUES { 'text': '...', 'category': '...' }, {...}, {...}
  [USING [HYBRID [DENSE MODEL '<m>' SPARSE MODEL '<m>'] | MODEL '<m>']]

UPDATE <name> SET VECTOR = [<float>, ...] WHERE id = <id>
UPDATE <name> SET PAYLOAD = {...} WHERE <filter_expression>
DELETE FROM <name> WHERE <filter_expression>
```

### Query
```sql
QUERY ['<text>' | <id> | RECOMMEND WITH (positive = (...), negative = (...)) [STRATEGY '<strategy>'] | CONTEXT PAIRS (...) | DISCOVER TARGET <id> CONTEXT PAIRS (...) | ORDER BY <field> [ASC|DESC] | SAMPLE]
FROM <collection>
  [PREFETCH ( <cte_name> [WHERE <filter>] [SCORE THRESHOLD <n>], ... ) FUSION <RRF | DBSF>]
  [LOOKUP FROM <collection> [VECTOR '<name>']]
  [USING [HYBRID [FUSION DBSF] | SPARSE | DENSE | '<vector_name>']]
  [WITH MODEL '<model>']
  [WHERE <filter_expression>]
  [GROUP BY <field> [GROUP_SIZE <m>] [WITH LOOKUP FROM <collection>]]
  [WITH (hnsw_ef = <n>, exact = <bool>, acorn = <bool>, mmr_diversity = <f>, mmr_candidates = <n>, rrf_k = <n>, rrf_weights = [...])]
  [WITH PAYLOAD [true | false | (include = ['<field>', ...], exclude = ['<field>', ...])]]
  [WITH VECTORS [true | false | ('<name>', ...)]]
  [BOOST (<expression>)]
  [DEFAULTS (<variable> = <float>, ...)]
  [RERANK [MODEL '<model>']]
  [EXACT]
  [LIMIT <n>] [OFFSET <n>] [SCORE THRESHOLD <float>]
```

### BOOST Formula Expressions
The `BOOST` clause applies a mathematical expression to modify search scores.
- **Variables:** `$score` (current score), bare names for payload fields (e.g., `popularity`, `freshness`)
- **Operators:** `+`, `-`, `*`, `/` with standard precedence (`*` and `/` before `+` and `-`)
- **Functions:** `ABS(x)`, `SQRT(x)`, `LOG(x)`, `LN(x)`, `EXP(x)`, `POW(base, exp)`
- **Geo:** `GEO_DISTANCE(lat, lon, field)` — distance in meters
- **Decay:** `GAUSS_DECAY(x, target, scale, midpoint)`, `EXP_DECAY(...)`, `LIN_DECAY(...)`
- **Conditional:** `CASE WHEN <filter> THEN <expr> ELSE <expr> END`
- **Defaults:** `DEFAULTS (var1 = 1.0, var2 = 0.0)` — fallback values for missing payload fields

Examples:
```sql
BOOST ($score + 0.3 * popularity)
BOOST (CASE WHEN category = 'premium' THEN $score * 2.0 ELSE $score END)
BOOST ($score * GAUSS_DECAY(GEO_DISTANCE(48.85, 2.35, location), 0, 5000, 0.5))
BOOST (SQRT($score) * LOG(citation_count + 1)) DEFAULTS (citation_count = 0)
```

### CTEs (Common Table Expressions)
```sql
WITH <name> AS (QUERY ...) [, <name> AS (QUERY ...)]
QUERY ... FROM <collection> PREFETCH (<name>, ...) FUSION RRF LIMIT <n>
```

**Notes:**
- `PREFETCH` references CTE names, not inline queries.
- Each prefetch ref can have an inline `WHERE` filter and `SCORE THRESHOLD`.
- `OFFSET` cannot be used with `GROUP BY`.
- Filters use standard SQL operators: `=`, `!=`, `>`, `<`, `BETWEEN ... AND ...`, `IN (...)`, `IS NULL`, `IS EMPTY`, `AND`, `OR`, `NOT`.

## Agent and Script Output Contract
For automation, use structured output:
- `qql-go exec --quiet --json "<query>"`
- `qql-go explain --quiet --json "<query>"`
- `qql-go execute --quiet --json <script.qql>`
- `qql-go doctor --quiet --json`
- `qql-go connect --quiet --json --url <url> ...`
- `qql-go dump --quiet --json [--batch-size <n>] <collection> <output.qql>`

**Script format:** `.qql` files use newline-delimited statements **WITHOUT semicolons**.
```sql
-- Comment
CREATE COLLECTION my_collection
INSERT INTO my_collection VALUES {'text': 'hello'}
QUERY 'hello' FROM my_collection LIMIT 5
```
