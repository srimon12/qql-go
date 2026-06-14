package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/filters"
	"github.com/srimon12/qql-go/internal/sparse"
)

func (e *Executor) doSearch(n *ast.SearchStmt) (*ExecResponse, error) {
	ctx := context.Background()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	model := e.resolveDenseModel(n.Model)
	sparseModel := e.resolveSparseModel(n.SparseModel)
	if err := validateSearchMMRUsage(n); err != nil {
		return nil, err
	}

	topo, err := e.resolveVectorTopology(ctx, n.Collection)
	hasRerankVector := topo != nil && topo.RerankVector != nil
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}

	limit := uint64(n.Limit)
	if limit == 0 {
		limit = 10
	}

	fetchLimit := effectiveSearchLimit(limit, n.Rerank)

	denseName, sparseName := denseVectorName, sparseVectorName
	if topo != nil && topo.DenseVector != nil && *topo.DenseVector != "" {
		denseName = *topo.DenseVector
	}
	if topo != nil && topo.SparseVector != nil && *topo.SparseVector != "" {
		sparseName = *topo.SparseVector
	}
	if n.DenseVector != nil {
		denseName = *n.DenseVector
	}
	if n.SparseVector != nil {
		sparseName = *n.SparseVector
	}
	searchReq, err := e.buildSearchRequest(ctx, n, model, sparseModel, hasRerankVector, fetchLimit, denseName, sparseName)
	if err != nil {
		return nil, err
	}

	var filter *qdrant.Filter
	if n.QueryFilter != nil {
		filter, err = filters.NewFilterConverter().BuildFilter(n.QueryFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to build filter: %w", err)
		}
	}

	if n.Offset > 0 {
		searchReq.Offset = qdrant.PtrOf(uint64(n.Offset))
	}
	if n.ScoreThreshold != nil {
		searchReq.ScoreThreshold = qdrant.PtrOf(float32(*n.ScoreThreshold))
	}
	if n.LookupFrom != "" {
		searchReq.LookupFrom = &qdrant.LookupLocation{
			CollectionName: n.LookupFrom,
		}
		if n.LookupVector != nil && *n.LookupVector != "" {
			searchReq.LookupFrom.VectorName = n.LookupVector
		}
	}

	if n.GroupBy != "" {
		groupReq := buildGroupSearchRequest(n, searchReq, filter)
		results, err := e.client.QueryGroups(ctx, groupReq)
		if err != nil {
			return nil, fmt.Errorf("grouped search failed: %w", err)
		}
		message, groups := formatGroupSearchResults(n.GroupBy, n.Hybrid, n.SparseOnly, results)
		return &ExecResponse{
			OK:        true,
			Operation: "search",
			Message:   message,
			Data: map[string]any{
				"count":      len(groups),
				"groups":     groups,
				"collection": n.Collection,
				"group_by":   n.GroupBy,
				"hybrid":     n.Hybrid,
			},
		}, nil
	}

	searchReq.Filter = filter

	results, err := e.client.Query(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	message, hits := e.formatSearchResults(results)
	return &ExecResponse{
		OK:        true,
		Operation: "search",
		Message:   message,
		Data: map[string]any{
			"count":      len(hits),
			"results":    hits,
			"collection": n.Collection,
			"hybrid":     n.Hybrid,
			"rerank":     n.Rerank,
		},
	}, nil
}

func (e *Executor) doRecommend(n *ast.RecommendStmt) (*ExecResponse, error) {
	ctx := context.Background()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	topo, err := e.resolveVectorTopology(ctx, n.Collection)
	denseName := denseVectorName
	if topo != nil && topo.DenseVector != nil && *topo.DenseVector != "" {
		denseName = *topo.DenseVector
	}
	if n.Using != nil {
		denseName = *n.Using
	}
	req, err := buildRecommendRequest(n, denseName)
	if err != nil {
		return nil, err
	}

	results, err := e.client.Query(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("recommend failed: %w", err)
	}

	message, hits := e.formatSearchResults(results)
	return &ExecResponse{
		OK:        true,
		Operation: "recommend",
		Message:   message,
		Data: map[string]any{
			"count":      len(hits),
			"results":    hits,
			"collection": n.Collection,
		},
	}, nil
}

func (e *Executor) buildSearchRequest(ctx context.Context, n *ast.SearchStmt, denseModel, sparseModel string, hasRerankVector bool, limit uint64, denseName, sparseName string) (*qdrant.QueryPoints, error) {
	params := searchParamsFromWithClause(n.WithClause)

	if n.SparseOnly {
		if n.Rerank {
			if e.usesLocalEmbeddings() {
				return nil, fmt.Errorf("RERANK is currently only available in cloud inference mode")
			}
			if !hasRerankVector {
				return nil, fmt.Errorf("collection '%s' does not support rerank; create it with HYBRID RERANK", n.Collection)
			}
			rerankModel := rerankModelDefault
			if n.RerankModel != nil && *n.RerankModel != "" {
				rerankModel = *n.RerankModel
			}
			sparsePrefetch := &qdrant.PrefetchQuery{
				Query:  qdrant.NewQueryDocument(&qdrant.Document{Text: n.QueryText, Model: sparseModel}),
				Using:  qdrant.PtrOf(sparseName),
				Limit:  qdrant.PtrOf(limit * rerankPrefetchFactor),
				Params: params,
			}
			if e.usesLocalEmbeddings() {
				sv := sparse.BuildQuery(n.QueryText)
				sparsePrefetch.Query = qdrant.NewQuerySparse(sv.Indices, sv.Values)
			}
			return buildRerankSearchRequest(n.Collection, n.QueryText, rerankModel, limit, []*qdrant.PrefetchQuery{sparsePrefetch}, params), nil
		}
		query := qdrant.NewQueryDocument(&qdrant.Document{
			Text:  n.QueryText,
			Model: sparseModel,
		})
		if e.usesLocalEmbeddings() {
			sv := sparse.BuildQuery(n.QueryText)
			query = qdrant.NewQuerySparse(sv.Indices, sv.Values)
		}
		return &qdrant.QueryPoints{
			CollectionName: n.Collection,
			Query:          query,
			Using:          qdrant.PtrOf(sparseName),
			Limit:          qdrant.PtrOf(limit),
			Params:         params,
		}, nil
	}

	if n.Rerank {
		if e.usesLocalEmbeddings() {
			return nil, fmt.Errorf("RERANK is currently only available in cloud inference mode")
		}
		if !hasRerankVector {
			return nil, fmt.Errorf("collection '%s' does not support rerank; create it with HYBRID RERANK", n.Collection)
		}
		rerankModel := rerankModelDefault
		if n.RerankModel != nil && *n.RerankModel != "" {
			rerankModel = *n.RerankModel
		}
		prefetch, err := e.buildSearchPrefetches(ctx, n.QueryText, denseModel, sparseModel, limit, params, nil, denseName, sparseName)
		if err != nil {
			return nil, err
		}
		return buildRerankSearchRequest(n.Collection, n.QueryText, rerankModel, limit, prefetch, params), nil
	}

	if n.Hybrid {
		prefetch, err := e.buildSearchPrefetches(ctx, n.QueryText, denseModel, sparseModel, limit, params, n.WithClause, denseName, sparseName)
		if err != nil {
			return nil, err
		}
		fusionMode := qdrant.Fusion_RRF
		if n.Fusion != nil && *n.Fusion == "dbsf" {
			fusionMode = qdrant.Fusion_DBSF
		}
		return &qdrant.QueryPoints{
			CollectionName: n.Collection,
			Prefetch:       prefetch,
			Query:          qdrant.NewQueryFusion(fusionMode),
			Limit:          qdrant.PtrOf(limit),
			Params:         params,
		}, nil
	}

	query := qdrant.NewQueryDocument(&qdrant.Document{
		Text:  n.QueryText,
		Model: denseModel,
	})
	var mmrNearest *qdrant.VectorInput
	if e.usesLocalEmbeddings() {
		embedClient, err := e.embeddingClient(denseModel)
		if err != nil {
			return nil, err
		}
		denseVector, err := embedClient.Embed(ctx, n.QueryText)
		if err != nil {
			return nil, fmt.Errorf("failed to embed search query: %w", err)
		}
		query = qdrant.NewQueryDense(denseVector)
		if hasMMR(n.WithClause) {
			mmrNearest = qdrant.NewVectorInputDense(denseVector)
		}
	} else if hasMMR(n.WithClause) {
		mmrNearest = qdrant.NewVectorInputDocument(&qdrant.Document{
			Text:  n.QueryText,
			Model: denseModel,
		})
	}

	if hasMMR(n.WithClause) {
		query = qdrant.NewQueryMMR(mmrNearest, &qdrant.Mmr{
			Diversity:       float32PtrFromFloat64(n.WithClause.MmrDiversity),
			CandidatesLimit: uint32PtrFromInt(n.WithClause.MmrCandidates),
		})
	}

	return &qdrant.QueryPoints{
		CollectionName: n.Collection,
		Query:          query,
		Using:          qdrant.PtrOf(denseName),
		Limit:          qdrant.PtrOf(limit),
		Params:         params,
	}, nil
}

func buildRecommendRequest(n *ast.RecommendStmt, usingName string) (*qdrant.QueryPoints, error) {
	if hasMMR(n.WithClause) {
		return nil, fmt.Errorf("MMR is supported only for SEARCH statements")
	}
	query := qdrant.NewQueryRecommend(&qdrant.RecommendInput{
		Positive: buildRecommendVectorInputs(n.PositiveIDs),
		Negative: buildRecommendVectorInputs(n.NegativeIDs),
	})
	if n.Strategy != nil && *n.Strategy != "" {
		strategy, ok := recommendStrategy(*n.Strategy)
		if !ok {
			return nil, fmt.Errorf("unknown recommend strategy '%s'", *n.Strategy)
		}
		query = qdrant.NewQueryRecommend(&qdrant.RecommendInput{
			Positive: buildRecommendVectorInputs(n.PositiveIDs),
			Negative: buildRecommendVectorInputs(n.NegativeIDs),
			Strategy: strategy.Enum(),
		})
	}

	using := usingName
	if n.Using != nil {
		using = *n.Using
	}

	req := &qdrant.QueryPoints{
		CollectionName: n.Collection,
		Query:          query,
		Limit:          qdrant.PtrOf(uint64(n.Limit)),
		Using:          qdrant.PtrOf(using),
		Params:         searchParamsFromWithClause(n.WithClause),
	}
	if n.Offset > 0 {
		req.Offset = qdrant.PtrOf(uint64(n.Offset))
	}
	if n.ScoreThreshold != nil {
		req.ScoreThreshold = qdrant.PtrOf(float32(*n.ScoreThreshold))
	}
	if n.LookupFrom != "" {
		req.LookupFrom = &qdrant.LookupLocation{
			CollectionName: n.LookupFrom,
		}
		if n.LookupVector != nil && *n.LookupVector != "" {
			req.LookupFrom.VectorName = n.LookupVector
		}
	}
	if n.QueryFilter != nil {
		filter, err := filters.NewFilterConverter().BuildFilter(n.QueryFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to build filter: %w", err)
		}
		req.Filter = addExcludedIDsToFilter(filter, append(append([]any{}, n.PositiveIDs...), n.NegativeIDs...))
	} else {
		req.Filter = addExcludedIDsToFilter(nil, append(append([]any{}, n.PositiveIDs...), n.NegativeIDs...))
	}

	return req, nil
}

func (e *Executor) buildSearchPrefetches(ctx context.Context, queryText, denseModel, sparseModel string, limit uint64, params *qdrant.SearchParams, withClause *ast.SearchWith, denseName, sparseName string) ([]*qdrant.PrefetchQuery, error) {
	denseQuery := qdrant.NewQueryDocument(&qdrant.Document{
		Text:  queryText,
		Model: denseModel,
	})
	var mmrNearest *qdrant.VectorInput
	sparseQuery := qdrant.NewQueryDocument(&qdrant.Document{
		Text:  queryText,
		Model: sparseModel,
	})
	if e.usesLocalEmbeddings() {
		embedClient, err := e.embeddingClient(denseModel)
		if err != nil {
			return nil, fmt.Errorf("failed to create embedding client for search: %w", err)
		}
		denseVector, sv, err := embedConcurrentQuery(ctx, embedClient, queryText)
		if err != nil {
			return nil, fmt.Errorf("failed to embed search query: %w", err)
		}
		denseQuery = qdrant.NewQueryDense(denseVector)
		if hasMMR(withClause) {
			mmrNearest = qdrant.NewVectorInputDense(denseVector)
		}
		sparseQuery = qdrant.NewQuerySparse(sv.Indices, sv.Values)
	} else if hasMMR(withClause) {
		mmrNearest = qdrant.NewVectorInputDocument(&qdrant.Document{
			Text:  queryText,
			Model: denseModel,
		})
	}

	if hasMMR(withClause) {
		denseQuery = qdrant.NewQueryMMR(mmrNearest, &qdrant.Mmr{
			Diversity:       float32PtrFromFloat64(withClause.MmrDiversity),
			CandidatesLimit: uint32PtrFromInt(withClause.MmrCandidates),
		})
	}

	return []*qdrant.PrefetchQuery{
		{
			Query:  sparseQuery,
			Using:  qdrant.PtrOf(sparseName),
			Limit:  qdrant.PtrOf(limit),
			Params: params,
		},
		{
			Query:  denseQuery,
			Using:  qdrant.PtrOf(denseName),
			Limit:  qdrant.PtrOf(limit),
			Params: params,
		},
	}, nil
}

func buildGroupSearchRequest(n *ast.SearchStmt, req *qdrant.QueryPoints, filter *qdrant.Filter) *qdrant.QueryPointGroups {
	groupSize := uint64(n.GroupSize)
	if groupSize == 0 {
		groupSize = 3
	}

	return &qdrant.QueryPointGroups{
		CollectionName: req.GetCollectionName(),
		Prefetch:       req.GetPrefetch(),
		Query:          req.GetQuery(),
		Using:          req.Using,
		Filter:         filter,
		Params:         req.GetParams(),
		Limit:          req.Limit,
		GroupSize:      qdrant.PtrOf(groupSize),
		GroupBy:        n.GroupBy,
		WithPayload:    qdrant.NewWithPayload(true),
		ScoreThreshold: req.ScoreThreshold,
		LookupFrom:     req.LookupFrom,
	}
}

func buildRerankSearchRequest(collection, queryText, rerankModel string, limit uint64, prefetch []*qdrant.PrefetchQuery, params *qdrant.SearchParams) *qdrant.QueryPoints {
	return &qdrant.QueryPoints{
		CollectionName: collection,
		Prefetch:       prefetch,
		Query: qdrant.NewQueryDocument(&qdrant.Document{
			Text:  queryText,
			Model: rerankModel,
		}),
		Using:  qdrant.PtrOf(rerankVectorName),
		Limit:  qdrant.PtrOf(limit),
		Params: params,
	}
}

func buildRecommendVectorInputs(ids []any) []*qdrant.VectorInput {
	if len(ids) == 0 {
		return nil
	}
	inputs := make([]*qdrant.VectorInput, 0, len(ids))
	for _, id := range ids {
		inputs = append(inputs, qdrant.NewVectorInputID(newPointID(id)))
	}
	return inputs
}

func recommendStrategy(value string) (qdrant.RecommendStrategy, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "average_vector":
		return qdrant.RecommendStrategy_AverageVector, true
	case "best_score":
		return qdrant.RecommendStrategy_BestScore, true
	case "sum_scores":
		return qdrant.RecommendStrategy_SumScores, true
	default:
		return 0, false
	}
}

func (e *Executor) formatSearchResults(results []*qdrant.ScoredPoint) (string, []SearchHit) {
	if len(results) == 0 {
		return "No results found", []SearchHit{}
	}

	var resultLines []string
	hits := make([]SearchHit, 0, len(results))
	for _, r := range results {
		id := fmt.Sprintf("%v", r.GetId())
		jsonID := pointIDString(r.GetId())
		score := r.GetScore()
		payload := r.GetPayload()
		text := ""
		if p, ok := payload["text"]; ok {
			if sv, ok := p.GetKind().(*qdrant.Value_StringValue); ok {
				text = sv.StringValue
			}
		}
		resultLines = append(resultLines, fmt.Sprintf("id:%s score:%.4f payload:%s", id, score, text))
		hits = append(hits, SearchHit{
			ID:    jsonID,
			Score: score,
			Text:  text,
		})
	}

	return fmt.Sprintf("Found %d result(s):\n%s", len(results), strings.Join(resultLines, "\n")), hits
}

func formatGroupSearchResults(groupBy string, hybrid, sparseOnly bool, groups []*qdrant.PointGroup) (string, []GroupedSearchResult) {
	if len(groups) == 0 {
		return fmt.Sprintf("Found 0 group(s) by '%s' (grouped)", groupBy), []GroupedSearchResult{}
	}

	label := "grouped"
	if hybrid {
		label = "hybrid, grouped"
	} else if sparseOnly {
		label = "sparse, grouped"
	}

	lines := make([]string, 0, len(groups))
	formatted := make([]GroupedSearchResult, 0, len(groups))
	for _, group := range groups {
		groupID := groupIDString(group.GetId())
		if groupID == "" {
			groupID = fmt.Sprintf("%v", group.GetId())
		}
		lines = append(lines, fmt.Sprintf("group:%s hits:%d", groupID, len(group.GetHits())))

		hits := make([]SearchHit, 0, len(group.GetHits()))
		for _, hit := range group.GetHits() {
			jsonID := pointIDString(hit.GetId())
			text := ""
			if payload := hit.GetPayload(); payload != nil {
				if value, ok := payload["text"]; ok {
					if sv, ok := value.GetKind().(*qdrant.Value_StringValue); ok {
						text = sv.StringValue
					}
				}
			}
			hits = append(hits, SearchHit{
				ID:    jsonID,
				Score: hit.GetScore(),
				Text:  text,
			})
		}
		formatted = append(formatted, GroupedSearchResult{GroupID: groupID, Hits: hits})
	}

	return fmt.Sprintf("Found %d group(s) by '%s' (%s)\n%s", len(groups), groupBy, label, strings.Join(lines, "\n")), formatted
}

func effectiveSearchLimit(limit uint64, rerank bool) uint64 {
	if !rerank || limit == 0 {
		return limit
	}

	if limit > ^uint64(0)/rerankPrefetchFactor {
		return limit
	}

	boosted := limit * rerankPrefetchFactor
	if boosted <= rerankPrefetchCap {
		return boosted
	}
	if limit > rerankPrefetchCap {
		return limit
	}
	return rerankPrefetchCap
}

func hasMMR(withClause *ast.SearchWith) bool {
	return withClause != nil && (withClause.MmrDiversity != nil || withClause.MmrCandidates != nil)
}

func validateSearchMMRUsage(n *ast.SearchStmt) error {
	if !hasMMR(n.WithClause) {
		return nil
	}
	if n.SparseOnly {
		return fmt.Errorf("MMR is not supported with USING SPARSE yet")
	}
	return nil
}

func searchParamsFromWithClause(withClause *ast.SearchWith) *qdrant.SearchParams {
	if withClause == nil {
		return nil
	}

	params := &qdrant.SearchParams{}
	if withClause.HnswEf > 0 {
		params.HnswEf = qdrant.PtrOf(uint64(withClause.HnswEf))
	}
	if withClause.Exact {
		params.Exact = qdrant.PtrOf(true)
	}
	if withClause.Acorn {
		params.Acorn = &qdrant.AcornSearchParams{Enable: qdrant.PtrOf(true)}
	}
	if withClause.IndexedOnly {
		params.IndexedOnly = qdrant.PtrOf(true)
	}
	if withClause.Quantization != nil {
		params.Quantization = &qdrant.QuantizationSearchParams{}
		if withClause.Quantization.Ignore != nil {
			params.Quantization.Ignore = qdrant.PtrOf(*withClause.Quantization.Ignore)
		}
		if withClause.Quantization.Rescore != nil {
			params.Quantization.Rescore = qdrant.PtrOf(*withClause.Quantization.Rescore)
		}
		if withClause.Quantization.Oversampling != nil {
			params.Quantization.Oversampling = qdrant.PtrOf(*withClause.Quantization.Oversampling)
		}
	}

	if params.HnswEf == nil && params.Exact == nil && params.Acorn == nil && params.IndexedOnly == nil && params.Quantization == nil {
		return nil
	}

	return params
}
