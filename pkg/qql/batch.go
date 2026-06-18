package qql

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/cli/commands"
	"github.com/srimon12/qql-go/internal/config"
)

// ExecBatch executes multiple QQL queries against the given Qdrant client.
// Supports mixed statement types (INSERT, CREATE, QUERY, etc.).
// If stopOnError is true, execution stops at the first error.
//
// For pure QUERY batches, use BatchQuery instead — it uses Qdrant's native
// QueryBatch API for a single round-trip.
func ExecBatch(ctx context.Context, client QdrantClient, queries []string, stopOnError bool) ([]*Result, error) {
	return ExecBatchWithConfig(ctx, client, queries, stopOnError, nil)
}

// ExecBatchWithConfig executes multiple QQL queries with explicit config.
func ExecBatchWithConfig(ctx context.Context, client QdrantClient, queries []string, stopOnError bool, cfg *config.Config) ([]*Result, error) {
	results := make([]*Result, len(queries))
	for i, query := range queries {
		result, err := ExecWithConfig(ctx, client, query, cfg)
		if err != nil {
			if stopOnError {
				return results[:i], fmt.Errorf("query %d failed: %w", i, err)
			}
			results[i] = ErrorResult(err)
			continue
		}
		results[i] = result
	}
	return results, nil
}

// BatchQuery executes multiple QQL QUERY statements using Qdrant's native
// QueryBatch API. All queries are sent in a single round-trip per collection.
//
// Only QUERY statements are supported. For mixed statement types (INSERT, CREATE, etc.),
// use ExecBatch instead.
func BatchQuery(ctx context.Context, client QdrantClient, queries []string) ([]*Result, error) {
	return BatchQueryWithConfig(ctx, client, queries, nil)
}

// BatchQueryWithConfig executes multiple QQL QUERY statements with explicit config.
func BatchQueryWithConfig(ctx context.Context, client QdrantClient, queries []string, cfg *config.Config) ([]*Result, error) {
	if len(queries) == 0 {
		return nil, fmt.Errorf("BatchQuery requires at least one query")
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	executor := commands.NewExecutor(client, cfg)

	type collectionBatch struct {
		indices []int
		points  []*qdrant.QueryPoints
	}

	batches := make(map[string]*collectionBatch)
	var orderedKeys []string

	for i, query := range queries {
		qp, err := executor.BuildQueryPoints(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("query %d parse error: %w", i, err)
		}
		coll := qp.GetCollectionName()
		b, ok := batches[coll]
		if !ok {
			b = &collectionBatch{}
			batches[coll] = b
			orderedKeys = append(orderedKeys, coll)
		}
		b.indices = append(b.indices, i)
		b.points = append(b.points, qp)
	}

	results := make([]*Result, len(queries))

	for _, coll := range orderedKeys {
		batch := batches[coll]
		batchResults, err := client.QueryBatch(ctx, &qdrant.QueryBatchPoints{
			CollectionName: coll,
			QueryPoints:    batch.points,
		})
		if err != nil {
			return nil, fmt.Errorf("batch query on '%s' failed: %w", coll, err)
		}

		for j, br := range batchResults {
			origIdx := batch.indices[j]
			formatted := make([]map[string]any, len(br.GetResult()))
			for k, hit := range br.GetResult() {
				formatted[k] = map[string]any{
					"id":    pointIDToString(hit.GetId()),
					"score": hit.GetScore(),
				}
				if textVal, ok := hit.GetPayload()["text"]; ok {
					formatted[k]["text"] = textVal.GetStringValue()
				}
			}
			results[origIdx] = &Result{
				OK:        true,
				Operation: "QUERY",
				Message:   fmt.Sprintf("Found %d hits", len(formatted)),
				Data:      formatted,
			}
		}
	}

	return results, nil
}

func pointIDToString(id *qdrant.PointId) string {
	if id == nil {
		return ""
	}
	if uuid := id.GetUuid(); uuid != "" {
		return uuid
	}
	return fmt.Sprintf("%d", id.GetNum())
}

// BatchQueryASTWithConfig executes multiple pre-parsed QQL QUERY AST statements
// using Qdrant's native QueryBatch API. This is used by the gateway to batch
// execute queries after policy filters have been injected.
func BatchQueryASTWithConfig(ctx context.Context, client QdrantClient, nodes []ast.ASTNode, cfg *config.Config) ([]*Result, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("BatchQueryAST requires at least one node")
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	executor := commands.NewExecutor(client, cfg)

	type collectionBatch struct {
		indices []int
		points  []*qdrant.QueryPoints
	}

	batches := make(map[string]*collectionBatch)
	var orderedKeys []string

	for i, node := range nodes {
		qp, err := executor.BuildQueryPointsNode(ctx, node)
		if err != nil {
			return nil, fmt.Errorf("query %d build error: %w", i, err)
		}
		coll := qp.GetCollectionName()
		b, ok := batches[coll]
		if !ok {
			b = &collectionBatch{}
			batches[coll] = b
			orderedKeys = append(orderedKeys, coll)
		}
		b.indices = append(b.indices, i)
		b.points = append(b.points, qp)
	}

	results := make([]*Result, len(nodes))

	for _, coll := range orderedKeys {
		batch := batches[coll]
		batchResults, err := client.QueryBatch(ctx, &qdrant.QueryBatchPoints{
			CollectionName: coll,
			QueryPoints:    batch.points,
		})
		if err != nil {
			return nil, fmt.Errorf("batch query on '%s' failed: %w", coll, err)
		}

		for j, br := range batchResults {
			origIdx := batch.indices[j]
			formatted := make([]map[string]any, len(br.GetResult()))
			for k, hit := range br.GetResult() {
				formatted[k] = map[string]any{
					"id":    pointIDToString(hit.GetId()),
					"score": hit.GetScore(),
				}
				if textVal, ok := hit.GetPayload()["text"]; ok {
					formatted[k]["text"] = textVal.GetStringValue()
				}
			}
			results[origIdx] = &Result{
				OK:        true,
				Operation: "QUERY",
				Message:   fmt.Sprintf("Found %d hits", len(formatted)),
				Data:      formatted,
			}
		}
	}

	return results, nil
}
