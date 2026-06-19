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

func TestParseFormulaMatch(t *testing.T) {
	query := `QUERY 'test' FROM my_col
	BOOST ($score + 0.5 * MATCH(tag, ['h1', 'h2', 'h3']) + 0.25 * MATCH(category, 'premium'))`

	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(query)
	require.NoError(t, err)

	p := NewParser()
	stmt, err := p.Parse(tokens)
	require.NoError(t, err)

	qStmt, ok := stmt.(*ast.QueryStmt)
	require.True(t, ok)
	assert.NotNil(t, qStmt.Formula)

	sum, ok := qStmt.Formula.(ast.FormulaSum)
	require.True(t, ok, "expected FormulaSum at top level, got %T", qStmt.Formula)

	// score + (0.5 * MATCH(tag, [...])) on the left
	innerSum, ok := sum.Left.(ast.FormulaSum)
	require.True(t, ok, "expected inner FormulaSum for score + mul, got %T", sum.Left)

	v, ok := innerSum.Left.(ast.FormulaVariable)
	require.True(t, ok)
	assert.Equal(t, "$score", v.Name)

	// 0.5 * MATCH(tag, ['h1', 'h2', 'h3'])
	mul1, ok := innerSum.Right.(ast.FormulaMul)
	require.True(t, ok, "expected FormulaMul, got %T", innerSum.Right)

	konst, ok := mul1.Left.(ast.FormulaConstant)
	require.True(t, ok)
	assert.Equal(t, 0.5, konst.Value)

	match1, ok := mul1.Right.(ast.FormulaMatchCondition)
	require.True(t, ok, "expected FormulaMatchCondition, got %T", mul1.Right)
	assert.Equal(t, "tag", match1.Field)
	assert.Len(t, match1.Values, 3)
	assert.Equal(t, []any{"h1", "h2", "h3"}, match1.Values)

	// 0.25 * MATCH(category, 'premium')
	mul2, ok := sum.Right.(ast.FormulaMul)
	require.True(t, ok, "expected FormulaMul, got %T", sum.Right)

	konst2, ok := mul2.Left.(ast.FormulaConstant)
	require.True(t, ok)
	assert.Equal(t, 0.25, konst2.Value)

	match2, ok := mul2.Right.(ast.FormulaMatchCondition)
	require.True(t, ok, "expected FormulaMatchCondition, got %T", mul2.Right)
	assert.Equal(t, "category", match2.Field)
	assert.Len(t, match2.Values, 1)
	assert.Equal(t, "premium", match2.Values[0])
}

func TestParseFormulaDivDefault(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		hasDefault bool
		val        float64
	}{
		{
			name:       "div without default",
			query:      "QUERY 'test' FROM my_col BOOST ($score / popularity)",
			hasDefault: false,
		},
		{
			name:       "div with default",
			query:      "QUERY 'test' FROM my_col BOOST ($score / popularity [default=1.5])",
			hasDefault: true,
			val:        1.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.query)
			require.NoError(t, err)

			p := NewParser()
			stmt, err := p.Parse(tokens)
			require.NoError(t, err)

			qStmt, ok := stmt.(*ast.QueryStmt)
			require.True(t, ok)

			div, ok := qStmt.Formula.(ast.FormulaDiv)
			require.True(t, ok)

			if tt.hasDefault {
				require.NotNil(t, div.ByZeroDefault)
				assert.Equal(t, tt.val, *div.ByZeroDefault)
			} else {
				assert.Nil(t, div.ByZeroDefault)
			}
		})
	}
}

func TestParseFormulaDecayParams(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
		check   func(t *testing.T, expr ast.FormulaExpr)
	}{
		{
			name:    "decay with decay parameter",
			query:   "QUERY 'test' FROM my_col BOOST (gauss_decay(geo_distance(48.8, 2.3, location), target=0.0, scale=1000.0, decay=0.8))",
			wantErr: false,
			check: func(t *testing.T, expr ast.FormulaExpr) {
				d, ok := expr.(ast.FormulaDecay)
				require.True(t, ok)
				require.NotNil(t, d.Scale)
				assert.Equal(t, 1000.0, *d.Scale)
				require.NotNil(t, d.Midpoint)
				assert.Equal(t, 0.8, *d.Midpoint)
			},
		},
		{
			name:    "decay with scale expression errors",
			query:   "QUERY 'test' FROM my_col BOOST (gauss_decay(geo_distance(48.8, 2.3, location), target=0.0, scale=popularity, midpoint=0.5))",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.query)
			require.NoError(t, err)

			p := NewParser()
			stmt, err := p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				qStmt, ok := stmt.(*ast.QueryStmt)
				require.True(t, ok)
				tt.check(t, qStmt.Formula)
			}
		})
	}
}
