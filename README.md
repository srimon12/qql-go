# qql-go

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go 1.24+](https://img.shields.io/badge/Go-1.24%2B-00ADD8.svg)](https://go.dev/)
[![Version](https://img.shields.io/badge/Version-0.4.0-blue.svg)](VERSION)
[![Platforms](https://img.shields.io/badge/Platforms-Windows%20%7C%20Linux%20%7C%20macOS-blue.svg)](https://github.com/srimon12/qql-go/releases)

A single-binary CLI, SDK, and gateway for operating Qdrant vector databases with a SQL-like query language.

```bash
qql-go exec "QUERY 'emergency care' FROM medical LIMIT 5 USING HYBRID RERANK"
```

## What it is

`qql-go` is a compact operational interface for Qdrant. It gives developers, CI pipelines, and AI agents one deterministic command surface for collection management, vector search, score shaping, and data operations — without writing SDK code for every task.

Use the Qdrant SDK when you're building application logic. Use `qql-go` when you need repeatable commands, stable JSON output, version-controlled scripts, and a binary that runs anywhere.

## Install

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/srimon12/qql-go/main/install.sh | sh

# Windows
irm https://raw.githubusercontent.com/srimon12/qql-go/main/install.ps1 | iex

# From source
go install github.com/srimon12/qql-go/cmd/qql-go@latest
```

## Quick start

```bash
# Connect to Qdrant
qql-go connect --url http://localhost:6334

# Create a collection with hybrid search
qql-go exec "CREATE COLLECTION docs HYBRID"

# Insert documents
qql-go exec "INSERT INTO docs VALUES {'id': 1, 'text': 'Qdrant stores vectors', 'topic': 'search'} USING HYBRID"

# Search
qql-go exec "QUERY 'vector database' FROM docs LIMIT 5 USING HYBRID"

# Explain without executing
qql-go explain "QUERY 'vector database' FROM docs LIMIT 5 USING HYBRID RERANK"
```

## Features

### Retrieval modes

Dense, sparse, hybrid, recommendation, context-aware, discover, order-by, and random sampling — all through a unified `QUERY` statement:

```sql
-- Dense search
QUERY 'emergency care' FROM docs LIMIT 5

-- Hybrid with parameterized RRF
QUERY 'emergency care' FROM docs LIMIT 5 USING HYBRID WITH (rrf_k = 30, rrf_weights = [0.7, 0.3])

-- Multi-stage retrieval with CTEs
WITH dense AS (QUERY 'care' USING dense LIMIT 200),
     sparse AS (QUERY 'care' USING sparse LIMIT 300)
QUERY 'care' FROM docs LIMIT 10 PREFETCH (dense, sparse) FUSION RRF

-- Recommendation by example
QUERY RECOMMEND WITH (positive = ('id-1', 'id-2'), negative = ('id-3')) FROM docs LIMIT 5

-- Score shaping with BOOST
QUERY 'emergency' FROM docs LIMIT 10
  BOOST (CASE WHEN priority = 'high' THEN 2.0 ELSE 1.0 END + GAUSS_DECAY(date, target: datetime('2026-01-01'), scale: 30d))
```

### Collection management

```sql
CREATE COLLECTION docs HYBRID
  WITH HNSW (m = 32, ef_construct = 100)
  WITH QUANTIZATION (type = 'scalar', quantile = 0.95, always_ram = true)

ALTER COLLECTION docs WITH VECTORS (on_disk = true)

CREATE INDEX ON docs FOR tags TYPE keyword WITH (is_tenant = true)

SHOW COLLECTION docs
```

### Data operations

```sql
INSERT INTO docs VALUES {'id': 1, 'text': 'hello', 'topic': 'search'} USING HYBRID
SELECT * FROM docs WHERE id = 1
SCROLL FROM docs WHERE topic = 'search' LIMIT 10
UPDATE docs SET PAYLOAD = {'status': 'reviewed'} WHERE id = 1
DELETE FROM docs WHERE status = 'archived'
```

### Score boosting (BOOST)

Full expression algebra executed directly inside Qdrant:

```sql
QUERY 'emergency care' FROM docs LIMIT 10
  BOOST (
    $score * 2
    + CASE WHEN priority = 'high' THEN 10 ELSE 0 END
    + ABS(GEO_DISTANCE({lat: 40.7, lon: -74.0}, location)) * -0.1
  )
  DEFAULTS (priority_weight = 1.5)
```

Arithmetic, math functions (`ABS`, `SQRT`, `LOG`, `LN`, `EXP`, `POW`), geo-distance, decay functions, datetime expressions, and `CASE WHEN` conditionals.

### Filtering

```sql
WHERE status = 'active' AND year >= 2024
WHERE priority IN ('high', 'medium') AND tags IS NOT NULL
WHERE (team = 'search' OR team = 'infra') AND severity BETWEEN 3 AND 5
WHERE title MATCH PHRASE 'vector search'
```

## SDKs

### Go (`pkg/qql`)

```go
import "github.com/srimon12/qql-go/pkg/qql"

client, _ := qql.NewQdrantClient(qql.ClientConfig{URL: "localhost:6334"})
result, _ := qql.Exec(ctx, client, "QUERY 'search' FROM docs LIMIT 5")
```

### Python

```python
from qql import QQLClient
client = QQLClient("http://localhost:50051")
result = client.exec("QUERY 'search' FROM docs LIMIT 5")
```

### TypeScript

```typescript
import { QQLClient } from "qql"
const client = new QQLClient("http://localhost:50051")
const result = await client.exec("QUERY 'search' FROM docs LIMIT 5")
```

## Gateway (Connect RPC)

Start a gRPC-compatible server for remote access:

```bash
qql-go gateway --listen :50051 --qdrant-url http://localhost:6334
```

Exposes `Exec`, `ExecBatch`, `Explain`, and `Health` via Connect RPC (gRPC + gRPC-Web + HTTP/1.1). Auto-detects pure QUERY batches and routes to Qdrant's native `QueryBatch` API for single round-trip execution.

## Agent and scripting mode

```bash
# Structured JSON output for agents and CI
qql-go exec --quiet --json "SHOW COLLECTIONS"
qql-go explain --quiet --json "QUERY 'search' FROM docs LIMIT 5"
qql-go doctor --quiet --json

# Script execution
qql-go execute workflow.qql
qql-go execute --stop-on-error workflow.qql

# Collection dump
qql-go dump docs backup.qql
```

## Documentation

| | |
|---|---|
| [Syntax Reference](docs/syntax.md) | Complete QQL statement reference |
| [Filter Reference](docs/filters.md) | WHERE clause predicates and operators |
| [Release Notes](docs/releases/) | Per-version release notes |
| [CHANGELOG.md](CHANGELOG.md) | User-facing changes |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Contributor guide |
| [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) | Maintainer and release workflow |

## Examples

First-class operational examples under [`examples/`](examples/):

- **release-validation** — retrieval regression checks, fits into CI
- **medical-retrieval-ops** — vertical benchmark workflow
- **retrieval-debug-runbook** — debugging and diagnostics

Agent-oriented demos under [`skills/qql-skill/scripts/`](skills/qql-skill/scripts/).

## Architecture

```
┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────────┐
│  Lexer   │───>│  Parser  │───>│   AST    │───>│   Executor   │
│          │    │ (Pratt)  │    │          │    │              │
│ O(1)     │    │ Zero-alloc│    │ Typed    │    │ Pipeline DAG │
│ keywords │    │ compare  │    │ nodes    │    │              │
└──────────┘    └──────────┘    └──────────┘    └──────┬──────┘
                                                       │
                              ┌─────────────────────────┼───────┐
                              │                         │       │
                     ┌────────▼───┐   ┌─────────┐  ┌───▼────┐  │
                     │  Pipeline  │   │ Filters │  │ Sparse │  │
                     │            │   │         │  │  BM25  │  │
                     │ EmbedNode  │   │ Type-   │  │ Atomic │  │
                     │ FusionNode │   │ switch  │  │ cache  │  │
                     │ RerankNode │   │ (no     │  └────────┘  │
                     │ FormulaNode│   │reflect) │              │
                     └─────┬──────┘   └─────────┘              │
                           │                                    │
                    ┌──────▼────────────────────────────────────┘
                    │
              ┌─────▼─────┐    ┌───────────┐
              │   Qdrant   │    │  Gateway  │
              │  gRPC API  │    │ (Connect  │
              │            │    │   RPC)    │
              └───────────┘    └───────────┘
```

## Performance

Benchmarked on i5-10400F, Go 1.24:

| Operation | Latency | Allocs |
|---|---|---|
| Lex (simple) | 304 ns/op | 2 |
| Lex (full query) | 945 ns/op | 2 |
| Parse (simple) | 477 ns/op | 4 |
| Parse (full query) | 1,470 ns/op | 8 |

Lexer uses O(1) stack-buffer keyword lookup. Parser uses zero-allocation byte-level comparison. Filters use type-switch instead of `reflect`. BM25 params cached with `atomic.Pointer`.

## License

[MIT](LICENSE)
