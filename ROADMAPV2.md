# QQL Roadmap V2 — Go Implementation

> **Generated:** 2026-06-15
> **Baseline:** Post red-team audit. All 30 critical bugs fixed. `go test`, `go vet`, `gofmt` clean.
> **Context:** This roadmap is grounded in the actual Qdrant `QueryPoints` gRPC API surface (go-client v1.18.2) and the current qql-go codebase state. Every gap below has been verified against the protobuf definitions.

---

## Status Summary

| Category | Implemented | Gap |
|----------|-------------|-----|
| Query modes (NEAREST/RECOMMEND/DISCOVER/CONTEXT) | ✅ All 4 | — |
| Fusion (RRF/DBSF) | ✅ | Parameterized RRF missing |
| MMR | ✅ | — |
| Rerank | ✅ cloud-only | Local rerank missing |
| GROUP BY + GROUP_SIZE | ✅ | — |
| OFFSET / SCORE THRESHOLD / LOOKUP FROM | ✅ | — |
| WHERE / WITH / EXACT / STRATEGY / MODEL | ✅ | — |
| Formula / Score boosting | ❌ | Entirely missing |
| ORDER BY | ❌ | No syntax |
| Sample (random) | ❌ | No syntax |
| WithPayload / WithVectors selectors | ❌ | No syntax |
| Per-prefetch filter/score/lookup | ❌ | No syntax |
| Parameterized RRF (K, weights) | ❌ | Only bare fusion |
| Relevance feedback | ❌ | No syntax |
| ExplainResult for QueryStmt | ⚠️ | Shows only mode/collection/limit |
| ReadConsistency / ShardKeySelector / Timeout | ❌ | No syntax |

---

## Phase 1 — Query Completeness

Features that fill gaps in the QUERY statement itself. These are the highest-value additions because they unlock capabilities users already have in the Qdrant API but can't express in QQL.

### 1. WithPayload / WithVectors Selectors `[OPEN]`

**Problem:** QQL hardcodes payload return behavior. Users cannot request vectors in query results, exclude specific payload fields, or include only certain fields. The Qdrant API supports `WithPayload` and `WithVectors` on every query.

**Qdrant API fields:**
- `QueryPoints.WithPayload` — `*WithPayloadSelector` (include/exclude fields, boolean toggle)
- `QueryPoints.WithVectors` — `*WithVectorsSelector` (boolean or named vector list)

**Proposed syntax:**
```sql
QUERY 'search text' FROM docs LIMIT 10
  WITH PAYLOAD {include: ["title", "url"], exclude: ["embedding"]}
  WITH VECTORS true

QUERY 'search text' FROM docs LIMIT 10
  WITH PAYLOAD false
  WITH VECTORS ["dense", "colbert"]
```

**Implementation:**
- Add `WithPayload` and `WithVectors` fields to `ast.SearchWith`
- Extend `parseWithClause` to handle `payload` and `vectors` keys
- Add `WithPayload`/`WithVectors` to `pipeline.QueryState`
- Wire in `BuildFlatRequest` and `BuildGroupedRequest`
- Update `executeFlatQuery`/`executeGroupedQuery` to pass through selectors to response formatting

**Touches:** `ast/ast.go`, `parser/parse_search.go`, `pipeline/pipeline.go`, `cli/commands/exec_query.go`, `cli/commands/utils.go`

---

### 2. ORDER BY Query Mode `[OPEN]`

**Problem:** Qdrant supports `QueryOrderBy` as a query variant that returns results ordered by a payload field instead of vector similarity. QQL has no syntax for this.

**Qdrant API:** `Query.order_by` — `*OrderBy` (field name, direction)

**Proposed syntax:**
```sql
QUERY ORDER BY 'created_at' FROM docs LIMIT 20
QUERY ORDER BY 'price' DESC FROM docs LIMIT 20
QUERY ORDER BY 'timestamp' ASC FROM logs LIMIT 100 WHERE level = 'error'
```

**Implementation:**
- Add `QueryModeOrderBy` to `ast.QueryMode`
- Parse `ORDER BY <field> [ASC|DESC]` in `parseQuery`
- Add `OrderByNode` pipeline node
- Wire `NewQueryOrderBy` in request builder

**Touches:** `ast/nodes.go`, `parser/parse_query.go`, `pipeline/nodes.go`, `cli/commands/exec_query.go`

---

### 3. SAMPLE RANDOM Query Mode `[OPEN]`

**Problem:** Qdrant supports `QuerySample` for random point sampling. Useful for exploration, testing, and dashboards.

**Qdrant API:** `Query.sample` — `Sample_Random`

**Proposed syntax:**
```sql
QUERY SAMPLE FROM docs LIMIT 10
QUERY SAMPLE FROM docs LIMIT 10 WHERE category = 'tech'
```

**Implementation:**
- Add `QueryModeSample` to `ast.QueryMode`
- Parse `SAMPLE` keyword in `parseQuery`
- Add `SampleNode` pipeline node
- Wire `NewQuerySample` in request builder

**Touches:** `ast/nodes.go`, `parser/parse_query.go`, `pipeline/nodes.go`, `cli/commands/exec_query.go`, `lexer/tokenkind.go`

---

### 4. Formula / Score Boosting `[OPEN]`

**Problem:** Qdrant's `FormulaQuery` enables payload-aware score shaping — freshness boosts, category weighting, geo-distance decay, conditional scoring. QQL has no syntax for this. Users are forced into external rerankers for what a formula can achieve.

**Qdrant API:** `Query.formula` — `*Formula` containing:
- `Expression` — 19 variants: constants, variables, arithmetic (`Sum`, `Mult`, `Div`, `Neg`, `Abs`, `Sqrt`, `Pow`, `Exp`, `Log10`, `Ln`), geo (`GeoDistance`), decay (`ExpDecay`, `GaussDecay`, `LinDecay`), conditional (`Condition`)
- `Defaults` — `map[string]*Value` for default variable values

**Proposed syntax (Phase 1 — common cases only):**
```sql
-- Score boost by payload field
QUERY 'search' FROM docs LIMIT 10
  BOOST { sum: [score, mult(0.5, match(tag, ["h1", "h2"]))] }

-- Geo-distance decay
QUERY 'search' FROM docs LIMIT 10
  BOOST { sum: [score, gauss_decay(geo_distance(origin, location_field), 5000)] }
```

**Implementation (staged):**
1. Define `FormulaExpr` AST nodes for the common subset: `Sum`, `Mult`, `Variable`, `Constant`, `Condition`, `GeoDistance`, decay functions
2. Parse a `BOOST` clause with a constrained expression grammar
3. Add `FormulaNode` pipeline node that builds `qdrant.Formula`
4. Wire `NewQueryFormula` in request builder

**Scoping guidance:** Start with score arithmetic (`sum`, `mult`, `$score` variable) and payload field conditions. Geo decay and full expression algebra are Phase 2.

**Touches:** `ast/nodes.go`, `ast/ast.go`, `parser/` (new file: `parse_formula.go`), `pipeline/nodes.go`, `cli/commands/exec_query.go`, `lexer/tokenkind.go`

---

## Phase 2 — Retrieval Quality

Features that improve search relevance and enable advanced retrieval patterns.

### 5. Parameterized RRF `[DONE]`

**Problem:** Qdrant supports `Rrf` as a structured query variant with `K` (rank constant) and `Weights` (per-prefetch source weights). QQL only uses the bare `Fusion_RRF` enum, which uses default K=60 and equal weights.

**Qdrant API:** `Query.rrf` — `*Rrf` with `K *uint32` and `Weights []float32`

**Proposed syntax:**
```sql
QUERY 'search' FROM docs LIMIT 10
  USING HYBRID
  WITH (rrf_k = 30, rrf_weights: [0.7, 0.3])
```

**Implementation:**
- Extend `SearchWith` with `RrfK` and `RrfWeights` fields
- Parse in `parseWithClause`
- In `FusionNode.Execute`, check if K or Weights are set and use `NewQueryRRF` instead of `NewQueryFusion`

**Touches:** `ast/ast.go`, `parser/parse_search.go`, `pipeline/nodes.go`

---

### 6. Per-Prefetch Filtering and Score Threshold `[OPEN]`

**Problem:** `PrefetchQuery` supports per-prefetch `Filter`, `ScoreThreshold`, and `LookupFrom`. QQL only sets these at the top-level request. This limits multi-stage retrieval — users can't apply different filters to different prefetch stages.

**Qdrant API fields on `PrefetchQuery`:**
- `Filter` — `*Filter`
- `ScoreThreshold` — `*float32`
- `LookupFrom` — `*LookupLocation`

**Proposed syntax:**
```sql
-- This requires manual prefetch DAG syntax, which is a larger feature.
-- Phase 1: expose via WITH { prefetch_filter: {...} } on hybrid queries.
-- Phase 2: full manual prefetch syntax.
```

**Guidance:** This is blocked by manual prefetch DAG construction (Phase 3). For now, the auto-generated hybrid prefetches use the top-level filter, which covers most cases.

---

### 7. Relevance Feedback Query `[OPEN]`

**Problem:** Qdrant's `RelevanceFeedbackInput` enables iterative refinement — feed back scored results to improve subsequent queries. QQL has no syntax.

**Qdrant API:** `Query.relevance_feedback` — `*RelevanceFeedbackInput` with `Target`, `Feedback` (scored items), `Strategy`

**Guidance:** This is an advanced feature. Defer until Formula and parameterized RRF are stable. The use case is narrow (RAG pipelines, iterative search).

---

## Phase 3 — Query Architecture

Features that change how queries are constructed, not just what fields they support.

### 8. Manual Prefetch DAG Construction `[DONE]`

**Problem:** QQL's pipeline auto-generates prefetches for hybrid search (dense + sparse + fusion). But the Qdrant API supports arbitrary nested prefetch DAGs for multi-stage retrieval. QQL has no syntax for manual prefetch construction.

**Qdrant API:** `PrefetchQuery` supports nested `Prefetch` arrays, enabling:
- Multi-stage retrieval (broad → narrow)
- Cross-collection prefetching
- Per-stage filtering and scoring
- ColBERT late-interaction re-ranking

**Proposed syntax (future):**
```sql
QUERY 'search' FROM docs LIMIT 10
  PREFETCH (
    QUERY 'search' FROM docs LIMIT 100 USING "dense" WHERE category = 'tech',
    QUERY 'search' FROM docs LIMIT 100 USING "sparse"
  )
  FUSION RRF
```

**Guidance:** This is a significant grammar addition. Start by exposing per-prefetch fields on the auto-generated hybrid prefetches (Phase 2 #6), then design the manual syntax.

---

### 9. Cross-Collection Lookup (`WITH LOOKUP`) `[OPEN]**

**Problem:** `QueryPointGroups.WithLookup` enables cross-collection group ID lookup — query collection A, but look up group IDs in collection B. QQL has no syntax.

**Qdrant API:** `QueryPointGroups.WithLookup` — `*WithLookup` with `Collection` and optional `Options`

**Proposed syntax:**
```sql
QUERY 'search' FROM docs LIMIT 10
  GROUP BY "category"
  GROUP_SIZE 5
  LOOKUP FROM metadata
```

**Guidance:** This extends the existing `GROUP BY` path. Low effort once `WithLookup` is added to `QueryPointGroups` builder.

---

## Phase 4 — Operational Completeness

Features that improve operational control and result formatting.

### 10. ReadConsistency Control `[OPEN]`

**Problem:** Qdrant supports `ReadConsistency` (majority, quorum, or specific node). QQL has no syntax. Useful for distributed deployments where consistency guarantees matter.

**Qdrant API:** `QueryPoints.ReadConsistency` — `*ReadConsistency`

**Proposed syntax:**
```sql
QUERY 'search' FROM docs LIMIT 10 WITH CONSISTENCY majority
```

---

### 11. Shard Key Selector `[OPEN]`

**Problem:** Qdrant supports targeting specific shards. QQL has no syntax. Useful for multi-tenant deployments with shard-per-tenant.

**Qdrant API:** `QueryPoints.ShardKeySelector` — `*ShardKeySelector`

**Guidance:** Low priority. Most users don't interact with shards directly.

---

### 12. Per-Request Timeout Override `[OPEN]`

**Problem:** Qdrant supports per-request timeout. QQL uses `defaultContext()` with a hardcoded 30s timeout. Users cannot override per-query.

**Qdrant API:** `QueryPoints.Timeout` — `*uint64` (milliseconds)

**Proposed syntax:**
```sql
QUERY 'search' FROM docs LIMIT 10 WITH TIMEOUT 5000
```

---

### 13. ExplainResult Completeness for QueryStmt `[OPEN]`

**Problem:** `ExplainResult` for `QueryStmt` only shows mode, collection, limit, and query text/ID. It omits 12+ fields: USING, WITH params, WITH MODEL, WHERE filter, OFFSET, SCORE THRESHOLD, LOOKUP FROM, GROUP BY, GROUP_SIZE, RERANK, STRATEGY, EXACT, context pairs, target, positive/negative IDs.

**Implementation:** Extend the `case *ast.QueryStmt` branch in `ExplainResult` (executor.go:621-628) to render all parsed fields.

**Touches:** `cli/commands/executor.go`

---

## Phase 5 — Collection & Index Polish

Items that broaden the collection management surface.

### 14. Broader Collection Config `[OPEN]`

**Status:** Current implementation covers: VECTORS, HNSW, OPTIMIZERS, PARAMS, QUANTIZE, ALTER COLLECTION. Remaining SDK models:

| SDK Model | Status | Priority |
|---|---|---|
| `WalConfig` | Not exposed | Medium — durability tuning |
| `StrictModeConfig` | Not exposed | Medium — rate limiting |
| `ShardKey / ShardingMethod` | Not exposed | Low — operational |
| `SparseVectorConfig` per-vector | Not exposed | Medium — sparse-only collections |
| `Bm25Config` | Now configurable via `Config` | ✅ Done |
| Per-vector `hnsw_config` / `quantization_config` | Not exposed | High — per-vector tuning |
| `MultiVectorConfig` general | Rerank-only | Low |

---

### 15. Dump / Restore Fidelity — Phase 2 `[OPEN]`

**Status:** Dumper preserves HNSW, OPTIMIZERS, PARAMS, VECTORS, QUANTIZE. Missing:
- Payload indexes (highest value)
- `USING MODEL` preservation
- Vector-preserving round trips (dump vectors, restore with vectors)

---

## Phase 6 — Platform

Features that expand qql-go as a platform.

### 16. Go Library API `[OPEN]`

**Problem:** `qql-go` has no programmatic API. Go applications cannot embed QQL as a library. The CLI is the only interface.

**Design:** Expose:
```go
package qql

func Parse(input string) (ast.ASTNode, error)
func Execute(ctx context.Context, client qdrant.Client, node ast.ASTNode) (*ExecResponse, error)
```

Keep the CLI as a thin wrapper over this library surface.

---

### 17. Local / External Rerank in Go `[OPEN]`

**Problem:** Rerank is cloud-only in Go. Python supports local FastEmbed cross-encoder rerank.

**Options:**
- Port FastEmbed cross-encoder logic to Go
- Integrate with an external rerank endpoint (e.g., Jina, Cohere)
- Use ONNX runtime in Go

---

### 18. Batch Query / Mutation `[OPEN]`

**Problem:** Qdrant supports `QueryBatch`, `SearchBatch`, `RecommendBatch`. QQL has no batch syntax. Useful for throughput-sensitive workloads.

**Guidance:** Defer until the programmatic API (#16) is designed. Batch operations are more natural as API calls than CLI syntax.

---

## Execution Order

```
Phase 1 — Query Completeness (high impact, moderate effort)
  1.  WithPayload / WithVectors selectors     [Most requested, small scope]
  2.  ORDER BY query mode                      [New retrieval primitive]
  3.  SAMPLE RANDOM query mode                 [Small, self-contained]
  4.  Formula / score boosting (Phase 1)       [Largest Phase 1 item]

Phase 2 — Retrieval Quality (advanced features)
  5.  Parameterized RRF (K, weights)           [DONE]
  6.  Per-prefetch filter/score threshold      [Blocked by #8]
  7.  Relevance feedback query                 [Defer — narrow use case]

Phase 3 — Query Architecture (structural)
  8.  Manual prefetch DAG construction         [DONE]
  9.  Cross-collection lookup (WITH LOOKUP)    [Small, extends GROUP BY]

Phase 4 — Operational Completeness
  10. ReadConsistency control                  [Small]
  11. ShardKeySelector                         [Low priority]
  12. Per-request timeout override             [Small]
  13. ExplainResult completeness for QueryStmt [Pure polish]

Phase 5 — Collection & Index Polish
  14. Per-vector HNSW/quantization overrides   [High value]
  15. WalConfig / StrictModeConfig             [Medium value]
  16. Dump fidelity Phase 2 (payload indexes)  [High value]

Phase 6 — Platform
  17. Go library API                           [Foundation for embedding]
  18. Local / external rerank                  [Close Python parity]
  19. Batch query / mutation                   [Throughput]
```

---

## Python ↔ Go Parity Tracking

| Area | Python | Go | Notes |
|------|--------|-----|-------|
| Collection CRUD + ALTER | ✅ | ✅ | Parity |
| Config blocks (VECTORS/HNSW/OPTIMIZERS/PARAMS) | ✅ | ✅ | Parity |
| Quantization (create + alter + disable) | ✅ | ✅ | Parity |
| SHOW COLLECTION diagnostics | ✅ | ✅ | Parity |
| Dumper fidelity | ✅ | ✅ | Parity |
| Script runner | ✅ | ✅ | Parity |
| Hybrid detection | ✅ | ✅ | Parity |
| QUERY modes (4) | ✅ | ✅ | Parity |
| GROUP BY + GROUP_SIZE | ✅ | ✅ | Parity |
| OFFSET / SCORE THRESHOLD / LOOKUP FROM | ✅ | ✅ | Parity |
| MMR | ✅ | ✅ | Parity |
| Rerank | ✅ local+cloud | ⚠️ cloud-only | Go gap |
| Formula / score boosting | ❌ | ❌ | Both missing |
| ORDER BY | ❌ | ❌ | Both missing |
| SAMPLE RANDOM | ❌ | ❌ | Both missing |
| WithPayload/WithVectors selectors | ❌ | ❌ | Both missing |
| Parameterized RRF | ❌ | ✅ | Python gap |
| Programmatic API | ✅ `run_query()` | ❌ | Go gap |

---

*Last updated: 2026-06-15*
