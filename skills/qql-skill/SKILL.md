---
name: qql-skill
description: "Use QQL to create collections, create payload indexes, insert documents (single or bulk), search with dense, sparse, or hybrid retrieval, recommend by example IDs, use exact and query-time search params, explain plans, execute scripts, dump collections, and delete data. Use when Codex needs to write or review QQL statements for the Go CLI, choose between dense, sparse, hybrid, and reranked search, or explain what QQL can and cannot do in the current Go implementation."
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
- `CREATE INDEX ON COLLECTION <name> FOR <field> TYPE <kind>`
- `SHOW COLLECTIONS`
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
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING SPARSE`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING SPARSE MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WHERE <filter>`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> EXACT`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WITH { hnsw_ef, exact, acorn }`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID RERANK`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) NEGATIVE IDS (<id>, ...) LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) STRATEGY '<strategy>' LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> OFFSET <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> SCORE THRESHOLD <f>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n> WITH { exact: true, hnsw_ef: <n> }`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LOOKUP FROM <collection> [VECTOR '<name>'] LIMIT <n>`
- `RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) USING '<vector_name>' LIMIT <n>`
- `DELETE FROM <name> WHERE ...`
- `qql-go explain <statement>`
- `qql-go execute <script.qql>`
- `qql-go dump <collection> <output.qql>`

## Inference Modes

`qql-go` supports three inference modes configured at `connect` time:

### Cloud mode (default)

- `qql-go connect --url <qdrant-cloud-url> --secret <api-key>`
- Text `INSERT` and `SEARCH ... SIMILAR TO ...` use Qdrant Cloud inference via `qdrant.Document` objects.
- `RERANK` is available.

### Local mode

- `qql-go connect --url http://localhost:6334 --inference-mode local --embedding-endpoint <url> --embedding-model <name> [--embedding-dimension <n>]`
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
- `qql-go execute --quiet --json <script.qql>`
- `qql-go dump --quiet --json <collection> <output.qql>`

For human debugging, use the text path (`qql-go exec "..."`, `qql-go explain "..."`, `qql-go doctor`).

`qql-go explain --quiet "<query>"` prints the raw plan text without the titled section wrapper.

If `qql-go` is not installed yet, use [references/qql-install.md](references/qql-install.md) first.

## Choose The Mode Before Writing The Query

Use this decision sequence.

### Dense search

Use plain `SEARCH` when the request is mostly semantic and exact keyword matching is not important.

### Hybrid search

Use `USING HYBRID` when exact terms, model names, acronyms, codes, or domain vocabulary matter alongside semantic similarity.

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
- pagination or scroll
- update or upsert by explicit id (you can overwrite with `INSERT` + explicit `id`)
- collection diagnostics beyond `doctor`

Use [references/qql-gaps.md](references/qql-gaps.md) for the current boundary.

## Use The Demo Scripts Sparingly

Use the bundled scripts when a runnable example is actually useful:

- [scripts/demo_retrieval_modes.py](scripts/demo_retrieval_modes.py)
- [scripts/demo_medical_records.py](scripts/demo_medical_records.py)
- [scripts/demo_kitchen_sink.py](scripts/demo_kitchen_sink.py)

These demos use [scripts/_qql_cli.py](scripts/_qql_cli.py), which calls `qql-go exec --quiet --json ...`.

Do not dump demos into the answer when one query would do.
