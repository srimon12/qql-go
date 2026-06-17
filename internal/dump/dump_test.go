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
	info   *qdrant.CollectionInfo
	points [][]*qdrant.RetrievedPoint
}

func (f *fakeClient) CollectionExists(context.Context, string) (bool, error) { return true, nil }
func (f *fakeClient) GetCollectionInfo(context.Context, string) (*qdrant.CollectionInfo, error) {
	if f.info != nil {
		return f.info, nil
	}
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
	require.Contains(t, text, "INSERT INTO docs VALUES")
	require.Contains(t, text, "USING HYBRID")
	require.Contains(t, text, "'id': '123e4567-e89b-12d3-a456-426614174000'")
}

func TestCollectionDumpsRuntimeConfigBlocks(t *testing.T) {
	client := &fakeClient{
		info: &qdrant.CollectionInfo{
			Config: &qdrant.CollectionConfig{
				Params: &qdrant.CollectionParams{
					VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
						"dense": {Size: 3, Distance: qdrant.Distance_Cosine, OnDisk: qdrant.PtrOf(true)},
					}),
					OnDiskPayload: true,
				},
				OptimizerConfig: &qdrant.OptimizersConfigDiff{
					MaxOptimizationThreads: &qdrant.MaxOptimizationThreads{
						Variant: &qdrant.MaxOptimizationThreads_Value{Value: 3},
					},
				},
				QuantizationConfig: qdrant.NewQuantizationTurbo(&qdrant.TurboQuantization{
					Bits:      qdrant.TurboQuantBitSize_Bits1_5.Enum(),
					AlwaysRam: qdrant.PtrOf(true),
				}),
			},
		},
		points: [][]*qdrant.RetrievedPoint{
			{
				{
					Id: qdrant.NewIDNum(1),
					Payload: qdrant.NewValueMap(map[string]any{
						"text": "hello",
					}),
				},
			},
		},
	}

	outputPath := filepath.Join(t.TempDir(), "dump.qql")
	_, _, err := Collection(context.Background(), client, "docs", outputPath, 50)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	text := string(data)
	require.Contains(t, text, "WITH VECTORS (on_disk = true)")
	require.Contains(t, text, "WITH OPTIMIZERS (max_optimization_threads = 3)")
	require.Contains(t, text, "WITH PARAMS (on_disk_payload = true)")
	require.Contains(t, text, "WITH QUANTIZATION (type = 'turbo', bits = 1.5, always_ram = true)")
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

func TestCollectionDumpsPayloadIndexes(t *testing.T) {
	client := &fakeClient{
		info: &qdrant.CollectionInfo{
			Config: &qdrant.CollectionConfig{
				Params: &qdrant.CollectionParams{
					VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
						"dense": {Size: 3, Distance: qdrant.Distance_Cosine},
					}),
				},
			},
			PayloadSchema: map[string]*qdrant.PayloadSchemaInfo{
				"category": {DataType: qdrant.PayloadSchemaType_Keyword},
				"title": {
					DataType: qdrant.PayloadSchemaType_Text,
					Params:   qdrant.NewPayloadIndexParamsText(&qdrant.TextIndexParams{Tokenizer: qdrant.TokenizerType_Word, Lowercase: qdrant.PtrOf(true)}),
				},
				"uuid_field": {
					DataType: qdrant.PayloadSchemaType_Uuid,
					Params:   qdrant.NewPayloadIndexParamsUUID(&qdrant.UuidIndexParams{IsTenant: qdrant.PtrOf(true)}),
				},
			},
		},
		points: [][]*qdrant.RetrievedPoint{
			{
				{
					Id: qdrant.NewIDNum(1),
					Payload: qdrant.NewValueMap(map[string]any{
						"text": "hello",
					}),
				},
			},
		},
	}

	outputPath := filepath.Join(t.TempDir(), "dump.qql")
	_, _, err := Collection(context.Background(), client, "docs", outputPath, 50)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	text := string(data)
	require.Contains(t, text, "CREATE INDEX ON docs FOR title TYPE text WITH (")
	require.Contains(t, text, "CREATE INDEX ON docs FOR uuid_field TYPE uuid WITH (")
	require.NotContains(t, text, "CREATE INDEX ON docs FOR category")
}

func TestCollectionWithModel(t *testing.T) {
	client := &fakeClient{
		hybrid: true,
		points: [][]*qdrant.RetrievedPoint{
			{
				{
					Id: qdrant.NewIDNum(1),
					Payload: qdrant.NewValueMap(map[string]any{
						"text": "hello",
					}),
				},
			},
		},
	}

	outputPath := filepath.Join(t.TempDir(), "dump.qql")
	written, skipped, err := CollectionWithModel(context.Background(), client, "docs", outputPath, 50, "all-MiniLM-L6-v2", "sparse-model")
	require.NoError(t, err)
	require.Equal(t, 1, written)
	require.Equal(t, 0, skipped)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	text := string(data)
	require.Contains(t, text, "-- Default model: all-MiniLM-L6-v2")
	require.Contains(t, text, "-- Sparse model : sparse-model")
	require.Contains(t, text, "CREATE COLLECTION docs HYBRID DENSE MODEL 'all-MiniLM-L6-v2' SPARSE MODEL 'sparse-model'")
	require.Contains(t, text, "USING HYBRID DENSE MODEL 'all-MiniLM-L6-v2' SPARSE MODEL 'sparse-model'")
}

func TestCollectionWithModelDenseOnly(t *testing.T) {
	client := &fakeClient{
		points: [][]*qdrant.RetrievedPoint{
			{
				{
					Id: qdrant.NewIDNum(1),
					Payload: qdrant.NewValueMap(map[string]any{
						"text": "hello",
					}),
				},
			},
		},
	}

	outputPath := filepath.Join(t.TempDir(), "dump.qql")
	written, skipped, err := CollectionWithModel(context.Background(), client, "docs", outputPath, 50, "all-MiniLM-L6-v2", "")
	require.NoError(t, err)
	require.Equal(t, 1, written)
	require.Equal(t, 0, skipped)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	text := string(data)
	require.Contains(t, text, "-- Default model: all-MiniLM-L6-v2")
	require.Contains(t, text, "CREATE COLLECTION docs USING MODEL 'all-MiniLM-L6-v2'")
	require.Contains(t, text, "USING MODEL 'all-MiniLM-L6-v2'")
}

func TestCollectionDumpsVectors(t *testing.T) {
	client := &fakeClient{
		points: [][]*qdrant.RetrievedPoint{
			{
				{
					Id: qdrant.NewIDNum(1),
					Payload: qdrant.NewValueMap(map[string]any{
						"text":  "hello",
						"topic": "test",
					}),
					Vectors: &qdrant.VectorsOutput{
						VectorsOptions: &qdrant.VectorsOutput_Vector{
							Vector: &qdrant.VectorOutput{
								Vector: &qdrant.VectorOutput_Dense{
									Dense: &qdrant.DenseVector{Data: []float32{0.1, 0.2, 0.3}},
								},
							},
						},
					},
				},
			},
		},
	}

	outputPath := filepath.Join(t.TempDir(), "dump.qql")
	written, skipped, err := Collection(context.Background(), client, "docs", outputPath, 50)
	require.NoError(t, err)
	require.Equal(t, 1, written)
	require.Equal(t, 0, skipped)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	text := string(data)
	require.Contains(t, text, "'_v'")
	require.Contains(t, text, "0.1")
	require.Contains(t, text, "0.2")
	require.Contains(t, text, "0.3")
}

func TestCollectionDumpsNamedVectors(t *testing.T) {
	client := &fakeClient{
		info: &qdrant.CollectionInfo{
			Config: &qdrant.CollectionConfig{
				Params: &qdrant.CollectionParams{
					VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
						"my_dense": {Size: 3, Distance: qdrant.Distance_Cosine},
					}),
				},
			},
		},
		points: [][]*qdrant.RetrievedPoint{
			{
				{
					Id: qdrant.NewIDNum(1),
					Payload: qdrant.NewValueMap(map[string]any{
						"text": "hello",
					}),
					Vectors: &qdrant.VectorsOutput{
						VectorsOptions: &qdrant.VectorsOutput_Vectors{
							Vectors: &qdrant.NamedVectorsOutput{
								Vectors: map[string]*qdrant.VectorOutput{
									"my_dense": {
										Vector: &qdrant.VectorOutput_Dense{
											Dense: &qdrant.DenseVector{Data: []float32{0.5, 0.6}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	outputPath := filepath.Join(t.TempDir(), "dump.qql")
	written, skipped, err := Collection(context.Background(), client, "docs", outputPath, 50)
	require.NoError(t, err)
	require.Equal(t, 1, written)
	require.Equal(t, 0, skipped)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	text := string(data)
	require.Contains(t, text, "'_v_my__dense'")
}
