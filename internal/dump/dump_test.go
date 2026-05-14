package dump

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/stretchr/testify/require"
)

type fakeClient struct {
	hybrid bool
	points [][]*qdrant.RetrievedPoint
}

func (f *fakeClient) CollectionExists(context.Context, string) (bool, error) { return true, nil }
func (f *fakeClient) GetCollectionInfo(context.Context, string) (*qdrant.CollectionInfo, error) {
	info := &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
					"dense": {Size: 3, Distance: qdrant.Distance_Cosine},
				}),
			},
		},
	}
	if f.hybrid {
		info.Config.Params.SparseVectorsConfig = qdrant.NewSparseVectorsConfig(map[string]*qdrant.SparseVectorParams{
			"sparse": {},
		})
	}
	return info, nil
}
func (f *fakeClient) Count(context.Context, *qdrant.CountPoints) (uint64, error) {
	total := 0
	for _, batch := range f.points {
		total += len(batch)
	}
	return uint64(total), nil
}
func (f *fakeClient) ScrollAndOffset(_ context.Context, request *qdrant.ScrollPoints) ([]*qdrant.RetrievedPoint, *qdrant.PointId, error) {
	if request.Offset == nil {
		if len(f.points) == 0 {
			return nil, nil, nil
		}
		if len(f.points) == 1 {
			return f.points[0], nil, nil
		}
		return f.points[0], qdrant.NewIDNum(2), nil
	}
	if request.Offset.GetNum() == 2 && len(f.points) > 1 {
		return f.points[1], nil, nil
	}
	return nil, nil, nil
}

func TestCollection(t *testing.T) {
	client := &fakeClient{
		hybrid: true,
		points: [][]*qdrant.RetrievedPoint{
			{
				{
					Id: qdrant.NewIDUUID("123e4567-e89b-12d3-a456-426614174000"),
					Payload: qdrant.NewValueMap(map[string]any{
						"text":  "hello",
						"topic": "search",
					}),
				},
				{
					Id: qdrant.NewIDNum(7),
					Payload: qdrant.NewValueMap(map[string]any{
						"topic": "skip-me",
					}),
				},
			},
		},
	}

	outputPath := filepath.Join(t.TempDir(), "dump.qql")
	written, skipped, err := Collection(context.Background(), client, "docs", outputPath, 50)
	require.NoError(t, err)
	require.Equal(t, 1, written)
	require.Equal(t, 1, skipped)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	text := string(data)
	require.Contains(t, text, "CREATE COLLECTION docs HYBRID")
	require.Contains(t, text, "INSERT BULK INTO COLLECTION docs VALUES [")
	require.Contains(t, text, "USING HYBRID")
	require.Contains(t, text, "'id': '123e4567-e89b-12d3-a456-426614174000'")
}

func TestCollectionRejectsInvalidBatchSize(t *testing.T) {
	client := &fakeClient{}
	outputPath := filepath.Join(t.TempDir(), "dump.qql")

	written, skipped, err := Collection(context.Background(), client, "docs", outputPath, 0)
	require.Error(t, err)
	require.Zero(t, written)
	require.Zero(t, skipped)
	require.Contains(t, err.Error(), "batch size must be greater than 0")
}
