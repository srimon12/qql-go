# QQL / Qdrant Compatibility Matrix

> This document tracks which QQL versions support which Qdrant features, across both Python and Go implementations.
> Last updated: 2026-05-15

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
| Create with custom HNSW | `hnsw_config` | ❌ | ❌ | |
| Create with quantization | `quantization_config` | ✅ v1.4 | ✅ v0.1.4 | `QUANTIZE SCALAR|BINARY|PRODUCT|TURBO` |
| Create with on-disk payload | `on_disk_payload` | ❌ | ❌ | |
| Create with sparse vectors | `sparse_vectors_config` | ✅ v1.0 | ✅ v0.1 | Via `HYBRID` |
| Create with multivectors | `multivector_config` | ❌ | ❌ | |
| Drop collection | `delete_collection` | ✅ v1.0 | ✅ v0.1 | |
| List collections | `get_collections` | ✅ v1.0 | ✅ v0.1 | `SHOW COLLECTIONS` |
| Collection info | `get_collection` | ❌ | ✅ v0.1.5 | Go supports `SHOW COLLECTION <name>`; Python `DESCRIBE` is not implemented |
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
| Delete points by filter | `delete` (filter) | ❌ | ✅ v0.1 | Python README claims support but parser only handles `WHERE id = ...` |
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
| Search pagination | `offset` | ❌ | ❌ | Gap |
| Batch search | `search_batch` | ❌ | ❌ | Gap |
| MMR diversity | `diversity` param (v1.15+) | ❌ | ❌ | Gap |
| Score boosting | `rescore` / `formula` | ❌ | ❌ | Gap |
| Multivector search | `multivector` | ❌ | ❌ | Gap |
| Rerank (cross-encoder) | `rerank` / Fastembed | ✅ v1.0 (local) | ⚠️ v0.1 (cloud only) | Sparse-only rerank added v0.1.4 |
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
| Keyword index | `create_payload_index` | ❌ | ✅ v0.1 | Python README claims support but parser does not implement it |
| Integer index | `create_payload_index` | ❌ | ✅ v0.1 | Python README claims support but parser does not implement it |
| Float index | `create_payload_index` | ❌ | ✅ v0.1 | Python README claims support but parser does not implement it |
| Bool index | `create_payload_index` | ❌ | ✅ v0.1 | Python README claims support but parser does not implement it |
| Full-text index | `create_payload_index` (text) | ❌ | ⚠️ | `MATCH` works in both; explicit index creation only in Go |
| Geo index | `create_payload_index` (geo) | ❌ | ❌ | |
| Datetime index | `create_payload_index` (datetime) | ❌ | ❌ | |

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
| Python 1.4.0 | 1.13.0 | 1.13.x | 1.13.0 |
| Go 0.1.5 | 1.13.0 | 1.13.x | 1.13.0 |

### Language Support

| QQL Version | Python `qql-cli` | Go `qql-go` | Feature Parity |
|---|---|---|---|
| Current | 2.3.0 | 0.1.5 | ~95% |
| Target (Phase 1) | 2.3.0 | 0.2.0 | ~95% |
| Target (Phase 2) | 1.6.0 | 0.3.0 | ~98% |

---

## Inference Mode Compatibility

| Feature | Cloud | Local | External |
|---|---|---|---|
| Dense insert/search | ✅ | ✅ | ✅ |
| Hybrid insert/search | ✅ | ✅ | ✅ |
| Sparse-only search | ✅ | ✅ | ✅ |
| Rerank | ✅ (Python: local) | ✅ (Python only) | ❌ (Go) |
| Recommend | ✅ | ✅ | ✅ |

---

## Known Bugs / Mismatches

| Issue | Python | Go | Tracking |
|---|---|---|---|
| README shows `DELETE BY FILTER` but only ID deletion is implemented | Affected | Not affected — Go supports field deletion | #TBD |
| Local rerank not available in Go | N/A | Affected | #TBD |
| `qql-go` has no programmatic API equivalent to Python's `run_query()` | N/A | Affected | #TBD |

---

## How to Update This Document

When adding a new feature:
1. Update the relevant section above
2. Update the version matrix
3. Update the "Known Bugs" section if fixing a mismatch
4. Update the `Last updated` date at the top

This file is the **single source of truth** for QQL/Qdrant compatibility. Both Python and Go repos should keep their copies in sync.
