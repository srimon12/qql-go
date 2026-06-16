package pipeline

import (
	"context"
	"errors"
	"testing"

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
