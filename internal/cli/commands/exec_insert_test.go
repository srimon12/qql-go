package commands

import (
	"context"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/srimon12/qql-go/internal/parser"
	"github.com/stretchr/testify/require"
)

func TestBuildInsertVectorsIncludesColbertOnlyWhenEnabled(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{InferenceMode: "cloud"})
	vectors, err := exec.buildInsertVectors(context.Background(), "hello world", "dense-model", "sparse-model", true, true, "test-collection", "dense", "sparse")
	require.NoError(t, err)

	dense := vectors[denseVectorName]
	require.NotNil(t, dense)
	require.NotNil(t, dense.GetDocument())
	require.Equal(t, "hello world", dense.GetDocument().GetText())
	require.Equal(t, "dense-model", dense.GetDocument().GetModel())

	rerank := vectors[rerankVectorName]
	require.NotNil(t, rerank)
	require.NotNil(t, rerank.GetDocument())
	require.Equal(t, rerankModelDefault, rerank.GetDocument().GetModel())

	sparse := vectors[sparseVectorName]
	require.NotNil(t, sparse)
	require.NotNil(t, sparse.GetDocument())
	require.Equal(t, "sparse-model", sparse.GetDocument().GetModel())

	withoutRerank, err := exec.buildInsertVectors(context.Background(), "hello world", "dense-model", "sparse-model", true, false, "test-collection", "dense", "sparse")
	require.NoError(t, err)
	require.NotContains(t, withoutRerank, rerankVectorName)
}

func TestInsertPointIDAndPayloadHonorsExplicitID(t *testing.T) {
	pointID, payload, err := insertPointIDAndPayload(42, map[string]any{"text": "hello"})
	require.NoError(t, err)
	require.Equal(t, 42, pointID)
	require.Equal(t, map[string]any{"text": "hello"}, payload)
}

func TestInsertPointIDAndPayloadExtractsIDFromValues(t *testing.T) {
	id := "123e4567-e89b-12d3-a456-426614174000"
	pointID, payload, err := insertPointIDAndPayload(nil, map[string]any{"id": id, "text": "hello"})
	require.NoError(t, err)
	require.Equal(t, id, pointID)
	require.Equal(t, map[string]any{"text": "hello"}, payload)
}

func TestInsertAutoCreatesMissingCollection(t *testing.T) {
	client := newFakeQdrantClient()
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	resp, err := exec.doInsert(&ast.InsertStmt{
		Collection: "docs",
		Values:     map[string]any{"text": "hello"},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Equal(t, "insert", resp.Operation)
	data := resp.Data.(map[string]any)
	require.True(t, data["created"].(bool))
	require.Len(t, client.createRequests, 1)
	require.Len(t, client.upserts, 1)
	require.False(t, client.createRequests[0].SparseVectorsConfig != nil)
}

func TestInsertPreservesHybridAutodetectionOnExistingCollection(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				VectorsConfig: qdrant.NewVectorsConfigMap(collectionVectorParams(denseVectorSize, false)),
				SparseVectorsConfig: qdrant.NewSparseVectorsConfig(map[string]*qdrant.SparseVectorParams{
					sparseVectorName: {Modifier: qdrant.Modifier_Idf.Enum()},
				}),
			},
		},
	}

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	resp, err := exec.doInsert(&ast.InsertStmt{
		Collection: "docs",
		Values:     map[string]any{"text": "hello"},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	data := resp.Data.(map[string]any)
	require.True(t, data["hybrid"].(bool))
	require.Len(t, client.createRequests, 0)
	require.Len(t, client.upserts, 1)
}

func TestParserKeepsInsertHybridAutoDetection(t *testing.T) {
	tokens, err := (&lexer.Lexer{}).Tokenize("INSERT INTO COLLECTION docs VALUES {'text': 'hello'} USING HYBRID")
	require.NoError(t, err)
	node, err := parser.NewParser().Parse(tokens)
	require.NoError(t, err)
	insert, ok := node.(*ast.InsertStmt)
	require.True(t, ok)
	require.True(t, insert.Hybrid)
}

func TestBuildInsertVectorsLocalModeGeneratesExplicitVectors(t *testing.T) {
	server := newEmbeddingServer(t, []float32{1, 2, 3})
	defer server.Close()

	exec := NewExecutor(nil, &config.Config{
		InferenceMode:      "local",
		EmbeddingEndpoint:  server.URL + "/v1/embeddings",
		EmbeddingModel:     "test-model",
		EmbeddingDimension: 3,
	})

	vectors, err := exec.buildInsertVectors(context.Background(), "hello world", "dense-model", "", true, false, "test_local", "dense", "sparse")
	require.NoError(t, err)

	dense := vectors[denseVectorName]
	require.NotNil(t, dense)

	sparseVec := vectors[sparseVectorName]
	require.NotNil(t, sparseVec)

	require.NotContains(t, vectors, rerankVectorName)
}

func TestBuildInsertVectorsLocalModeRejectsCustomSparseModel(t *testing.T) {
	server := newEmbeddingServer(t, []float32{1, 2, 3})
	defer server.Close()

	exec := NewExecutor(nil, &config.Config{
		InferenceMode:      "local",
		EmbeddingEndpoint:  server.URL + "/v1/embeddings",
		EmbeddingModel:     "test-model",
		EmbeddingDimension: 3,
	})

	_, err := exec.buildInsertVectors(context.Background(), "hello", "dense-model", "sparse-model", true, false, "test_local", "dense", "sparse")
	require.Error(t, err)
	require.Contains(t, err.Error(), "local embedding mode only supports BM25 sparse vectors")
}

func TestBuildInsertVectorsLocalModeRejectsRerank(t *testing.T) {
	server := newEmbeddingServer(t, []float32{1, 2, 3})
	defer server.Close()

	exec := NewExecutor(nil, &config.Config{
		InferenceMode:      "local",
		EmbeddingEndpoint:  server.URL + "/v1/embeddings",
		EmbeddingModel:     "test-model",
		EmbeddingDimension: 3,
	})

	_, err := exec.buildInsertVectors(context.Background(), "hello", "dense-model", "", true, true, "test_local", "dense", "sparse")
	require.Error(t, err)
	require.Contains(t, err.Error(), "rerank vectors are not implemented yet")
}
