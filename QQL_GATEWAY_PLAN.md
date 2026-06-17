# QQL Gateway — Full Plan

> A query gateway that sits in front of Qdrant and exposes QQL as a Connect RPC service.
> Any language can send QQL strings over HTTP. The gateway parses, validates, and executes against Qdrant.

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    qql-go                                │
│                                                         │
│  ┌──────────┐  ┌──────────────┐  ┌──────────┐          │
│  │ CLI      │  │ Connect RPC  │  │ Library  │          │
│  │ binary   │  │ server       │  │ pkg/qql  │          │
│  └────┬─────┘  └──────┬───────┘  └────┬─────┘          │
│       │               │               │                 │
│       └───────────────┼───────────────┘                 │
│                       │                                 │
│                ┌──────┴──────┐                          │
│                │  internal/  │  ← existing code, unchanged│
│                │  parser     │                          │
│                │  pipeline   │                          │
│                │  executor   │                          │
│                └──────┬──────┘                          │
│                       │                                 │
│                ┌──────┴──────┐                          │
│                │  Qdrant     │                          │
│                │  gRPC conn  │                          │
│                └─────────────┘                          │
└──────────────────────────────────────────────────────────┘
```

## What gets built

### Phase 1: Proto + pkg/qql (this session)

1. **proto/qql.proto** — Connect RPC service definition
2. **buf.yaml + buf.gen.yaml** — buf codegen config
3. **pkg/qql/parse.go** — `Parse(input) (ASTNode, error)`
4. **pkg/qql/exec.go** — `Exec(ctx, client, query) (*Result, error)`
5. **pkg/qql/batch.go** — `ExecBatch(ctx, client, queries) ([]*Result, error)`
6. **pkg/qql/explain.go** — `Explain(input) (string, error)`
7. **pkg/qql/types.go** — public types (Result, Error, etc.)
8. **pkg/qql/client.go** — `NewQdrantClient(url, opts) (Client, error)` — wraps qdrant client creation

### Phase 2: Connect RPC server (this session)

9. **server/handler.go** — implements `qqlpb.QQLHandler` (Exec, ExecBatch, Explain, Health)
10. **server/server.go** — HTTP server setup with Connect mux
11. **cmd/qql-go/serve.go** — `qql-go serve` command (cobra)

### Phase 3: Generated clients (next session)

12. **buf generate** — Go stubs in gen/qqlpb/
13. **sdks/python/** — pip-installable Python client
14. **sdks/typescript/** — npm-installable TS client

### Phase 4: Gateway features (future)

15. Connection pooling tuning
16. Auth middleware (API key, JWT)
17. Query logging + metrics
18. Rate limiting
19. Query caching (same string → cached result)
20. Streaming for large result sets

## Proto definition

```protobuf
syntax = "proto3";
package qql;

service QQL {
  rpc Exec(ExecRequest) returns (ExecResponse);
  rpc ExecBatch(ExecBatchRequest) returns (ExecBatchResponse);
  rpc Explain(ExplainRequest) returns (ExplainResponse);
  rpc Health(HealthRequest) returns (HealthResponse);
}
```

## pkg/qql public API

```go
package qql

// Parse turns a QQL string into an AST node. No client needed.
func Parse(input string) (ast.ASTNode, error)

// Explain returns the execution plan for a QQL query.
func Explain(input string) (string, error)

// Exec parses and executes a QQL query against a Qdrant client.
func Exec(ctx context.Context, client QdrantClient, query string) (*Result, error)

// ExecBatch executes multiple queries. stopOnError controls early termination.
func ExecBatch(ctx context.Context, client QdrantClient, queries []string, stopOnError bool) ([]*Result, error)

// Result is the outcome of a single query execution.
type Result struct {
    OK        bool
    Operation string
    Message   string
    Data      any
}

// QdrantClient is the interface pkg/qql needs from a Qdrant connection.
// Implemented by *qdrant.Client and by the test fake.
type QdrantClient interface {
    ListCollections(ctx context.Context) ([]string, error)
    CollectionExists(ctx context.Context, name string) (bool, error)
    GetCollectionInfo(ctx context.Context, name string) (*qdrant.CollectionInfo, error)
    CreateCollection(ctx context.Context, req *qdrant.CreateCollection) error
    UpdateCollection(ctx context.Context, req *qdrant.UpdateCollection) error
    DeleteCollection(ctx context.Context, name string) error
    Upsert(ctx context.Context, req *qdrant.UpsertPoints) (*qdrant.UpdateResult, error)
    Query(ctx context.Context, req *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error)
    QueryGroups(ctx context.Context, req *qdrant.QueryPointGroups) ([]*qdrant.PointGroup, error)
    Delete(ctx context.Context, req *qdrant.DeletePoints) (*qdrant.UpdateResult, error)
    UpdateVectors(ctx context.Context, req *qdrant.UpdatePointVectors) (*qdrant.UpdateResult, error)
    SetPayload(ctx context.Context, req *qdrant.SetPayloadPoints) (*qdrant.UpdateResult, error)
    CreateFieldIndex(ctx context.Context, req *qdrant.CreateFieldIndexCollection) (*qdrant.UpdateResult, error)
    Count(ctx context.Context, req *qdrant.CountPoints) (uint64, error)
    ScrollAndOffset(ctx context.Context, req *qdrant.ScrollPoints) ([]*qdrant.RetrievedPoint, *qdrant.PointId, error)
    Get(ctx context.Context, req *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error)
}
```

## How pkg/qql wraps internal/

pkg/qql does NOT re-implement anything. It delegates:

```
pkg/qql.Parse()     → lexer.Tokenize() + parser.Parse()
pkg/qql.Exec()      → commands.NewExecutor(client, cfg).ExecuteResult(query)
pkg/qql.Explain()   → commands.NewExecutor(nil, nil).ExplainResult(query)
```

The key refactoring: extract the qdrantClient interface from internal/cli/commands
so pkg/qql can define its own copy. The internal executor keeps working as-is.

## Server (Connect RPC)

```go
type QQLHandler struct {
    core *qql.Core  // wraps the pkg/qql functions
}

func (h *QQLHandler) Exec(ctx context.Context, req *connect.Request[qqlpb.ExecRequest]) (*connect.Response[qqlpb.ExecResponse], error) {
    result, err := qql.Exec(ctx, h.client, req.Msg.Query)
    // ...
}
```

Mounted on a standard http.ServeMux. Works with any HTTP middleware.

## Files to create/modify

| File | Action | Description |
|------|--------|-------------|
| proto/qql.proto | CREATE | Service definition |
| buf.yaml | CREATE | Buf module config |
| buf.gen.yaml | CREATE | Buf codegen config |
| pkg/qql/parse.go | CREATE | Parse function |
| pkg/qql/exec.go | CREATE | Exec function |
| pkg/qql/batch.go | CREATE | ExecBatch function |
| pkg/qql/explain.go | CREATE | Explain function |
| pkg/qql/types.go | CREATE | Public types |
| pkg/qql/client.go | CREATE | QdrantClient interface + constructor |
| server/handler.go | CREATE | Connect RPC handler |
| server/server.go | CREATE | HTTP server setup |
| cmd/qql-go/serve.go | CREATE | serve cobra command |
| go.mod | MODIFY | Add connectrpc.com/connect dependency |

## Build order

1. proto/qql.proto — define the contract
2. pkg/qql/types.go — public types
3. pkg/qql/client.go — QdrantClient interface
4. pkg/qql/parse.go — Parse function
5. pkg/qql/exec.go — Exec function (delegates to internal executor)
6. pkg/qql/explain.go — Explain function
7. pkg/qql/batch.go — ExecBatch function
8. go get connectrpc dependencies
9. buf generate (or manual Go stubs)
10. server/handler.go — Connect handler
11. server/server.go — HTTP server
12. cmd/qql-go/serve.go — serve command
13. go test ./pkg/qql/... ./server/...
14. Manual smoke test: qql-go serve + curl
