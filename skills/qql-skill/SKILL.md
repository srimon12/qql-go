---
name: qql-skill
description: "Use QQL to create collections, create payload indexes, insert documents, search with dense or hybrid retrieval, use exact and query-time search params, explain plans, and delete data. Use when Codex needs to write or review QQL statements for the Go CLI, choose between dense, hybrid, and reranked search, or explain what QQL can and cannot do in the current Go implementation."
---

# QQL Skill

Use this skill to turn retrieval intent into valid QQL for the current Go implementation.

Treat QQL as a query language and execution surface, not as a retrieval strategy engine. Write the smallest correct statement for the mode the user actually needs.

## Source Of Truth

Use [README.md](../../README.md) as the canonical public contract.

If you need the compact agent-facing mirror, use:

- [references/qql-capabilities.md](references/qql-capabilities.md) for supported syntax
- [references/qql-query-patterns.md](references/qql-query-patterns.md) for short runnable examples
- [references/qql-gaps.md](references/qql-gaps.md) for unsupported features only

Supported syntax in this repo includes:

- `CREATE COLLECTION <name>`
- `CREATE COLLECTION <name> HYBRID`
- `CREATE COLLECTION <name> HYBRID RERANK`
- `CREATE INDEX ON COLLECTION <name> FOR <field> TYPE <kind>`
- `SHOW COLLECTIONS`
- `DROP COLLECTION <name>`
- `INSERT INTO COLLECTION <name> VALUES {...}`
- `INSERT INTO COLLECTION <name> VALUES {...} USING MODEL '<model>'`
- `INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID`
- `INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'`
- `INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID SPARSE MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n>`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WHERE <filter>`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> EXACT`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WITH { hnsw_ef, exact, acorn }`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK MODEL '<model>'`
- `SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID RERANK`
- `DELETE FROM <name> WHERE ...`
- `EXPLAIN <statement>`

Current inference boundary:

- Text `INSERT` and text `SEARCH ... SIMILAR TO ...` are cloud inference paths.
- `USING HYBRID` and `RERANK` are cloud-only behavior for now.
- For self-hosted/local URLs, prefer non-inference statements (`SHOW`, `CREATE`, `DROP`, `CREATE INDEX`, `DELETE`).

## Agent and Script Output Contract

For automation, do not parse human CLI prose.

Use structured output:

- `qql exec --quiet --json "<query>"`
- `qql explain --quiet --json "<query>"`
- `qql doctor --quiet --json`
- `qql connect --quiet --json --url <url> [--secret <secret>]`

For human debugging, use the text path (`qql exec "..."`, `qql explain "..."`, `qql doctor`).

`qql explain --quiet "<query>"` prints the raw plan text without the titled section wrapper.

## Choose The Mode Before Writing The Query

Use this decision sequence.

### Dense search

Use plain `SEARCH` when the request is mostly semantic and exact keyword matching is not important.

### Hybrid search

Use `USING HYBRID` when exact terms, model names, acronyms, codes, or domain vocabulary matter.

Do not recommend hybrid for self-hosted-only flows in the current Go build.

### Exact baseline

Use `EXACT` when the user is debugging recall and wants to compare exact search against HNSW behavior.

### Query-time tuning

Use `WITH { hnsw_ef: ... }` when recall needs tuning without changing collection settings.

Use `WITH { acorn: true }` only when filtered-query recall is the actual problem.

### Rerank

Use `RERANK` when the right candidates are likely already retrieved but the top ordering is weak.

In the current Go implementation, reranking depends on Qdrant Cloud query-time inference. Do not recommend it casually for self-hosted flows.

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
- State clearly when a request is outside current QQL support.

## Explain Limits Clearly

When the request needs unsupported Qdrant features, say so directly and stop at the boundary.

Examples of current gaps:

- recommend or discovery APIs
- MMR or diversity controls
- score boosting
- relevance feedback
- pagination or scroll
- update or upsert by explicit id
- collection diagnostics

Use [references/qql-gaps.md](references/qql-gaps.md) for the current boundary.

## Use The Demo Scripts Sparingly

Use the bundled scripts when a runnable example is actually useful:

- [scripts/demo_retrieval_modes.py](scripts/demo_retrieval_modes.py)
- [scripts/demo_medical_records.py](scripts/demo_medical_records.py)
- [scripts/demo_kitchen_sink.py](scripts/demo_kitchen_sink.py)

These demos use [scripts/_qql_cli.py](scripts/_qql_cli.py), which calls `qql exec --quiet --json ...`.

Do not dump demos into the answer when one query would do.
