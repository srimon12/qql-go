package qql

import (
	"testing"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseQuery(t *testing.T) {
	node, err := Parse("QUERY 'search' FROM docs LIMIT 10")
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)
	assert.Equal(t, "docs", stmt.Collection)
	assert.Equal(t, 10, stmt.Limit)
}

func TestParseHybridQuery(t *testing.T) {
	node, err := Parse("QUERY 'search' FROM docs LIMIT 5 USING HYBRID")
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)
	assert.Equal(t, ast.QueryTypeHybrid, stmt.Type)
}

func TestParseWithPrefetch(t *testing.T) {
	input := `WITH a AS (QUERY 'search' USING dense LIMIT 100 WHERE category = 'tech'), b AS (QUERY 'search' USING sparse LIMIT 100)
QUERY 'search' FROM docs LIMIT 10 PREFETCH (a, b) FUSION RRF`
	node, err := Parse(input)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)
	require.Len(t, stmt.PrefetchRefs, 2)
	assert.Equal(t, "a", stmt.PrefetchRefs[0].CTEName)
	assert.Equal(t, "b", stmt.PrefetchRefs[1].CTEName)
}

func TestParseInvalidQuery(t *testing.T) {
	_, err := Parse("NOT VALID QQL AT ALL")
	assert.Error(t, err)
}

func TestExplain(t *testing.T) {
	plan, err := Explain("QUERY 'search' FROM docs LIMIT 10 USING HYBRID")
	require.NoError(t, err)
	assert.Contains(t, plan, "QUERY")
	assert.Contains(t, plan, "docs")
}

func TestExplainInvalid(t *testing.T) {
	_, err := Explain("GARBAGE")
	assert.Error(t, err)
}

func TestResultDataJSON(t *testing.T) {
	r := &Result{
		OK:        true,
		Operation: "QUERY",
		Message:   "Found 5 results",
		Data:      []map[string]any{{"id": "1", "score": 0.9}},
	}
	data, err := r.DataJSON()
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "score")
}

func TestErrorResult(t *testing.T) {
	r := ErrorResult(assert.AnError)
	assert.False(t, r.OK)
	assert.NotEmpty(t, r.Message)
}
