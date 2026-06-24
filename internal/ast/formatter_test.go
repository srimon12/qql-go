package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatFilterExprCompare(t *testing.T) {
	expr := CompareExpr{Field: "age", Op: ">=", Value: float64(18)}
	assert.Equal(t, "age >= 18", FormatFilterExpr(expr))
}

func TestFormatFilterExprBetween(t *testing.T) {
	expr := BetweenExpr{Field: "price", Low: float64(10), High: float64(100)}
	assert.Equal(t, "price BETWEEN 10 AND 100", FormatFilterExpr(expr))
}

func TestFormatFilterExprIn(t *testing.T) {
	expr := InExpr{Field: "status", Values: []any{"active", "pending"}}
	assert.Equal(t, "status IN ('active', 'pending')", FormatFilterExpr(expr))
}

func TestFormatFilterExprNotIn(t *testing.T) {
	expr := NotInExpr{Field: "status", Values: []any{"deleted"}}
	assert.Equal(t, "status NOT IN ('deleted')", FormatFilterExpr(expr))
}

func TestFormatFilterExprIsNull(t *testing.T) {
	assert.Equal(t, "field IS NULL", FormatFilterExpr(IsNullExpr{Field: "field"}))
}

func TestFormatFilterExprIsNotNull(t *testing.T) {
	assert.Equal(t, "field IS NOT NULL", FormatFilterExpr(IsNotNullExpr{Field: "field"}))
}

func TestFormatFilterExprIsEmpty(t *testing.T) {
	assert.Equal(t, "field IS EMPTY", FormatFilterExpr(IsEmptyExpr{Field: "field"}))
}

func TestFormatFilterExprIsNotEmpty(t *testing.T) {
	assert.Equal(t, "field IS NOT EMPTY", FormatFilterExpr(IsNotEmptyExpr{Field: "field"}))
}

func TestFormatFilterExprMatchText(t *testing.T) {
	expr := MatchTextExpr{Field: "title", Text: "hello"}
	assert.Equal(t, "title MATCH 'hello'", FormatFilterExpr(expr))
}

func TestFormatFilterExprMatchAny(t *testing.T) {
	expr := MatchAnyExpr{Field: "title", Text: "hello"}
	assert.Equal(t, "title MATCH ANY 'hello'", FormatFilterExpr(expr))
}

func TestFormatFilterExprMatchPhrase(t *testing.T) {
	expr := MatchPhraseExpr{Field: "title", Text: "hello world"}
	assert.Equal(t, "title MATCH PHRASE 'hello world'", FormatFilterExpr(expr))
}

func TestFormatFilterExprAnd(t *testing.T) {
	expr := AndExpr{
		Operands: []FilterExpr{
			CompareExpr{Field: "age", Op: ">=", Value: float64(18)},
			CompareExpr{Field: "status", Op: "=", Value: "active"},
		},
	}
	assert.Equal(t, "age >= 18 AND status = 'active'", FormatFilterExpr(expr))
}

func TestFormatFilterExprOr(t *testing.T) {
	expr := OrExpr{
		Operands: []FilterExpr{
			CompareExpr{Field: "role", Op: "=", Value: "admin"},
			CompareExpr{Field: "role", Op: "=", Value: "mod"},
		},
	}
	assert.Equal(t, "(role = 'admin' OR role = 'mod')", FormatFilterExpr(expr))
}

func TestFormatFilterExprNot(t *testing.T) {
	expr := NotExpr{Operand: CompareExpr{Field: "archived", Op: "=", Value: true}}
	assert.Equal(t, "NOT archived = true", FormatFilterExpr(expr))
}

func TestFormatQueryStmtBasic(t *testing.T) {
	q := &QueryStmt{
		Collection: "docs",
		Mode:       QueryModeNearest,
		QueryText:  strPtr("search term"),
		Model:      strPtr("all-MiniLM-L6-v2"),
		Limit:      10,
	}
	result := FormatQueryStmt(q)
	assert.Contains(t, result, "QUERY 'search term'")
	assert.Contains(t, result, "FROM docs")
	assert.Contains(t, result, "WITH MODEL 'all-MiniLM-L6-v2'")
	assert.Contains(t, result, "LIMIT 10")
}

func TestFormatQueryStmtHybrid(t *testing.T) {
	q := &QueryStmt{
		Collection: "docs",
		Mode:       QueryModeNearest,
		QueryText:  strPtr("test"),
		Limit:      5,
		Type:       QueryTypeHybrid,
	}
	result := FormatQueryStmt(q)
	assert.Contains(t, result, "QUERY 'test'")
	assert.Contains(t, result, "USING HYBRID")
}

func TestFormatQueryStmtWithFilter(t *testing.T) {
	q := &QueryStmt{
		Collection:  "docs",
		Mode:        QueryModeNearest,
		QueryText:   strPtr("test"),
		Limit:       5,
		QueryFilter: CompareExpr{Field: "status", Op: "=", Value: "active"},
	}
	result := FormatQueryStmt(q)
	assert.Contains(t, result, "WHERE status = 'active'")
}

func TestFormatQueryStmtRecommend(t *testing.T) {
	q := &QueryStmt{
		Collection:  "docs",
		Mode:        QueryModeRecommend,
		PositiveIDs: []any{"id1", "id2"},
		NegativeIDs: []any{"id3"},
	}
	result := FormatQueryStmt(q)
	assert.Contains(t, result, "QUERY RECOMMEND")
	assert.Contains(t, result, "positive = ('id1', 'id2')")
	assert.Contains(t, result, "negative = ('id3')")
}

func TestFormatQueryStmtDiscover(t *testing.T) {
	q := &QueryStmt{
		Collection: "docs",
		Mode:       QueryModeDiscover,
		Target:     "target-id",
	}
	result := FormatQueryStmt(q)
	assert.Contains(t, result, "QUERY DISCOVER TARGET 'target-id'")
}

func TestFormatQueryStmtGroupBy(t *testing.T) {
	q := &QueryStmt{
		Collection: "docs",
		Mode:       QueryModeNearest,
		QueryText:  strPtr("test"),
		Limit:      10,
		GroupBy:    strPtr("category"),
		GroupSize:  intPtr(3),
	}
	result := FormatQueryStmt(q)
	assert.Contains(t, result, "GROUP BY 'category'")
	assert.Contains(t, result, "GROUP_SIZE 3")
}

func TestFormatQueryStmtScoreThreshold(t *testing.T) {
	q := &QueryStmt{
		Collection:     "docs",
		Mode:           QueryModeNearest,
		QueryText:      strPtr("test"),
		Limit:          10,
		ScoreThreshold: float64Ptr(0.5),
	}
	result := FormatQueryStmt(q)
	assert.Contains(t, result, "SCORE THRESHOLD 0.5")
}

func TestFormatQueryStmtOrderBy(t *testing.T) {
	asc := true
	q := &QueryStmt{
		Collection:    "docs",
		Mode:          QueryModeOrderBy,
		OrderByField:  strPtr("price"),
		OrderByAsc:    &asc,
		Limit:         10,
	}
	result := FormatQueryStmt(q)
	assert.Contains(t, result, "QUERY ORDER BY price ASC")
}

func TestFormatQueryStmtSample(t *testing.T) {
	q := &QueryStmt{
		Collection: "docs",
		Mode:       QueryModeSample,
		Limit:      5,
	}
	result := FormatQueryStmt(q)
	assert.Contains(t, result, "QUERY SAMPLE")
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func float64Ptr(f float64) *float64 { return &f }
