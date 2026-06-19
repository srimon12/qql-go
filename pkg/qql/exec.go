package qql

import (
	"context"
	"fmt"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/cli/commands"
	"github.com/srimon12/qql-go/internal/config"
)

// Exec parses and executes a single QQL query against the given Qdrant client.
func Exec(ctx context.Context, client QdrantClient, query string) (*Result, error) {
	return ExecWithConfig(ctx, client, query, nil)
}

// ExecWithConfig parses and executes a single QQL query with explicit config
// for model resolution, BM25 parameters, and cloud inference options.
func ExecWithConfig(ctx context.Context, client QdrantClient, query string, cfg *config.Config) (*Result, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
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

// ExecAST executes a pre-parsed AST node. This is used by the gateway to
// execute policy-transformed ASTs where filters or limits have been injected
// before the query reaches Qdrant.
func ExecAST(ctx context.Context, client QdrantClient, node ast.ASTNode, cfg *config.Config) (*Result, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	executor := commands.NewExecutor(client, cfg)
	resp, err := executor.ExecuteNode(node)
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
