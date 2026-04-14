# QQL Capabilities

This is a compact mirror of the current Go CLI surface for agent use.
If it disagrees with [README.md](../../../README.md), follow the README.

## Statements

### Collection management

- `CREATE COLLECTION <name>`
- `CREATE COLLECTION <name> HYBRID`
- `CREATE COLLECTION <name> HYBRID RERANK`
- `SHOW COLLECTIONS`
- `DROP COLLECTION <name>`

### Payload indexes

- `CREATE INDEX ON COLLECTION <name> FOR <field> TYPE keyword`
- `CREATE INDEX ON COLLECTION <name> FOR <field> TYPE integer`
- `CREATE INDEX ON COLLECTION <name> FOR <field> TYPE float`
- `CREATE INDEX ON COLLECTION <name> FOR <field> TYPE bool`

### Insert

- `INSERT INTO COLLECTION <name> VALUES {...}`
- `INSERT INTO COLLECTION <name> VALUES {...} USING MODEL '<model>'`
- `INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID`
- `INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'`
- `INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID SPARSE MODEL '<model>'`
- keys inside `VALUES {...}` can be bare identifiers or quoted strings

### Search

- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n>`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WHERE <filter>`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> EXACT`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WITH { hnsw_ef: <n> }`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WITH { exact: true|false }`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WITH { acorn: true|false }`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID RERANK`

### Delete

- `DELETE FROM <name> WHERE id = '<uuid>'`
- `DELETE FROM <name> WHERE <field> = '<value>'`

### Explain

- `qql-go explain <statement>`

## CLI Output Modes

Human-readable defaults:

- `qql-go exec "<query>"`
- `qql-go explain "<query>"`
- `qql-go doctor`

Structured JSON for automation:

- `qql-go exec --json "<query>"`
- `qql-go exec --quiet --json "<query>"` (compact JSON)
- `qql-go explain --json "<query>"`
- `qql-go explain --quiet --json "<query>"` (compact JSON)
- `qql-go doctor --json`
- `qql-go doctor --quiet --json` (compact JSON)
- `qql-go connect --json --url <url> [--secret <secret>]`
- `qql-go connect --quiet --json --url <url> [--secret <secret>]` (compact JSON)

Text-mode quiet behavior:

- `qql-go explain --quiet "<query>"` prints the raw plan text without the titled section.

## Search Modes

### Dense

Use plain `SEARCH` for semantic retrieval.

Default dense model:
- `sentence-transformers/all-MiniLM-L6-v2`

### Hybrid

Use `USING HYBRID` when both semantic similarity and exact term matching matter.

Default sparse model:
- `qdrant/bm25`

`USING HYBRID` also supports explicit dense and sparse model overrides.

### Rerank

Use `RERANK` only when top ordering matters enough to pay for it.

Current rerank model:
- `answerdotai/answerai-colbert-small-v1`

Current rerank caveat:
- depends on Qdrant Cloud query-time inference

`RERANK MODEL '<model>'` lets you pin the reranker when the cloud path is available.

## Inference Modes

- `cloud` - active user-facing mode for server-side embeddings and hybrid search in the current Go CLI
- `external` - planned/partial wiring; do not present as generally available for text insert/search yet
- `local` - planned; do not describe as a production-ready user-facing mode in this Go build

## Query-Time Params

Supported params:

- `EXACT`
- `WITH { hnsw_ef: 128 }`
- `WITH { exact: true }`
- `WITH { acorn: true }`

Use them for:

- exact recall baselines
- query-time recall tuning
- filtered-query experiments

## Filters

Supported predicates:

- `field = value`
- `field != value`
- `field > value`
- `field >= value`
- `field < value`
- `field <= value`
- `field BETWEEN low AND high`
- `field IN (...)`
- `field NOT IN (...)`
- `field IS NULL`
- `field IS NOT NULL`
- `field IS EMPTY`
- `field IS NOT EMPTY`

Supported logical composition:

- `A AND B`
- `A OR B`
- `NOT A`
- parentheses

## Constraints

- `INSERT` requires a `text` field.
- Text `INSERT` and text `SEARCH ... SIMILAR TO ...` currently depend on Qdrant Cloud inference.
- Use payload indexes before relying on `WHERE`.
- Rerank is a Qdrant Cloud path, not a self-hosted default.
- Hybrid collections assume named vectors such as `dense` and `sparse`.
- Stay inside the implemented syntax. Do not invent clauses because Qdrant supports them in principle.
