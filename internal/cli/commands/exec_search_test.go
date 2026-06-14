package commands

import (
	"context"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/stretchr/testify/require"
)

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
			HnswEf:        128,
			Exact:         true,
			Acorn:         true,
			IndexedOnly:   true,
			MmrDiversity:  float64Ptr(0.5),
			MmrCandidates: intPtr(50),
			Quantization: &ast.QuantizationSearchWith{
				Ignore:       boolPtr(true),
				Rescore:      boolPtr(false),
				Oversampling: float64Ptr(2.5),
			},
		},
	}, "dense-model", sparseModel, false, 5, "dense", "sparse")
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
	require.NotNil(t, prefetch[1].GetQuery().GetNearestWithMmr())
	require.Equal(t, "dense-model", prefetch[1].GetQuery().GetNearestWithMmr().GetNearest().GetDocument().GetModel())
	require.InDelta(t, 0.5, prefetch[1].GetQuery().GetNearestWithMmr().GetMmr().GetDiversity(), 0.0001)
	require.Equal(t, uint32(50), prefetch[1].GetQuery().GetNearestWithMmr().GetMmr().GetCandidatesLimit())
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
	}, "dense-model", "custom-sparse", false, 5, "dense", "sparse")
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
			HnswEf:        128,
			Acorn:         true,
			MmrDiversity:  float64Ptr(0.4),
			MmrCandidates: intPtr(25),
		},
		GroupBy:   "category",
		GroupSize: 2,
	}, "dense-model", sparseModel, false, 5, "dense", "sparse")
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
	require.NotNil(t, groupReq.GetPrefetch()[1].GetQuery().GetNearestWithMmr())
	require.Equal(t, "dense-model", groupReq.GetPrefetch()[1].GetQuery().GetNearestWithMmr().GetNearest().GetDocument().GetModel())
	require.InDelta(t, 0.4, groupReq.GetPrefetch()[1].GetQuery().GetNearestWithMmr().GetMmr().GetDiversity(), 0.0001)
	require.Equal(t, uint32(25), groupReq.GetPrefetch()[1].GetQuery().GetNearestWithMmr().GetMmr().GetCandidatesLimit())
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
	}, "dense-model", "custom-sparse", false, 5, "dense", "sparse")
	require.Error(t, err)
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

func TestDoSearchForwardsSearchParityFields(t *testing.T) {
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

	_, err := exec.doSearch(&ast.SearchStmt{
		Collection:     "docs",
		QueryText:      "vector database",
		Limit:          5,
		Offset:         10,
		ScoreThreshold: float64Ptr(0.8),
		LookupFrom:     "profiles",
		LookupVector:   strPtr("preferences"),
	})
	require.NoError(t, err)
	require.Len(t, client.queryRequests, 1)
	req := client.queryRequests[0]
	require.Equal(t, uint64(10), req.GetOffset())
	require.InDelta(t, float32(0.8), req.GetScoreThreshold(), 0.0001)
	require.NotNil(t, req.GetLookupFrom())
	require.Equal(t, "profiles", req.GetLookupFrom().GetCollectionName())
	require.Equal(t, "preferences", req.GetLookupFrom().GetVectorName())
}

func TestBuildGroupSearchRequestCarriesScoreThresholdAndLookup(t *testing.T) {
	req := &qdrant.QueryPoints{
		CollectionName: "docs",
		Limit:          qdrant.PtrOf(uint64(5)),
		ScoreThreshold: qdrant.PtrOf(float32(0.7)),
		LookupFrom:     &qdrant.LookupLocation{CollectionName: "profiles", VectorName: strPtr("preferences")},
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(false),
	}

	groupReq := buildGroupSearchRequest(&ast.SearchStmt{
		Collection: "docs",
		GroupBy:    "author",
		GroupSize:  2,
	}, req, nil)

	require.Equal(t, "author", groupReq.GetGroupBy())
	require.InDelta(t, float32(0.7), groupReq.GetScoreThreshold(), 0.0001)
	require.NotNil(t, groupReq.GetLookupFrom())
	require.Equal(t, "profiles", groupReq.GetLookupFrom().GetCollectionName())
	require.Equal(t, "preferences", groupReq.GetLookupFrom().GetVectorName())
}

func TestDoSearchSupportsMMRWithHybrid(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.info = &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			Params: &qdrant.CollectionParams{
				VectorsConfig: qdrant.NewVectorsConfigMap(collectionVectorParams(denseVectorSize, false)),
				SparseVectorsConfig: qdrant.NewSparseVectorsConfig(map[string]*qdrant.SparseVectorParams{
					sparseVectorName: {},
				}),
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
	require.NoError(t, err)
	require.Len(t, client.queryRequests, 1)
	prefetch := client.queryRequests[0].GetPrefetch()
	require.Len(t, prefetch, 2)
	require.NotNil(t, prefetch[1].GetQuery().GetNearestWithMmr())
	require.InDelta(t, 0.5, prefetch[1].GetQuery().GetNearestWithMmr().GetMmr().GetDiversity(), 0.0001)
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
	exec := NewExecutor(nil, &config.Config{InferenceMode: "local"})
	_, err := exec.buildRecommendRequest(context.Background(), &ast.RecommendStmt{
		Collection:  "docs",
		PositiveIDs: []any{"a"},
		Limit:       5,
		WithClause:  &ast.SearchWith{MmrDiversity: float64Ptr(0.5)},
	}, "dense")
	require.Error(t, err)
	require.Contains(t, err.Error(), "MMR is supported only for standard NEAREST queries")
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
	}, "dense-model", "sparse-model", false, 5, "dense", "sparse")
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
	}, "dense-model", "sparse-model", true, 5, "dense", "sparse")
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
	}, "dense-model", "sparse-model", false, 5, "dense", "sparse")
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
	}, "dense-model", "sparse-model", false, 5, "dense", "sparse")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to embed dense search query")
}

func TestBuildRecommendRequestDefaults(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{InferenceMode: "local"})
	req, err := exec.buildRecommendRequest(context.Background(), &ast.RecommendStmt{
		Collection:  "docs",
		PositiveIDs: []any{"a"},
		Limit:       5,
	}, "dense")
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
	exec := NewExecutor(nil, &config.Config{InferenceMode: "local"})
	req, err := exec.buildRecommendRequest(context.Background(), &ast.RecommendStmt{
		Collection:     "docs",
		PositiveIDs:    []any{"a", "b"},
		NegativeIDs:    []any{"c"},
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
	}, "dense")
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
	exec := NewExecutor(nil, &config.Config{InferenceMode: "local"})
	req, err := exec.buildRecommendRequest(context.Background(), &ast.RecommendStmt{
		Collection:  "docs",
		PositiveIDs: []any{"a"},
		Limit:       5,
		LookupFrom:  "src",
	}, "dense")
	require.NoError(t, err)
	require.NotNil(t, req.GetLookupFrom())
	require.Equal(t, "src", req.GetLookupFrom().GetCollectionName())
	require.Empty(t, req.GetLookupFrom().GetVectorName())
}

func TestBuildRecommendRequestUnknownStrategy(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{InferenceMode: "local"})
	_, err := exec.buildRecommendRequest(context.Background(), &ast.RecommendStmt{
		Collection:  "docs",
		PositiveIDs: []any{"a"},
		Limit:       5,
		Strategy:    strPtr("bad_strategy"),
	}, "dense")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown recommend strategy")
}

func TestBuildRecommendRequestFilterExcludesIDs(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{InferenceMode: "local"})
	req, err := exec.buildRecommendRequest(context.Background(), &ast.RecommendStmt{
		Collection:  "docs",
		PositiveIDs: []any{"a"},
		NegativeIDs: []any{"b"},
		Limit:       5,
		QueryFilter: &ast.CompareExpr{Field: "status", Op: "=", Value: "active"},
	}, "dense")
	require.NoError(t, err)
	filter := req.GetFilter()
	require.NotNil(t, filter)
	require.Len(t, filter.GetMust(), 1)
	require.Len(t, filter.GetMustNot(), 1)
	require.Equal(t, "active", filter.GetMust()[0].GetField().GetMatch().GetKeyword())
	require.Len(t, filter.GetMustNot()[0].GetHasId().GetHasId(), 2)
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
	}, "dense-model", "sparse-model", false, 5, "dense", "sparse")
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
	}, "dense-model", "sparse-model", false, 5, "dense", "sparse")
	require.NoError(t, err)

	require.Equal(t, qdrant.Fusion_RRF, req.GetQuery().GetFusion())
}
