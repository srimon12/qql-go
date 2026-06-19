package parser

import (
	"testing"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseQueryNearest(t *testing.T) {
	input := "QUERY NEAREST 'vector search' FROM docs LIMIT 10 OFFSET 5 USING HYBRID RERANK WHERE topic = 'search' WITH (hnsw_ef = 128)"
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
	assert.Equal(t, ast.QueryTypeHybrid, stmt.Type)
	assert.True(t, stmt.Rerank)
	require.NotNil(t, stmt.QueryFilter)
	require.NotNil(t, stmt.WithClause)
	assert.Equal(t, 128, stmt.WithClause.HnswEf)
}

func TestParseQueryRecommend(t *testing.T) {
	input := "QUERY RECOMMEND WITH (positive = (1, 2), negative = (3)) FROM users"
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
		{"invalid mode", "QUERY SOMETHING 'text' FROM docs"},
		{"missing context pairs", "QUERY CONTEXT FROM docs"},
		{"missing discover target", "QUERY DISCOVER FROM docs"},
		{"missing positive ids", "QUERY RECOMMEND WITH (negative = [1]) FROM docs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			_, err = p.Parse(tokens)
			assert.Error(t, err)
		})
	}
}

func TestParseQueryPrefetch(t *testing.T) {
	input := `WITH p1 AS (QUERY 'search' USING dense LIMIT 100 WHERE category = 'tech' SCORE THRESHOLD 0.8), p2 AS (QUERY 'search' USING sparse LIMIT 100 WITH (exact = true))
QUERY 'search' FROM docs LIMIT 10 PREFETCH (p1, p2) FUSION RRF WITH (rrf_k = 10, rrf_weights = [0.7, 0.3])`
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	assert.Equal(t, "docs", stmt.Collection)
	assert.Equal(t, 10, stmt.Limit)
	require.Len(t, stmt.CTEs, 2)
	assert.Equal(t, "p1", stmt.CTEs[0].Name)
	assert.Equal(t, "p2", stmt.CTEs[1].Name)
	require.Len(t, stmt.PrefetchRefs, 2)
	assert.Equal(t, "p1", stmt.PrefetchRefs[0].CTEName)
	assert.Equal(t, "p2", stmt.PrefetchRefs[1].CTEName)

	require.NotNil(t, stmt.FusionType)
	assert.Equal(t, "RRF", *stmt.FusionType)
	require.NotNil(t, stmt.WithClause)
	require.NotNil(t, stmt.WithClause.RrfK)
	assert.Equal(t, 10, *stmt.WithClause.RrfK)
	require.Len(t, stmt.WithClause.RrfWeights, 2)
	assert.Equal(t, float32(0.7), stmt.WithClause.RrfWeights[0])
}

func TestParseQueryPrefetchCaseInsensitive(t *testing.T) {
	input := `WITH MyCte AS (QUERY 'search' USING dense LIMIT 100)
QUERY 'search' FROM docs LIMIT 10 PREFETCH (mycte)`
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	require.Len(t, stmt.CTEs, 1)
	assert.Equal(t, "mycte", stmt.CTEs[0].Name)
	require.Len(t, stmt.PrefetchRefs, 1)
	assert.Equal(t, "mycte", stmt.PrefetchRefs[0].CTEName)
}


func TestParseQueryWithLookup(t *testing.T) {
	input := "QUERY 'search' FROM docs LIMIT 10 GROUP BY 'category' GROUP_SIZE 5 WITH LOOKUP FROM metadata"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	assert.Equal(t, "docs", stmt.Collection)
	assert.Equal(t, 10, stmt.Limit)
	require.NotNil(t, stmt.GroupBy)
	assert.Equal(t, "category", *stmt.GroupBy)
	require.NotNil(t, stmt.GroupSize)
	assert.Equal(t, 5, *stmt.GroupSize)
	require.NotNil(t, stmt.WithLookupCollection)
	assert.Equal(t, "metadata", *stmt.WithLookupCollection)
}

func TestParseQueryPrefetchEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			"FUSION without PREFETCH or USING HYBRID",
			"QUERY 'test' FROM docs FUSION RRF",
			false,
		},
		{
			"Empty PREFETCH block",
			"QUERY 'test' FROM docs PREFETCH ()",
			false,
		},
		{
			"Duplicate FUSION clause",
			"QUERY 'test' FROM docs USING HYBRID FUSION RRF FUSION DBSF",
			true,
		},
		{
			"Nested CTE via PREFETCH refs",
			"WITH p1 AS (QUERY 'inner' USING dense LIMIT 50), p2 AS (QUERY 'outer' USING sparse LIMIT 100 PREFETCH (p1)) QUERY 'test' FROM docs PREFETCH (p2)",
			false,
		},
		{
			"FUSION DBSF",
			"QUERY 'test' FROM docs USING HYBRID FUSION DBSF",
			false,
		},
		{
			"CTE with RECOMMEND mode",
			"WITH p1 AS (QUERY RECOMMEND WITH (positive = (1, 2), negative = (3)) USING dense) QUERY 'test' FROM docs PREFETCH (p1)",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			if err != nil && tt.wantErr {
				return
			}
			require.NoError(t, err)

			p := NewParser()
			_, err = p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseQueryPrefetchPerRefFilter(t *testing.T) {
	input := `WITH a AS (QUERY 'search' USING dense LIMIT 100), b AS (QUERY 'search' USING sparse LIMIT 100)
QUERY 'search' FROM docs LIMIT 10 PREFETCH (a WHERE category = 'tech', b SCORE THRESHOLD 0.5) FUSION RRF`
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	require.Len(t, stmt.PrefetchRefs, 2)

	// First ref: a WHERE category = 'tech'
	assert.Equal(t, "a", stmt.PrefetchRefs[0].CTEName)
	require.NotNil(t, stmt.PrefetchRefs[0].Filter)
	assert.Nil(t, stmt.PrefetchRefs[0].ScoreThreshold)

	// Second ref: b SCORE THRESHOLD 0.5
	assert.Equal(t, "b", stmt.PrefetchRefs[1].CTEName)
	assert.Nil(t, stmt.PrefetchRefs[1].Filter)
	require.NotNil(t, stmt.PrefetchRefs[1].ScoreThreshold)
	assert.InDelta(t, 0.5, *stmt.PrefetchRefs[1].ScoreThreshold, 1e-6)
}

func TestParseQueryPrefetchPerRefBoth(t *testing.T) {
	input := `WITH a AS (QUERY 'search' USING dense LIMIT 100)
QUERY 'search' FROM docs LIMIT 10 PREFETCH (a WHERE priority = 'high' SCORE THRESHOLD 0.8) FUSION RRF`
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	require.Len(t, stmt.PrefetchRefs, 1)
	assert.Equal(t, "a", stmt.PrefetchRefs[0].CTEName)
	require.NotNil(t, stmt.PrefetchRefs[0].Filter)
	require.NotNil(t, stmt.PrefetchRefs[0].ScoreThreshold)
	assert.InDelta(t, 0.8, *stmt.PrefetchRefs[0].ScoreThreshold, 1e-6)
}

func TestParseQueryPrefetchPerRefLookup(t *testing.T) {
	input := `WITH a AS (QUERY 'search' USING dense LIMIT 100)
QUERY 'search' FROM docs LIMIT 10 PREFETCH (a LOOKUP FROM external_col VECTOR 'dense_vec') FUSION RRF`
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	require.Len(t, stmt.PrefetchRefs, 1)
	assert.Equal(t, "a", stmt.PrefetchRefs[0].CTEName)
	assert.Equal(t, "external_col", stmt.PrefetchRefs[0].LookupFrom)
	require.NotNil(t, stmt.PrefetchRefs[0].LookupVector)
	assert.Equal(t, "dense_vec", *stmt.PrefetchRefs[0].LookupVector)
}

func TestParseQueryOrderBy(t *testing.T) {
	input := "QUERY ORDER BY timestamp ASC FROM logs LIMIT 100"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	assert.Equal(t, ast.QueryModeOrderBy, stmt.Mode)
	require.NotNil(t, stmt.OrderByField)
	assert.Equal(t, "timestamp", *stmt.OrderByField)
	require.NotNil(t, stmt.OrderByAsc)
	assert.True(t, *stmt.OrderByAsc)
	assert.Equal(t, "logs", stmt.Collection)
	assert.Equal(t, 100, stmt.Limit)
}

func TestParseQuerySample(t *testing.T) {
	input := "QUERY SAMPLE FROM docs LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	assert.Equal(t, ast.QueryModeSample, stmt.Mode)
	assert.Equal(t, "docs", stmt.Collection)
	assert.Equal(t, 10, stmt.Limit)
}

func TestParseQuerySampleWithFilter(t *testing.T) {
	input := "QUERY SAMPLE FROM docs LIMIT 10 WHERE category = 'tech'"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	assert.Equal(t, ast.QueryModeSample, stmt.Mode)
	assert.Equal(t, "docs", stmt.Collection)
	assert.Equal(t, 10, stmt.Limit)
	require.NotNil(t, stmt.QueryFilter)
}

func TestParseQueryWithPayloadAndVectors(t *testing.T) {
	input := "QUERY 'search' FROM docs WITH PAYLOAD (include = ['title'], exclude = ['metadata']) WITH VECTORS true"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	require.NotNil(t, stmt.WithPayload)
	assert.ElementsMatch(t, []string{"title"}, stmt.WithPayload.Include)
	assert.ElementsMatch(t, []string{"metadata"}, stmt.WithPayload.Exclude)
	assert.Nil(t, stmt.WithPayload.Enable)

	require.NotNil(t, stmt.WithVectors)
	require.NotNil(t, stmt.WithVectors.Enable)
	assert.True(t, *stmt.WithVectors.Enable)
	assert.Empty(t, stmt.WithVectors.Vectors)

	// Test WITH VECTORS ('dense') and WITH PAYLOAD false
	input2 := "QUERY 'search' FROM docs WITH PAYLOAD false WITH VECTORS ('dense', 'sparse')"
	tokens2, err := l.Tokenize(input2)
	require.NoError(t, err)

	p2 := NewParser()
	node2, err := p2.Parse(tokens2)
	require.NoError(t, err)

	stmt2, ok := node2.(*ast.QueryStmt)
	require.True(t, ok)

	require.NotNil(t, stmt2.WithPayload)
	require.NotNil(t, stmt2.WithPayload.Enable)
	assert.False(t, *stmt2.WithPayload.Enable)

	require.NotNil(t, stmt2.WithVectors)
	assert.Nil(t, stmt2.WithVectors.Enable)
	assert.ElementsMatch(t, []string{"dense", "sparse"}, stmt2.WithVectors.Vectors)
}

func TestParseQueryMultipleWithClauses(t *testing.T) {
	input := "QUERY 'search' FROM docs WITH MODEL 'foo' WITH PAYLOAD (include = ['title']) WITH VECTORS true WITH (exact = true)"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)

	require.NotNil(t, stmt.Model)
	assert.Equal(t, "foo", *stmt.Model)

	require.NotNil(t, stmt.WithPayload)
	assert.ElementsMatch(t, []string{"title"}, stmt.WithPayload.Include)

	require.NotNil(t, stmt.WithVectors)
	require.NotNil(t, stmt.WithVectors.Enable)
	assert.True(t, *stmt.WithVectors.Enable)

	require.NotNil(t, stmt.WithClause)
	assert.True(t, stmt.WithClause.Exact)
}

func TestParseQueryWithPayloadVectorsErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid payload key",
			input: "QUERY FROM docs WITH PAYLOAD (badkey = ['a'])",
		},
		{
			name:  "missing parens",
			input: "QUERY FROM docs WITH PAYLOAD include = ['a']",
		},
		{
			name:  "missing bracket",
			input: "QUERY FROM docs WITH PAYLOAD (include = 'a')",
		},
		{
			name:  "missing equals",
			input: "QUERY FROM docs WITH PAYLOAD (include ['a'])",
		},
		{
			name:  "missing string in list",
			input: "QUERY FROM docs WITH PAYLOAD (include = [123])",
		},
		{
			name:  "missing closing bracket",
			input: "QUERY FROM docs WITH PAYLOAD (include = ['a'",
		},
		{
			name:  "missing closing paren",
			input: "QUERY FROM docs WITH PAYLOAD (include = ['a']",
		},
		{
			name:  "missing string in vectors",
			input: "QUERY FROM docs WITH VECTORS (123)",
		},
		{
			name:  "missing closing paren vectors",
			input: "QUERY FROM docs WITH VECTORS ('dense'",
		},
		{
			name:  "invalid vectors format",
			input: "QUERY FROM docs WITH VECTORS (['dense'])",
		},
		{
			name:  "invalid order by",
			input: "QUERY ORDER BY FROM docs",
		},
		{
			name:  "order by without by",
			input: "QUERY ORDER timestamp FROM docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, _ := l.Tokenize(tt.input)
			p := NewParser()
			_, err := p.Parse(tokens)
			assert.Error(t, err)
		})
	}
}

func TestParseQueryRawVector(t *testing.T) {
	input := "QUERY [0.1, 0.2, 0.3] FROM docs LIMIT 5"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)
	assert.Equal(t, "docs", stmt.Collection)
	assert.Len(t, stmt.RawVector, 3)
	assert.InDelta(t, 0.1, stmt.RawVector[0], 1e-9)
	assert.InDelta(t, 0.2, stmt.RawVector[1], 1e-9)
	assert.InDelta(t, 0.3, stmt.RawVector[2], 1e-9)
	assert.Equal(t, 5, stmt.Limit)
}

func TestParseCTEQueryRawVector(t *testing.T) {
	input := "WITH _pf0 AS (QUERY [0.5, 0.6] LIMIT 100) QUERY 'search' FROM docs LIMIT 10 PREFETCH (_pf0)"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.QueryStmt)
	require.True(t, ok)
	require.Len(t, stmt.CTEs, 1)
	cte := stmt.CTEs[0]
	assert.Equal(t, "_pf0", cte.Name)
	assert.Len(t, cte.Stmt.RawVector, 2)
	assert.InDelta(t, 0.5, cte.Stmt.RawVector[0], 1e-9)
}
