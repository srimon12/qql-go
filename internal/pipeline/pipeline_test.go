package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockNode struct {
	executeFunc func(ctx context.Context, state *QueryState) error
}

func (m *mockNode) Execute(ctx context.Context, state *QueryState) error {
	return m.executeFunc(ctx, state)
}

func TestQueryPipeline_Execute_Success(t *testing.T) {
	p := NewQueryPipeline()
	state := &QueryState{QueryText: "test"}

	executed1 := false
	p.Add(&mockNode{
		executeFunc: func(ctx context.Context, state *QueryState) error {
			executed1 = true
			return nil
		},
	})

	executed2 := false
	p.Add(&mockNode{
		executeFunc: func(ctx context.Context, state *QueryState) error {
			executed2 = true
			return nil
		},
	})

	err := p.Execute(context.Background(), state)
	require.NoError(t, err)
	assert.True(t, executed1)
	assert.True(t, executed2)
}

func TestQueryPipeline_Execute_ErrorStopsExecution(t *testing.T) {
	p := NewQueryPipeline()
	state := &QueryState{QueryText: "test"}

	executed1 := false
	p.Add(&mockNode{
		executeFunc: func(ctx context.Context, state *QueryState) error {
			executed1 = true
			return errors.New("node 1 failed")
		},
	})

	executed2 := false
	p.Add(&mockNode{
		executeFunc: func(ctx context.Context, state *QueryState) error {
			executed2 = true
			return nil
		},
	})

	err := p.Execute(context.Background(), state)
	require.Error(t, err)
	assert.Equal(t, "node 1 failed", err.Error())
	assert.True(t, executed1)
	assert.False(t, executed2)
}

func TestBuildFlatRequest_SetsAllFields(t *testing.T) {
	p := NewQueryPipeline()
	state := &QueryState{
		CollectionName: "docs",
		VectorName:     "dense",
		Limit:          10,
		Offset:         5,
		ScoreThreshold: qdrant.PtrOf(float32(0.5)),
		RequestTimeout: qdrant.PtrOf(uint64(30)),
		TargetQuery:    qdrant.NewQuerySample(qdrant.Sample_Random),
	}

	req := p.BuildFlatRequest(state)

	assert.Equal(t, "docs", req.GetCollectionName())
	assert.Equal(t, uint64(10), req.GetLimit())
	assert.Equal(t, uint64(5), req.GetOffset())
	assert.Equal(t, "dense", req.GetUsing())
	assert.InDelta(t, 0.5, req.GetScoreThreshold(), 1e-6)
	assert.Equal(t, uint64(30), req.GetTimeout())
}

func TestBuildFlatRequest_OmitsZeroOffset(t *testing.T) {
	p := NewQueryPipeline()
	state := &QueryState{
		CollectionName: "docs",
		Limit:          10,
		Offset:         0,
	}

	req := p.BuildFlatRequest(state)
	assert.Nil(t, req.Offset)
}

func TestBuildGroupedRequest_InheritsFlatFields(t *testing.T) {
	p := NewQueryPipeline()
	state := &QueryState{
		CollectionName: "docs",
		Limit:          10,
		GroupBy:        "category",
		GroupSize:      3,
		TargetQuery:    qdrant.NewQuerySample(qdrant.Sample_Random),
	}

	req := p.BuildGroupedRequest(state)

	assert.Equal(t, "docs", req.GetCollectionName())
	assert.Equal(t, "category", req.GetGroupBy())
	assert.Equal(t, uint64(3), req.GetGroupSize())
}

func TestGetDocOptions_CachesResult(t *testing.T) {
	state := &QueryState{
		CloudModelOptions: map[string]string{"key": "val"},
	}

	opts1 := state.GetDocOptions()
	opts2 := state.GetDocOptions()

	require.NotNil(t, opts1)
	assert.Equal(t, "val", opts1["key"].GetStringValue())
	// Verify caching: both calls return the same map (pointer identity via field)
	assert.Equal(t, opts1, opts2)
}

func TestGetDocOptions_NilForEmptyConfig(t *testing.T) {
	state := &QueryState{}
	assert.Nil(t, state.GetDocOptions())
}

func TestBuildExpression_MatchCondition(t *testing.T) {
	expr, err := BuildExpression(ast.FormulaMatchCondition{
		Field:  "tag",
		Values: []any{"h1", "h2", "h3"},
	})
	require.NoError(t, err)
	require.NotNil(t, expr)

	cond := expr.GetCondition()
	require.NotNil(t, cond)

	fieldCond := cond.GetField()
	require.NotNil(t, fieldCond)
	assert.Equal(t, "tag", fieldCond.GetKey())

	match := fieldCond.GetMatch()
	require.NotNil(t, match)
	assert.NotNil(t, match.GetKeywords())
	assert.Equal(t, []string{"h1", "h2", "h3"}, match.GetKeywords().GetStrings())
}

func TestBuildExpression_MatchConditionSingle(t *testing.T) {
	expr, err := BuildExpression(ast.FormulaMatchCondition{
		Field:  "category",
		Values: []any{"premium"},
	})
	require.NoError(t, err)
	require.NotNil(t, expr)

	cond := expr.GetCondition()
	require.NotNil(t, cond)

	fieldCond := cond.GetField()
	require.NotNil(t, fieldCond)
	assert.Equal(t, "category", fieldCond.GetKey())

	match := fieldCond.GetMatch()
	require.NotNil(t, match)
	assert.Equal(t, "premium", match.GetKeyword())
}

func TestBuildExpression_MatchConditionNumeric(t *testing.T) {
	expr, err := BuildExpression(ast.FormulaMatchCondition{
		Field:  "count",
		Values: []any{1, 2, 3},
	})
	require.NoError(t, err)
	require.NotNil(t, expr)

	cond := expr.GetCondition()
	require.NotNil(t, cond)

	fieldCond := cond.GetField()
	require.NotNil(t, fieldCond)
	assert.Equal(t, "count", fieldCond.GetKey())

	match := fieldCond.GetMatch()
	require.NotNil(t, match)
	assert.Equal(t, []int64{1, 2, 3}, match.GetIntegers().GetIntegers())
}
