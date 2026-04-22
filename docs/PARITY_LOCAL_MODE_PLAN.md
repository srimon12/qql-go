# QQL-Go Parity And Local-Mode Plan

## Purpose

This document compares the current Go port in `D:\Sativa\qql-go` against the Python source of truth in `D:\Sativa\qql`, using:

- Python release baseline: `97f7f1b99ac6f60feb657b9b43b3d906ad1b9302`
- Python latest feature commit called out for parity: `e95465dd39bc3ab962ecc9cb08af6c9f62021bd4`

It also defines the implementation plan for moving `qql-go` from Qdrant Cloud inference to a local-first model with:

- OpenAI-compatible dense embedding APIs, local or remote
- local BM25 / sparse vector generation
- upsert and search against local/self-hosted Qdrant

## Assumptions

- `D:\Sativa\qql` is the Python source-of-truth feature set.
- `D:\Sativa\qql-go` is the implementation target.
- We want real parity with Python where it matters, not just syntax parity.
- We should preserve the good parts of `qql-go` that are already better than Python.
- For local mode, the minimal correct design is to generate dense and sparse vectors in the client and send explicit vectors to Qdrant, instead of sending `qdrant.Document` objects and relying on Qdrant Cloud inference.
- For local BM25 parity, query-side sparse generation alone is not enough if inserts also move local; document-side sparse vectors need corpus-aware weighting or a clearly defined approximation.

## High-Level Read

`qql-go` is not in full parity with Python yet.

It is ahead of Python in CLI ergonomics and operational polish:

- structured `--json` and `--quiet --json` output
- `doctor`
- `version`
- `CREATE INDEX`
- delete-by-field
- cleaner explain-plan flow
- stronger installation and release packaging

It is behind Python in core QQL capability and local execution:

- no `RECOMMEND`
- no `INSERT BULK`
- no explicit insert `id` support
- no `USING SPARSE` search path
- no `CREATE COLLECTION ... USING MODEL`
- no `EXECUTE` script command
- no `DUMP COLLECTION`
- no local dense embedding path
- no local sparse/BM25 generation path
- no local rerank path

## Source References

### Python source of truth

- `D:\Sativa\qql\src\qql\parser.py`
- `D:\Sativa\qql\src\qql\executor.py`
- `D:\Sativa\qql\src\qql\cli.py`
- `D:\Sativa\qql\src\qql\script.py`
- `D:\Sativa\qql\src\qql\dumper.py`
- `D:\Sativa\qql\README.md`

### Go implementation today

- `D:\Sativa\qql-go\internal\lexer\tokenkind.go`
- `D:\Sativa\qql-go\internal\parser\parser.go`
- `D:\Sativa\qql-go\internal\ast\nodes.go`
- `D:\Sativa\qql-go\internal\cli\commands\commands.go`
- `D:\Sativa\qql-go\internal\repl\repl.go`
- `D:\Sativa\qql-go\README.md`

### Rust local sparse / hybrid references

- `D:\Sativa\repo-cli\repo-chunker-rs\search-core\src\sparse.rs`
- `D:\Sativa\repo-cli\repo-chunker-rs\search-core\src\ops\query.rs`
- `D:\Sativa\repo-cli\repo-chunker-rs\search-core\src\sync_ops\embed.rs`

## Parity Matrix

| Area | Python `qql` | Go `qql-go` | Status |
|---|---|---|---|
| Dense insert/search | Yes, local embedding path | Yes, but cloud inference path | Partial |
| Hybrid insert/search | Yes | Yes, but cloud inference path | Partial |
| Rerank | Yes | Yes, but cloud inference path and collection contract differs | Partial |
| Query-time params `EXACT` / `WITH` | Yes | Yes | Near parity |
| `RECOMMEND` | Yes | No | Missing |
| `INSERT BULK` | Yes | No | Missing |
| Explicit point `id` on insert | Yes | No | Missing |
| Sparse-only search | Yes | No | Missing |
| `CREATE COLLECTION ... USING MODEL` | Yes | No | Missing |
| `EXECUTE` script flow | Yes | No | Missing |
| `DUMP COLLECTION` | Yes | No | Missing |
| `CREATE INDEX` | No | Yes | Go better |
| Delete by field | Limited | Yes | Go better |
| Structured JSON output | No | Yes | Go better |
| `doctor` / `version` | No | Yes | Go better |

## Important Behavioral Gaps

### 1. Local execution is the biggest real gap

Python executes dense, sparse, and rerank locally through embedders. Go still relies on Qdrant inference by building `qdrant.Document` queries in:

- `internal/cli/commands/commands.go`
  - `buildPointVectors(...)`
  - `buildSearchRequest(...)`
  - `buildSearchPrefetches(...)`

This is the main architectural blocker for self-hosted/local parity.

### 2. `RECOMMEND` is missing entirely

Python already has:

- AST support
- parser support
- executor support
- tests

Go has no lexer tokens, AST node, parser branch, or executor path for recommendation queries.

### 3. Bulk ingestion and explicit IDs are missing

Python supports:

- `INSERT BULK ... VALUES [ ... ]`
- per-item validation
- explicit `id`
- stripping `id` from payload and using it as point ID

Go only supports one-row insert and always generates a UUID.

### 4. Sparse-only retrieval is missing

Python supports `USING SPARSE`. Go only supports dense and hybrid.

### 5. Script round-trip workflows are missing

Python supports:

- `EXECUTE`
- `DUMP COLLECTION`
- runnable sample script flows

Go currently lacks that shell-level workflow.

## Where Go Is Already Better

These are not regressions and should be preserved:

- machine-readable CLI output
- cleaner automation surface for agents
- index creation support
- doctor/version UX
- better release/install story
- richer filter surface in docs and tests

The parity plan should not erase these advantages just to look like Python.

## Local-Mode Design Direction

## Decision

Use a client-side vector generation architecture in `qql-go`.

That means:

- dense vectors come from an OpenAI-compatible embeddings API
- sparse vectors come from local BM25/hash-based generation in Go
- Qdrant receives explicit vectors, not document inference requests

This is the simplest design that matches your stated direction and the existing seams in `commands.go`.

## Why this is the right shape

- It keeps the existing parser and execution flow mostly intact.
- It removes the cloud-only inference dependency.
- It supports both local and remote embedding providers behind one interface.
- It aligns with the Rust reference design without over-copying unrelated retrieval orchestration.

## Non-goals for the first implementation

- no large retrieval framework rewrite
- no speculative plugin system
- no new query language beyond parity needs
- no graph-aware rerank/orchestration from `repo-cli`
- no extra collection-management features beyond what parity/local mode needs

## Rust Learnings To Reuse

### Reuse directly as concepts

- a small embedding client adapter with:
  - base URL
  - model
  - expected dimension
  - request timeout
  - strict dimension validation
- shared tokenization for sparse corpus and sparse query generation
- explicit sparse vector payloads shaped as:
  - `indices`
  - `values`
- hybrid query construction as:
  - dense prefetch
  - sparse prefetch
  - RRF fusion

### Do not copy blindly

- do not cargo-cult the Rust token hash implementation
- do not import graph-aware query orchestration into QQL
- do not introduce repo-cli-specific seeds or rerank stages into this CLI

## Proposed Architecture Changes In Go

### 1. Split inference from query planning

Create a narrow internal package for vector generation, for example:

- `internal/embedding`
- `internal/sparse`

Responsibilities:

- `internal/embedding`
  - OpenAI-compatible embeddings client
  - model + dimension validation
  - batch embed for inserts
  - single-query embed for search

- `internal/sparse`
  - tokenization
  - sparse query vector generation
  - sparse document vector generation
  - corpus stats storage contract

### 2. Add an inference mode abstraction

Today the docs mention `cloud`, `external`, and `local`, but the code path is still effectively cloud-only for text operations.

Add an explicit runtime mode in config:

- `cloud`
- `external`
- `local`

With behavior:

- `cloud`
  - keep current Qdrant inference path for backwards compatibility
- `external`
  - dense embeddings from OpenAI-compatible API
  - sparse vectors generated locally
- `local`
  - same vector-generation path as `external`, but default endpoint/config points at local services where applicable

Important: `external` and `local` can share almost all code. The difference is config defaults, not architecture.

### 3. Replace document inference in the current execution seams

Change these main seams in `internal/cli/commands/commands.go`:

- `doInsert(...)`
- `doSearch(...)`
- `buildPointVectors(...)`
- `buildSearchRequest(...)`
- `buildSearchPrefetches(...)`

Target behavior:

- cloud mode:
  - keep `qdrant.Document` path
- external/local mode:
  - build explicit dense vectors
  - build explicit sparse vectors
  - upsert/search against named vectors directly

### 4. Preserve named vector compatibility

Keep existing vector names:

- `dense`
- `sparse`
- `colbert` if rerank remains enabled

Do not change collection vector names during this migration.

### 5. Add config fields needed for real local mode

Current config is too small. Add only the fields required to execute the plan:

- inference mode
- embedding base URL
- embedding model
- embedding dimension
- embedding timeout
- sparse dimensions or hash space size
- optional rerank base URL/model only if rerank is kept in scope for this phase

## Sparse/BM25 Implementation Notes

This is the part that needs the most care.

### Query-side sparse generation

This is straightforward and should be first:

- tokenize query text
- normalize/lowercase consistently
- hash tokens into sparse dimensions
- weight query terms consistently
- emit Qdrant sparse vector payload

### Document-side sparse generation

If local insert is required, this is not optional.

You need one of these two approaches:

### Option A: true BM25-style corpus-aware sparse vectors

Pros:

- closer to Python intent
- better retrieval quality

Cons:

- requires corpus statistics
- harder to keep correct on incremental upsert/delete

### Option B: stable hashed sparse vectors with local term weighting

Pros:

- much simpler
- easier incremental behavior
- good first step if we need to move fast

Cons:

- not true BM25 parity
- likely weaker retrieval quality than a corpus-aware implementation

## Recommendation

Do this in two phases:

1. first ship query-side + document-side hashed sparse vectors for local mode
2. then decide whether quality requires full corpus-aware BM25 stats

This is the simpler path. It gets local hybrid working without blocking the whole migration on a perfect sparse index design.

If you want strict “BM25” semantics from day one, we should explicitly accept the extra state-management complexity now.

## Feature Plan

## Phase 0: Freeze the parity target

Goal:

- define exactly what “parity” means for the next implementation cycle

Deliverables:

- this document
- accepted parity scope

Verification:

- team agrees on must-have vs later features

## Phase 1: Close syntax and feature gaps that are independent of local mode

Implement:

- `RECOMMEND`
- explicit insert `id`
- `INSERT BULK`
- `CREATE COLLECTION ... USING MODEL`
- `USING SPARSE`

Maybe later in this phase, only if still desired:

- `EXECUTE`
- `DUMP COLLECTION`

Verification:

- parser tests for each new statement
- executor tests for each new behavior
- README and skill docs updated together

## Phase 2: Introduce vector-generation interfaces

Implement:

- embedding client package
- sparse generation package
- runtime mode config
- tests for vector generation and config parsing

Verification:

- unit tests for embedding response validation
- unit tests for sparse tokenization and vector emission
- no behavior change yet for default cloud mode

## Phase 3: Make insert local-capable

Implement:

- explicit dense vector generation for insert
- explicit sparse vector generation for hybrid insert
- correct handling of explicit point IDs
- batch insert path for `INSERT BULK`

Verification:

- executor tests for dense insert in non-cloud mode
- executor tests for hybrid insert in non-cloud mode
- tests for explicit IDs and payload stripping

## Phase 4: Make search local-capable

Implement:

- dense query embedding
- sparse query generation
- dense-only search
- sparse-only search
- hybrid RRF search
- preserve `EXACT` and `WITH` behavior

Verification:

- executor tests for dense, sparse, and hybrid search in non-cloud mode
- query-time params forwarded correctly
- parity checks against Python behavior where semantics are intended to match

## Phase 5: Add `RECOMMEND` execution

Implement:

- parser and AST support
- positive IDs
- optional negative IDs
- optional strategy
- seed-ID exclusion in results

Verification:

- parser tests
- executor tests
- explain output updated

## Phase 6: Decide rerank scope

There are two valid options:

### Option 1: keep rerank cloud-only for now

Pros:

- smaller scope
- faster delivery of local dense + local hybrid

Cons:

- parity still incomplete for full local mode

### Option 2: add rerank provider abstraction

Pros:

- cleaner long-term architecture
- true local/external rerank path possible

Cons:

- more scope immediately

## Recommendation

Do Option 1 first unless rerank is a release blocker.

Dense + hybrid + recommend + bulk insert + explicit IDs are higher-value parity items.

## Phase 7: Script parity

Implement:

- `EXECUTE`
- `DUMP COLLECTION`

Verification:

- round-trip script tests
- dump output re-executes successfully

## Recommended Implementation Order

1. `RECOMMEND` parser + AST + executor
2. explicit insert `id`
3. `INSERT BULK`
4. `CREATE COLLECTION ... USING MODEL`
5. `USING SPARSE`
6. embedding client abstraction
7. sparse generation package
8. local/external insert path
9. local/external search path
10. rerank decision
11. `EXECUTE`
12. `DUMP COLLECTION`

## Why this order

- It lands obvious parity wins early.
- It separates syntax work from infrastructure work.
- It avoids mixing local-mode plumbing with every feature gap at once.
- It keeps tests and docs manageable.

## Verification Plan

For each feature, use the project rule:

1. add or update tests that express the expected behavior
2. implement the smallest code change that makes them pass
3. update docs only for shipped behavior

Minimum checks per milestone:

- `go test ./...`
- parser coverage for new grammar
- executor coverage for real behavior changes
- README examples for every user-visible feature added

## Risks

### Sparse quality risk

If local sparse generation is too naive, hybrid quality may look worse than cloud inference.

Mitigation:

- keep sparse implementation isolated
- add deterministic tests
- compare a few fixed corpora against Python behavior before calling parity complete

### Config sprawl risk

Too many knobs will make the CLI harder to use.

Mitigation:

- add only required fields
- keep sane defaults
- avoid user-facing configuration for features we do not actually support yet

### Backwards-compatibility risk

Changing vector names or default collection layout will break existing users.

Mitigation:

- preserve current named vectors
- keep cloud mode working during migration

## Success Criteria

We should call this effort successful when:

- `qql-go` supports all core Python QQL statements that matter for day-to-day usage
- local/self-hosted Qdrant can do text insert and text search without Qdrant Cloud inference
- hybrid dense+sparse works without cloud inference
- recommendation queries work
- bulk insert and explicit IDs work
- docs describe the real boundary accurately
- Go-only operational advantages are preserved

## Suggested Immediate Next Step

Start with a narrow parity patch set before the local-mode plumbing:

1. add `RECOMMEND` syntax and executor
2. add explicit insert `id`
3. add `INSERT BULK`

Then start the local-mode infrastructure behind a small embedding interface and sparse package.
