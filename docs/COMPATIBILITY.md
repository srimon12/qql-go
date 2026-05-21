# QQL / Qdrant Compatibility Matrix

> This document tracks which QQL versions support which Qdrant features, across both Python and Go implementations.
> Last updated: 2026-05-21

## Legend

| Symbol | Meaning |
|---|---|
| ✅ | Fully supported |
| ⚠️ | Partially supported or known limitations |
| ❌ | Not supported |
| 🐍 | Python (`qql-cli`) only |
| 🐹 | Go (`qql-go`) only |
| N/A | Not applicable |

---

## Qdrant Feature Coverage

### Collection Management

| Feature | Qdrant API | Python `qql-cli` | Go `qql-go` | Notes |
|---|---|---|---|---|
| Create collection | `create_collection` | ✅ v1.0 | ✅ v0.1 | Go auto-creates on insert; Python supports explicit create and insert-time create paths |
| Create with custom distance | `distance: DOT / EUCLID / MANHATTAN / COSINE` | ❌ | ❌ | Locked to COSINE |
| Create with quantization | `quantization_config` | ✅ v1.4 | ✅ v0.1.4 | `QUANTIZE SCALAR|BINARY|PRODUCT|TURBO` |
| Create with on-disk payload | `on_disk_payload` | ❌ | ✅ v0.2.0 | Go supports via `WITH PARAMS { on_disk_payload: true }` |
| Create with HNSW config | `hnsw_config` | ✅ v2.5.0 | ✅ v0.2.0 | Go and Python now support `WITH HNSW { ... }` block |
| Create with optimizer config | `optimizers_config` | ✅ v2.5.0 | ✅ v0.2.0 | Via `WITH OPTIMIZERS { ... }` block |
| Create with vectors config | `vectors_config` | ✅ v2.5.0 | ✅ v0.2.0 | Via `WITH VECTORS { on_disk: true }` |
| Create with collection params | `CollectionParams` | ✅ v2.5.0 | ✅ v0.2.0 | Via `WITH PARAMS { replication_factor, write_consistency_factor, on_disk_payload }` |
| Alter collection | `update_collection` | ✅ v2.5.0 | ✅ v0.2.0 | `ALTER COLLECTION ... WITH ... QUANTIZE` |
| Alter with quantization disable | `Disabled.DISABLED` | ✅ v2.5.0 | ✅ v0.2.0 | `QUANTIZE DISABLED` |
| Create with sparse vectors | `sparse_vectors_config` | ✅ v1.0 | ✅ v0.1 | Via `HYBRID` |
| Create with multivectors | `multivector_config` | ❌ | ⚠️ v0.1 | Go supports `CREATE COLLECTION ... HYBRID RERANK` with a ColBERT multivector; Python has no equivalent collection topology |
| Drop collection | `delete_collection` | ✅ v1.0 | ✅ v0.1 | |
| List collections | `get_collections` | ✅ v1.0 | ✅ v0.1 | `SHOW COLLECTIONS` |
| Collection info | `get_collection` | ✅ v2.5.0 | ✅ v0.2.0 | Both support `SHOW COLLECTION <name>`. Go now shows per-vector on_disk, hnsw inline_storage, read_fan_out_factor, on_disk_payload |
| Collection aliases | `create_alias` / `delete_alias` | ❌ | ❌ | |
| Collection snapshots | `create_snapshot` | ❌ | ❌ | |

### Points / Documents

| Feature | Qdrant API | Python `qql-cli` | Go `qql-go` | Notes |
|---|---|---|---|---|
| Insert single point | `upsert` | ✅ v1.0 | ✅ v0.1 | Requires `text` field |
| Insert bulk | `upsert` (batch) | ✅ v1.0 | ✅ v0.1 | |
| Get point by ID | `retrieve` | ✅ v2.2 | ✅ v0.1.4 | `SELECT` statement |
| Update payload | `set_payload` | ✅ v2.3 | ✅ v0.1.5 | `UPDATE ... SET PAYLOAD` |
| Update vector | `update_vectors` | ✅ v2.3 | ✅ v0.1.5 | `UPDATE ... SET VECTOR` |
| Delete point by ID | `delete` | ✅ v1.0 | ✅ v0.1 | |
| Delete points by filter | `delete` (filter) | ✅ v2.5.0 | ✅ v0.1 | Both support filter-based `DELETE FROM ... WHERE ...` |
| Delete payload keys | `delete_payload` | ❌ | ❌ | Gap |
| Count points | `count` | ❌ | ❌ | Gap |
| Scroll points | `scroll` | ✅ v2.2 | ✅ v0.1.4 | `SCROLL` statement |

### Search

| Feature | Qdrant API | Python `qql-cli` | Go `qql-go` | Notes |
|---|---|---|---|---|
| Dense search | `search` / `query_points` | ✅ v1.0 | ✅ v0.1 | |
| Hybrid search (RRF) | `query_points` + `Fusion.RRF` | ✅ v1.0 | ✅ v0.1 | |
| Sparse-only search | `query_points` (sparse vector) | ✅ v1.0 | ✅ v0.1 | |
| Exact search | `exact=true` | ✅ v1.0 | ✅ v0.1 | `EXACT` shorthand |
| HNSW ef tuning | `search_params.hnsw_ef` | ✅ v1.0 | ✅ v0.1 | `WITH { hnsw_ef: N }` |
| ACORN filtered search | `search_params.acorn` | ✅ v1.0 | ✅ v0.1 | `WITH { acorn: true }` |
| Search with filters | `filter` | ✅ v1.0 | ✅ v0.1 | `WHERE` clause |
| Grouped search | `query_points_groups` | ✅ v2.3 | ✅ v0.1.5 | `GROUP BY ... [GROUP_SIZE ...]` |
| Search pagination | `offset` | ✅ v2.5.0 | ✅ v0.2.0 | `OFFSET <n>`; not compatible with `GROUP BY` |
| Batch search | `search_batch` | ❌ | ❌ | Gap |
| MMR diversity | `diversity` param (v1.15+) | ✅ v1.0 | ✅ v0.1 | `WITH { mmr_diversity, mmr_candidates }` |
| Score boosting | `rescore` / `formula` | ❌ | ❌ | Gap |
| Multivector search | `multivector` | ❌ | ⚠️ v0.1 | Go supports the ColBERT multivector path for cloud rerank collections, not a general multivector search surface |
| Rerank (cross-encoder) | `rerank` / Fastembed | ✅ v1.0 (local FastEmbed post-processing) | ⚠️ v0.1 (cloud only) | Go supports cloud rerank queries, but local/external rerank is not implemented |
| Relevance feedback | `feedback` query | ❌ | ❌ | Gap |

### Recommend

| Feature | Qdrant API | Python `qql-cli` | Go `qql-go` | Notes |
|---|---|---|---|---|
| Recommend by examples | `recommend` | ✅ v1.0 | ✅ v0.1 | `RECOMMEND FROM` |
| Positive/negative IDs | `positive` / `negative` | ✅ v1.0 | ✅ v0.1 | |
| Strategy selection | `strategy` | ✅ v1.0 | ✅ v0.1 | `average_vector`, `best_score`, `sum_scores` |
| Cross-collection lookup | `lookup_from` | ✅ v1.0 | ✅ v0.1 | `LOOKUP FROM` |
| Named vector usage | `using` | ✅ v1.0 | ✅ v0.1 | `USING '<vector>'` |
| Offset | `offset` | ✅ v1.0 | ✅ v0.1 | |
| Score threshold | `score_threshold` | ✅ v1.0 | ✅ v0.1 | |
| Filtered recommend | `filter` | ✅ v1.0 | ✅ v0.1 | `WHERE` clause |

### Payload Indexes

| Feature | Qdrant API | Python `qql-cli` | Go `qql-go` | Notes |
|---|---|---|---|---|
| Keyword index | `create_payload_index` | ✅ v2.5.0 | ✅ v0.1 | Both support keyword indexes; both support advanced keyword params via `WITH { is_tenant, on_disk, enable_hnsw }` |
| Integer index | `create_payload_index` | ✅ v2.5.0 | ✅ v0.1 | |
| Float index | `create_payload_index` | ✅ v2.5.0 | ✅ v0.1 | |
| Bool index | `create_payload_index` | ✅ v2.5.0 | ✅ v0.1 | |
| Full-text index | `create_payload_index` (text) | ✅ v2.5.0 | ✅ v0.1 | Both support text tokenizer/index tuning via `WITH { ... }` |
| Geo index | `create_payload_index` (geo) | ✅ v2.5.0 | ✅ v0.1 | Basic geo index support in both |
| Datetime index | `create_payload_index` (datetime) | ✅ v2.5.0 | ✅ v0.1 | |
| UUID index | `create_payload_index` (uuid) | ✅ v2.5.0 | ✅ v0.1 | Both support advanced UUID params via `WITH { is_tenant, on_disk, enable_hnsw }` |

### Filtering

| Feature | Qdrant API | Python `qql-cli` | Go `qql-go` | Notes |
|---|---|---|---|---|
| Equality | `MatchValue` | ✅ v1.0 | ✅ v0.1 | `=` |
| Inequality | `must_not` + `MatchValue` | ✅ v1.0 | ✅ v0.1 | `!=` |
| Range | `Range` | ✅ v1.0 | ✅ v0.1 | `>`, `<`, `>=`, `<=` |
| Between | `Range` (gte/lte) | ✅ v1.0 | ✅ v0.1 | `BETWEEN ... AND` |
| In list | `MatchAny` | ✅ v1.0 | ✅ v0.1 | `IN (...)` |
| Not in list | `MatchExcept` | ✅ v1.0 | ✅ v0.1 | `NOT IN (...)` |
| Is null | `IsNull` | ✅ v1.0 | ✅ v0.1 | `IS NULL` |
| Is empty | `IsEmpty` | ✅ v1.0 | ✅ v0.1 | `IS EMPTY` |
| Full-text match | `MatchText` | ✅ v1.0 | ✅ v0.1 | `MATCH` |
| Match any term | `MatchTextAny` | ✅ v1.0 | ✅ v0.1 | `MATCH ANY` |
| Match phrase | `MatchPhrase` | ✅ v1.0 | ✅ v0.1 | `MATCH PHRASE` |
| AND / OR / NOT | `must` / `should` / `must_not` | ✅ v1.0 | ✅ v0.1 | |
| Nested fields (dot) | `key: "a.b"` | ✅ v1.0 | ✅ v0.1 | `meta.source` |
| Nested array access | `key: "a[].b"` | ⚠️ | ⚠️ | Documented but limited testing |

---

## Version Matrix

### QQL Version vs Qdrant Version

| QQL Version | Minimum Qdrant | Recommended Qdrant | Tested Qdrant Versions |
|---|---|---|---|
| Python 2.5.0 | 1.13.0 | 1.13.x | 1.13.0 |
| Go 0.2.0 | 1.13.0 | 1.13.x | 1.13.0 |

### Language Support

| QQL Version | Python `qql-cli` | Go `qql-go` | Feature Parity |
|---|---|---|---|
| Current | 2.5.0 | 0.2.0 | ~98% |
| Target (Phase 1) | 2.5.0 | 0.2.0 | ~98% |
| Target (Phase 2) | 2.5.0 | 0.3.0 | ~99% |

---

## Inference Mode Compatibility

| Feature | Cloud | Local | External |
|---|---|---|---|
| Dense insert/search | ✅ | ✅ | ✅ |
| Hybrid insert/search | ✅ | ✅ | ✅ |
| Sparse-only search | ✅ | ✅ | ✅ |
| Rerank | ✅ (Python and Go) | ✅ (Python only) | ✅ (Python only) |
| Recommend | ✅ | ✅ | ✅ |

---

## Known Bugs / Mismatches

| Issue | Python | Go | Tracking |
|---|---|---|---|
| Local rerank not available in Go | N/A | Affected | #TBD |
| Sparse BM25 implementation differs | Python uses FastEmbed `SparseTextEmbedding("Qdrant/bm25")` | Go uses repo-local sparse weighting plus Qdrant sparse `idf` modifier | #TBD |
| Go-only rerank collection topology | Python reranks locally without a ColBERT multivector collection | Go uses `HYBRID RERANK` / ColBERT multivector collections for cloud rerank | #TBD |
| `qql-go` has no programmatic API equivalent to Python's `run_query()` | N/A | Affected | #TBD |
| `CREATE COLLECTION ... HNSW {...}` (without WITH) | No longer valid syntax | No longer valid syntax | ✅ Fixed — both now require `WITH HNSW { ... }` |
| ALTER COLLECTION support | ✅ v2.5.0 | ✅ v0.2.0 | ✅ Now at parity |
| SHOW COLLECTION shows outdated diagnostics | ✅ v2.5.0 | ✅ v0.2.0 | ✅ Now at parity (on_disk, inline_storage, read_fan_out_factor, on_disk_payload) |
| Dumper omits quantization and config blocks | ✅ v2.5.0 | ✅ v0.2.0 | ✅ Now at parity |
| Comment stripping breaks `--` inside strings | ✅ v2.5.0 | ✅ v0.2.0 | ✅ Now at parity |

---

## How to Update This Document

When adding a new feature:
1. Update the relevant section above
2. Update the version matrix
3. Update the "Known Bugs" section if fixing a mismatch
4. Update the `Last updated` date at the top

This file is the **single source of truth** for QQL/Qdrant compatibility. Both Python and Go repos should keep their copies in sync.
