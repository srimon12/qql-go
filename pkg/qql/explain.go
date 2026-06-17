package qql

import (
	"fmt"

	"github.com/srimon12/qql-go/internal/cli/commands"
	"github.com/srimon12/qql-go/internal/config"
)

// Explain returns the execution plan for a QQL query without running it.
func Explain(query string) (string, error) {
	executor := commands.NewExecutor(nil, &config.Config{})
	plan, err := executor.Explain(query)
	if err != nil {
		return "", fmt.Errorf("explain error: %w", err)
	}
	return plan, nil
}

// ExplainResult returns a structured explain result.
func ExplainResult(query string) (bool, string, string, error) {
	executor := commands.NewExecutor(nil, &config.Config{})
	resp, err := executor.ExplainResult(query)
	if err != nil {
		return false, "", "", fmt.Errorf("explain error: %w", err)
	}
	return resp.OK, resp.Query, resp.Plan, nil
}
