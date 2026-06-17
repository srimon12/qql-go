package commands

import (
	"context"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildDeleteRequestByFieldUsesFilterSelector(t *testing.T) {
	req, err := buildDeleteRequest(&ast.DeleteStmt{
		Collection: "demo",
		Field:      "status",
		Value:      "archived",
	}, nil)
	require.NoError(t, err)

	require.Equal(t, "demo", req.GetCollectionName())
	filter := req.GetPoints().GetFilter()
	require.NotNil(t, filter)
	require.Len(t, filter.GetMust(), 1)
	match := filter.GetMust()[0].GetField().GetMatch()
	require.NotNil(t, match)
	require.Equal(t, "archived", match.GetKeyword())
}

func TestBuildDeleteRequestByIDUsesPointSelector(t *testing.T) {
	req, err := buildDeleteRequest(&ast.DeleteStmt{
		Collection: "demo",
		PointID:    "point-123",
	}, nil)
	require.NoError(t, err)

	require.Equal(t, "demo", req.GetCollectionName())
	points, ok := req.GetPoints().GetPointsSelectorOneOf().(*qdrant.PointsSelector_Points)
	require.True(t, ok)
	require.NotNil(t, points.Points)
	require.Len(t, points.Points.GetIds(), 1)
	require.Equal(t, "point-123", points.Points.GetIds()[0].GetUuid())
}

func TestBuildUpdateVectorRequestUsesNamedDenseVectorForHybridCollections(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
					denseVectorName: {Size: uint64(denseVectorSize), Distance: qdrant.Distance_Cosine},
				}),
			},
		},
	}
	exec := NewExecutor(client, &config.Config{})

	req, err := exec.buildUpdateVectorRequest(context.Background(), &ast.UpdateVectorStmt{
		Collection: "docs",
		PointID:    7,
		Vector:     []float32{0.1, 0.2},
	}, "dense", true)
	require.NoError(t, err)
	require.Len(t, req.GetPoints(), 1)
	require.NotNil(t, req.GetPoints()[0].GetVectors().GetVectors().GetVectors()[denseVectorName])
}

func TestBuildUpdatePayloadRequestSupportsFilterSelector(t *testing.T) {
	req, err := buildUpdatePayloadRequest(&ast.UpdatePayloadStmt{
		Collection:  "docs",
		QueryFilter: &ast.CompareExpr{Field: "status", Op: "=", Value: "draft"},
		Payload:     map[string]any{"status": "published"},
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, req.GetPointsSelector().GetFilter())
	require.Equal(t, "published", req.GetPayload()["status"].GetStringValue())
}
