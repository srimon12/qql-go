# QQL Capabilities

This is a compact mirror of the current Go CLI surface for agent use.
If it disagrees with [README.md](../../../README.md), follow the README.

## Statements

### Collection management

- `CREATE COLLECTION <name>`
- `CREATE COLLECTION <name> HYBRID`
- `CREATE COLLECTION <name> HYBRID RERANK`
- `CREATE COLLECTION <name> USING MODEL '<dense_model>'`
- `CREATE COLLECTION <name> USING HYBRID`
- `CREATE COLLECTION <name> USING HYBRID DENSE MODEL '<model>'`
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
- `INSERT BULK INTO COLLECTION <name> VALUES [{...}, {...}]`
- `INSERT BULK INTO COLLECTION <name> VALUES [{...}, {...}] USING HYBRID`
- keys inside `VALUES {...}` can be bare identifiers or quoted strings
- explicit `id` is accepted inside `VALUES` (unsigned int or UUID string)

### Search

- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n>`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING SPARSE`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING SPARSE MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WHERE <filter>`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> EXACT`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WITH { hnsw_ef: <n> }`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WITH { exact: true|false }`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WITH { acorn: true|false }`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID RERANK`

### Recommend

- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) NEGATIVE IDS (<id>, ...) LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) STRATEGY '<strategy>' LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> OFFSET <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> SCORE THRESHOLD <f>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> WITH { exact: true, hnsw_ef: <n> }`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> LOOKUP FROM <collection>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> LOOKUP FROM <collection> VECTOR '<name>'`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> USING '<vector_name>'`

Supported strategies: `average_vector`, `best_score`, `sum_scores`.

All recommend clauses can be combined in order: `POSITIVE IDS`, `NEGATIVE IDS`, `STRATEGY`, `LOOKUP FROM`, `USING`, `LIMIT`, `OFFSET`, `SCORE THRESHOLD`, `WHERE`, `WITH`.

### Delete

- `DELETE FROM <name> WHERE id = '<uuid>'`
- `DELETE FROM <name> WHERE <field> = '<value>'`

### Explain

- `qql-go explain <statement>`

### Script execution

- `qql-go execute <script.qql>` — execute a `.qql` script file
- `qql-go execute --stop-on-error <script.qql>`

### Dump

- `qql-go dump <collection> <output.qql>` — dump collection to a `.qql` script

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

Default sparse model name (for cloud inference):
- `qdrant/bm25`

In local/external mode, sparse vectors are generated client-side using BM25-style weighting with corpus statistics stored in `~/.qql/corpus/<collection>.json`.

### Sparse-only

Use `USING SPARSE` when the request is purely keyword/BM25 retrieval with no semantic component.

### Rerank

Use `RERANK` only when top ordering matters enough to pay for it.

Current rerank model:
- `answerdotai/answerai-colbert-small-v1`

Rerank caveat:
- **Cloud mode only** in the current Go build. Local/external mode explicitly rejects `RERANK`.

`RERANK MODEL '<model>'` lets you pin the reranker when the cloud path is available.

## Inference Modes

- `cloud` — Qdrant Cloud inference (default). Sends `qdrant.Document` objects for server-side embedding.
- `external` — any Qdrant instance + external OpenAI-compatible embeddings API. Dense and sparse vectors are generated client-side.
- `local` — local Qdrant + local OpenAI-compatible embedding server (e.g., LM Studio, llamafile). Same vector-generation path as `external`; differs only in config defaults.

Local/external mode requirements:
- `--embedding-endpoint` pointing to an OpenAI-compatible `/v1/embeddings` endpoint
- `--embedding-model` name
- `--embedding-dimension` — optional; auto-probed from the endpoint if omitted and reachable

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

- `INSERT` and `INSERT BULK` require a `text` field in every row.
- In **cloud mode**, text `INSERT` and text `SEARCH ... SIMILAR TO ...` depend on Qdrant Cloud inference.
- In **local/external mode**, text operations work against any Qdrant instance with client-side vector generation.
- Use payload indexes before relying on `WHERE`.
- Rerank is **cloud-only**.
- Hybrid collections use named vectors: `dense` and `sparse`.
- Stay inside the implemented syntax. Do not invent clauses because Qdrant supports them in principle.
