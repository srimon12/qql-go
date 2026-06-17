package parser

import (
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseQueryFormula(t *testing.T) {
	query := `QUERY 'test' FROM my_col LIMIT 10
	BOOST ($score * 2.0 + ABS(match_count * 0.1))
	DEFAULTS (popularity = 1.0, rating = 0.0)`

	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(query)
	require.NoError(t, err)

	p := NewParser()
	stmt, err := p.Parse(tokens)
	require.NoError(t, err)

	qStmt, ok := stmt.(*ast.QueryStmt)
	require.True(t, ok)

	assert.NotNil(t, qStmt.Formula)

	// Check FormulaDefaults
	require.Len(t, qStmt.FormulaDefaults, 2)
	assert.Equal(t, 1.0, qStmt.FormulaDefaults["popularity"])
	assert.Equal(t, 0.0, qStmt.FormulaDefaults["rating"])

	// Top level is Sum
	sum, ok := qStmt.Formula.(ast.FormulaSum)
	require.True(t, ok)

	// Left is Mul
	mul, ok := sum.Left.(ast.FormulaMul)
	require.True(t, ok)

	val1, ok := mul.Left.(ast.FormulaVariable)
	require.True(t, ok)
	assert.Equal(t, "$score", val1.Name)

	val2, ok := mul.Right.(ast.FormulaConstant)
	require.True(t, ok)
	assert.Equal(t, 2.0, val2.Value)

	// Right is Abs
	abs, ok := sum.Right.(ast.FormulaAbs)
	require.True(t, ok)

	innerMul, ok := abs.X.(ast.FormulaMul)
	require.True(t, ok)

	val3, ok := innerMul.Left.(ast.FormulaVariable)
	require.True(t, ok)
	assert.Equal(t, "match_count", val3.Name)
}

func TestParseFormulaCase(t *testing.T) {
	query := `QUERY 'test' FROM my_col
	BOOST (CASE WHEN category = 'premium' THEN $score * 2.0 ELSE $score END)`

	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(query)
	require.NoError(t, err)

	p := NewParser()
	stmt, err := p.Parse(tokens)
	require.NoError(t, err)

	qStmt, ok := stmt.(*ast.QueryStmt)
	require.True(t, ok)
	assert.NotNil(t, qStmt.Formula)

	c, ok := qStmt.Formula.(ast.FormulaCase)
	require.True(t, ok, "Expected FormulaCase, got %T", qStmt.Formula)

	cond, ok := c.Cond.(*ast.CompareExpr)
	require.True(t, ok)
	assert.Equal(t, "category", cond.Field)
	assert.Equal(t, "premium", cond.Value)

	thenExpr, ok := c.Then_.(ast.FormulaMul)
	require.True(t, ok)

	val1, ok := thenExpr.Left.(ast.FormulaVariable)
	require.True(t, ok)
	assert.Equal(t, "$score", val1.Name)

	elseExpr, ok := c.Else_.(ast.FormulaVariable)
	require.True(t, ok)
	assert.Equal(t, "$score", elseExpr.Name)
}
