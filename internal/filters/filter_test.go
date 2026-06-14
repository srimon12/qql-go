package filters

import (
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFilterTypedEquality(t *testing.T) {
	tests := []struct {
		name      string
		expr      ast.FilterExpr
		checkFunc func(t *testing.T, filter *qdrant.Filter)
	}{
		{
			name: "string equality",
			expr: ast.CompareExpr{Field: "status", Op: "=", Value: "active"},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Must, 1)
				match := filter.Must[0].GetField().GetMatch()
				require.NotNil(t, match)
				assert.Equal(t, "active", match.GetKeyword())
			},
		},
		{
			name: "int equality",
			expr: ast.CompareExpr{Field: "count", Op: "=", Value: 42},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Must, 1)
				match := filter.Must[0].GetField().GetMatch()
				require.NotNil(t, match)
				assert.Equal(t, int64(42), match.GetInteger())
			},
		},
		{
			name: "float equality",
			expr: ast.CompareExpr{Field: "score", Op: "=", Value: 3.14},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Must, 1)
				rng := filter.Must[0].GetField().GetRange()
				require.NotNil(t, rng)
				assert.Equal(t, 3.14, rng.GetGte())
				assert.Equal(t, 3.14, rng.GetLte())
			},
		},
		{
			name: "bool equality",
			expr: ast.CompareExpr{Field: "is_active", Op: "=", Value: true},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Must, 1)
				match := filter.Must[0].GetField().GetMatch()
				require.NotNil(t, match)
				assert.True(t, match.GetBoolean())
			},
		},
	}

	converter := NewFilterConverter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := converter.BuildFilter(tt.expr)
			require.NoError(t, err)
			require.NotNil(t, filter)
			tt.checkFunc(t, filter)
		})
	}
}

func TestBuildFilterTypedInequality(t *testing.T) {
	tests := []struct {
		name      string
		expr      ast.FilterExpr
		checkFunc func(t *testing.T, filter *qdrant.Filter)
	}{
		{
			name: "string inequality",
			expr: ast.CompareExpr{Field: "status", Op: "!=", Value: "archived"},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Must, 1)
				match := filter.Must[0].GetField().GetMatch()
				require.NotNil(t, match)
				assert.Equal(t, []string{"archived"}, match.GetExceptKeywords().GetStrings())
			},
		},
		{
			name: "int inequality",
			expr: ast.CompareExpr{Field: "count", Op: "!=", Value: 7},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Must, 1)
				match := filter.Must[0].GetField().GetMatch()
				require.NotNil(t, match)
				assert.Equal(t, []int64{7}, match.GetExceptIntegers().GetIntegers())
			},
		},
		{
			name: "float inequality",
			expr: ast.CompareExpr{Field: "score", Op: "!=", Value: 1.5},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.MustNot, 1)
				rng := filter.MustNot[0].GetField().GetRange()
				require.NotNil(t, rng)
				assert.Equal(t, 1.5, rng.GetGte())
				assert.Equal(t, 1.5, rng.GetLte())
			},
		},
		{
			name: "bool inequality",
			expr: ast.CompareExpr{Field: "is_active", Op: "!=", Value: false},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.MustNot, 1)
				match := filter.MustNot[0].GetField().GetMatch()
				require.NotNil(t, match)
				assert.False(t, match.GetBoolean())
			},
		},
	}

	converter := NewFilterConverter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := converter.BuildFilter(tt.expr)
			require.NoError(t, err)
			require.NotNil(t, filter)
			tt.checkFunc(t, filter)
		})
	}
}

func TestBuildFilterTypedInAndNotIn(t *testing.T) {
	tests := []struct {
		name      string
		expr      ast.FilterExpr
		checkFunc func(t *testing.T, filter *qdrant.Filter)
	}{
		{
			name: "string in",
			expr: ast.InExpr{Field: "status", Values: []any{"active", "pending"}},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Must, 1)
				match := filter.Must[0].GetField().GetMatch()
				require.NotNil(t, match)
				assert.Equal(t, []string{"active", "pending"}, match.GetKeywords().GetStrings())
			},
		},
		{
			name: "int in",
			expr: ast.InExpr{Field: "count", Values: []any{1, 2}},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Must, 1)
				match := filter.Must[0].GetField().GetMatch()
				require.NotNil(t, match)
				assert.Equal(t, []int64{1, 2}, match.GetIntegers().GetIntegers())
			},
		},
		{
			name: "float in",
			expr: ast.InExpr{Field: "score", Values: []any{1.25, 2.5}},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Should, 2)
				for i, want := range []float64{1.25, 2.5} {
					rng := filter.Should[i].GetField().GetRange()
					require.NotNil(t, rng)
					assert.Equal(t, want, rng.GetGte())
					assert.Equal(t, want, rng.GetLte())
				}
			},
		},
		{
			name: "bool in",
			expr: ast.InExpr{Field: "is_active", Values: []any{true, false}},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Should, 2)
				assert.True(t, filter.Should[0].GetField().GetMatch().GetBoolean())
				assert.False(t, filter.Should[1].GetField().GetMatch().GetBoolean())
			},
		},
		{
			name: "string not in",
			expr: ast.NotInExpr{Field: "status", Values: []any{"deleted", "archived"}},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Must, 1)
				match := filter.Must[0].GetField().GetMatch()
				require.NotNil(t, match)
				assert.Equal(t, []string{"deleted", "archived"}, match.GetExceptKeywords().GetStrings())
			},
		},
		{
			name: "int not in",
			expr: ast.NotInExpr{Field: "count", Values: []any{3, 4}},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.Must, 1)
				match := filter.Must[0].GetField().GetMatch()
				require.NotNil(t, match)
				assert.Equal(t, []int64{3, 4}, match.GetExceptIntegers().GetIntegers())
			},
		},
		{
			name: "float not in",
			expr: ast.NotInExpr{Field: "score", Values: []any{4.5, 9.0}},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.MustNot, 2)
				for i, want := range []float64{4.5, 9.0} {
					rng := filter.MustNot[i].GetField().GetRange()
					require.NotNil(t, rng)
					assert.Equal(t, want, rng.GetGte())
					assert.Equal(t, want, rng.GetLte())
				}
			},
		},
		{
			name: "bool not in",
			expr: ast.NotInExpr{Field: "is_active", Values: []any{true, false}},
			checkFunc: func(t *testing.T, filter *qdrant.Filter) {
				require.Len(t, filter.MustNot, 2)
				assert.True(t, filter.MustNot[0].GetField().GetMatch().GetBoolean())
				assert.False(t, filter.MustNot[1].GetField().GetMatch().GetBoolean())
			},
		},
	}

	converter := NewFilterConverter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := converter.BuildFilter(tt.expr)
			require.NoError(t, err)
			require.NotNil(t, filter)
			tt.checkFunc(t, filter)
		})
	}
}

func TestBuildFilterRejectsMixedInTypes(t *testing.T) {
	converter := NewFilterConverter()

	_, err := converter.BuildFilter(ast.InExpr{
		Field:  "mixed",
		Values: []any{"active", 1},
	})

	require.Error(t, err)
}

func TestBuildFilterSupportsPointerExpressions(t *testing.T) {
	converter := NewFilterConverter()

	filter, err := converter.BuildFilter(&ast.CompareExpr{Field: "status", Op: "=", Value: "active"})

	require.NoError(t, err)
	require.NotNil(t, filter)
	require.Len(t, filter.Must, 1)
	assert.Equal(t, "active", filter.Must[0].GetField().GetMatch().GetKeyword())
}

func TestBuildFilterLogicalExpressions(t *testing.T) {
	converter := NewFilterConverter()

	filter, err := converter.BuildFilter(&ast.OrExpr{
		Operands: []ast.FilterExpr{
			&ast.AndExpr{
				Operands: []ast.FilterExpr{
					&ast.CompareExpr{Field: "status", Op: "=", Value: "active"},
					&ast.BetweenExpr{Field: "score", Low: 1, High: 5},
				},
			},
			&ast.NotExpr{
				Operand: &ast.IsNullExpr{Field: "category"},
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, filter)
	require.Len(t, filter.Should, 2)
	require.Len(t, filter.MustNot, 0)

	left := filter.Should[0].GetFilter()
	require.NotNil(t, left)
	require.Len(t, left.Must, 2)
	assert.Equal(t, "active", left.Must[0].GetField().GetMatch().GetKeyword())
	rangeCond := left.Must[1].GetField().GetRange()
	require.NotNil(t, rangeCond)
	assert.Equal(t, 1.0, rangeCond.GetGte())
	assert.Equal(t, 5.0, rangeCond.GetLte())

	right := filter.Should[1].GetFilter()
	require.NotNil(t, right)
	require.Len(t, right.MustNot, 1)
	assert.Equal(t, "category", right.MustNot[0].GetIsNull().GetKey())
}

func TestBuildFilterMatchExpressions(t *testing.T) {
	tests := []struct {
		name string
		expr ast.FilterExpr
		text string
		read func(match *qdrant.Match) string
	}{
		{
			name: "match text",
			expr: &ast.MatchTextExpr{Field: "content", Text: "hello world"},
			text: "hello world",
			read: func(match *qdrant.Match) string { return match.GetText() },
		},
		{
			name: "match any",
			expr: &ast.MatchAnyExpr{Field: "content", Text: "hello world"},
			text: "hello world",
			read: func(match *qdrant.Match) string { return match.GetTextAny() },
		},
		{
			name: "match phrase",
			expr: &ast.MatchPhraseExpr{Field: "content", Text: "hello world"},
			text: "hello world",
			read: func(match *qdrant.Match) string { return match.GetPhrase() },
		},
	}

	converter := NewFilterConverter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := converter.BuildFilter(tt.expr)
			require.NoError(t, err)
			require.NotNil(t, filter)
			require.Len(t, filter.Must, 1)
			assert.Equal(t, tt.text, tt.read(filter.Must[0].GetField().GetMatch()))
		})
	}
}
