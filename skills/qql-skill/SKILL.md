---
name: qql-skill
description: "Use QQL to create collections, create payload indexes, insert documents (single or bulk), search with dense, sparse, hybrid, or grouped retrieval, recommend by example IDs, update vectors or payloads, use exact and query-time search params, explain plans, execute scripts, dump collections, and delete data. Use when Codex needs to write or review QQL statements for the Go CLI, choose between dense, sparse, hybrid, grouped, and reranked search, or explain what QQL can and cannot do in the current Go implementation."
---

# QQL Skill

Use this skill to turn retrieval intent into valid QQL for the current Go implementation.

Treat QQL as a query language and execution surface, not as a retrieval strategy engine. Write the smallest correct statement for the mode the user actually needs.

## Skill References

Use these bundled references first:

- [references/qql-install.md](references/qql-install.md) for first-time CLI install and local mode setup
- [references/qql-capabilities.md](references/qql-capabilities.md) for supported syntax
- [references/qql-query-patterns.md](references/qql-query-patterns.md) for short runnable examples
- [references/qql-gaps.md](references/qql-gaps.md) for unsupported features only

If the skill is being used from the source repo checkout, [README.md](../../README.md) is the broader project overview.

Supported syntax in this repo includes:

- `CREATE COLLECTION <name>`
- `CREATE COLLECTION <name> HYBRID`
- `CREATE COLLECTION <name> HYBRID RERANK`
- `CREATE COLLECTION <name> USING MODEL '<model>'`
- `CREATE COLLECTION <name> USING HYBRID`
- `CREATE COLLECTION <name> QUANTIZE SCALAR [QUANTILE <0.0-1.0>] [ALWAYS RAM]`
- `CREATE COLLECTION <name> QUANTIZE BINARY [ALWAYS RAM]`
- `CREATE COLLECTION <name> QUANTIZE PRODUCT [ALWAYS RAM]`
- `CREATE COLLECTION <name> QUANTIZE TURBO [BITS <1|1.5|2|4>] [ALWAYS RAM]`
- `CREATE INDEX ON COLLECTION <name> FOR <field> TYPE <kind>`
- `SHOW COLLECTIONS`
- `SHOW COLLECTION <name>`
- `DROP COLLECTION <name>`
- `INSERT INTO COLLECTION <name> VALUES {...}`
- `INSERT INTO COLLECTION <name> VALUES {...} USING MODEL '<model>'`
- `INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID`
- `INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'`
- `INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID SPARSE MODEL '<model>'`
- `INSERT BULK INTO COLLECTION <name> VALUES [{...}, {...}]`
- `INSERT BULK INTO COLLECTION <name> VALUES [{...}, {...}] USING HYBRID`
- keys inside `VALUES {...}` can be bare identifiers or quoted strings
- explicit `id` inside `VALUES` is accepted (unsigned int or UUID string)
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n>`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID FUSION 'rrf|dbsf'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING SPARSE`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING SPARSE MODEL '<model>'` (cloud only)
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WHERE <filter>`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> GROUP BY <field>`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> GROUP BY <field> GROUP_SIZE <m>`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID GROUP BY <field> [GROUP_SIZE <m>]`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> EXACT`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WITH { hnsw_ef, exact, acorn }`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID RERANK`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING SPARSE RERANK`
- `SELECT * FROM <name> WHERE id = '<uuid>'`
- `SELECT * FROM <name> WHERE id = <integer>`
- `SCROLL FROM <name> LIMIT <n>`
- `SCROLL FROM <name> WHERE <filter> LIMIT <n>`
- `SCROLL FROM <name> AFTER '<point_id>' LIMIT <n>`
- `SCROLL FROM <name> WHERE <filter> AFTER <point_id> LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) NEGATIVE IDS (<id>, ...) LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) STRATEGY '<strategy>' LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> OFFSET <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> SCORE THRESHOLD <f>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> WITH { exact: true, hnsw_ef: <n> }`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LOOKUP FROM <collection> [VECTOR '<name>'] LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) USING '<vector_name>' LIMIT <n>`
- `DELETE FROM <name> WHERE ...`
- `UPDATE <name> SET VECTOR WHERE id = <id> [<float>, ...]`
- `UPDATE <name> SET PAYLOAD WHERE id = <id> {...}`
- `UPDATE <name> SET PAYLOAD WHERE <filter> {...}`
- `qql-go explain <statement>`
- `qql-go execute <script.qql>`
- `qql-go execute --stop-on-error <script.qql>`
- `qql-go dump <collection> <output.qql>`
- `qql-go dump --batch-size <n> <collection> <output.qql>`
- `qql-go disconnect`
- `qql-go version`
- `qql-go repl` (interactive shell)

## Inference Modes

`qql-go` supports three inference modes configured at `connect` time:

### Cloud mode (default)

- `qql-go connect --url <qdrant-cloud-url> --secret <api-key>`
- Text `INSERT` and `SEARCH ... SIMILAR TO ...` use Qdrant Cloud inference via `qdrant.Document` objects.
- `RERANK` is available.

### Local mode

- `qql-go connect --url http://localhost:6334 --inference-mode local --embedding-endpoint <url> [--embedding-key <key>] --embedding-model <name> [--embedding-dimension <n>]`
- Dense vectors come from an OpenAI-compatible embeddings API (e.g., LM Studio, llamafile).
- Sparse vectors are generated client-side with BM25-style weighting.
- Corpus statistics are stored in `~/.qql/corpus/<collection>.json`.
- `RERANK` is **not** available in local mode.

### External mode

- Same vector-generation path as local, but intended for remote embedding endpoints and remote Qdrant.
- `RERANK` is **not** available in external mode.

## Agent and Script Output Contract

For automation, do not parse human CLI prose.

Use structured output:

- `qql-go exec --quiet --json "<query>"`
- `qql-go explain --quiet --json "<query>"`
- `qql-go doctor --quiet --json`
- `qql-go connect --quiet --json --url <url> [--secret <secret>]`
- `qql-go disconnect --quiet --json`
- `qql-go version --quiet --json`
- `qql-go execute --quiet --json <script.qql>`
- `qql-go execute --stop-on-error --quiet --json <script.qql>`
- `qql-go dump --quiet --json <collection> <output.qql>`
- `qql-go dump --quiet --json --batch-size <n> <collection> <output.qql>`

### Script File Format

Script files (`.qql`) use **newline-delimited statements WITHOUT semicolons**:

```sql
-- Comment
CREATE COLLECTION my_collection
SHOW COLLECTIONS
SHOW COLLECTION my_collection
INSERT INTO COLLECTION my_collection VALUES {'text': 'hello'}
SEARCH my_collection SIMILAR TO 'hello' LIMIT 5
DROP COLLECTION my_collection
```

For human debugging, use the text path (`qql-go exec "..."`, `qql-go explain "..."`, `qql-go doctor`).

`qql-go explain --quiet "<query>"` prints the raw plan text without the titled section wrapper.

If `qql-go` is not installed yet, use [references/qql-install.md](references/qql-install.md) first.

## Choose The Mode Before Writing The Query

Use this decision sequence.

### Dense search

Use plain `SEARCH` when the request is mostly semantic and exact keyword matching is not important.

### Hybrid search

Use `USING HYBRID` when exact terms, model names, acronyms, codes, or domain vocabulary matter alongside semantic similarity.

Hybrid search uses `RRF` by default. Add `FUSION 'dbsf'` only when you want to explicitly switch the fusion strategy.

Works in cloud, local, and external modes.

### Sparse-only search

Use `USING SPARSE` when the request is purely keyword/BM25 retrieval with no semantic component.

Works in cloud, local, and external modes.

### Exact baseline

Use `EXACT` when the user is debugging recall and wants to compare exact search against HNSW behavior.

### Query-time tuning

Use `WITH { hnsw_ef: ... }` when recall needs tuning without changing collection settings.

Use `WITH { acorn: true }` only when filtered-query recall is the actual problem.

### Rerank

Use `RERANK` when the right candidates are likely already retrieved but the top ordering is weak.

**Cloud mode only.** In local/external mode, reranking returns an explicit error.

### Grouped search

Use `GROUP BY` when the user wants the top matches grouped by a payload field instead of one flat ranked list.

Use `GROUP_SIZE` to cap how many hits each group returns.

Do not combine `GROUP BY` with `RERANK`.

### Point lookup

Use `SELECT` when the user already knows the exact point ID and wants the stored payload back.

### Collection browse

Use `SCROLL` when the user needs pagination, export preparation, or a filtered walk through points.

### Recommend

Use `RECOMMEND` when the user has example document IDs and wants to find similar items.

Works in all modes because it operates on stored vectors, not query-time inference.

## Index Before You Filter

If a query uses `WHERE`, create payload indexes first.

Use:

```sql
CREATE INDEX ON COLLECTION docs FOR specialty TYPE keyword
CREATE INDEX ON COLLECTION docs FOR year TYPE integer
```

Then write the filtered search.

Do not pretend that unindexed filtering is the happy path.

## Write Conservatively

- Prefer one valid statement over a long explanation.
- Do not invent syntax that the parser does not support.
- Do not recommend hybrid by default when the real issue may be model quality or bad chunking.
- Do not recommend rerank when latency matters more than precision.
- Do not recommend rerank in local/external mode.
- State clearly when a request is outside current QQL support.

## Explain Limits Clearly

When the request needs unsupported Qdrant features, say so directly and stop at the boundary.

Examples of current gaps:

- local/external rerank
- discovery API
- MMR or diversity controls
- score boosting
- relevance feedback
- collection-level HNSW tuning
- custom distance metrics or vector sizes in `CREATE COLLECTION`

Use [references/qql-gaps.md](references/qql-gaps.md) for the current boundary.

## Use The Demo Scripts Sparingly

Use the bundled scripts when a runnable example is actually useful:

- [scripts/demo_retrieval_modes.py](scripts/demo_retrieval_modes.py)
- [scripts/demo_medical_records.py](scripts/demo_medical_records.py)
- [scripts/demo_kitchen_sink.py](scripts/demo_kitchen_sink.py)

These demos use [scripts/_qql_cli.py](scripts/_qql_cli.py), which calls `qql-go exec --quiet --json ...`.

Do not dump demos into the answer when one query would do.
