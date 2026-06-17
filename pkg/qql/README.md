# QQL Go SDK (`pkg/qql`)

Go library for executing QQL queries against Qdrant. Embed QQL in your Go applications without the CLI.

## Install

```bash
go get github.com/srimon12/qql-go/pkg/qql
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    "github.com/srimon12/qql-go/pkg/qql"
)

func main() {
    client, _ := qql.NewQdrantClient(qql.ClientConfig{
        URL: "http://localhost:6334",
    })
    ctx := context.Background()

    // Simple search
    res, _ := qql.Exec(ctx, client, "QUERY 'emergency triage' FROM docs LIMIT 5 USING HYBRID")
    fmt.Println(res.Data)

    // Explain without executing
    plan, _ := qql.Explain("QUERY 'test' FROM docs LIMIT 5 BOOST (score * 2.0)")
    fmt.Println(plan)
}
```

## API

### `Parse(input string) (ast.ASTNode, error)`

Parse QQL into an AST node without executing. No Qdrant client needed.

```go
node, err := qql.Parse("QUERY 'search' FROM docs LIMIT 10 USING HYBRID")
// node is *ast.QueryStmt
```

### `Exec(ctx, client, query) (*Result, error)`

Execute a single QQL query.

```go
res, err := qql.Exec(ctx, client, "QUERY 'stroke' FROM medical LIMIT 5")
// res.OK, res.Operation, res.Message, res.Data
```

### `ExecBatch(ctx, client, queries, stopOnError) ([]*Result, error)`

Execute multiple queries in sequence.

```go
results, err := qql.ExecBatch(ctx, client, []string{
    "INSERT INTO docs VALUES {'text': 'hello'}",
    "QUERY 'hello' FROM docs LIMIT 5",
}, true)
```

### `Explain(query) (string, error)`

Return the execution plan without running.

```go
plan, err := qql.Explain("QUERY 'test' FROM docs LIMIT 5 BOOST (CASE WHEN priority = 'high' THEN 2.0 ELSE 1.0 END)")
```

## Production Examples

### Hybrid Search with Score Boosting

```go
query := `
WITH
  dense AS (QUERY 'kubernetes deployment' USING dense LIMIT 200 WHERE priority = 'high'),
  sparse AS (QUERY 'kubernetes deployment' USING sparse LIMIT 300)
QUERY 'kubernetes deployment' FROM incidents LIMIT 10
  PREFETCH (dense SCORE THRESHOLD 0.6, sparse SCORE THRESHOLD 0.3)
  FUSION RRF
  WITH (rrf_k = 20, rrf_weights = [0.6, 0.4])
  BOOST (CASE WHEN priority = 'critical' THEN score * 2.0 ELSE score END)
`
res, _ := qql.Exec(ctx, client, query)
```

### Batch Ingest + Search

```go
queries := []string{
    "CREATE COLLECTION docs HYBRID WITH HNSW (m = 32) WITH QUANTIZATION (type = 'turbo', bits = 2)",
    "CREATE INDEX ON COLLECTION docs FOR category TYPE keyword",
    "INSERT INTO docs VALUES {'text': 'first doc', 'category': 'tech'} USING HYBRID",
    "QUERY 'first doc' FROM docs LIMIT 5",
}
results, _ := qql.ExecBatch(ctx, client, queries, true)
```

### Random Sampling for Dashboards

```go
res, _ := qql.Exec(ctx, client, "QUERY SAMPLE FROM docs LIMIT 20 WHERE status = 'active'")
```

### Paginated Browse by Field

```go
res, _ := qql.Exec(ctx, client, "QUERY ORDER BY created_at DESC FROM docs LIMIT 20 OFFSET 40")
```

## Interface

The `QdrantClient` interface allows injecting mock clients for testing:

```go
type QdrantClient interface {
    ListCollections(context.Context) ([]string, error)
    CollectionExists(context.Context, string) (bool, error)
    GetCollectionInfo(context.Context, string) (*qdrant.CollectionInfo, error)
    CreateCollection(context.Context, *qdrant.CreateCollection) error
    UpdateCollection(context.Context, *qdrant.UpdateCollection) error
    DeleteCollection(context.Context, string) error
    Upsert(context.Context, *qdrant.UpsertPoints) (*qdrant.UpdateResult, error)
    Query(context.Context, *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error)
    QueryGroups(context.Context, *qdrant.QueryPointGroups) ([]*qdrant.PointGroup, error)
    Delete(context.Context, *qdrant.DeletePoints) (*qdrant.UpdateResult, error)
    UpdateVectors(context.Context, *qdrant.UpdatePointVectors) (*qdrant.UpdateResult, error)
    SetPayload(context.Context, *qdrant.SetPayloadPoints) (*qdrant.UpdateResult, error)
    CreateFieldIndex(context.Context, *qdrant.CreateFieldIndexCollection) (*qdrant.UpdateResult, error)
    Count(context.Context, *qdrant.CountPoints) (uint64, error)
    ScrollAndOffset(context.Context, *qdrant.ScrollPoints) ([]*qdrant.RetrievedPoint, *qdrant.PointId, error)
    Get(context.Context, *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error)
}
```
