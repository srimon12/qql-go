package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/srimon12/qql-go/internal/output"
	"github.com/srimon12/qql-go/internal/parser"
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

func TestCollectionVectorParamsOmitsColbertByDefault(t *testing.T) {
	vectors := collectionVectorParams(denseVectorSize, false)

	dense := vectors[denseVectorName]
	require.NotNil(t, dense)
	require.EqualValues(t, denseVectorSize, dense.GetSize())
	require.Equal(t, qdrant.Distance_Cosine, dense.GetDistance())
	require.NotContains(t, vectors, rerankVectorName)
}

func TestCollectionVectorParamsIncludesColbertWhenEnabled(t *testing.T) {
	vectors := collectionVectorParams(denseVectorSize, true)

	rerank := vectors[rerankVectorName]
	require.NotNil(t, rerank)
	require.EqualValues(t, rerankVectorSize, rerank.GetSize())
	require.Equal(t, qdrant.Distance_Cosine, rerank.GetDistance())
	require.NotNil(t, rerank.GetMultivectorConfig())
	require.Equal(t, qdrant.MultiVectorComparator_MaxSim, rerank.GetMultivectorConfig().GetComparator())
	require.NotNil(t, rerank.GetHnswConfig())
	require.Zero(t, rerank.GetHnswConfig().GetM())
}

func TestBuildInsertVectorsIncludesColbertOnlyWhenEnabled(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{})
	vectors, err := exec.buildInsertVectors(context.Background(), "hello world", "dense-model", "sparse-model", true, true, "test")
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

	withoutRerank, err := exec.buildInsertVectors(context.Background(), "hello world", "dense-model", "sparse-model", true, false, "test")
	require.NoError(t, err)
	require.NotContains(t, withoutRerank, rerankVectorName)
}

func TestBuildRerankSearchRequestTargetsColbertVector(t *testing.T) {
	prefetch := []*qdrant.PrefetchQuery{{}, {}}
	req := buildRerankSearchRequest("demo", "late interaction query", "answerai-colbert-test", 7, prefetch, nil)

	require.Equal(t, "demo", req.GetCollectionName())
	require.Equal(t, rerankVectorName, req.GetUsing())
	require.Equal(t, uint64(7), req.GetLimit())
	require.Len(t, req.GetPrefetch(), 2)
	require.NotNil(t, req.GetQuery().GetNearest())
	require.NotNil(t, req.GetQuery().GetNearest().GetDocument())
	require.Equal(t, "answerai-colbert-test", req.GetQuery().GetNearest().GetDocument().GetModel())
}

func TestEffectiveSearchLimit(t *testing.T) {
	tests := []struct {
		name   string
		limit  uint64
		rerank bool
		want   uint64
	}{
		{name: "plain search", limit: 12, want: 12},
		{name: "rerank small", limit: 10, rerank: true, want: 40},
		{name: "rerank capped", limit: 60, rerank: true, want: rerankPrefetchCap},
		{name: "rerank large keeps limit", limit: 500, rerank: true, want: 500},
		{name: "rerank overflow falls back to limit", limit: ^uint64(0), rerank: true, want: ^uint64(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, effectiveSearchLimit(tt.limit, tt.rerank))
		})
	}
}

func TestBuildSearchRequestAppliesWithClauseAndSparseOverride(t *testing.T) {
	sparseModel := "custom-sparse"
	exec := NewExecutor(nil, &config.Config{})
	req, err := exec.buildSearchRequest(context.Background(), &ast.SearchStmt{
		Collection:  "demo",
		QueryText:   "vector database",
		Limit:       5,
		Hybrid:      true,
		SparseModel: &sparseModel,
		WithClause: &ast.SearchWith{
			HnswEf:      128,
			Exact:       true,
			Acorn:       true,
			IndexedOnly: true,
			Quantization: &ast.QuantizationSearchWith{
				Ignore:       boolPtr(true),
				Rescore:      boolPtr(false),
				Oversampling: float64Ptr(2.5),
			},
		},
	}, "dense-model", sparseModel, false, 5)
	require.NoError(t, err)

	require.Equal(t, "demo", req.GetCollectionName())
	require.Equal(t, uint64(5), req.GetLimit())
	require.NotNil(t, req.GetParams())
	require.Equal(t, uint64(128), req.GetParams().GetHnswEf())
	require.True(t, req.GetParams().GetExact())
	require.NotNil(t, req.GetParams().GetAcorn())
	require.True(t, req.GetParams().GetAcorn().GetEnable())
	require.True(t, req.GetParams().GetIndexedOnly())
	require.NotNil(t, req.GetParams().GetQuantization())
	require.True(t, req.GetParams().GetQuantization().GetIgnore())
	require.False(t, req.GetParams().GetQuantization().GetRescore())
	require.InDelta(t, 2.5, req.GetParams().GetQuantization().GetOversampling(), 0.0001)
	require.NotNil(t, req.GetQuery().GetFusion())

	prefetch := req.GetPrefetch()
	require.Len(t, prefetch, 2)
	require.Equal(t, "custom-sparse", prefetch[0].GetQuery().GetNearest().GetDocument().GetModel())
	require.Equal(t, "dense-model", prefetch[1].GetQuery().GetNearest().GetDocument().GetModel())
	require.NotNil(t, prefetch[0].GetParams())
	require.Equal(t, uint64(128), prefetch[0].GetParams().GetHnswEf())
}

func TestBuildSearchRequestDenseMMR(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{})
	req, err := exec.buildSearchRequest(context.Background(), &ast.SearchStmt{
		Collection: "demo",
		QueryText:  "vector database",
		Limit:      5,
		WithClause: &ast.SearchWith{
			MmrDiversity:  float64Ptr(0.5),
			MmrCandidates: intPtr(50),
		},
	}, "dense-model", "custom-sparse", false, 5)
	require.NoError(t, err)
	require.NotNil(t, req.GetQuery().GetNearestWithMmr())
	require.InDelta(t, 0.5, req.GetQuery().GetNearestWithMmr().GetMmr().GetDiversity(), 0.0001)
	require.Equal(t, uint32(50), req.GetQuery().GetNearestWithMmr().GetMmr().GetCandidatesLimit())
	require.NotNil(t, req.GetQuery().GetNearestWithMmr().GetNearest().GetDocument())
	require.Equal(t, "dense-model", req.GetQuery().GetNearestWithMmr().GetNearest().GetDocument().GetModel())
}

func TestBuildGroupSearchRequestCarriesHybridPrefetchParamsAndCustomModels(t *testing.T) {
	sparseModel := "custom-sparse"
	exec := NewExecutor(nil, &config.Config{})
	req, err := exec.buildSearchRequest(context.Background(), &ast.SearchStmt{
		Collection:  "demo",
		QueryText:   "vector database",
		Limit:       5,
		Hybrid:      true,
		SparseModel: &sparseModel,
		WithClause: &ast.SearchWith{
			HnswEf: 128,
			Acorn:  true,
		},
		GroupBy:   "category",
		GroupSize: 2,
	}, "dense-model", sparseModel, false, 5)
	require.NoError(t, err)

	groupReq := buildGroupSearchRequest(&ast.SearchStmt{
		Collection:  "demo",
		QueryText:   "vector database",
		Limit:       5,
		Hybrid:      true,
		SparseModel: &sparseModel,
		GroupBy:     "category",
		GroupSize:   2,
	}, req, nil)

	require.Equal(t, "category", groupReq.GetGroupBy())
	require.Equal(t, uint64(2), groupReq.GetGroupSize())
	require.Len(t, groupReq.GetPrefetch(), 2)
	require.Equal(t, "custom-sparse", groupReq.GetPrefetch()[0].GetQuery().GetNearest().GetDocument().GetModel())
	require.Equal(t, "dense-model", groupReq.GetPrefetch()[1].GetQuery().GetNearest().GetDocument().GetModel())
	require.Equal(t, uint64(128), groupReq.GetPrefetch()[0].GetParams().GetHnswEf())
	require.NotNil(t, groupReq.GetPrefetch()[0].GetParams().GetAcorn())
	require.True(t, groupReq.GetPrefetch()[0].GetParams().GetAcorn().GetEnable())
}

func TestBuildSearchRequestRejectsRerankWithoutCollectionSupport(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{})
	_, err := exec.buildSearchRequest(context.Background(), &ast.SearchStmt{
		Collection: "demo",
		QueryText:  "vector database",
		Limit:      5,
		Rerank:     true,
	}, "dense-model", "custom-sparse", false, 5)
	require.Error(t, err)
}

func TestInsertPointIDAndPayloadHonorsExplicitID(t *testing.T) {
	pointID, payload, err := insertPointIDAndPayload(42, map[string]interface{}{"text": "hello"})
	require.NoError(t, err)
	require.Equal(t, 42, pointID)
	require.Equal(t, map[string]interface{}{"text": "hello"}, payload)
}

func TestInsertPointIDAndPayloadExtractsIDFromValues(t *testing.T) {
	id := "123e4567-e89b-12d3-a456-426614174000"
	pointID, payload, err := insertPointIDAndPayload(nil, map[string]interface{}{"id": id, "text": "hello"})
	require.NoError(t, err)
	require.Equal(t, id, pointID)
	require.Equal(t, map[string]interface{}{"text": "hello"}, payload)
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
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doCreateCollection(&ast.CreateCollectionStmt{
		Collection: "docs",
		Quantization: &ast.QuantizationConfig{
			Type:      ast.QuantizationTypeScalar,
			Quantile:  float64Ptr(0.99),
			AlwaysRAM: true,
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
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doCreateCollection(&ast.CreateCollectionStmt{
		Collection: "docs",
		Hybrid:     true,
		Rerank:     true,
		Quantization: &ast.QuantizationConfig{
			Type:      ast.QuantizationTypeBinary,
			AlwaysRAM: true,
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
	exec := NewExecutor(client, &config.Config{})

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
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doAlterCollection(&ast.AlterCollectionStmt{
		Collection: "docs",
		Config: &ast.CollectionConfig{
			Vectors:    &ast.VectorsConfig{OnDisk: boolPtr(true)},
			Hnsw:       &ast.HnswRuntimeConfig{FullScanThreshold: uint64Ptr(5000)},
			Optimizers: &ast.OptimizersRuntimeConfig{IndexingThreshold: uint64Ptr(10000)},
			Params:     &ast.CollectionParamsConfig{OnDiskPayload: boolPtr(false), ReadFanOutFactor: uint64Ptr(4)},
		},
		Quantization: &ast.QuantizationUpdate{
			Config: &ast.QuantizationConfig{Type: ast.QuantizationTypeBinary},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Len(t, client.updateRequests, 1)
	req := client.updateRequests[0]

	vsMap := req.VectorsConfig.GetParamsMap().GetMap()
	require.NotNil(t, vsMap)
	require.True(t, vsMap[""].GetOnDisk())

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

func TestAlterCollectionCanDisableQuantization(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doAlterCollection(&ast.AlterCollectionStmt{
		Collection:   "docs",
		Quantization: &ast.QuantizationUpdate{Disabled: true},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Len(t, client.updateRequests, 1)
	require.NotNil(t, client.updateRequests[0].QuantizationConfig.GetDisabled())
}

func TestDoCreateIndexSupportsKeywordOptions(t *testing.T) {
	client := newFakeQdrantClient()
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doCreateIndex(&ast.CreateIndexStmt{
		Collection: "docs",
		Field:      "tenant_id",
		FieldType:  "keyword",
		Options: map[string]interface{}{
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
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doCreateIndex(&ast.CreateIndexStmt{
		Collection: "docs",
		Field:      "title",
		FieldType:  "text",
		Options: map[string]interface{}{
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
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doCreateIndex(&ast.CreateIndexStmt{
		Collection: "docs",
		Field:      "doc_id",
		FieldType:  "uuid",
		Options: map[string]interface{}{
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
	exec := NewExecutor(client, &config.Config{})

	_, err := exec.doCreateIndex(&ast.CreateIndexStmt{
		Collection: "docs",
		Field:      "tenant_id",
		FieldType:  "keyword",
		Options: map[string]interface{}{
			"tokenizer": "word",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unknown CREATE INDEX option")
}

func TestInsertAutoCreatesMissingCollection(t *testing.T) {
	client := newFakeQdrantClient()
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doInsert(&ast.InsertStmt{
		Collection: "docs",
		Values:     map[string]interface{}{"text": "hello"},
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

	exec := NewExecutor(client, &config.Config{})
	resp, err := exec.doInsert(&ast.InsertStmt{
		Collection: "docs",
		Values:     map[string]interface{}{"text": "hello"},
	})
	require.NoError(t, err)
	require.True(t, resp.OK)
	data := resp.Data.(map[string]any)
	require.True(t, data["hybrid"].(bool))
	require.Len(t, client.createRequests, 0)
	require.Len(t, client.upserts, 1)
}

func TestBuildDeleteRequestByFieldUsesFilterSelector(t *testing.T) {
	req, err := buildDeleteRequest(&ast.DeleteStmt{
		Collection: "demo",
		Field:      "status",
		Value:      "archived",
	})
	require.NoError(t, err)

	require.Equal(t, "demo", req.GetCollectionName())
	filter := req.GetPoints().GetFilter()
	require.NotNil(t, filter)
	require.Len(t, filter.GetMust(), 1)
	match := filter.GetMust()[0].GetField().GetMatch()
	require.NotNil(t, match)
	require.Equal(t, "archived", match.GetKeyword())
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

func TestBuildDeleteRequestByIDUsesPointSelector(t *testing.T) {
	req, err := buildDeleteRequest(&ast.DeleteStmt{
		Collection: "demo",
		PointID:    "point-123",
	})
	require.NoError(t, err)

	require.Equal(t, "demo", req.GetCollectionName())
	points, ok := req.GetPoints().GetPointsSelectorOneOf().(*qdrant.PointsSelector_Points)
	require.True(t, ok)
	require.NotNil(t, points.Points)
	require.Len(t, points.Points.GetIds(), 1)
	require.Equal(t, "point-123", points.Points.GetIds()[0].GetUuid())
}

func TestPointIDString(t *testing.T) {
	require.Equal(t, "abc", pointIDString(qdrant.NewIDUUID("abc")))
	require.Equal(t, "42", pointIDString(qdrant.NewIDNum(42)))
	require.Equal(t, "", pointIDString(nil))
}

func strPtr(s string) *string {
	return &s
}

func TestExecutorExplainDocumentedQueries(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{})

	tests := []struct {
		name  string
		query string
		wants []string
	}{
		{
			name:  "create hybrid rerank",
			query: "CREATE COLLECTION docs HYBRID RERANK",
			wants: []string{
				"Statement: CREATE COLLECTION docs",
				"Type: HYBRID + RERANK (dense + sparse + ColBERT multivector)",
			},
		},
		{
			name:  "create with quantization",
			query: "CREATE COLLECTION docs QUANTIZE SCALAR QUANTILE 0.95 ALWAYS RAM",
			wants: []string{
				"Statement: CREATE COLLECTION docs",
				"Quantization: scalar",
				"Quantile: 0.9500",
				"Quantization storage: ALWAYS RAM",
			},
		},
		{
			name:  "hybrid search rerank",
			query: "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 USING HYBRID RERANK",
			wants: []string{
				"Statement: SEARCH docs SIMILAR TO 'vector database' LIMIT 5",
				"Search: HYBRID (dense + sparse)",
				"Rerank: enabled",
				"Rerank vector: colbert",
			},
		},
		{
			name:  "search with with clause",
			query: "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 WITH { hnsw_ef: 128, exact: true }",
			wants: []string{
				"Search params: EXACT (bypass HNSW)",
				"Search params: hnsw_ef=128",
			},
		},
		{
			name:  "search with filter",
			query: "SEARCH notes SIMILAR TO 'vector search' LIMIT 5 USING HYBRID WHERE topic = 'search'",
			wants: []string{
				"Search: HYBRID (dense + sparse)",
				"Filter:",
				"topic = search",
			},
		},
		{
			name:  "delete by id",
			query: "DELETE FROM notes WHERE id = 'uuid'",
			wants: []string{
				"Statement: DELETE FROM notes WHERE id = 'uuid'",
				"Action: Delete point by ID",
			},
		},
		{
			name:  "delete by field",
			query: "DELETE FROM notes WHERE status = 'archived'",
			wants: []string{
				"Statement: DELETE FROM notes WHERE status = 'archived'",
				"Action: Delete points by filter",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := exec.Explain(tt.query)
			require.NoError(t, err)
			for _, want := range tt.wants {
				require.Contains(t, got, want)
			}
		})
	}
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
			cfg, err := buildClientConfig(tt.input, "api-key")
			require.NoError(t, err)
			require.Equal(t, tt.wantHost, cfg.Host)
			require.Equal(t, tt.wantPort, cfg.Port)
			require.Equal(t, tt.wantTLS, cfg.UseTLS)
			require.Equal(t, "api-key", cfg.APIKey)
			require.True(t, cfg.SkipCompatibilityCheck)
		})
	}
}

func TestExplainCommandDoesNotNeedSavedConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewExplainCmd(out)
		err := cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
		require.NoError(t, err)
	})

	require.Contains(t, stdout, "Query Plan")
	require.Contains(t, stdout, "Statement: SHOW COLLECTIONS")
	require.Empty(t, stderr)
}

func TestExplainCommandJSON(t *testing.T) {
	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewExplainCmd(out)
		require.NoError(t, cmd.Flags().Set("json", "true"))
		err := cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	var payload ExplainResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.True(t, payload.OK)
	require.Equal(t, "SHOW COLLECTIONS", payload.Query)
	require.Contains(t, payload.Plan, "Statement: SHOW COLLECTIONS")
}

func TestExplainCommandQuietJSONIsCompact(t *testing.T) {
	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewExplainCmd(out)
		require.NoError(t, cmd.Flags().Set("json", "true"))
		require.NoError(t, cmd.Flags().Set("quiet", "true"))
		err := cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.NotContains(t, stdout, "\n  ")
	var payload ExplainResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.True(t, payload.OK)
}

func TestExplainCommandQuietTextOmitsSectionHeader(t *testing.T) {
	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewExplainCmd(out)
		require.NoError(t, cmd.Flags().Set("quiet", "true"))
		err := cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.Contains(t, stdout, "Statement: SHOW COLLECTIONS")
	require.NotContains(t, stdout, "\033[1mQuery Plan\033[0m")
}

func TestDisconnectCommandJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewDisconnectCmd(out)
		require.NoError(t, cmd.Flags().Set("json", "true"))
		err := cmd.RunE(cmd, nil)
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	var payload CommandResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.True(t, payload.OK)
	require.Equal(t, "disconnect", payload.Command)
	require.Equal(t, "Disconnected. Config removed.", payload.Message)
}

func TestVersionCommandSupportsQuietAndJSON(t *testing.T) {
	t.Run("quiet text", func(t *testing.T) {
		stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
			cmd := NewVersionCmd(out)
			require.NoError(t, cmd.Flags().Set("quiet", "true"))
			err := cmd.RunE(cmd, nil)
			require.NoError(t, err)
		})

		require.Empty(t, stderr)
		require.Equal(t, displayVersion()+"\n", stdout)
	})

	t.Run("json", func(t *testing.T) {
		stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
			cmd := NewVersionCmd(out)
			require.NoError(t, cmd.Flags().Set("json", "true"))
			require.NoError(t, cmd.Flags().Set("quiet", "true"))
			err := cmd.RunE(cmd, nil)
			require.NoError(t, err)
		})

		require.Empty(t, stderr)
		var payload VersionResponse
		require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
		require.True(t, payload.OK)
		require.Equal(t, "version", payload.Command)
		require.Equal(t, displayVersion(), payload.Version)
	})
}

func TestVersionCommandDefaultsToDevWhenVersionBlank(t *testing.T) {
	original := Version
	Version = "   "
	t.Cleanup(func() {
		Version = original
	})

	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewVersionCmd(out)
		require.NoError(t, cmd.Flags().Set("quiet", "true"))
		require.NoError(t, cmd.RunE(cmd, nil))
	})

	require.Empty(t, stderr)
	require.Equal(t, "dev\n", stdout)
}

func TestDumpCommandRejectsInvalidBatchSize(t *testing.T) {
	stdout, stderr, err := captureCommandResult(t, func(out *output.Outputter) error {
		cmd := NewDumpCmd(out)
		require.NoError(t, cmd.Flags().Set("batch-size", "0"))
		return cmd.RunE(cmd, []string{"docs", "backup.qql"})
	})

	require.Error(t, err)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "--batch-size must be greater than 0")
}

func TestConnectCommandMissingURLReturnsPrintedError(t *testing.T) {
	stdout, stderr, err := captureCommandResult(t, func(out *output.Outputter) error {
		cmd := NewConnectCmd(out)
		return cmd.RunE(cmd, nil)
	})

	require.Empty(t, stdout)
	require.Error(t, err)
	require.True(t, ErrorPrinted(err))
	require.Equal(t, 1, ExitCode(err))
	require.Contains(t, stderr, "--url is required")
}

func TestExplainCommandInvalidJSONReturnsPrintedExitError(t *testing.T) {
	stdout, stderr, err := captureCommandResult(t, func(out *output.Outputter) error {
		cmd := NewExplainCmd(out)
		require.NoError(t, cmd.Flags().Set("json", "true"))
		return cmd.RunE(cmd, []string{"EXPLAIN SHOW COLLECTIONS"})
	})

	require.Empty(t, stderr)
	require.Error(t, err)
	require.True(t, ErrorPrinted(err))
	require.Equal(t, 1, ExitCode(err))

	var payload ErrorResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.False(t, payload.OK)
	require.Equal(t, "explain", payload.Command)
	require.Contains(t, payload.Error, "parse error")
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

func TestLoadSavedConfigAndClientWrapsInvalidURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	require.NoError(t, config.SaveConfig(&config.Config{URL: "http://localhost:bad-port"}))

	loaded, client, err := loadSavedConfigAndClient()
	require.Nil(t, loaded)
	require.Nil(t, client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "connection failed")
}

func TestSavedConfigMessageUsesResolvedPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	path, err := config.ConfigPath()
	require.NoError(t, err)
	require.Equal(t, "Connected. Config saved to "+path, savedConfigMessage())
}

func TestWriteJSONWrapsEncoderFailures(t *testing.T) {
	err := writeJSON(output.NewOutputterWithWriters(failingWriter{}, &bytes.Buffer{}), map[string]any{"ok": true}, false)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to write JSON")
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

func TestExplainResultReturnsErrorForInvalidQuery(t *testing.T) {
	_, err := NewExecutor(nil, nil).ExplainResult("EXPLAIN SEARCH docs SIMILAR TO 'x' LIMIT 1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse error")
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

func captureCommandStreams(t *testing.T, fn func(*output.Outputter)) (string, string) {
	t.Helper()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	fn(output.NewOutputterWithWriters(stdout, stderr))
	return stdout.String(), stderr.String()
}

func captureCommandResult(t *testing.T, fn func(*output.Outputter) error) (string, string, error) {
	t.Helper()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err := fn(output.NewOutputterWithWriters(stdout, stderr))
	return stdout.String(), stderr.String(), err
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, context.DeadlineExceeded
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

	vectors, err := exec.buildInsertVectors(context.Background(), "hello world", "dense-model", "sparse-model", true, false, "test_local")
	require.NoError(t, err)

	dense := vectors[denseVectorName]
	require.NotNil(t, dense)

	sparseVec := vectors[sparseVectorName]
	require.NotNil(t, sparseVec)

	require.NotContains(t, vectors, rerankVectorName)
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

	_, err := exec.buildInsertVectors(context.Background(), "hello", "dense-model", "sparse-model", true, true, "test_local")
	require.Error(t, err)
	require.Contains(t, err.Error(), "rerank vectors are not implemented yet")
}

type fakeQdrantClient struct {
	mu                  sync.Mutex
	exists              bool
	info                *qdrant.CollectionInfo
	createRequests      []*qdrant.CreateCollection
	updateRequests      []*qdrant.UpdateCollection
	fieldIndexRequests  []*qdrant.CreateFieldIndexCollection
	upserts             []*qdrant.UpsertPoints
	queryRequests       []*qdrant.QueryPoints
	groupRequests       []*qdrant.QueryPointGroups
	groupResults        []*qdrant.PointGroup
	updateVectors       []*qdrant.UpdatePointVectors
	setPayloads         []*qdrant.SetPayloadPoints
	scrollRecords       []*qdrant.RetrievedPoint
	scrollOffset        *qdrant.PointId
	getRecords          []*qdrant.RetrievedPoint
}

func newFakeQdrantClient() *fakeQdrantClient { return &fakeQdrantClient{} }

func (f *fakeQdrantClient) ListCollections(context.Context) ([]string, error) { return nil, nil }
func (f *fakeQdrantClient) CollectionExists(context.Context, string) (bool, error) {
	return f.exists, nil
}
func (f *fakeQdrantClient) GetCollectionInfo(context.Context, string) (*qdrant.CollectionInfo, error) {
	if f.info == nil {
		return nil, errors.New("missing collection")
	}
	return f.info, nil
}
func (f *fakeQdrantClient) CreateCollection(_ context.Context, req *qdrant.CreateCollection) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createRequests = append(f.createRequests, req)
	f.exists = true
	f.info = &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			QuantizationConfig: req.QuantizationConfig,
			Params: &qdrant.CollectionParams{
				VectorsConfig:       req.VectorsConfig,
				SparseVectorsConfig: req.SparseVectorsConfig,
			},
		},
	}
	return nil
}
func (f *fakeQdrantClient) UpdateCollection(_ context.Context, req *qdrant.UpdateCollection) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateRequests = append(f.updateRequests, req)
	return nil
}
func (f *fakeQdrantClient) DeleteCollection(context.Context, string) error { return nil }
func (f *fakeQdrantClient) Upsert(_ context.Context, req *qdrant.UpsertPoints) (*qdrant.UpdateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upserts = append(f.upserts, req)
	return &qdrant.UpdateResult{}, nil
}
func (f *fakeQdrantClient) Query(_ context.Context, req *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryRequests = append(f.queryRequests, req)
	return nil, nil
}
func (f *fakeQdrantClient) QueryGroups(_ context.Context, req *qdrant.QueryPointGroups) ([]*qdrant.PointGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.groupRequests = append(f.groupRequests, req)
	return f.groupResults, nil
}
func (f *fakeQdrantClient) Delete(context.Context, *qdrant.DeletePoints) (*qdrant.UpdateResult, error) {
	return &qdrant.UpdateResult{}, nil
}
func (f *fakeQdrantClient) UpdateVectors(_ context.Context, req *qdrant.UpdatePointVectors) (*qdrant.UpdateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateVectors = append(f.updateVectors, req)
	return &qdrant.UpdateResult{}, nil
}
func (f *fakeQdrantClient) SetPayload(_ context.Context, req *qdrant.SetPayloadPoints) (*qdrant.UpdateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setPayloads = append(f.setPayloads, req)
	return &qdrant.UpdateResult{}, nil
}
func (f *fakeQdrantClient) CreateFieldIndex(_ context.Context, req *qdrant.CreateFieldIndexCollection) (*qdrant.UpdateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fieldIndexRequests = append(f.fieldIndexRequests, req)
	return &qdrant.UpdateResult{}, nil
}
func (f *fakeQdrantClient) Count(context.Context, *qdrant.CountPoints) (uint64, error) { return 0, nil }
func (f *fakeQdrantClient) ScrollAndOffset(context.Context, *qdrant.ScrollPoints) ([]*qdrant.RetrievedPoint, *qdrant.PointId, error) {
	return f.scrollRecords, f.scrollOffset, nil
}
func (f *fakeQdrantClient) Get(context.Context, *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error) {
	return f.getRecords, nil
}

func TestDoSearchUsesQueryGroupsForGroupedSearch(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
					denseVectorName:  {Size: uint64(denseVectorSize), Distance: qdrant.Distance_Cosine},
					sparseVectorName: {Size: uint64(denseVectorSize), Distance: qdrant.Distance_Cosine},
				}),
			},
		},
	}
	client.groupResults = []*qdrant.PointGroup{
		{
			Id: qdrant.NewGroupIDString("tech"),
			Hits: []*qdrant.ScoredPoint{
				{
					Id:      qdrant.NewIDNum(1),
					Score:   0.95,
					Payload: qdrant.NewValueMap(map[string]any{"text": "hello"}),
				},
			},
		},
	}
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doSearch(&ast.SearchStmt{
		Collection: "docs",
		QueryText:  "vector database",
		Limit:      5,
		Hybrid:     true,
		GroupBy:    "category",
		GroupSize:  2,
		WithClause: &ast.SearchWith{HnswEf: 128, Acorn: true},
	})
	require.NoError(t, err)
	require.Len(t, client.groupRequests, 1)
	require.Empty(t, client.queryRequests)
	require.Equal(t, "category", client.groupRequests[0].GetGroupBy())
	require.Equal(t, uint64(2), client.groupRequests[0].GetGroupSize())
	require.Equal(t, uint64(128), client.groupRequests[0].GetPrefetch()[0].GetParams().GetHnswEf())
	require.Contains(t, resp.Message, "Found 1 group(s)")
}

func TestDoSearchRejectsMMRWithHybrid(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				VectorsConfig: qdrant.NewVectorsConfigMap(collectionVectorParams(denseVectorSize, false)),
			},
		},
	}
	exec := NewExecutor(client, &config.Config{})

	_, err := exec.doSearch(&ast.SearchStmt{
		Collection: "docs",
		QueryText:  "vector database",
		Limit:      5,
		Hybrid:     true,
		WithClause: &ast.SearchWith{MmrDiversity: float64Ptr(0.5)},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "MMR is not supported with USING HYBRID yet")
}

func TestDoSearchRejectsMMRWithSparse(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				VectorsConfig: qdrant.NewVectorsConfigMap(collectionVectorParams(denseVectorSize, false)),
			},
		},
	}
	exec := NewExecutor(client, &config.Config{})

	_, err := exec.doSearch(&ast.SearchStmt{
		Collection: "docs",
		QueryText:  "vector database",
		Limit:      5,
		SparseOnly: true,
		WithClause: &ast.SearchWith{MmrDiversity: float64Ptr(0.5)},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "MMR is not supported with USING SPARSE yet")
}

func TestBuildRecommendRequestRejectsMMR(t *testing.T) {
	_, err := buildRecommendRequest(&ast.RecommendStmt{
		Collection:  "docs",
		PositiveIDs: []interface{}{"a"},
		Limit:       5,
		WithClause:  &ast.SearchWith{MmrDiversity: float64Ptr(0.5)},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "MMR is supported only for SEARCH statements")
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
	})
	require.NoError(t, err)
	require.Len(t, req.GetPoints(), 1)
	require.NotNil(t, req.GetPoints()[0].GetVectors().GetVectors().GetVectors()[denseVectorName])
}

func TestBuildUpdatePayloadRequestSupportsFilterSelector(t *testing.T) {
	req, err := buildUpdatePayloadRequest(&ast.UpdatePayloadStmt{
		Collection:  "docs",
		QueryFilter: &ast.CompareExpr{Field: "status", Op: "=", Value: "draft"},
		Payload:     map[string]interface{}{"status": "published"},
	})
	require.NoError(t, err)
	require.NotNil(t, req.GetPointsSelector().GetFilter())
	require.Equal(t, "published", req.GetPayload()["status"].GetStringValue())
}

func TestBuildSearchPrefetchesLocalModeReturnsExplicitQueries(t *testing.T) {
	server := newEmbeddingServer(t, []float32{1, 2, 3})
	defer server.Close()

	exec := NewExecutor(nil, &config.Config{
		InferenceMode:      "local",
		EmbeddingEndpoint:  server.URL + "/v1/embeddings",
		EmbeddingModel:     "test-model",
		EmbeddingDimension: 3,
	})

	prefetch, err := exec.buildSearchPrefetches(context.Background(), "hello world", "dense-model", "sparse-model", 5, nil)
	require.NoError(t, err)
	require.Len(t, prefetch, 2)

	// Sparse prefetch
	sparse := prefetch[0]
	require.Equal(t, sparseVectorName, sparse.GetUsing())
	require.NotNil(t, sparse.GetQuery().GetNearest().GetSparse())

	// Dense prefetch
	dense := prefetch[1]
	require.Equal(t, denseVectorName, dense.GetUsing())
	require.NotNil(t, dense.GetQuery().GetNearest().GetDense())
}

func TestBuildSearchPrefetchesLocalModePropagatesEmbeddingError(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{
		InferenceMode:      "local",
		EmbeddingEndpoint:  "http://localhost:1", // will fail
		EmbeddingModel:     "test-model",
		EmbeddingDimension: 3,
	})

	_, err := exec.buildSearchPrefetches(context.Background(), "hello world", "dense-model", "sparse-model", 5, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to embed search query")
}

func TestBuildSearchRequestSparseOnlyLocalMode(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{
		InferenceMode:      "local",
		EmbeddingEndpoint:  "http://localhost:1",
		EmbeddingModel:     "test-model",
		EmbeddingDimension: 3,
	})

	req, err := exec.buildSearchRequest(context.Background(), &ast.SearchStmt{
		Collection: "demo",
		QueryText:  "hello world",
		Limit:      5,
		SparseOnly: true,
	}, "dense-model", "sparse-model", false, 5)
	require.NoError(t, err)

	require.Equal(t, "demo", req.GetCollectionName())
	require.Equal(t, sparseVectorName, req.GetUsing())
	require.NotNil(t, req.GetQuery().GetNearest().GetSparse())
}

func TestBuildSearchRequestSparseOnlyRerankUsesSparsePrefetch(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{})
	req, err := exec.buildSearchRequest(context.Background(), &ast.SearchStmt{
		Collection: "demo",
		QueryText:  "hello world",
		Limit:      5,
		SparseOnly: true,
		Rerank:     true,
	}, "dense-model", "sparse-model", true, 5)
	require.NoError(t, err)

	require.Equal(t, "demo", req.GetCollectionName())
	require.Equal(t, rerankVectorName, req.GetUsing())
	require.Len(t, req.GetPrefetch(), 1)
	require.Equal(t, sparseVectorName, req.GetPrefetch()[0].GetUsing())
	require.NotNil(t, req.GetPrefetch()[0].GetQuery().GetNearest().GetDocument())
	require.Equal(t, "sparse-model", req.GetPrefetch()[0].GetQuery().GetNearest().GetDocument().GetModel())
}

func TestBuildSearchRequestHybridLocalMode(t *testing.T) {
	server := newEmbeddingServer(t, []float32{1, 2, 3})
	defer server.Close()

	exec := NewExecutor(nil, &config.Config{
		InferenceMode:      "local",
		EmbeddingEndpoint:  server.URL + "/v1/embeddings",
		EmbeddingModel:     "test-model",
		EmbeddingDimension: 3,
	})

	req, err := exec.buildSearchRequest(context.Background(), &ast.SearchStmt{
		Collection: "demo",
		QueryText:  "hello world",
		Limit:      5,
		Hybrid:     true,
	}, "dense-model", "sparse-model", false, 5)
	require.NoError(t, err)

	require.Equal(t, "demo", req.GetCollectionName())
	require.Len(t, req.GetPrefetch(), 2)
	require.NotNil(t, req.GetQuery().GetFusion())
}

func TestBuildSearchRequestHybridLocalModePropagatesError(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{
		InferenceMode:      "local",
		EmbeddingEndpoint:  "http://localhost:1",
		EmbeddingModel:     "test-model",
		EmbeddingDimension: 3,
	})

	_, err := exec.buildSearchRequest(context.Background(), &ast.SearchStmt{
		Collection: "demo",
		QueryText:  "hello world",
		Limit:      5,
		Hybrid:     true,
	}, "dense-model", "sparse-model", false, 5)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to embed search query")
}

func TestBuildRecommendRequestDefaults(t *testing.T) {
	req, err := buildRecommendRequest(&ast.RecommendStmt{
		Collection:  "docs",
		PositiveIDs: []interface{}{"a"},
		Limit:       5,
	})
	require.NoError(t, err)
	require.Equal(t, "docs", req.GetCollectionName())
	require.Equal(t, uint64(5), req.GetLimit())
	require.Equal(t, denseVectorName, req.GetUsing())
	require.Zero(t, req.GetOffset())
	require.Zero(t, req.GetScoreThreshold())
	require.Nil(t, req.GetLookupFrom())
	require.Nil(t, req.GetParams())
	require.NotNil(t, req.GetQuery().GetRecommend())
}

func TestBuildRecommendRequestWithAllNewFields(t *testing.T) {
	req, err := buildRecommendRequest(&ast.RecommendStmt{
		Collection:     "docs",
		PositiveIDs:    []interface{}{"a", "b"},
		NegativeIDs:    []interface{}{"c"},
		Limit:          5,
		Strategy:       strPtr("best_score"),
		Offset:         2,
		ScoreThreshold: float64Ptr(0.25),
		WithClause: &ast.SearchWith{
			Exact:  true,
			HnswEf: 128,
		},
		LookupFrom:   "src",
		LookupVector: strPtr("dense"),
		Using:        strPtr("sparse"),
	})
	require.NoError(t, err)
	require.Equal(t, "docs", req.GetCollectionName())
	require.Equal(t, uint64(5), req.GetLimit())
	require.Equal(t, uint64(2), req.GetOffset())
	require.Equal(t, "sparse", req.GetUsing())
	require.InDelta(t, float32(0.25), req.GetScoreThreshold(), 0.0001)
	require.NotNil(t, req.GetParams())
	require.True(t, req.GetParams().GetExact())
	require.Equal(t, uint64(128), req.GetParams().GetHnswEf())
	require.NotNil(t, req.GetLookupFrom())
	require.Equal(t, "src", req.GetLookupFrom().GetCollectionName())
	require.Equal(t, "dense", req.GetLookupFrom().GetVectorName())
	require.NotNil(t, req.GetQuery().GetRecommend())
	require.Equal(t, qdrant.RecommendStrategy_BestScore, req.GetQuery().GetRecommend().GetStrategy())
}

func TestBuildRecommendRequestWithLookupFromNoVector(t *testing.T) {
	req, err := buildRecommendRequest(&ast.RecommendStmt{
		Collection:  "docs",
		PositiveIDs: []interface{}{"a"},
		Limit:       5,
		LookupFrom:  "src",
	})
	require.NoError(t, err)
	require.NotNil(t, req.GetLookupFrom())
	require.Equal(t, "src", req.GetLookupFrom().GetCollectionName())
	require.Empty(t, req.GetLookupFrom().GetVectorName())
}

func TestBuildRecommendRequestUnknownStrategy(t *testing.T) {
	_, err := buildRecommendRequest(&ast.RecommendStmt{
		Collection:  "docs",
		PositiveIDs: []interface{}{"a"},
		Limit:       5,
		Strategy:    strPtr("bad_strategy"),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown recommend strategy")
}

func TestBuildRecommendRequestFilterExcludesIDs(t *testing.T) {
	req, err := buildRecommendRequest(&ast.RecommendStmt{
		Collection:  "docs",
		PositiveIDs: []interface{}{"a"},
		NegativeIDs: []interface{}{"b"},
		Limit:       5,
		QueryFilter: &ast.CompareExpr{Field: "status", Op: "=", Value: "active"},
	})
	require.NoError(t, err)
	filter := req.GetFilter()
	require.NotNil(t, filter)
	require.Len(t, filter.GetMust(), 1)
	require.Len(t, filter.GetMustNot(), 1)
	require.Equal(t, "active", filter.GetMust()[0].GetField().GetMatch().GetKeyword())
	require.Len(t, filter.GetMustNot()[0].GetHasId().GetHasId(), 2)
}

func TestDoSelectReturnsRecordOrNil(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		client := newFakeQdrantClient()
		client.exists = true
		client.getRecords = []*qdrant.RetrievedPoint{
			{
				Id: qdrant.NewIDUUID("pt-1"),
				Payload: qdrant.NewValueMap(map[string]any{
					"text":  "hello",
					"topic": "search",
				}),
			},
		}
		exec := NewExecutor(client, &config.Config{})

		resp, err := exec.doSelect(&ast.SelectStmt{Collection: "docs", PointID: "pt-1"})
		require.NoError(t, err)
		require.True(t, resp.OK)
		require.Equal(t, map[string]any{
			"id": "pt-1",
			"payload": map[string]any{
				"text":  "hello",
				"topic": "search",
			},
		}, resp.Data)
	})

	t.Run("missing", func(t *testing.T) {
		client := newFakeQdrantClient()
		client.exists = true
		exec := NewExecutor(client, &config.Config{})

		resp, err := exec.doSelect(&ast.SelectStmt{Collection: "docs", PointID: "pt-404"})
		require.NoError(t, err)
		require.True(t, resp.OK)
		require.Nil(t, resp.Data)
	})
}

func TestDoScrollReturnsUpstreamStylePayload(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.scrollRecords = []*qdrant.RetrievedPoint{
		{
			Id: qdrant.NewIDNum(7),
			Payload: qdrant.NewValueMap(map[string]any{
				"text":  "hello",
				"topic": "search",
			}),
		},
	}
	client.scrollOffset = qdrant.NewIDUUID("pt-next")
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doScroll(&ast.ScrollStmt{Collection: "docs", Limit: 5})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Equal(t, map[string]any{
		"points": []map[string]any{
			{
				"id": "7",
				"payload": map[string]any{
					"text":  "hello",
					"topic": "search",
				},
			},
		},
		"next_offset": "pt-next",
	}, resp.Data)
}

func TestDoScrollPreservesNumericNextOffsetType(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.scrollOffset = qdrant.NewIDNum(42)
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doScroll(&ast.ScrollStmt{Collection: "docs", Limit: 5})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Equal(t, uint64(42), resp.Data.(map[string]any)["next_offset"])
}

func float64Ptr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(v int) *int {
	return &v
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}

func newEmbeddingServer(t *testing.T, embedding []float32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/embeddings", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": embedding},
			},
		}))
	}))
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

func TestBuildSearchRequestHybridDBSF(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{})
	fusion := "dbsf"
	req, err := exec.buildSearchRequest(context.Background(), &ast.SearchStmt{
		Collection: "demo",
		QueryText:  "vector database",
		Limit:      5,
		Hybrid:     true,
		Fusion:     &fusion,
	}, "dense-model", "sparse-model", false, 5)
	require.NoError(t, err)

	require.Equal(t, qdrant.Fusion_DBSF, req.GetQuery().GetFusion())
}

func TestBuildSearchRequestHybridRRFByDefault(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{})
	req, err := exec.buildSearchRequest(context.Background(), &ast.SearchStmt{
		Collection: "demo",
		QueryText:  "vector database",
		Limit:      5,
		Hybrid:     true,
	}, "dense-model", "sparse-model", false, 5)
	require.NoError(t, err)

	require.Equal(t, qdrant.Fusion_RRF, req.GetQuery().GetFusion())
}

func TestExplainSelectAndScrollQueries(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{})

	t.Run("select", func(t *testing.T) {
		plan, err := exec.Explain(`SELECT * FROM docs WHERE id = 'pt-1'`)
		require.NoError(t, err)
		require.Contains(t, plan, "Statement: SELECT * FROM docs WHERE id = 'pt-1'")
		require.Contains(t, plan, "Retrieve a single point by ID")
	})

	t.Run("scroll basic", func(t *testing.T) {
		plan, err := exec.Explain(`SCROLL FROM docs LIMIT 10`)
		require.NoError(t, err)
		require.Contains(t, plan, "Statement: SCROLL FROM docs LIMIT 10")
		require.Contains(t, plan, "Scroll (paginate) through points")
	})

	t.Run("scroll with filter and after", func(t *testing.T) {
		plan, err := exec.Explain(`SCROLL FROM docs WHERE status = 'active' AFTER 'pt-5' LIMIT 20`)
		require.NoError(t, err)
		require.Contains(t, plan, "Filter:")
		require.Contains(t, plan, "After: pt-5")
	})

	t.Run("turbo quantization explain", func(t *testing.T) {
		plan, err := exec.Explain(`CREATE COLLECTION docs QUANTIZE TURBO BITS 2 ALWAYS RAM`)
		require.NoError(t, err)
		require.Contains(t, plan, "Quantization: turbo")
		require.Contains(t, plan, "Turbo bits: 2")
		require.Contains(t, plan, "Quantization storage: ALWAYS RAM")
	})
}

func TestTurboBitsEnum(t *testing.T) {
	require.Equal(t, qdrant.TurboQuantBitSize_Bits1, *turboBitsEnum(1.0))
	require.Equal(t, qdrant.TurboQuantBitSize_Bits1_5, *turboBitsEnum(1.5))
	require.Equal(t, qdrant.TurboQuantBitSize_Bits2, *turboBitsEnum(2.0))
	require.Equal(t, qdrant.TurboQuantBitSize_Bits4, *turboBitsEnum(4.0))
	require.Nil(t, turboBitsEnum(3.0))
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

	exec := NewExecutor(client, &config.Config{})
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

	exec := NewExecutor(client, &config.Config{})
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

	exec := NewExecutor(client, &config.Config{})
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

	exec := NewExecutor(client, &config.Config{})
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

	exec := NewExecutor(client, &config.Config{})
	resp, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "no_schema"})
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	assert.Nil(t, data["payload_schema"])
}

func TestShowCollectionNonexistentRaises(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = false

	exec := NewExecutor(client, &config.Config{})
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

	exec := NewExecutor(client, &config.Config{})
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

	exec := NewExecutor(client, &config.Config{})
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

	exec := NewExecutor(client, &config.Config{})
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

	exec := NewExecutor(client, &config.Config{})
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

	exec := NewExecutor(client, &config.Config{})
	_, err := exec.doShowCollection(&ast.ShowCollectionStmt{Collection: "broken"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no vector configuration")
}

func TestExplainShowCollection(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{})
	plan, err := exec.Explain("SHOW COLLECTION docs")
	require.NoError(t, err)
	require.Contains(t, plan, "Statement: SHOW COLLECTION docs")
	require.Contains(t, plan, "Inspect collection diagnostics")
}
