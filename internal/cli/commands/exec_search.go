package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/filters"
	"github.com/srimon12/qql-go/internal/pipeline"
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
	req, err := e.buildRecommendRequest(ctx, n, denseName)
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
	state := &pipeline.QueryState{
		QueryText:  n.QueryText,
		Params:     searchParamsFromWithClause(n.WithClause),
		LocalEmbed: e.usesLocalEmbeddings(),
		Embedder:   e,
	}

	if hasMMR(n.WithClause) {
		state.HasMMR = true
		if n.WithClause.MmrDiversity != nil {
			state.MmrDiversity = float32(*n.WithClause.MmrDiversity)
		}
		if n.WithClause.MmrCandidates != nil {
			state.MmrCandidates = uint32(*n.WithClause.MmrCandidates)
		}
	}

	p := pipeline.NewQueryPipeline()

	// 1. Core Embedding Node (Dense or Sparse)
	if n.SparseOnly {
		if n.Rerank && !hasRerankVector {
			return nil, fmt.Errorf("collection '%s' does not support rerank; create it with HYBRID RERANK", n.Collection)
		}
		p.Add(&pipeline.SparseEmbedNode{
			Model:      sparseModel,
			VectorName: sparseName,
			Limit:      limit * func() uint64 { if n.Rerank { return rerankPrefetchFactor }; return 1 }(),
			AsPrefetch: n.Rerank,
		})
	} else if n.Hybrid || n.Rerank {
		if n.Rerank && !hasRerankVector {
			return nil, fmt.Errorf("collection '%s' does not support rerank; create it with HYBRID RERANK", n.Collection)
		}
		// Hybrid needs both dense and sparse as prefetches for fusion
		p.Add(&pipeline.SparseEmbedNode{
			Model:      sparseModel,
			VectorName: sparseName,
			Limit:      limit * func() uint64 { if n.Rerank { return rerankPrefetchFactor }; return 1 }(),
			AsPrefetch: true,
		})
		p.Add(&pipeline.DenseEmbedNode{
			Model:      denseModel,
			VectorName: denseName,
			Limit:      limit * func() uint64 { if n.Rerank { return rerankPrefetchFactor }; return 1 }(),
			AsPrefetch: true,
		})
		// 2. Fusion
		fusionMode := "rrf"
		if n.Fusion != nil {
			fusionMode = *n.Fusion
		}
		p.Add(&pipeline.FusionNode{Mode: fusionMode})
	} else {
		// Standard dense search
		p.Add(&pipeline.DenseEmbedNode{
			Model:      denseModel,
			VectorName: denseName,
			Limit:      limit,
			AsPrefetch: false,
		})
	}

	// 3. Optional Rerank
	if n.Rerank {
		rerankVecName := "colbert"
		if hasRerankVector {
			rerankVecName = "" // Let Qdrant infer if collection only has 1 multivector
		}
		rerankModel := rerankModelDefault
		if n.RerankModel != nil && *n.RerankModel != "" {
			rerankModel = *n.RerankModel
		}
		p.Add(&pipeline.RerankNode{
			Model:      rerankModel,
			VectorName: rerankVecName,
			Limit:      limit,
		})
	}

	// Execute DAG
	if err := p.Execute(ctx, state); err != nil {
		return nil, err
	}

	usingName := ""
	if n.SparseOnly {
		if n.Rerank {
			usingName = rerankVectorName
		} else {
			usingName = sparseName
		}
	} else if n.Rerank {
		usingName = rerankVectorName
	} else if n.Hybrid {
		usingName = ""
	} else {
		usingName = denseName
	}

	return buildQueryPointsFromState(n, state, limit, usingName), nil
}

func buildQueryPointsFromState(n *ast.SearchStmt, state *pipeline.QueryState, limit uint64, usingName string) *qdrant.QueryPoints {
	req := &qdrant.QueryPoints{
		CollectionName: n.Collection,
		Query:          state.TargetQuery,
		Prefetch:       state.Prefetches,
		Limit:          qdrant.PtrOf(limit),
		Params:         state.Params,
	}
	if usingName != "" {
		req.Using = qdrant.PtrOf(usingName)
	}
	return req
}

func (e *Executor) buildRecommendRequest(ctx context.Context, n *ast.RecommendStmt, usingName string) (*qdrant.QueryPoints, error) {
	state := &pipeline.QueryState{
		Params:     searchParamsFromWithClause(n.WithClause),
		HasMMR:     hasMMR(n.WithClause),
		LocalEmbed: e.usesLocalEmbeddings(),
		Embedder:   e,
	}

	p := pipeline.NewQueryPipeline()
	p.Add(&pipeline.RecommendNode{
		PositiveIDs: n.PositiveIDs,
		NegativeIDs: n.NegativeIDs,
		Strategy:    n.Strategy,
	})

	if err := p.Execute(ctx, state); err != nil {
		return nil, err
	}

	using := usingName
	if n.Using != nil {
		using = *n.Using
	}

	req := &qdrant.QueryPoints{
		CollectionName: n.Collection,
		Query:          state.TargetQuery,
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
