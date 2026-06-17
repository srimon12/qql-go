package qql

import (
	"context"
	"fmt"
)

// ExecBatch executes multiple QQL queries against the given Qdrant client.
// If stopOnError is true, execution stops at the first error and returns
// partial results along with the error.
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
