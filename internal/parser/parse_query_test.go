package parser

import (
	"testing"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseQueryNearest(t *testing.T) {
	input := "QUERY NEAREST 'vector search' FROM docs LIMIT 10 OFFSET 5 USING HYBRID RERANK WHERE topic = 'search' WITH { hnsw_ef: 128 }"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	assert.Equal(t, ast.QueryModeNearest, stmt.Mode)
	assert.Equal(t, "docs", stmt.Collection)
	require.NotNil(t, stmt.QueryText)
	assert.Equal(t, "vector search", *stmt.QueryText)
	assert.Equal(t, 10, stmt.Limit)
	assert.Equal(t, 5, stmt.Offset)
	assert.True(t, stmt.Hybrid)
	assert.True(t, stmt.Rerank)
	require.NotNil(t, stmt.QueryFilter)
	require.NotNil(t, stmt.WithClause)
	assert.Equal(t, 128, stmt.WithClause.HnswEf)
}

func TestParseQueryRecommend(t *testing.T) {
	input := "QUERY RECOMMEND POSITIVE IDS (1, 2) NEGATIVE IDS (3) FROM users"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	assert.Equal(t, ast.QueryModeRecommend, stmt.Mode)
	assert.Equal(t, "users", stmt.Collection)
	assert.Equal(t, []any{1, 2}, stmt.PositiveIDs)
	assert.Equal(t, []any{3}, stmt.NegativeIDs)
	assert.Equal(t, 10, stmt.Limit) // Default limit
}

func TestParseQueryDiscover(t *testing.T) {
	input := "QUERY DISCOVER TARGET 100 CONTEXT PAIRS (1, 2), (3, 4) FROM products LIMIT 20"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	assert.Equal(t, ast.QueryModeDiscover, stmt.Mode)
	assert.Equal(t, "products", stmt.Collection)
	assert.Equal(t, 100, stmt.Target)
	assert.Len(t, stmt.ContextPairs, 2)
	assert.Equal(t, 1, stmt.ContextPairs[0].Positive)
	assert.Equal(t, 2, stmt.ContextPairs[0].Negative)
	assert.Equal(t, 3, stmt.ContextPairs[1].Positive)
	assert.Equal(t, 4, stmt.ContextPairs[1].Negative)
	assert.Equal(t, 20, stmt.Limit)
}

func TestParseQueryContext(t *testing.T) {
	input := "QUERY CONTEXT PAIRS ('uuid-1', 'uuid-2') FROM logs"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	assert.Equal(t, ast.QueryModeContext, stmt.Mode)
	assert.Equal(t, "logs", stmt.Collection)
	assert.Len(t, stmt.ContextPairs, 1)
	assert.Equal(t, "uuid-1", stmt.ContextPairs[0].Positive)
	assert.Equal(t, "uuid-2", stmt.ContextPairs[0].Negative)
}



func TestParseQueryErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing from", "QUERY NEAREST 'text' LIMIT 10"},
		{"invalid mode", "QUERY SOMETHING 'text' FROM docs"},
		{"missing context pairs", "QUERY CONTEXT FROM docs"},
		{"missing discover target", "QUERY DISCOVER FROM docs"},
		{"missing positive ids", "QUERY RECOMMEND NEGATIVE IDS 1 FROM docs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			if err != nil {
				return // Lexer error is fine
			}

			p := NewParser()
			_, err = p.Parse(tokens)
			assert.Error(t, err)
		})
	}
}
