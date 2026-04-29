# Changelog

All notable user-facing changes to this repository will be documented in this file.

The format is inspired by Keep a Changelog and uses calendar dates for repo releases.

## [Unreleased]

- No unreleased changes yet.

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
