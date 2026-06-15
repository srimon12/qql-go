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
- [references/qql-gaps.md](references/qql-gaps.md) — Read if a user asks for unsupported features (formula, ORDER BY, score boosting).
- [references/qql-examples.md](references/qql-examples.md) — Read for advanced examples (Manual PREFETCH DAGs, MMR, Context patterns).

For runnable demo scripts, see `scripts/demo_retrieval_modes.py`, `scripts/demo_medical_records.py`, and `scripts/demo_kitchen_sink.py`.

## Intent Mapping
Translate user intent directly into QQL syntax:
- Semantic similarity -> `QUERY '<text>' FROM <collection>`
- Exact terms also matter -> add `USING HYBRID`
- Hybrid retrieval with DBSF fusion -> `USING HYBRID FUSION DBSF`
- Hybrid retrieval with tuned RRF -> `USING HYBRID WITH { rrf_k: ..., rrf_weights: [...] }`
- Multi-stage retrieval -> `PREFETCH (...) FUSION RRF`
- Keyword-only retrieval -> `USING SPARSE`
- Query by point ID -> `QUERY <id> FROM <collection>`
- Recommendation by example -> `QUERY RECOMMEND POSITIVE IDS (...)`
- Context-aware search -> `QUERY CONTEXT PAIRS (...)`
- Exploration search -> `QUERY DISCOVER TARGET <id> CONTEXT PAIRS (...)`
- Recall debugging -> add `EXACT`
- Query-time recall tuning -> add `WITH { hnsw_ef: ... }`
- Filtered recall concern -> add `WITH { acorn: true }`
- Diverse dense/hybrid results -> add `WITH { mmr_diversity: ..., mmr_candidates: ... }`
- Better ordering (Cloud Only) -> add `RERANK` (can be combined with `USING HYBRID` / `USING SPARSE`)
- Grouped top results by field -> add `GROUP BY <field> [GROUP_SIZE <n>]`
- Exact point lookup -> `SELECT * FROM <collection> WHERE id = <id>`
- Browse points -> `SCROLL FROM <collection> [AFTER <id>] LIMIT <n>`
- Batch ingest -> `INSERT BULK INTO <collection> VALUES [...]`

## QQL Capabilities & Grammar

Use the following bracketed syntax. Elements in `[]` are optional. Elements separated by `|` are choices.

### Collection Management
```sql
CREATE COLLECTION <name> [HYBRID [RERANK]]
  [WITH HNSW { payload_m: <n> }]
  [WITH OPTIMIZERS { deleted_threshold: <f>, ... }]
  [WITH PARAMS { replication_factor: <n>, ... }]
  [USING MODEL '<model>' | USING HYBRID [DENSE MODEL '<model>']]
  [QUANTIZE [SCALAR [QUANTILE <f>] | BINARY | PRODUCT | TURBO [BITS <n>]] [ALWAYS RAM]]

ALTER COLLECTION <name> ... -- Supports WITH VECTORS, WITH HNSW, WITH OPTIMIZERS, WITH PARAMS, QUANTIZE
SHOW COLLECTIONS
SHOW COLLECTION <name>
DROP COLLECTION <name>
```

### Payload Indexes
Always index fields before using them in `WHERE` filters.
```sql
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE <keyword|integer|float|bool|uuid|text>
  [WITH {
    is_tenant: bool, on_disk: bool, enable_hnsw: bool,
    tokenizer: 'word|whitespace|prefix|multilingual', min_token_len: <n>, max_token_len: <n>,
    lowercase: bool, ascii_folding: bool, phrase_matching: bool, stopwords: ['en', ...]
  }]
```

### Insert & Update
```sql
INSERT [BULK] INTO COLLECTION <name> VALUES { 'text': '...', 'category': '...' } | [{...}, {...}]
  [USING [HYBRID [DENSE MODEL '<m>' SPARSE MODEL '<m>'] | MODEL '<m>']]

UPDATE <name> SET VECTOR WHERE id = <id> [<float>, ...]
UPDATE <name> SET PAYLOAD WHERE <filter_expression> {...}
DELETE FROM <name> WHERE <filter_expression>
```

### Query
```sql
QUERY ['<text>' | <id> | NEAREST '<text>' | RECOMMEND POSITIVE IDS (<id>, ...) [NEGATIVE IDS (<id>, ...)] [STRATEGY '<strategy>'] | CONTEXT PAIRS ((<pos>, <neg>), ...) | DISCOVER TARGET <id> CONTEXT PAIRS ((<pos>, <neg>), ...)]
FROM <collection>
  [PREFETCH ( <query_statement>, ... ) FUSION <RRF | DBSF>]
  [LOOKUP FROM <collection> [VECTOR '<name>']]
  [USING [HYBRID [FUSION DBSF] | SPARSE | '<vector_name>']]
  [WITH MODEL '<model>']
  [WHERE <filter_expression>]
  [GROUP BY <field> [GROUP_SIZE <m>]]
  [WITH { hnsw_ef: <n>, exact: bool, acorn: bool, mmr_diversity: <f>, mmr_candidates: <n>, rrf_k: <n>, rrf_weights: [...] }]
  [RERANK [MODEL '<model>']]
  [EXACT]
  [LIMIT <n>] [OFFSET <n>] [SCORE THRESHOLD <float>]
```
**Notes:**
- `PREFETCH` is mutually exclusive with `USING HYBRID`.
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
INSERT INTO COLLECTION my_collection VALUES {'text': 'hello'}
QUERY 'hello' FROM my_collection LIMIT 5
```
