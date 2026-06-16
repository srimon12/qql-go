package qql

import (
	"context"
	"fmt"

	"github.com/srimon12/qql-go/internal/cli/commands"
	"github.com/srimon12/qql-go/internal/config"
)

// Exec parses and executes a single QQL query against the given Qdrant client.
func Exec(ctx context.Context, client QdrantClient, query string) (*Result, error) {
	cfg := &config.Config{}
	executor := commands.NewExecutor(client, cfg)
	resp, err := executor.ExecuteResult(query)
	if err != nil {
		return nil, fmt.Errorf("execution error: %w", err)
	}
	return &Result{
		OK:        resp.OK,
		Operation: resp.Operation,
		Message:   resp.Message,
		Data:      resp.Data,
	}, nil
}
