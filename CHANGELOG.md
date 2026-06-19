# Changelog

All notable user-facing changes to this repository will be documented in this file.

The format is inspired by Keep a Changelog and uses calendar dates for repo releases.

## [Unreleased]

- No unreleased changes yet.

## [0.5.0] - 2026-06-19

### Added

- **Multivector (ColBERT) support** — `CREATE COLLECTION` accepts `WITH MULTIVECTOR (comparator = 'max_sim')` on named vectors for ColBERT/ColPali-style late interaction models. `HNSW (m = 0)` disables HNSW indexing on reranking vectors, reducing RAM and speeding up inserts.
- **Prefetch with `USING`** — Each CTE prefetch can target a different named vector: `WITH _pf0 AS (QUERY [...] USING 'dense' LIMIT 100), _pf1 AS (QUERY [...] USING 'sparse' LIMIT 100) QUERY ... USING 'original' PREFETCH (_pf0, _pf1)`.
- **Inline subqueries in `PREFETCH`** — `PREFETCH (QUERY [...] USING 'dense' LIMIT 100)` generates CTEs inline without explicit `WITH` blocks.
- **Insert with named vectors** — `INSERT INTO <col> VALUES {'id': 1, 'vector': {'dense': [...], 'colbert': [[...],[...]]}}` stores pre-computed dense and multivector vectors alongside payload.
- **`qql-go convert`** — Converts Qdrant REST API JSON to QQL. Accepts stdin, file, or wrapped `method+path+body` format. Supports all operations: create collection, upsert, search, recommend, discover, scroll, delete, set payload, create index.
- **Python SDK interceptor** — `sdks/python/qql_intercept.py` wraps `QdrantClient` at the HTTP layer, captures REST JSON calls, and pipes them through `qql-go convert` for migration.
- **`ConvertJSONBytesToQQL`** — `pkg/qql` exposes `[]byte` API to avoid string copies on high-throughput paths.
- **110-payload regression suite** — `all_payloads.json` covers PDF retrieval, 3-level nested prefetch, score_threshold, offset, group_by, 2D multivector query, wrapped endpoints, batch mixed recommend+search, insert with named vectors.

### Changed

- Converter formatters use `strings.Builder` and `strconv.FormatFloat` for zero-allocation output.
- `RESTPrefetch` and `RESTQueryRequest` structs handle `Using`, `Offset`, `ScoreThreshold`, `WithPayload` for complete JSON-to-QQL round-trips.
- `convertUpsert` includes vector data (dense and multivector) in output.
- `convertByStructure` routes text/indices queries with prefetch through formula path for CTE generation.
- Parser treats `DENSE`, `SPARSE`, `VECTOR` as contextual identifiers, allowing them as vector names in `CREATE COLLECTION`.
- Parser allows `HNSW (m = 0)` to disable indexing (was restricted to `m >= 4`).
- Executor maps `MultivectorConfig` to `qdrant.MultiVectorComparator_MaxSim` and shows multivector details in `EXPLAIN`.
- Converter test files updated for `[]byte` API.

## [0.4.0] - 2026-06-18

### Added

- **Connect RPC Gateway** — `qql-go gateway` starts a standalone gRPC-compatible server exposing `Exec`, `ExecBatch`, `Explain`, and `Health` via Connect RPC (gRPC + gRPC-Web + HTTP/1.1). Auto-detects pure QUERY batches and routes to Qdrant's native `QueryBatch` API.
- **Go SDK (`pkg/qql`)** — Public library with `Parse`, `Exec`, `ExecWithConfig`, `Explain`, `ExecBatch`, `BatchQuery`, `BatchQueryWithConfig`. Accepts `QdrantClient` interface and `*config.Config`.
- **Python SDK (`sdks/python/`)** — Connect RPC client with `QQLClient` wrapper, `exec()`, `exec_batch()`, `explain()`, `health()`.
- **TypeScript SDK (`sdks/typescript/`)** — Connect RPC client with `QQLClient` wrapper, dual CJS/ESM build.
- **Formula / Score Boosting (`BOOST`)** — Pratt parser expression engine for Qdrant's Score Builder API. All 19 Qdrant Expression variants covered: arithmetic (`+`, `-`, `*`, `/`), math functions (`ABS`, `SQRT`, `LOG`, `LN`, `EXP`, `POW`), geo-distance with dict syntax, decay functions (`EXP_DECAY`, `GAUSS_DECAY`, `LIN_DECAY`) with keyword arguments, datetime expressions (`datetime('...')`, `datetime_key('...')`), and `CASE WHEN <filter> THEN <expr> ELSE <expr> END` conditionals. `DEFAULTS (key = value, ...)` for fallback variables.
- **`QUERY SAMPLE`** — `QUERY SAMPLE FROM <collection> LIMIT <n>` for random point sampling via `Sample_Random`.
- **Unified config syntax** — All `WITH` blocks (`HNSW`, `OPTIMIZERS`, `PARAMS`, `VECTORS`, `QUANTIZATION`) use `(key = value)` instead of `{key: value}`.
- **`WITH QUANTIZATION`** — Replaces `QUANTIZE SCALAR` chain. Supports `type`, `quantile`, `always_ram`, `bits`, `disabled`. Per-vector quantization on vector definitions.
- **Per-prefetch lookup overrides** — `PREFETCH (cte LOOKUP FROM col VECTOR 'vec')`.
- **Per-vector HNSW config** — `dense VECTOR(384, COSINE) WITH QUANTIZATION (type = 'scalar')`.
- **Qdrant-native per-request timeout** — `Config.RequestTimeout` (seconds) sets `Timeout` on all Qdrant request types and controls Go context deadline.
- **`ExecWithConfig`, `ExecBatchWithConfig`, `BatchQueryWithConfig`** — SDK functions accepting explicit `*config.Config`.
- **Error chain support** — `QQLSyntaxError` and `QQLRuntimeError` implement `Unwrap()` with `Err error` field. Added `WrapQQLSyntaxError` and `WrapQQLRuntimeError`.
- **Comprehensive ExplainResult** — `EXPLAIN` shows all parsed fields: USING, MODEL, WITH params, PAYLOAD, VECTORS, WHERE, OFFSET, SCORE THRESHOLD, LOOKUP FROM, GROUP BY/SIZE, WITH LOOKUP, RERANK, STRATEGY, RECOMMEND IDs, CONTEXT pairs, DISCOVER target, CTEs, PREFETCH refs, FUSION, BOOST formula.

### Changed

- **BREAKING:** `QUANTIZE SCALAR [QUANTILE <f>] [ALWAYS RAM]` removed. Use `WITH QUANTIZATION (type = 'scalar', ...)`.
- **BREAKING:** `WITH HNSW { m: 16 }` removed. Use `WITH HNSW (m = 16)`.
- **BREAKING:** `WITH OPTIMIZERS { ... }` removed. Use `WITH OPTIMIZERS (...)`.
- **BREAKING:** `WITH PARAMS { ... }` removed. Use `WITH PARAMS (...)`.
- **BREAKING:** `WITH VECTORS { on_disk: true }` removed. Use `WITH VECTORS (on_disk = true)`.
- **BREAKING:** `CREATE INDEX ... WITH { ... }` removed. Use `CREATE INDEX ... WITH (...)`.
- **BREAKING:** `QUANTIZE DISABLED` removed. Use `WITH QUANTIZATION (disabled = true)`.
- **BREAKING:** `Result.DataJSON()` returns `([]byte, error)` instead of `[]byte`.
- **BREAKING:** Port 6333 rejected with error. Use port 6334 or omit.
- **BREAKING:** `INSERT` without `id` field errors instead of generating UUID.
- **Performance:** Lexer O(1) stack-buffer keyword lookup (~8x). Parser zero-allocation `asciiEqual`/`asciiEqualLower`. Filters removed `reflect`. Sparse byte-level ASCII fast path. BM25 params cached with `atomic.Pointer`. Pipeline cached `buildDocumentOptions`.
- `doQuery`/`BuildQueryPoints` deduplicated into `buildQueryStateAndPipeline`. CTE resolution shared. `BuildQueryPoints` accepts `context.Context`.
- Handler passes server config to SDK execution functions.
- Embedding client uses `http.Client{Timeout: 30s}` instead of `http.DefaultClient`.
- `CollectionConfig` AST embeds `Quantization` directly.
- Dump output uses unified `(key = value)` syntax.
- Multiple `WITH` clauses merge correctly.
- `FormulaNode` pushes `TargetQuery` into `Prefetches` before setting Formula.

### Fixed

- `BatchQuery` panicked on empty queries slice.
- Cross-collection `BatchQuery` silently sent all queries to first collection. Now groups by collection.
- SDK `Exec`/`BatchQuery` used empty `config.Config{}`, breaking model resolution.
- Server handler swallowed `json.Marshal` errors.
- `CREATE INDEX ... TYPE typo` silently created keyword index. Now errors.
- Script runner did not detect unmatched delimiters.
- Dump output used old `{key: value}` and `QUANTIZE SCALAR` syntax.
- Formula subtraction and conditional logic corrected for Qdrant's expression model.

## [0.3.0] - 2026-06-17

### Added

- **ORDER BY** — `QUERY ORDER BY <field> [ASC|DESC] FROM <collection> LIMIT <n>` for pagination and sorting without similarity scoring.
- **Payload and Vector Selectors** — `WITH PAYLOAD` and `WITH VECTORS` clauses for granular control over returned fields (e.g., `WITH PAYLOAD (include = ['title'], exclude = ['metadata']) WITH VECTORS ('dense')`).
- **Unified QUERY statement** — `QUERY` replaces `SEARCH` and `RECOMMEND` as a single statement with 4 modes: `NEAREST` (default), `RECOMMEND`, `CONTEXT`, and `DISCOVER`. All modes share the same clause surface: `LIMIT`, `OFFSET`, `SCORE THRESHOLD`, `LOOKUP FROM`, `USING`, `WITH`, `WHERE`, `RERANK`, `GROUP BY`, `GROUP_SIZE`, `STRATEGY`, `EXACT`.
- **Manual prefetch DAGs via CTEs** — `WITH <name> AS (QUERY ...), ... QUERY ... PREFETCH (<name>, ...) FUSION RRF` for explicit multi-stage retrieval with per-prefetch filters, limits, and nested CTE references.
- **Parameterized RRF** — `WITH (rrf_k = <n>, rrf_weights = [<float>, ...])` exposes Qdrant's parameterized Reciprocal Rank Fusion.
- **FUSION keyword** — `FUSION RRF` / `FUSION DBSF` for explicit fusion mode selection with manual prefetch DAGs.
- **Per-prefetch filtering and score thresholds** — `PREFETCH (a WHERE category = 'tech' SCORE THRESHOLD 0.8, b SCORE THRESHOLD 0.5)` applies independent filters and score thresholds to each CTE prefetch stage.
- **Cross-collection group lookup (`WITH LOOKUP FROM`)** — `QUERY ... GROUP BY <field> GROUP_SIZE <n> WITH LOOKUP FROM <collection>` enables cross-collection group ID lookup via `QueryPointGroups.WithLookup`.
- **CONTEXT and DISCOVER query modes** — `QUERY CONTEXT PAIRS ((pos, neg), ...)` and `QUERY DISCOVER TARGET <id> CONTEXT PAIRS (...)` for context-aware and exploration search.
- **Configurable BM25 parameters** — BM25 `k1`, `b`, and `avgdl` are now configurable from `~/.qql/config.json`.
- **Multi-vector DDL** — `CREATE COLLECTION <name> (name VECTOR(size, DISTANCE), ...)` for explicit named-vector schemas with custom sizes and distance metrics.
- **ALTER COLLECTION** — `ALTER COLLECTION` with `WITH VECTORS`, `WITH HNSW`, `WITH OPTIMIZERS`, `WITH PARAMS`, and `QUANTIZE` / `QUANTIZE DISABLED`.
- **Duplicate clause detection** — `EXACT`, `GROUP_SIZE`, `STRATEGY`, and `FUSION` clauses now reject duplicates with explicit syntax errors.

### Changed

- **BREAKING:** `SEARCH <collection> SIMILAR TO '<text>'` is removed. Use `QUERY '<text>' FROM <collection>` instead.
- **BREAKING:** `RECOMMEND FROM <collection> POSITIVE IDS (...)` is removed. Use `QUERY RECOMMEND WITH (positive = (...), negative = (...)) FROM <collection>` instead.
- **BREAKING:** `internal/utils` package deleted. All callers migrated to `strings.ToUpper` / `strings.ToLower`.
- Parser and executor broken into focused submodules: `exec_query.go`, `exec_insert.go`, `exec_manage.go`, `exec_select.go`, `exec_update.go`, `cli_cmds.go`, `client.go`, `utils.go` (from `commands.go`); `parse_query.go`, `parse_create.go`, `parse_insert.go`, `parse_update.go`, `parse_manage.go`, `parse_search.go` (from `parser.go`).
- Execution pipeline now uses a proper DAG (`DenseEmbedNode`, `SparseEmbedNode`, `FusionNode`, `RerankNode`, `RecommendNode`, `ContextNode`, `DiscoverNode`, `PrefetchNode`) with request assembly delegated to `BuildFlatRequest` / `BuildGroupedRequest`.
- All RPC calls now use a 30-second default timeout via `defaultContext()`.
- `Upsert` now sets `Wait` consistently with other write operations.
- `RecommendStrategy` now does case-insensitive matching (`'Average_Vector'` works).
- `SparseEmbedNode` now explicitly rejects MMR mode instead of silently ignoring it.
- Config file permissions restricted from `0o644` to `0o600` (contains secrets).
- Config global state protected by `sync.RWMutex` (was unsynchronized).
- Hash space expanded from 20-bit to full 32-bit (reduces BM25 token collision probability).
- Tokenizer `maybeToken` now uses `utf8.RuneCountInString` instead of `len` (correct for non-ASCII text).

### Fixed

- Hybrid search broken by `sparseVectorName` copying `denseVectorName` (both prefetches targeted the same vector).
- `buildInsertVectorsBatch` used `sparseVectorName` constant instead of `sparseName` parameter in local mode.
- Error from `resolveVectorTopology` used before check in insert and update paths.
- Error from `resolveVectorTopology` silently overwritten in `doUpdateVector`.
- Parser `advance()` panicked on empty token list.
- Non-deterministic map iteration for ID extraction (Go maps have random iteration order).
- `newPointID` cast negative `int` to `uint64` without validation (wraps to ~18 quintillion).
- `newPointID` mapped string `"123"` to `NewIDUUID` instead of coercing to `uint64` first.
- `int(v.IntegerValue)` truncated `int64` on 32-bit platforms in select and dump paths.
- `toFloat64` returned `nil` for unsupported types (silent filter corruption in range conditions).
- `parseInt64` / `toFloat64Value` returned `0` silently for unknown types in filters.
- `parseUint64` had no overflow protection and returned `0, nil` for empty strings.
- Timer leak in `waitForCollectionReady` (`time.After` in loop → `time.NewTimer`).
- Race conditions in `fake_client_test.go` (reads without mutex).
- `RunFile` swallowed the actual error from `executor.Execute` on stop-on-error.
- `buildDeleteRequest` used `fmt.Sprintf` + `parseUint64` roundtrip instead of `newPointID`.
- `buildUpdateVectorRequest` made redundant `GetCollectionInfo` RPC call (caller already had topology).
- `GROUP_SIZE` token was never consumed by `p.advance()` (was dead code).
- `seenWith` flag not set on first `WITH` clause, allowing duplicate `WITH` blocks.
- Variable shadowing in `parseEmbeddingOptions` where `denseVector` was declared then immediately shadowed.
- 8 false-positive parser tests that tested dead `SEARCH` syntax and passed only because `SEARCH` is not a keyword.
- `buildVectorInput` rejected `int64` and `uint64` types (common Go database ID types).
- `buildRecommendVectorInputs` silently dropped unsupported types and wrapped negative integers.
- `FusionNode` silently defaulted to RRF for unknown fusion modes.
- `insertPointIDAndPayload` did case-sensitive `"id"` lookup (missing `"ID"`, `"Id"`).
- `collectionHasRerankVector` and `formatSearchResults` were dead code (zero callers).
- Dump `escapeString` did not escape control characters (`\n`, `\r`, `\t`, `\0`).
- Dump `buildDumpCreateLine` panicked on nil config (missing nil check).
- `cloneConfig` shallow-copied `CloudModelOptions` map (shared mutation).

## [0.2.0] - 2026-05-21

### Added

- **SEARCH pagination** — `SEARCH ... LIMIT <n> OFFSET <n>` now forwards flat search offsets to Qdrant.
- **SEARCH score thresholds** — `SEARCH ... SCORE THRESHOLD <float|int>` filters low-score dense, sparse, hybrid, and grouped search results.
- **SEARCH cross-collection lookup** — `SEARCH ... LOOKUP FROM <collection> [VECTOR '<name>']` forwards Qdrant lookup location metadata across dense, sparse, hybrid, and grouped search paths.
- **Hybrid MMR** — `SEARCH ... USING HYBRID WITH { mmr_diversity: <0..1>, mmr_candidates: <n> }` now applies Qdrant native MMR to the dense prefetch before hybrid fusion.

### Changed

- Compatibility docs, README syntax, examples, release-validation checks, and bundled `qql-skill` references now align with Python QQL `2.5.0` and Go `0.2.0`.
- `GROUP BY` docs now call out that offset-style pagination is intentionally unsupported for grouped search.

### Fixed

- `ALTER COLLECTION ... WITH VECTORS { on_disk: ... }` now updates unnamed vectors and every named dense vector instead of only the first discovered dense vector.
- `ALTER COLLECTION ... QUANTIZE TURBO` now applies the Turbo quantization diff and reports invalid Turbo bit depths instead of silently dropping the update.
- Collection dumps now preserve `max_optimization_threads`, `on_disk_payload`, and Turbo bit values in generated `CREATE COLLECTION` statements.
- Collection config parsing is now deterministic for case-variant keys and rejects create-time `read_fan_out_factor` / `read_fan_out_delay_ms` case-insensitively.
- `RECOMMEND ... SCORE THRESHOLD 1` accepts integer literals, matching the Python parser behavior.

## [0.1.7] - 2026-05-17

### Added

- **Operational example suites** — the repo now ships runnable example folders for retrieval regression CI, retrieval debug runbooks, and a full medical retrieval benchmark workflow.
- **Medical retrieval benchmark pack** — `examples/medical-retrieval-ops/` now builds a full Hugging Face-backed benchmark corpus, compares dense/sparse/hybrid/exact retrieval modes, and records `hit@1` / `hit@5`.

### Changed

- **Local sparse retrieval baseline** — local and external sparse indexing now use a simpler Qdrant-native sparse weighting path that relies on Qdrant's sparse `idf` modifier instead of repo-managed corpus statistics.
- **Local sparse tokenization** — hyphenated domain terms such as `B-cell`, `anti-NMDA`, and `CD19-negative` now stay searchable in sparse mode instead of being degraded into missing single-character fragments.
- README, maintainer docs, bundled skill references, and runnable examples are now aligned around the new sparse baseline and the stronger operational example story.

### Fixed

- Local sparse document generation now uses the actual document-weighted sparse path during insert and bulk insert instead of the deprecated raw-term-frequency builder.
- The retrieval debug runbook example no longer ships a failing seed statement caused by using `product` as a field name.
- Release-validation docs and generated medical metadata no longer leak repo-local absolute paths.

## [0.1.6] - 2026-05-16

### Added

- **Expanded query-time search params** — `WITH` now exposes `indexed_only` plus quantization search params (`ignore`, `rescore`, `oversampling`) for `SEARCH` and `RECOMMEND`.
- **Native MMR** — `SEARCH ... WITH { mmr_diversity: <0..1>, mmr_candidates: <n> }` now maps to Qdrant's native MMR path for dense search and the dense leg of hybrid search, including grouped search.
- **Rich payload indexing** — `CREATE INDEX` now supports advanced `keyword`, `uuid`, and `text` index options, including `is_tenant`, `on_disk`, `enable_hnsw`, tokenizer settings, phrase matching, and stopwords.
- **Payload-aware HNSW config** — `CREATE COLLECTION ... HNSW { payload_m: <n> }` now exposes collection-level payload-aware HNSW tuning.

### Changed

- `SHOW COLLECTION <name>` now reports structured payload index params and `payload_m`, making advanced index configuration visible from the CLI.
- README, examples, release notes, and bundled `qql-skill` references now showcase dense MMR, tenant-aware indexing, text index tuning, and payload-aware HNSW setup.

### Fixed

- Filter predicates and `IN`/`NOT IN` lists now accept boolean literals such as `true` and `false`.
- `SCROLL` now preserves numeric `next_offset` values instead of coercing every cursor to a string.
- Bundled retrieval examples now keep index setup explicit and runnable without unsupported semicolon-separated CLI statements.

## [0.1.5] - 2026-05-15

### Added

- **Collection diagnostics** — `SHOW COLLECTION <name>` returns collection-level details for a single collection.
- **Grouped search** — `SEARCH ... GROUP BY <field> [GROUP_SIZE <n>]` groups top results by payload field across dense, sparse, and hybrid retrieval.
- **In-place point updates** — `UPDATE ... SET PAYLOAD` patches payload data by point ID or filter, and `UPDATE ... SET VECTOR WHERE id = <point_id>` replaces a stored vector.

### Changed

- Parser and explain-plan support now cover grouped retrieval and update statements, including validation for duplicate `GROUP BY`, invalid `GROUP_SIZE`, and unsupported `GROUP BY` + `RERANK` combinations.
- README, examples, compatibility notes, and bundled skill references now document `SHOW COLLECTION`, grouped search, and update statements consistently.

### Notes

- This release brings `qql-go` to the upstream QQL 2.3.0 parity line for collection info, grouped retrieval, payload updates, and vector updates.
- `GROUP BY` cannot be combined with `RERANK`, and `GROUP_SIZE` defaults to `3`.

## [0.1.4] - 2026-05-14

### Added

- **SELECT statement** — `SELECT * FROM <collection> WHERE id = <point_id>` retrieves a single point by ID without vectors.
- **SCROLL statement** — `SCROLL FROM <collection> [WHERE ...] [AFTER <id>] LIMIT <n>` supports pagination through points with optional filters and cursor offset.
- **Hybrid FUSION 'dbsf'** — `SEARCH ... USING HYBRID FUSION 'dbsf'` enables DBSF fusion strategy as an alternative to the default RRF.
- **TURBO quantization** — `CREATE COLLECTION ... QUANTIZE TURBO [BITS <1|1.5|2|4>] [ALWAYS RAM]` supports 1-4 bit turbo quantization with configurable bit depth.
- **RERANK with sparse-only search** — `SEARCH ... USING SPARSE RERANK` now supports reranking on sparse-only results (cloud mode).
- **Configurable dump batch size** — `qql-go dump --batch-size <n>` controls the number of points per INSERT BULK batch in dump output.

### Changed

- **Qdrant Go SDK upgraded** from v1.17.0 to v1.18.1 to support TURBO quantization.
- Collection creation messages now include turbo quantization details.

### Fixed

- RERANK now works with sparse-only (`USING SPARSE RERANK`) searches in cloud inference mode.
- Invalid `QUANTIZE TURBO BITS ...` values now fail fast instead of silently falling back to server defaults.
- Invalid `qql-go dump --batch-size` values now fail fast instead of silently resetting to the default batch size.
- `SELECT` and `SCROLL` return a simpler payload shape aligned with the upstream QQL response contract.

### Docs

- README, REPL help text, release docs, and bundled skill references now describe `SELECT`, `SCROLL`, `TURBO`, sparse rerank, hybrid fusion defaults, and `--embedding-key` consistently.

## [0.1.3] - 2026-04-29

### Added

- **Create-time quantization** — `CREATE COLLECTION` now supports `QUANTIZE SCALAR`, `QUANTIZE BINARY`, and `QUANTIZE PRODUCT`.
- **Scalar quantile validation** — scalar quantization accepts `QUANTILE` values in the inclusive `0..1` range, including integer boundaries `0` and `1`.

### Changed

- Collection creation docs, syntax references, compatibility notes, and skill references now describe quantization support and insert-time auto-creation behavior accurately.

## [0.1.2] - 2026-04-22

### Added

- **Local mode** — `qql-go connect --inference-mode local` generates dense and sparse vectors client-side via an OpenAI-compatible embeddings API (e.g., LM Studio, llamafile) and upserts explicit vectors to any Qdrant instance.
- **External mode** — `qql-go connect --inference-mode external` for remote Qdrant + remote embedding endpoints.
- **BM25 sparse vector generation** — client-side sparse vectors with Rust-compatible tokenization, length-prefixed FNV hashing, and corpus-level BM25 weighting. Stats are persisted in `~/.qql/corpus/<collection>.json`.
- **Auto-probing embedding dimension** — `--embedding-dimension` is now optional for local/external mode; the CLI probes the endpoint automatically.
- **`RECOMMEND`** — recommend similar points by positive (and optional negative) example IDs with configurable strategy (`average_vector`, `best_score`, `sum_scores`).
- **`RECOMMEND ... OFFSET <n>`** — pagination for recommendation queries.
- **`RECOMMEND ... SCORE THRESHOLD <f>`** — minimum score filter for recommendations.
- **`RECOMMEND ... WITH { exact: true, hnsw_ef: <n> }`** — query-time search params for recommend (exact KNN, HNSW tuning).
- **`RECOMMEND ... LOOKUP FROM <collection> [VECTOR '<name>']`** — cross-collection recommendation, looking up example IDs from a different collection.
- **`RECOMMEND ... USING '<vector_name>'`** — target a specific named vector (e.g., `sparse`) in the target collection for recommendation.
- **`INSERT BULK`** — batch insert multiple documents in a single statement.
- **Explicit insert IDs** — `INSERT` and `INSERT BULK` accept explicit `id` fields (unsigned int or UUID string).
- **`USING SPARSE`** — sparse-only keyword search without dense vectors.
- **`CREATE COLLECTION ... USING MODEL`** — create dense-only collections sized to a specific model.
- **Script execution** — `qql-go execute <script.qql>` for running multi-statement scripts.
- **Collection dump** — `qql-go dump <collection> <output.qql>` for exporting collections as runnable QQL scripts.
- **REPL shortcuts** — `\e <file>` and `\dump <collection> <file>` for script execution and dump from the interactive shell.
- **Connect-time endpoint validation** — the embedding endpoint is test-called during `connect` to surface misconfiguration early.

### Changed

- `buildSearchPrefetches` now returns errors instead of silently swallowing embedding failures.
- `RECOMMEND` now targets the `dense` named vector explicitly, fixing failures on hybrid collections.
- Corpus stats are automatically cleaned up when a collection is dropped.
- Corrupt corpus stats files are detected and rebuilt automatically on next insert.

### Fixed

- Error propagation in hybrid search prefetches when the embedding endpoint is unreachable.
- `RECOMMEND` execution on collections with named vectors.

### Notes

- `RERANK` remains **cloud-only** in this release. Local and external modes reject rerank queries with a clear error.
- The `cloud` inference mode behavior is unchanged and fully backward compatible.

## [0.1.1] - 2026-04-14

### Changed

- Renamed the shipped CLI binary and release artifacts from `qql` to `qql-go`.
- Updated CLI help text, docs, skill references, and helper scripts to use `qql-go`.
- Made GitHub release publishing idempotent so reruns update assets instead of failing when a release already exists.

### Notes

- This is a maintenance follow-up to `0.1.0` focused on packaging, naming, and release automation polish.

## [0.1.0] - 2026-04-14

### Added

- Standalone Go CLI for Qdrant with `connect`, `disconnect`, `exec`, `explain`, `doctor`, `repl`, and `version` commands.
- SQL-like QQL support for collection management, inserts, search, explain plans, and deletes.
- Structured JSON output mode for script and agent workflows.
- Public `skills/qql-skill` package for agent installation through the `skills` CLI.
- Basic GitHub Actions CI that runs tests and verifies the CLI builds on push and pull request.
- Tagged release automation for publishing prebuilt binaries to GitHub Releases.
- Open-source repo docs for release notes, changelog tracking, and skill authoring.

### Notes

- In the current Go build, text `INSERT`, text `SEARCH ... SIMILAR TO ...`, `USING HYBRID`, and `RERANK` depend on Qdrant Cloud inference paths.
- Self-hosted or local Qdrant is currently best suited to non-inference operations such as collection and payload index management.
