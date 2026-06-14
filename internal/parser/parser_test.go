package parser

import (
	"strings"
	"testing"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDocumentedExamples(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		check   func(t *testing.T, node ast.ASTNode)
		wantErr bool
	}{
		{
			name:  "readme create hybrid collection",
			input: "CREATE COLLECTION docs HYBRID",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.CreateCollectionStmt)
				require.True(t, ok, "expected CreateCollectionStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.True(t, stmt.Hybrid)
			},
		},
		{
			name:  "readme create hybrid rerank collection",
			input: "CREATE COLLECTION docs HYBRID RERANK",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.CreateCollectionStmt)
				require.True(t, ok, "expected CreateCollectionStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.True(t, stmt.Hybrid)
				assert.True(t, stmt.Rerank)
			},
		},
		{
			name:  "readme hybrid insert",
			input: "INSERT INTO COLLECTION docs VALUES {'text': 'Qdrant stores vectors', 'topic': 'search'} USING HYBRID",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.InsertStmt)
				require.True(t, ok, "expected InsertStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.True(t, stmt.Hybrid)
				assert.Equal(t, map[string]any{"text": "Qdrant stores vectors", "topic": "search"}, stmt.Values)
			},
		},
		{
			name:  "readme hybrid search",
			input: "QUERY NEAREST 'vector database' FROM docs LIMIT 5 USING HYBRID",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.QueryStmt)
				require.True(t, ok, "expected QueryStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.Equal(t, "vector database", *stmt.QueryText)
				assert.Equal(t, 5, stmt.Limit)
			},
		},
		{
			name:  "readme hybrid search with filter",
			input: "QUERY NEAREST 'vector search' FROM notes LIMIT 5 USING HYBRID WHERE topic = 'search'",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.QueryStmt)
				require.True(t, ok, "expected QueryStmt")
				assert.Equal(t, "notes", stmt.Collection)
				require.NotNil(t, stmt.QueryFilter)
				assertFilterExprEqual(t, &ast.CompareExpr{Field: "topic", Op: "=", Value: "search"}, stmt.QueryFilter)
			},
		},
		{
			name:  "readme hybrid rerank search",
			input: "QUERY NEAREST 'vector database' FROM docs LIMIT 5 USING HYBRID RERANK",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.QueryStmt)
				require.True(t, ok, "expected QueryStmt")
				assert.Equal(t, "docs", stmt.Collection)
			},
		},
		{
			name:  "readme delete by id",
			input: "DELETE FROM notes WHERE id = 'uuid'",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.DeleteStmt)
				require.True(t, ok, "expected DeleteStmt")
				assert.Equal(t, "notes", stmt.Collection)
				assert.Equal(t, "uuid", stmt.PointID)
			},
		},
		{
			name:  "readme delete by field",
			input: "DELETE FROM notes WHERE specialty = 'search'",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.DeleteStmt)
				require.True(t, ok, "expected DeleteStmt")
				assert.Equal(t, "notes", stmt.Collection)
				assert.Equal(t, "specialty", stmt.Field)
				assert.Equal(t, "search", stmt.Value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, tt.check)
			tt.check(t, node)
		})
	}
}

func TestParseFilterComparison(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ast.FilterExpr
	}{
		{
			name:     "equals",
			input:    "SCROLL FROM c WHERE field = 'value' LIMIT 10",
			expected: &ast.CompareExpr{Field: "field", Op: "=", Value: "value"},
		},
		{
			name:     "not equals",
			input:    "SCROLL FROM c WHERE field != 'value' LIMIT 10",
			expected: &ast.CompareExpr{Field: "field", Op: "!=", Value: "value"},
		},
		{
			name:     "greater than",
			input:    "SCROLL FROM c WHERE count > 5 LIMIT 10",
			expected: &ast.CompareExpr{Field: "count", Op: ">", Value: 5},
		},
		{
			name:     "greater than or equals",
			input:    "SCROLL FROM c WHERE count >= 5 LIMIT 10",
			expected: &ast.CompareExpr{Field: "count", Op: ">=", Value: 5},
		},
		{
			name:     "less than",
			input:    "SCROLL FROM c WHERE count < 10 LIMIT 10",
			expected: &ast.CompareExpr{Field: "count", Op: "<", Value: 10},
		},
		{
			name:     "less than or equals",
			input:    "SCROLL FROM c WHERE count <= 10 LIMIT 10",
			expected: &ast.CompareExpr{Field: "count", Op: "<=", Value: 10},
		},
		{
			name:     "equals integer",
			input:    "SCROLL FROM c WHERE count = 42 LIMIT 10",
			expected: &ast.CompareExpr{Field: "count", Op: "=", Value: 42},
		},
		{
			name:     "equals float",
			input:    "SCROLL FROM c WHERE score = 3.14 LIMIT 10",
			expected: &ast.CompareExpr{Field: "score", Op: "=", Value: 3.14},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			require.NoError(t, err)

			stmt, ok := node.(*ast.ScrollStmt)
			require.True(t, ok)
			require.NotNil(t, stmt.QueryFilter)
			assertFilterExprEqual(t, tt.expected, stmt.QueryFilter)
		})
	}
}

func TestParseFilterBetween(t *testing.T) {
	input := "SCROLL FROM c WHERE age BETWEEN 18 AND 65 LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	between, ok := stmt.QueryFilter.(*ast.BetweenExpr)
	require.True(t, ok)
	assert.Equal(t, "age", between.Field)
	assert.Equal(t, 18, between.Low)
	assert.Equal(t, 65, between.High)
}

func TestParseFilterIn(t *testing.T) {
	input := "SCROLL FROM c WHERE status IN ('active', 'pending') LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	inExpr, ok := stmt.QueryFilter.(*ast.InExpr)
	require.True(t, ok)
	assert.Equal(t, "status", inExpr.Field)
	assert.Equal(t, []any{"active", "pending"}, inExpr.Values)
}

func TestParseFilterNotIn(t *testing.T) {
	input := "SCROLL FROM c WHERE status NOT IN ('deleted', 'archived') LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	notIn, ok := stmt.QueryFilter.(*ast.NotInExpr)
	require.True(t, ok)
	assert.Equal(t, "status", notIn.Field)
	assert.Equal(t, []any{"deleted", "archived"}, notIn.Values)
}

func TestParseFilterIsNull(t *testing.T) {
	input := "SCROLL FROM c WHERE field IS NULL LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isNull, ok := stmt.QueryFilter.(*ast.IsNullExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isNull.Field)
}

func TestParseFilterIsNotNull(t *testing.T) {
	input := "SCROLL FROM c WHERE field IS NOT NULL LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isNotNull, ok := stmt.QueryFilter.(*ast.IsNotNullExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isNotNull.Field)
}

func TestParseFilterIsEmpty(t *testing.T) {
	input := "SCROLL FROM c WHERE field IS EMPTY LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isEmpty, ok := stmt.QueryFilter.(*ast.IsEmptyExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isEmpty.Field)
}

func TestParseFilterIsNotEmpty(t *testing.T) {
	input := "SCROLL FROM c WHERE field IS NOT EMPTY LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isNotEmpty, ok := stmt.QueryFilter.(*ast.IsNotEmptyExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isNotEmpty.Field)
}

func TestParseFilterMatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ast.FilterExpr
	}{
		{
			name:     "match text",
			input:    "SCROLL FROM c WHERE content MATCH 'hello world' LIMIT 10",
			expected: &ast.MatchTextExpr{Field: "content", Text: "hello world"},
		},
		{
			name:     "match any",
			input:    "SCROLL FROM c WHERE content MATCH ANY 'hello world' LIMIT 10",
			expected: &ast.MatchAnyExpr{Field: "content", Text: "hello world"},
		},
		{
			name:     "match phrase",
			input:    "SCROLL FROM c WHERE content MATCH PHRASE 'hello world' LIMIT 10",
			expected: &ast.MatchPhraseExpr{Field: "content", Text: "hello world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			require.NoError(t, err)

			stmt, ok := node.(*ast.ScrollStmt)
			require.True(t, ok)
			require.NotNil(t, stmt.QueryFilter)
			assertFilterExprEqual(t, tt.expected, stmt.QueryFilter)
		})
	}
}

func TestParseFilterAnd(t *testing.T) {
	input := "SCROLL FROM c WHERE a = 1 AND b = 2 LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	andExpr, ok := stmt.QueryFilter.(*ast.AndExpr)
	require.True(t, ok)
	require.Len(t, andExpr.Operands, 2)
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "a", Op: "=", Value: 1}, andExpr.Operands[0])
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "b", Op: "=", Value: 2}, andExpr.Operands[1])
}

func TestParseFilterOr(t *testing.T) {
	input := "SCROLL FROM c WHERE a = 1 OR b = 2 LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	orExpr, ok := stmt.QueryFilter.(*ast.OrExpr)
	require.True(t, ok)
	require.Len(t, orExpr.Operands, 2)
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "a", Op: "=", Value: 1}, orExpr.Operands[0])
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "b", Op: "=", Value: 2}, orExpr.Operands[1])
}

func TestParseFilterNot(t *testing.T) {
	input := "SCROLL FROM c WHERE NOT a = 1 LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	notExpr, ok := stmt.QueryFilter.(*ast.NotExpr)
	require.True(t, ok)
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "a", Op: "=", Value: 1}, notExpr.Operand)
}

func TestParseFilterComplex(t *testing.T) {
	input := "SCROLL FROM c WHERE (a = 1 AND b = 2) OR (c = 3 AND NOT d = 4) LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	orExpr, ok := stmt.QueryFilter.(*ast.OrExpr)
	require.True(t, ok)
	assertFilterExprEqual(t, &ast.OrExpr{
		Operands: []ast.FilterExpr{
			&ast.AndExpr{
				Operands: []ast.FilterExpr{
					&ast.CompareExpr{Field: "a", Op: "=", Value: 1},
					&ast.CompareExpr{Field: "b", Op: "=", Value: 2},
				},
			},
			&ast.AndExpr{
				Operands: []ast.FilterExpr{
					&ast.CompareExpr{Field: "c", Op: "=", Value: 3},
					&ast.NotExpr{
						Operand: &ast.CompareExpr{Field: "d", Op: "=", Value: 4},
					},
				},
			},
		},
	}, orExpr)
}

func TestParseFilterPrecedence(t *testing.T) {
	input := "SCROLL FROM c WHERE a = 1 AND b = 2 OR c = 3 LIMIT 10"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.ScrollStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	orExpr, ok := stmt.QueryFilter.(*ast.OrExpr)
	require.True(t, ok)
	assertFilterExprEqual(t, &ast.OrExpr{
		Operands: []ast.FilterExpr{
			&ast.AndExpr{
				Operands: []ast.FilterExpr{
					&ast.CompareExpr{Field: "a", Op: "=", Value: 1},
					&ast.CompareExpr{Field: "b", Op: "=", Value: 2},
				},
			},
			&ast.CompareExpr{Field: "c", Op: "=", Value: 3},
		},
	}, orExpr)
}

func TestParseError(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "invalid statement",
			input:   "INVALID KEYWORD",
			wantErr: true,
		},
		{
			name:    "insert missing values",
			input:   "INSERT INTO COLLECTION test",
			wantErr: true,
		},
		{
			name:    "search missing from",
			input:   "QUERY NEAREST 'text'",
			wantErr: true,
		},
		{
			name:    "search missing query text",
			input:   "QUERY NEAREST FROM test",
			wantErr: true,
		},
		{
			name:    "search with invalid with boolean",
			input:   "QUERY NEAREST 'text' FROM test LIMIT 10 WITH {exact: maybe}",
			wantErr: true,
		},
		{
			name:    "reject trailing tokens",
			input:   "INSERT INTO COLLECTION test VALUES {\"text\": \"hello\"} EXTRA",
			wantErr: true,
		},
		{
			name:    "reject explain in parser",
			input:   "EXPLAIN QUERY NEAREST 'text' FROM test LIMIT 10",
			wantErr: true,
		},
		{
			name:    "reject overflowing limit",
			input:   "QUERY NEAREST 'text' FROM test LIMIT 999999999999999999999999999",
			wantErr: true,
		},
		{
			name:    "reject overflowing integer literal",
			input:   "DELETE FROM test WHERE id = 999999999999999999999999999",
			wantErr: true,
		},
		{
			name:    "reject overflowing float literal",
			input:   "QUERY NEAREST 'text' FROM test LIMIT 10 WHERE score = " + strings.Repeat("9", 400) + ".0",
			wantErr: true,
		},
		{
			name:    "reject duplicate where clause",
			input:   "QUERY NEAREST 'text' FROM test LIMIT 10 WHERE a = 1 WHERE b = 2",
			wantErr: true,
		},
		{
			name:    "reject duplicate with clause",
			input:   "QUERY NEAREST 'text' FROM test LIMIT 10 WITH {exact: true} WITH {acorn: true}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			if tt.name == "reject overflowing limit" || tt.name == "reject overflowing integer literal" || tt.name == "reject overflowing float literal" {
				if err != nil {
					// Lexer will catch overflow errors during tokenization, which is fine
					return
				}
			} else {
				require.NoError(t, err)
			}

			p := NewParser()
			_, err = p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func float64Ptr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(v int) *int {
	return &v
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}

func assertFilterExprEqual(t *testing.T, expected, actual ast.FilterExpr) {
	t.Helper()

	switch e := expected.(type) {
	case *ast.CompareExpr:
		a, ok := actual.(*ast.CompareExpr)
		require.True(t, ok, "expected CompareExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Op, a.Op)
		assert.Equal(t, e.Value, a.Value)
	case *ast.BetweenExpr:
		a, ok := actual.(*ast.BetweenExpr)
		require.True(t, ok, "expected BetweenExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Low, a.Low)
		assert.Equal(t, e.High, a.High)
	case *ast.InExpr:
		a, ok := actual.(*ast.InExpr)
		require.True(t, ok, "expected InExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Values, a.Values)
	case *ast.NotInExpr:
		a, ok := actual.(*ast.NotInExpr)
		require.True(t, ok, "expected NotInExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Values, a.Values)
	case *ast.IsNullExpr:
		a, ok := actual.(*ast.IsNullExpr)
		require.True(t, ok, "expected IsNullExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.IsNotNullExpr:
		a, ok := actual.(*ast.IsNotNullExpr)
		require.True(t, ok, "expected IsNotNullExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.IsEmptyExpr:
		a, ok := actual.(*ast.IsEmptyExpr)
		require.True(t, ok, "expected IsEmptyExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.IsNotEmptyExpr:
		a, ok := actual.(*ast.IsNotEmptyExpr)
		require.True(t, ok, "expected IsNotEmptyExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.MatchTextExpr:
		a, ok := actual.(*ast.MatchTextExpr)
		require.True(t, ok, "expected MatchTextExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Text, a.Text)
	case *ast.MatchAnyExpr:
		a, ok := actual.(*ast.MatchAnyExpr)
		require.True(t, ok, "expected MatchAnyExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Text, a.Text)
	case *ast.MatchPhraseExpr:
		a, ok := actual.(*ast.MatchPhraseExpr)
		require.True(t, ok, "expected MatchPhraseExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Text, a.Text)
	case *ast.AndExpr:
		a, ok := actual.(*ast.AndExpr)
		require.True(t, ok, "expected AndExpr")
		require.Len(t, a.Operands, len(e.Operands))
		for i := range e.Operands {
			assertFilterExprEqual(t, e.Operands[i], a.Operands[i])
		}
	case *ast.OrExpr:
		a, ok := actual.(*ast.OrExpr)
		require.True(t, ok, "expected OrExpr")
		require.Len(t, a.Operands, len(e.Operands))
		for i := range e.Operands {
			assertFilterExprEqual(t, e.Operands[i], a.Operands[i])
		}
	case *ast.NotExpr:
		a, ok := actual.(*ast.NotExpr)
		require.True(t, ok, "expected NotExpr")
		assertFilterExprEqual(t, e.Operand, a.Operand)
	default:
		t.Fatalf("unexpected type %T", expected)
	}
}
