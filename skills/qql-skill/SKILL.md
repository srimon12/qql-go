---
name: qql-skill
description: "Use QQL to create collections, create payload indexes, insert documents (single or bulk), search with dense, sparse, hybrid, grouped, paginated, score-thresholded, or lookup-from retrieval, recommend by example IDs, update vectors or payloads, use exact and query-time search params, explain plans, execute scripts, dump collections, and delete data. Use when Codex needs to write or review QQL statements for the Go CLI, choose between dense, sparse, hybrid, grouped, and reranked search, or explain what QQL can and cannot do in the current Go implementation."
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
- `CREATE COLLECTION <name> WITH HNSW { payload_m: <n> }`
- `CREATE INDEX ON COLLECTION <name> FOR <field> TYPE <kind>`
- `CREATE INDEX ON COLLECTION <name> FOR <field> TYPE keyword WITH { is_tenant, on_disk, enable_hnsw }`
- `CREATE INDEX ON COLLECTION <name> FOR <field> TYPE uuid WITH { is_tenant, on_disk, enable_hnsw }`
- `CREATE INDEX ON COLLECTION <name> FOR <field> TYPE text WITH { tokenizer, min_token_len, max_token_len, lowercase, ascii_folding, phrase_matching, stopwords, on_disk, enable_hnsw }`
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
- `QUERY '<text>' FROM <name> LIMIT <n>`
- `QUERY '<text>' FROM <name> LIMIT <n> OFFSET <n>`
- `QUERY '<text>' FROM <name> LIMIT <n> SCORE THRESHOLD <float|int>`
- `QUERY '<text>' FROM <name> LIMIT <n> LOOKUP FROM <collection> [VECTOR '<name>']`
- `QUERY '<text>' FROM <name> LIMIT <n> USING MODEL '<model>'`
- `QUERY '<text>' FROM <name> LIMIT <n> USING HYBRID`
- `QUERY '<text>' FROM <name> LIMIT <n> USING HYBRID FUSION DBSF`
- `QUERY '<text>' FROM <name> LIMIT <n> USING HYBRID WITH { rrf_k: <n>, rrf_weights: [...] }`
- `QUERY '<text>' FROM <name> LIMIT <n> USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'`
- `QUERY '<text>' FROM <name> LIMIT <n> USING SPARSE`
- `QUERY '<text>' FROM <name> LIMIT <n> WHERE <filter>`
- `QUERY '<text>' FROM <name> LIMIT <n> GROUP BY <field>`
- `QUERY '<text>' FROM <name> LIMIT <n> GROUP BY <field> GROUP_SIZE <m>`
- `QUERY '<text>' FROM <name> LIMIT <n> USING HYBRID GROUP BY <field> [GROUP_SIZE <m>]`
- `QUERY '<text>' FROM <name> LIMIT <n> EXACT`
- `QUERY '<text>' FROM <name> LIMIT <n> WITH { hnsw_ef, exact, acorn }`
- `QUERY '<text>' FROM <name> LIMIT <n> WITH { indexed_only, quantization }`
- `QUERY '<text>' FROM <name> LIMIT <n> WITH { mmr_diversity, mmr_candidates }`
- `QUERY '<text>' FROM <name> LIMIT <n> RERANK`
- `QUERY '<text>' FROM <name> LIMIT <n> RERANK MODEL '<model>'`
- `QUERY '<text>' FROM <name> LIMIT <n> USING HYBRID RERANK`
- `QUERY '<text>' FROM <name> LIMIT <n> USING SPARSE RERANK`
- `QUERY RECOMMEND POSITIVE IDS (<id>, ...) FROM <name> LIMIT <n>`
- `QUERY RECOMMEND POSITIVE IDS (<id>, ...) NEGATIVE IDS (<id>, ...) FROM <name> LIMIT <n>`
- `QUERY RECOMMEND POSITIVE IDS (<id>, ...) STRATEGY '<strategy>' FROM <name> LIMIT <n>`
- `QUERY RECOMMEND POSITIVE IDS (<id>, ...) FROM <name> LIMIT <n> OFFSET <n>`
- `QUERY RECOMMEND POSITIVE IDS (<id>, ...) FROM <name> LIMIT <n> SCORE THRESHOLD <f>`
- `QUERY RECOMMEND POSITIVE IDS (<id>, ...) FROM <name> LIMIT <n> WITH { exact: true, hnsw_ef: <n> }`
- `QUERY RECOMMEND POSITIVE IDS (<id>, ...) FROM <name> LOOKUP FROM <collection> [VECTOR '<name>'] LIMIT <n>`
- `QUERY CONTEXT PAIRS ((<pos>, <neg>), ...) FROM <name> LIMIT <n>`
- `QUERY DISCOVER TARGET <id> CONTEXT PAIRS ((<pos>, <neg>), ...) FROM <name> LIMIT <n>`
- `QUERY '<text>' FROM <name> LIMIT <n> PREFETCH (...) FUSION RRF`
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
- Text `INSERT` and `QUERY` use Qdrant Cloud inference via `qdrant.Document` objects.
- `RERANK` is available.

### Local mode

- `qql-go connect --url http://localhost:6334 --inference-mode local --embedding-endpoint <url> [--embedding-key <key>] --embedding-model <name> [--embedding-dimension <n>]`
- Dense vectors come from an OpenAI-compatible embeddings API (e.g., LM Studio, llamafile).
- Sparse vectors are generated client-side and rely on Qdrant's sparse `idf` modifier for collection-wide rarity weighting.
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
QUERY 'hello' FROM my_collection LIMIT 5
DROP COLLECTION my_collection
```

For human debugging, use the text path (`qql-go exec "..."`, `qql-go explain "..."`, `qql-go doctor`).

`qql-go explain --quiet "<query>"` prints the raw plan text without the titled section wrapper.

If `qql-go` is not installed yet, use [references/qql-install.md](references/qql-install.md) first.

## Choose The Mode Before Writing The Query

Use this decision sequence.

### Dense search

Use plain `QUERY` when the request is mostly semantic and exact keyword matching is not important.

### Hybrid search

Use `USING HYBRID` when exact terms, model names, acronyms, codes, or domain vocabulary matter alongside semantic similarity.

Hybrid search uses `RRF` by default. Add `FUSION DBSF` only when you want to explicitly switch the fusion strategy.

Works in cloud, local, and external modes.

### Sparse-only search

Use `USING SPARSE` when the request is purely keyword/BM25 retrieval with no semantic component.

Works in cloud, local, and external modes.

### Exact baseline

Use `EXACT` when the user is debugging recall and wants to compare exact search against HNSW behavior.

### Query-time tuning

Use `WITH { hnsw_ef: ... }` when recall needs tuning without changing collection settings.

Use `WITH { acorn: true }` only when filtered-query recall is the actual problem.

Use `WITH { mmr_diversity: ..., mmr_candidates: ... }` when the user wants semantic diversity inside dense results or the dense leg of hybrid retrieval.

MMR currently works for dense `QUERY`, `USING HYBRID`, and their `GROUP BY` variants.
Do not suggest it for `USING SPARSE` or `QUERY RECOMMEND`.

### Rerank

Use `RERANK` when the right candidates are likely already retrieved but the top ordering is weak.

**Cloud mode only.** In local/external mode, reranking returns an explicit error.

### Grouped search

Use `GROUP BY` when the user wants the top matches grouped by a payload field instead of one flat ranked list.

Use `GROUP_SIZE` to cap how many hits each group returns.

Do not combine `GROUP BY` with `RERANK`.

Do not combine `GROUP BY` with `OFFSET`.

### Point lookup

Use `SELECT` when the user already knows the exact point ID and wants the stored payload back.

### Collection browse

Use `SCROLL` when the user needs pagination, export preparation, or a filtered walk through points.

### Recommend

Use `QUERY RECOMMEND POSITIVE IDS (...)` when the user has example document IDs and wants to find similar items.

Works in all modes because it operates on stored vectors, not query-time inference.

### Context and Discover

Use `QUERY CONTEXT PAIRS ((pos, neg), ...)` for context-aware search.

Use `QUERY DISCOVER TARGET <id> CONTEXT PAIRS (...)` for exploration search.

### Manual prefetch DAGs

Use `PREFETCH (...) FUSION RRF` when the user needs per-prefetch filters, limits, or score thresholds for multi-stage retrieval.

Combine with `WITH { rrf_k: <n>, rrf_weights: [...] }` for parameterized RRF tuning.

`PREFETCH` and `USING HYBRID` are mutually exclusive.

## Index Before You Filter

If a query uses `WHERE`, create payload indexes first.

Use:

```sql
CREATE INDEX ON COLLECTION docs FOR specialty TYPE keyword
CREATE INDEX ON COLLECTION docs FOR year TYPE integer
CREATE INDEX ON COLLECTION docs FOR tenant_id TYPE keyword WITH { is_tenant: true, on_disk: true }
CREATE INDEX ON COLLECTION docs FOR external_id TYPE uuid WITH { on_disk: true }
CREATE INDEX ON COLLECTION docs FOR title TYPE text WITH { tokenizer: 'word', min_token_len: 2, lowercase: true }
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
- score boosting
- relevance feedback
- custom distance metrics or vector sizes in `CREATE COLLECTION`

Use [references/qql-gaps.md](references/qql-gaps.md) for the current boundary.

## Use The Demo Scripts Sparingly

Use the bundled scripts when a runnable example is actually useful:

- [scripts/demo_retrieval_modes.py](scripts/demo_retrieval_modes.py)
- [scripts/demo_medical_records.py](scripts/demo_medical_records.py)
- [scripts/demo_kitchen_sink.py](scripts/demo_kitchen_sink.py)

These demos use [scripts/_qql_cli.py](scripts/_qql_cli.py), which calls `qql-go exec --quiet --json ...`.

Do not dump demos into the answer when one query would do.
