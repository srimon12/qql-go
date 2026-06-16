package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfiguredModelUsesConfigOverride(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{InferenceModel: "test-model"})

	require.Equal(t, "test-model", exec.configuredModel())
}

func TestConfiguredModelFallsBackToDenseDefault(t *testing.T) {
	exec := NewExecutor(nil, nil)

	require.Equal(t, denseModelDefault, exec.configuredModel())
}

func TestBuildQuantizationConfigScalar(t *testing.T) {
	cfg, err := buildQuantizationConfig(&ast.QuantizationConfig{
		Type:      ast.QuantizationTypeScalar,
		Quantile:  float64Ptr(0.95),
		AlwaysRAM: true,
	})
	require.NoError(t, err)

	require.NotNil(t, cfg)
	scalar := cfg.GetScalar()
	require.NotNil(t, scalar)
	require.Equal(t, qdrant.QuantizationType_Int8, scalar.GetType())
	require.InDelta(t, float32(0.95), scalar.GetQuantile(), 0.0001)
	require.True(t, scalar.GetAlwaysRam())
}

func TestBuildQuantizationConfigBinary(t *testing.T) {
	cfg, err := buildQuantizationConfig(&ast.QuantizationConfig{
		Type:      ast.QuantizationTypeBinary,
		AlwaysRAM: true,
	})
	require.NoError(t, err)

	require.NotNil(t, cfg)
	binary := cfg.GetBinary()
	require.NotNil(t, binary)
	require.True(t, binary.GetAlwaysRam())
}

func TestBuildQuantizationConfigProduct(t *testing.T) {
	cfg, err := buildQuantizationConfig(&ast.QuantizationConfig{
		Type: ast.QuantizationTypeProduct,
	})
	require.NoError(t, err)

	require.NotNil(t, cfg)
	product := cfg.GetProduct()
	require.NotNil(t, product)
	require.Equal(t, qdrant.CompressionRatio_x4, product.GetCompression())
	require.False(t, product.GetAlwaysRam())
}

func TestCreateCollectionIncludesQuantizationConfig(t *testing.T) {
	client := newFakeQdrantClient()
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	resp, err := exec.doCreateCollection(&ast.CreateCollectionStmt{
		Collection: "docs",
		Config: &ast.CollectionConfig{
			Quantization: &ast.QuantizationConfig{
				Type:      ast.QuantizationTypeScalar,
				Quantile:  float64Ptr(0.99),
				AlwaysRAM: true,
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Len(t, client.createRequests, 1)
	require.Contains(t, resp.Message, "scalar quantization")

	quantization := client.createRequests[0].GetQuantizationConfig()
	require.NotNil(t, quantization)
	require.NotNil(t, quantization.GetScalar())
	require.InDelta(t, float32(0.99), quantization.GetScalar().GetQuantile(), 0.0001)
	require.True(t, quantization.GetScalar().GetAlwaysRam())
}

func TestCreateCollectionHybridRerankIncludesBinaryQuantizationConfig(t *testing.T) {
	client := newFakeQdrantClient()
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	resp, err := exec.doCreateCollection(&ast.CreateCollectionStmt{
		Collection: "docs",
		Hybrid:     true,
		Rerank:     true,
		Config: &ast.CollectionConfig{
			Quantization: &ast.QuantizationConfig{
				Type:      ast.QuantizationTypeBinary,
				AlwaysRAM: true,
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Len(t, client.createRequests, 1)
	require.NotNil(t, client.createRequests[0].GetSparseVectorsConfig())
	require.NotNil(t, client.createRequests[0].GetQuantizationConfig().GetBinary())
	require.True(t, client.createRequests[0].GetQuantizationConfig().GetBinary().GetAlwaysRam())
}

func TestCreateCollectionIncludesPayloadM(t *testing.T) {
	client := newFakeQdrantClient()
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	resp, err := exec.doCreateCollection(&ast.CreateCollectionStmt{
		Collection: "docs",
		Config: &ast.CollectionConfig{
			Hnsw: &ast.HnswRuntimeConfig{
				PayloadM: uint64Ptr(24),
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Len(t, client.createRequests, 1)
	require.NotNil(t, client.createRequests[0].GetHnswConfig())
	require.Equal(t, uint64(24), client.createRequests[0].GetHnswConfig().GetPayloadM())
}

func TestAlterCollectionPassesConfigBlocks(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
					denseVectorName: {Size: denseVectorSize, Distance: qdrant.Distance_Cosine},
				}),
			},
		},
	}
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	resp, err := exec.doAlterCollection(&ast.AlterCollectionStmt{
		Collection: "docs",
		Config: &ast.CollectionConfig{
			Vectors:    &ast.VectorsConfig{OnDisk: boolPtr(true)},
			Hnsw:       &ast.HnswRuntimeConfig{FullScanThreshold: uint64Ptr(5000)},
			Optimizers: &ast.OptimizersRuntimeConfig{IndexingThreshold: uint64Ptr(10000)},
			Params:     &ast.CollectionParamsConfig{OnDiskPayload: boolPtr(false), ReadFanOutFactor: uint64Ptr(4)},
			QuantizationUpdate: &ast.QuantizationUpdate{
				Config: &ast.QuantizationConfig{Type: ast.QuantizationTypeBinary},
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Len(t, client.updateRequests, 1)
	req := client.updateRequests[0]

	vsMap := req.VectorsConfig.GetParamsMap().GetMap()
	require.NotNil(t, vsMap)
	require.True(t, vsMap[denseVectorName].GetOnDisk())

	require.NotNil(t, req.HnswConfig)
	require.Equal(t, uint64(5000), req.HnswConfig.GetFullScanThreshold())

	require.NotNil(t, req.OptimizersConfig)
	require.Equal(t, uint64(10000), req.OptimizersConfig.GetIndexingThreshold())

	require.NotNil(t, req.Params)
	require.False(t, req.Params.GetOnDiskPayload())
	require.Equal(t, uint32(4), req.Params.GetReadFanOutFactor())

	require.NotNil(t, req.QuantizationConfig)
	require.NotNil(t, req.QuantizationConfig.GetBinary())
}

func TestAlterCollectionVectorsOnDiskAppliesToAllDenseVectors(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
					"text":           {Size: denseVectorSize, Distance: qdrant.Distance_Cosine},
					"image":          {Size: denseVectorSize, Distance: qdrant.Distance_Cosine},
					rerankVectorName: {Size: rerankVectorSize, Distance: qdrant.Distance_Cosine},
				}),
				SparseVectorsConfig: qdrant.NewSparseVectorsConfig(map[string]*qdrant.SparseVectorParams{
					sparseVectorName: {},
				}),
			},
		},
	}
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	_, err := exec.doAlterCollection(&ast.AlterCollectionStmt{
		Collection: "docs",
		Config: &ast.CollectionConfig{
			Vectors: &ast.VectorsConfig{OnDisk: boolPtr(true)},
		},
	})
	require.NoError(t, err)

	vsMap := client.updateRequests[0].VectorsConfig.GetParamsMap().GetMap()
	require.Len(t, vsMap, 2)
	require.True(t, vsMap["text"].GetOnDisk())
	require.True(t, vsMap["image"].GetOnDisk())
	require.NotContains(t, vsMap, sparseVectorName)
	require.NotContains(t, vsMap, rerankVectorName)
}

func TestAlterCollectionCanSetTurboQuantization(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	resp, err := exec.doAlterCollection(&ast.AlterCollectionStmt{
		Collection: "docs",
		Config: &ast.CollectionConfig{
			QuantizationUpdate: &ast.QuantizationUpdate{
				Config: &ast.QuantizationConfig{
					Type:      ast.QuantizationTypeTurbo,
					TurboBits: float64Ptr(2.0),
					AlwaysRAM: true,
				},
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Len(t, client.updateRequests, 1)
	turbo := client.updateRequests[0].QuantizationConfig.GetTurboquant()
	require.NotNil(t, turbo)
	require.True(t, turbo.GetAlwaysRam())
	require.Equal(t, qdrant.TurboQuantBitSize_Bits2, turbo.GetBits())
}

func TestAlterCollectionRejectsInvalidTurboQuantization(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	_, err := exec.doAlterCollection(&ast.AlterCollectionStmt{
		Collection: "docs",
		Config: &ast.CollectionConfig{
			QuantizationUpdate: &ast.QuantizationUpdate{
				Config: &ast.QuantizationConfig{
					Type:      ast.QuantizationTypeTurbo,
					TurboBits: float64Ptr(3.0),
				},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported TURBO bit depth")
	require.Empty(t, client.updateRequests)
}

func TestAlterCollectionCanDisableQuantization(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	resp, err := exec.doAlterCollection(&ast.AlterCollectionStmt{
		Collection: "docs",
		Config: &ast.CollectionConfig{
			QuantizationUpdate: &ast.QuantizationUpdate{Disabled: true},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Len(t, client.updateRequests, 1)
	require.NotNil(t, client.updateRequests[0].QuantizationConfig.GetDisabled())
}

func TestDoCreateIndexSupportsKeywordOptions(t *testing.T) {
	client := newFakeQdrantClient()
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	resp, err := exec.doCreateIndex(&ast.CreateIndexStmt{
		Collection: "docs",
		Field:      "tenant_id",
		FieldType:  "keyword",
		Options: map[string]any{
			"is_tenant":   true,
			"on_disk":     true,
			"enable_hnsw": false,
		},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Len(t, client.fieldIndexRequests, 1)
	params := client.fieldIndexRequests[0].GetFieldIndexParams().GetKeywordIndexParams()
	require.NotNil(t, params)
	require.True(t, params.GetIsTenant())
	require.True(t, params.GetOnDisk())
	require.False(t, params.GetEnableHnsw())
}

func TestDoCreateIndexSupportsTextOptions(t *testing.T) {
	client := newFakeQdrantClient()
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	resp, err := exec.doCreateIndex(&ast.CreateIndexStmt{
		Collection: "docs",
		Field:      "title",
		FieldType:  "text",
		Options: map[string]any{
			"tokenizer":       "word",
			"min_token_len":   2,
			"max_token_len":   20,
			"lowercase":       true,
			"phrase_matching": true,
		},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Len(t, client.fieldIndexRequests, 1)
	params := client.fieldIndexRequests[0].GetFieldIndexParams().GetTextIndexParams()
	require.NotNil(t, params)
	require.Equal(t, qdrant.TokenizerType_Word, params.GetTokenizer())
	require.Equal(t, uint64(2), params.GetMinTokenLen())
	require.Equal(t, uint64(20), params.GetMaxTokenLen())
	require.True(t, params.GetLowercase())
	require.True(t, params.GetPhraseMatching())
}

func TestDoCreateIndexSupportsUUIDOptions(t *testing.T) {
	client := newFakeQdrantClient()
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	resp, err := exec.doCreateIndex(&ast.CreateIndexStmt{
		Collection: "docs",
		Field:      "doc_id",
		FieldType:  "uuid",
		Options: map[string]any{
			"on_disk": true,
		},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Len(t, client.fieldIndexRequests, 1)
	params := client.fieldIndexRequests[0].GetFieldIndexParams().GetUuidIndexParams()
	require.NotNil(t, params)
	require.True(t, params.GetOnDisk())
}

func TestDoCreateIndexRejectsUnknownOption(t *testing.T) {
	client := newFakeQdrantClient()
	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})

	_, err := exec.doCreateIndex(&ast.CreateIndexStmt{
		Collection: "docs",
		Field:      "tenant_id",
		FieldType:  "keyword",
		Options: map[string]any{
			"tokenizer": "word",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unknown CREATE INDEX option")
}

func TestBuildClientConfigNormalizesSchemeAndPort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantPort int
		wantTLS  bool
	}{
		{
			name:     "host only",
			input:    "qdrant.local",
			wantHost: "qdrant.local",
			wantPort: 6334,
		},
		{
			name:     "http with default rest port",
			input:    "http://localhost:6333",
			wantHost: "localhost",
			wantPort: 6334,
		},
		{
			name:     "https with trailing slash",
			input:    "https://example.com/",
			wantHost: "example.com",
			wantPort: 6334,
			wantTLS:  true,
		},
		{
			name:     "explicit grpc port",
			input:    "http://localhost:6334",
			wantHost: "localhost",
			wantPort: 6334,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := buildClientConfig(tt.input, "api-key", false, "")
			require.NoError(t, err)
			require.Equal(t, tt.wantHost, cfg.Host)
			require.Equal(t, tt.wantPort, cfg.Port)
			require.Equal(t, tt.wantTLS, cfg.UseTLS)
			require.Equal(t, "api-key", cfg.APIKey)
			require.True(t, cfg.SkipCompatibilityCheck)
		})
	}
}

func TestExecCommandWithoutConfigReturnsPrintedError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	stdout, stderr, err := captureCommandResult(t, func(out *output.Outputter) error {
		cmd := NewExecCmd(out)
		return cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
	})

	require.Empty(t, stdout)
	require.Error(t, err)
	require.True(t, ErrorPrinted(err))
	require.Equal(t, 1, ExitCode(err))
	require.Contains(t, stderr, "not connected. Run: qql-go connect --url <url>")
}

func TestDoctorCommandWithoutConfigReturnsPrintedError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	stdout, stderr, err := captureCommandResult(t, func(out *output.Outputter) error {
		cmd := NewDoctorCmd(out)
		return cmd.RunE(cmd, nil)
	})

	require.Empty(t, stdout)
	require.Error(t, err)
	require.True(t, ErrorPrinted(err))
	require.Equal(t, 1, ExitCode(err))
	require.Contains(t, stderr, "not connected. Run: qql-go connect --url <url>")
}

func TestREPLCommandWithoutConfigReturnsError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cmd := NewREPLCmd(output.NewOutputterWithWriters(&bytes.Buffer{}, &bytes.Buffer{}))
	err := cmd.RunE(cmd, nil)
	require.EqualError(t, err, "not connected. Run: qql-go connect --url <url>")
	require.False(t, ErrorPrinted(err))
}

func TestWaitForCollectionReadyMessages(t *testing.T) {
	err := waitForCollectionReady(context.Background(), "docs", 5*time.Millisecond, time.Millisecond, func(context.Context, string) (bool, bool, error) {
		return false, false, nil
	})
	require.EqualError(t, err, "collection 'docs' did not become visible within 5ms")

	err = waitForCollectionReady(context.Background(), "docs", 5*time.Millisecond, time.Millisecond, func(context.Context, string) (bool, bool, error) {
		return true, false, nil
	})
	require.EqualError(t, err, "collection 'docs' exists but is not ready yet after 5ms")
}

func TestWaitForCollectionReadyWrapsProbeErrors(t *testing.T) {
	err := waitForCollectionReady(context.Background(), "docs", 5*time.Millisecond, time.Millisecond, func(context.Context, string) (bool, bool, error) {
		return true, false, context.DeadlineExceeded
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "collection 'docs' did not become ready within 5ms")
}

func TestExecuteResultForShowCollectionsRequiresNoOutputParsing(t *testing.T) {
	result := &ExecResponse{
		OK:        true,
		Operation: "show_collections",
		Message:   "Found 2 collection(s): a, b",
		Data: map[string]any{
			"count":       2,
			"collections": []string{"a", "b"},
		},
	}

	encoded, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded ExecResponse
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.True(t, decoded.OK)
	require.Equal(t, "show_collections", decoded.Operation)
	require.Equal(t, "Found 2 collection(s): a, b", decoded.Message)
}

func TestBuildQuantizationConfigTurbo(t *testing.T) {
	cfg, err := buildQuantizationConfig(&ast.QuantizationConfig{
		Type:      ast.QuantizationTypeTurbo,
		TurboBits: float64Ptr(4.0),
		AlwaysRAM: true,
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)
	turbo := cfg.GetTurboquant()
	require.NotNil(t, turbo)
	require.True(t, turbo.GetAlwaysRam())
	require.Equal(t, qdrant.TurboQuantBitSize_Bits4, turbo.GetBits())
}

func TestBuildQuantizationConfigTurboDefaultBits(t *testing.T) {
	cfg, err := buildQuantizationConfig(&ast.QuantizationConfig{
		Type: ast.QuantizationTypeTurbo,
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)
	turbo := cfg.GetTurboquant()
	require.NotNil(t, turbo)
	require.Nil(t, turbo.Bits)
	require.False(t, turbo.GetAlwaysRam())
}

func TestBuildQuantizationConfigTurboBits1_5(t *testing.T) {
	cfg, err := buildQuantizationConfig(&ast.QuantizationConfig{
		Type:      ast.QuantizationTypeTurbo,
		TurboBits: float64Ptr(1.5),
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)
	turbo := cfg.GetTurboquant()
	require.NotNil(t, turbo)
	require.Equal(t, qdrant.TurboQuantBitSize_Bits1_5, turbo.GetBits())
}

func TestBuildQuantizationConfigTurboRejectsInvalidBits(t *testing.T) {
	cfg, err := buildQuantizationConfig(&ast.QuantizationConfig{
		Type:      ast.QuantizationTypeTurbo,
		TurboBits: float64Ptr(3.0),
	})
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "unsupported TURBO bit depth")
}

func TestShowCollectionDense(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Status:              qdrant.CollectionStatus_Green,
		SegmentsCount:       3,
		PointsCount:         qdrant.PtrOf(uint64(100)),
		IndexedVectorsCount: qdrant.PtrOf(uint64(100)),
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				ShardNumber:            1,
				ReplicationFactor:      qdrant.PtrOf(uint32(1)),
				WriteConsistencyFactor: qdrant.PtrOf(uint32(1)),
				VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
					Size:     384,
					Distance: qdrant.Distance_Cosine,
				}),
			},
			HnswConfig: &qdrant.HnswConfigDiff{
				M:           qdrant.PtrOf(uint64(16)),
				EfConstruct: qdrant.PtrOf(uint64(100)),
			},
		},
	}

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	resp, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "docs"})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Equal(t, "show_collection", resp.Operation)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "docs", data["name"])
	assert.Equal(t, "Green", data["status"])
	assert.Equal(t, uint64(100), data["points_count"])
	assert.Equal(t, uint64(100), data["indexed_vectors_count"])
	assert.Equal(t, "dense", data["topology"])
	assert.Nil(t, data["sparse_vectors"])
	assert.Nil(t, data["quantization"])
	assert.Nil(t, data["payload_schema"])

	vectors := data["vectors"].(map[string]map[string]any)
	assert.Equal(t, uint64(384), vectors[""]["size"])
	assert.Equal(t, "Cosine", vectors[""]["distance"])

	hnsw := data["hnsw_config"].(map[string]any)
	assert.Equal(t, uint64(16), hnsw["m"])
	assert.Equal(t, uint64(100), hnsw["ef_construct"])

	sharding := data["sharding"].(map[string]any)
	assert.Equal(t, uint32(1), sharding["shard_number"])
	assert.Equal(t, uint32(1), sharding["replication_factor"])
	assert.Equal(t, uint32(1), sharding["write_consistency_factor"])
}

func TestShowCollectionHybrid(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Status:              qdrant.CollectionStatus_Green,
		SegmentsCount:       2,
		PointsCount:         qdrant.PtrOf(uint64(50)),
		IndexedVectorsCount: qdrant.PtrOf(uint64(50)),
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				ShardNumber:            1,
				ReplicationFactor:      qdrant.PtrOf(uint32(1)),
				WriteConsistencyFactor: qdrant.PtrOf(uint32(1)),
				VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
					"dense": {Size: 768, Distance: qdrant.Distance_Cosine},
				}),
				SparseVectorsConfig: qdrant.NewSparseVectorsConfig(map[string]*qdrant.SparseVectorParams{
					"sparse": {Modifier: qdrant.Modifier_Idf.Enum()},
				}),
			},
			HnswConfig: &qdrant.HnswConfigDiff{
				M:           qdrant.PtrOf(uint64(16)),
				EfConstruct: qdrant.PtrOf(uint64(100)),
			},
		},
	}

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	resp, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "hybrid_docs"})
	require.NoError(t, err)
	require.True(t, resp.OK)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "hybrid", data["topology"])

	vectors := data["vectors"].(map[string]map[string]any)
	assert.Equal(t, uint64(768), vectors["dense"]["size"])

	sparseVectors := data["sparse_vectors"].(map[string]map[string]any)
	assert.Equal(t, "Idf", sparseVectors["sparse"]["modifier"])
}

func TestShowCollectionNamedDenseNotHybrid(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Status:              qdrant.CollectionStatus_Green,
		SegmentsCount:       1,
		PointsCount:         qdrant.PtrOf(uint64(10)),
		IndexedVectorsCount: qdrant.PtrOf(uint64(10)),
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				ShardNumber:            1,
				ReplicationFactor:      qdrant.PtrOf(uint32(1)),
				WriteConsistencyFactor: qdrant.PtrOf(uint32(1)),
				VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
					"dense": {Size: 384, Distance: qdrant.Distance_Cosine},
				}),
			},
			HnswConfig: &qdrant.HnswConfigDiff{
				M:           qdrant.PtrOf(uint64(16)),
				EfConstruct: qdrant.PtrOf(uint64(100)),
			},
		},
	}

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	resp, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "dense_only"})
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "dense", data["topology"])
	assert.Nil(t, data["sparse_vectors"])
}

func TestShowCollectionWithPayloadSchema(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Status:              qdrant.CollectionStatus_Green,
		SegmentsCount:       1,
		PointsCount:         qdrant.PtrOf(uint64(5)),
		IndexedVectorsCount: qdrant.PtrOf(uint64(5)),
		PayloadSchema: map[string]*qdrant.PayloadSchemaInfo{
			"category": {
				DataType: qdrant.PayloadSchemaType_Keyword,
				Params: qdrant.NewPayloadIndexParamsKeyword(&qdrant.KeywordIndexParams{
					IsTenant: qdrant.PtrOf(true),
					OnDisk:   qdrant.PtrOf(true),
				}),
			},
			"year": {DataType: qdrant.PayloadSchemaType_Integer},
		},
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				ShardNumber:            1,
				ReplicationFactor:      qdrant.PtrOf(uint32(1)),
				WriteConsistencyFactor: qdrant.PtrOf(uint32(1)),
				VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
					Size:     384,
					Distance: qdrant.Distance_Cosine,
				}),
			},
			HnswConfig: &qdrant.HnswConfigDiff{
				M:           qdrant.PtrOf(uint64(16)),
				EfConstruct: qdrant.PtrOf(uint64(100)),
			},
		},
	}

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	resp, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "indexed"})
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	payloadSchema := data["payload_schema"].(map[string]any)
	category := payloadSchema["category"].(map[string]any)
	assert.Equal(t, "keyword", category["type"])
	assert.Equal(t, map[string]any{"is_tenant": true, "on_disk": true}, category["params"])
	year := payloadSchema["year"].(map[string]any)
	assert.Equal(t, "integer", year["type"])
}

func TestShowCollectionHandlesMissingPayloadSchema(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Status:              qdrant.CollectionStatus_Green,
		SegmentsCount:       1,
		PointsCount:         qdrant.PtrOf(uint64(3)),
		IndexedVectorsCount: qdrant.PtrOf(uint64(3)),
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				ShardNumber:            1,
				ReplicationFactor:      qdrant.PtrOf(uint32(1)),
				WriteConsistencyFactor: qdrant.PtrOf(uint32(1)),
				VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
					Size:     384,
					Distance: qdrant.Distance_Cosine,
				}),
			},
			HnswConfig: &qdrant.HnswConfigDiff{
				M:           qdrant.PtrOf(uint64(16)),
				EfConstruct: qdrant.PtrOf(uint64(100)),
			},
		},
	}

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	resp, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "no_schema"})
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	assert.Nil(t, data["payload_schema"])
}

func TestShowCollectionNonexistentRaises(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = false

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	_, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "missing"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestShowCollectionWithQuantization(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Status:              qdrant.CollectionStatus_Green,
		SegmentsCount:       1,
		PointsCount:         qdrant.PtrOf(uint64(200)),
		IndexedVectorsCount: qdrant.PtrOf(uint64(200)),
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				ShardNumber:            1,
				ReplicationFactor:      qdrant.PtrOf(uint32(1)),
				WriteConsistencyFactor: qdrant.PtrOf(uint32(1)),
				VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
					Size:     384,
					Distance: qdrant.Distance_Cosine,
				}),
			},
			HnswConfig: &qdrant.HnswConfigDiff{
				M:           qdrant.PtrOf(uint64(16)),
				EfConstruct: qdrant.PtrOf(uint64(100)),
			},
			QuantizationConfig: &qdrant.QuantizationConfig{
				Quantization: &qdrant.QuantizationConfig_Scalar{
					Scalar: &qdrant.ScalarQuantization{},
				},
			},
		},
	}

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	resp, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "quantized"})
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "scalar", data["quantization"])
	require.Contains(t, resp.Message, "Quantization         : scalar")
}

func TestShowCollectionHnswExtraFields(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Status:              qdrant.CollectionStatus_Green,
		SegmentsCount:       1,
		PointsCount:         qdrant.PtrOf(uint64(30)),
		IndexedVectorsCount: qdrant.PtrOf(uint64(30)),
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				ShardNumber:            1,
				ReplicationFactor:      qdrant.PtrOf(uint32(2)),
				WriteConsistencyFactor: qdrant.PtrOf(uint32(2)),
				VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
					Size:     384,
					Distance: qdrant.Distance_Cosine,
				}),
			},
			HnswConfig: &qdrant.HnswConfigDiff{
				M:                  qdrant.PtrOf(uint64(32)),
				EfConstruct:        qdrant.PtrOf(uint64(200)),
				FullScanThreshold:  qdrant.PtrOf(uint64(10000)),
				MaxIndexingThreads: qdrant.PtrOf(uint64(8)),
				OnDisk:             qdrant.PtrOf(true),
				PayloadM:           qdrant.PtrOf(uint64(16)),
			},
		},
	}

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	resp, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "hnsw_conf"})
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	hnsw := data["hnsw_config"].(map[string]any)
	assert.Equal(t, uint64(32), hnsw["m"])
	assert.Equal(t, uint64(200), hnsw["ef_construct"])
	assert.Equal(t, uint64(10000), hnsw["full_scan_threshold"])
	assert.Equal(t, uint64(8), hnsw["max_indexing_threads"])
	assert.Equal(t, true, hnsw["on_disk"])
	assert.Equal(t, uint64(16), hnsw["payload_m"])
}

func TestShowCollectionWithTurboQuantization(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Status:              qdrant.CollectionStatus_Green,
		SegmentsCount:       1,
		PointsCount:         qdrant.PtrOf(uint64(25)),
		IndexedVectorsCount: qdrant.PtrOf(uint64(25)),
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				ShardNumber:            1,
				ReplicationFactor:      qdrant.PtrOf(uint32(1)),
				WriteConsistencyFactor: qdrant.PtrOf(uint32(1)),
				VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
					Size:     384,
					Distance: qdrant.Distance_Cosine,
				}),
			},
			QuantizationConfig: qdrant.NewQuantizationTurbo(&qdrant.TurboQuantization{}),
		},
	}

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	resp, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "turbo_quantized"})
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "turbo", data["quantization"])
	require.Contains(t, resp.Message, "Quantization         : turbo")
}

func TestShowCollectionWithoutHnswConfigDoesNotPanic(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Status:              qdrant.CollectionStatus_Green,
		SegmentsCount:       1,
		PointsCount:         qdrant.PtrOf(uint64(7)),
		IndexedVectorsCount: qdrant.PtrOf(uint64(7)),
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				ShardNumber:            1,
				ReplicationFactor:      qdrant.PtrOf(uint32(1)),
				WriteConsistencyFactor: qdrant.PtrOf(uint32(1)),
				VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
					Size:     384,
					Distance: qdrant.Distance_Cosine,
				}),
			},
		},
	}

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	resp, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "no_hnsw"})
	require.NoError(t, err)
	require.True(t, resp.OK)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Nil(t, data["hnsw_config"])
	require.NotContains(t, resp.Message, "HNSW M")
}

func TestShowCollectionWithoutVectorConfigReturnsError(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Status:        qdrant.CollectionStatus_Green,
		SegmentsCount: 1,
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{},
		},
	}

	exec := NewExecutor(client, &config.Config{InferenceMode: "cloud"})
	_, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "broken"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no vector configuration")
}

func TestExplainShowCollection(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{InferenceMode: "cloud"})
	plan, err := exec.Explain("SHOW COLLECTION docs")
	require.NoError(t, err)
	require.Contains(t, plan, "Statement: SHOW COLLECTION docs")
	require.Contains(t, plan, "Inspect collection diagnostics")
}
