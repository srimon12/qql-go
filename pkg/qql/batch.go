package qql

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
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
	results := make([]*Result, len(queries))
	for i, query := range queries {
		result, err := Exec(ctx, client, query)
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
// QueryBatch API. All queries are sent in a single round-trip.
//
// Only QUERY statements are supported. For mixed statement types (INSERT, CREATE, etc.),
// use ExecBatch instead.
func BatchQuery(ctx context.Context, client QdrantClient, queries []string) ([]*Result, error) {
	cfg := &config.Config{}
	executor := commands.NewExecutor(client, cfg)

	// Build all QueryPoints requests
	var queryPoints []*qdrant.QueryPoints

	for i, query := range queries {
		qp, err := executor.BuildQueryPoints(query)
		if err != nil {
			return nil, fmt.Errorf("query %d parse error: %w", i, err)
		}
		queryPoints = append(queryPoints, qp)
	}

	// Single round-trip to Qdrant
	batchResults, err := client.QueryBatch(ctx, &qdrant.QueryBatchPoints{
		CollectionName: queryPoints[0].GetCollectionName(),
		QueryPoints:    queryPoints,
	})
	if err != nil {
		return nil, fmt.Errorf("batch query failed: %w", err)
	}

	// Format results
	results := make([]*Result, len(batchResults))
	for i, br := range batchResults {
		formatted := make([]map[string]any, len(br.GetResult()))
		for j, hit := range br.GetResult() {
			formatted[j] = map[string]any{
				"id":    pointIDToString(hit.GetId()),
				"score": hit.GetScore(),
			}
			if textVal, ok := hit.GetPayload()["text"]; ok {
				formatted[j]["text"] = textVal.GetStringValue()
			}
		}
		results[i] = &Result{
			OK:        true,
			Operation: "QUERY",
			Message:   fmt.Sprintf("Found %d hits", len(formatted)),
			Data:      formatted,
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
