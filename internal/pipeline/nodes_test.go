package pipeline

import (
	"context"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEmbedder struct {
	dense         []float32
	sparseIndices []uint32
	sparseValues  []float32
	err           error
}

func (m *mockEmbedder) EmbedDense(ctx context.Context, text string, model string) ([]float32, error) {
	return m.dense, m.err
}

func (m *mockEmbedder) EmbedSparse(ctx context.Context, text string) ([]uint32, []float32, error) {
	return m.sparseIndices, m.sparseValues, m.err
}

func TestDenseEmbedNode_Execute_Cloud(t *testing.T) {
	n := &DenseEmbedNode{
		Model:      "test-model",
		VectorName: "dense",
		Limit:      10,
	}

	state := &QueryState{
		QueryText:  "hello",
		LocalEmbed: false,
	}

	err := n.Execute(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.TargetQuery)
	doc := state.TargetQuery.GetNearest().GetDocument()
	require.NotNil(t, doc)
	assert.Equal(t, "hello", doc.GetText())
	assert.Equal(t, "test-model", doc.GetModel())
}

func TestDenseEmbedNode_Execute_Local(t *testing.T) {
	n := &DenseEmbedNode{
		Model:      "test-model",
		VectorName: "dense",
		Limit:      10,
	}

	state := &QueryState{
		QueryText:  "hello",
		LocalEmbed: true,
		Embedder: &mockEmbedder{
			dense: []float32{0.1, 0.2, 0.3},
		},
	}

	err := n.Execute(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.TargetQuery)
	dense := state.TargetQuery.GetNearest().GetDense().GetData()
	require.NotNil(t, dense)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, dense)
}

func TestSparseEmbedNode_Execute_Cloud(t *testing.T) {
	n := &SparseEmbedNode{
		Model:      "test-sparse-model",
		VectorName: "sparse",
		Limit:      10,
	}

	state := &QueryState{
		QueryText:  "hello",
		LocalEmbed: false,
	}

	err := n.Execute(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.TargetQuery)
	doc := state.TargetQuery.GetNearest().GetDocument()
	require.NotNil(t, doc)
	assert.Equal(t, "hello", doc.GetText())
	assert.Equal(t, "test-sparse-model", doc.GetModel())
}

func TestSparseEmbedNode_Execute_Local(t *testing.T) {
	n := &SparseEmbedNode{
		Model:      "test-sparse-model",
		VectorName: "sparse",
		Limit:      10,
	}

	state := &QueryState{
		QueryText:  "hello",
		LocalEmbed: true,
		Embedder: &mockEmbedder{
			sparseIndices: []uint32{1, 5, 9},
			sparseValues:  []float32{0.1, 0.5, 0.9},
		},
	}

	err := n.Execute(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.TargetQuery)
	sparse := state.TargetQuery.GetNearest().GetSparse()
	require.NotNil(t, sparse)
	assert.Equal(t, []uint32{1, 5, 9}, sparse.Indices)
	assert.Equal(t, []float32{0.1, 0.5, 0.9}, sparse.Values)
}

func TestFusionNode_Execute(t *testing.T) {
	n := &FusionNode{Mode: "rrf"}
	state := &QueryState{}

	err := n.Execute(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.TargetQuery)
	assert.Equal(t, qdrant.Fusion_RRF, state.TargetQuery.GetFusion())

	n2 := &FusionNode{Mode: "dbsf"}
	err = n2.Execute(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, qdrant.Fusion_DBSF, state.TargetQuery.GetFusion())
}

func TestRerankNode_Execute(t *testing.T) {
	n := &RerankNode{Model: "rerank-model"}
	state := &QueryState{
		QueryText:  "hello",
		LocalEmbed: false,
	}

	err := n.Execute(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.TargetQuery)
	doc := state.TargetQuery.GetNearest().GetDocument()
	require.NotNil(t, doc)
	assert.Equal(t, "hello", doc.GetText())
	assert.Equal(t, "rerank-model", doc.GetModel())
}

func TestRecommendNode_Execute(t *testing.T) {
	n := &RecommendNode{
		PositiveIDs: []any{"123e4567-e89b-12d3-a456-426614174000", 42},
		NegativeIDs: []any{"123e4567-e89b-12d3-a456-426614174001"},
	}
	state := &QueryState{}

	err := n.Execute(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.TargetQuery)
	rec := state.TargetQuery.GetRecommend()
	require.NotNil(t, rec)
	assert.Len(t, rec.Positive, 2)
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", rec.Positive[0].GetId().GetUuid())
	assert.Equal(t, uint64(42), rec.Positive[1].GetId().GetNum())
	assert.Len(t, rec.Negative, 1)
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174001", rec.Negative[0].GetId().GetUuid())
}
