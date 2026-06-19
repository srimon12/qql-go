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

// queryBuildResult holds the shared output of building a query state and pipeline.
type queryBuildResult struct {
	state    *pipeline.QueryState
	pipeline *pipeline.QueryPipeline
	stmt     *ast.QueryStmt
}

// buildQueryStateAndPipeline resolves vector topology, models, filters, and constructs
// the QueryState + QueryPipeline from a parsed QueryStmt. Shared by doQuery and
// BuildQueryPoints to avoid duplicating ~180 lines of identical logic.
func (e *Executor) buildQueryStateAndPipeline(ctx context.Context, stmt *ast.QueryStmt) (*queryBuildResult, error) {
	denseVectorName := ""
	sparseVectorName := ""
	if stmt.Using != nil {
		denseVectorName = *stmt.Using
		sparseVectorName = *stmt.Using
	} else {
		topo, err := e.resolveVectorTopology(ctx, stmt.Collection)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve vector topology for collection '%s': %w", stmt.Collection, err)
		}
		if topo != nil && topo.DenseVector != nil && *topo.DenseVector != "" {
			denseVectorName = *topo.DenseVector
			if topo.SparseVector != nil && *topo.SparseVector != "" {
				sparseVectorName = *topo.SparseVector
			}
		}
	}

	denseModel := e.resolveDenseModel(stmt.Model)
	var sparseModel *string
	if stmt.Type == ast.QueryTypeHybrid {
		sm := e.resolveSparseModel(stmt.Model)
		sparseModel = &sm
	}

	var qdrantFilter *qdrant.Filter
	if stmt.QueryFilter != nil {
		var err error
		qdrantFilter, err = filters.NewFilterConverter().BuildFilter(stmt.QueryFilter)
		if err != nil {
			return nil, err
		}
	}

	state := &pipeline.QueryState{
		Embedder:          e,
		LocalEmbed:        e.usesLocalEmbeddings(),
		Params:            pipeline.BuildSearchParams(stmt.WithClause),
		HasMMR:            hasMMR(stmt.WithClause),
		CloudModelOptions: e.cloudModelOptions(),
		DenseModel:        denseModel,
		CollectionName:    stmt.Collection,
		Limit:             uint64(stmt.Limit),
		Offset:            uint64(stmt.Offset),
		QdrantFilter:      qdrantFilter,
		RequestTimeout:    e.requestTimeout(),
	}
	if stmt.WithClause != nil {
		if stmt.WithClause.RrfK != nil || len(stmt.WithClause.RrfWeights) > 0 {
			state.FusionConfig = &qdrant.Rrf{}
			if stmt.WithClause.RrfK != nil {
				state.FusionConfig.K = qdrant.PtrOf(uint32(*stmt.WithClause.RrfK))
			}
			if len(stmt.WithClause.RrfWeights) > 0 {
				state.FusionConfig.Weights = stmt.WithClause.RrfWeights
			}
		}
	}
	if stmt.ScoreThreshold != nil {
		state.ScoreThreshold = qdrant.PtrOf(float32(*stmt.ScoreThreshold))
	}
	if stmt.LookupFrom != "" {
		state.LookupFrom = &qdrant.LookupLocation{
			CollectionName: stmt.LookupFrom,
			VectorName:     stmt.LookupVector,
		}
	}
	if stmt.WithPayload != nil {
		state.WithPayload = buildWithPayload(stmt.WithPayload)
	}
	if stmt.WithVectors != nil {
		state.WithVectors = buildWithVectors(stmt.WithVectors)
	}
	if stmt.GroupBy != nil {
		state.GroupBy = *stmt.GroupBy
	}
	if stmt.GroupSize != nil {
		state.GroupSize = uint64(*stmt.GroupSize)
	}
	if stmt.WithLookupCollection != nil {
		state.WithLookup = &qdrant.WithLookup{
			Collection: *stmt.WithLookupCollection,
		}
	}
	if state.HasMMR {
		if stmt.WithClause.MmrDiversity != nil {
			state.MmrDiversity = float32(*stmt.WithClause.MmrDiversity)
		}
		if stmt.WithClause.MmrCandidates != nil {
			state.MmrCandidates = uint32(*stmt.WithClause.MmrCandidates)
		}
	}

	execPipeline := pipeline.NewQueryPipeline()

	switch stmt.Mode {
	case ast.QueryModeOrderBy:
		asc := true
		if stmt.OrderByAsc != nil {
			asc = *stmt.OrderByAsc
		}
		execPipeline.Add(&pipeline.OrderByNode{Field: *stmt.OrderByField, Asc: asc})
	case ast.QueryModeSample:
		execPipeline.Add(&pipeline.SampleNode{})

	case ast.QueryModeRelevanceFeedback:
		feedback := make([]struct {
			Example any
			Score   float64
		}, len(stmt.FeedbackItems))
		for i, item := range stmt.FeedbackItems {
			feedback[i] = struct {
				Example any
				Score   float64
			}{Example: item.Example, Score: item.Score}
		}
		node := &pipeline.RelevanceFeedbackNode{
			Target:   stmt.FeedbackTarget,
			Feedback: feedback,
		}
		if stmt.FeedbackStrategy != nil {
			node.Strategy = &struct{ A, B, C float64 }{
				A: stmt.FeedbackStrategy.A,
				B: stmt.FeedbackStrategy.B,
				C: stmt.FeedbackStrategy.C,
			}
		}
		execPipeline.Add(node)

	case ast.QueryModeNearest:
		if len(stmt.RawVector) > 0 {
			if stmt.Type == ast.QueryTypeHybrid {
				if stmt.QueryText == nil {
					return nil, fmt.Errorf("USING HYBRID with a raw dense vector requires a text query for the sparse vector")
				}
				execPipeline.Add(&pipeline.RawVectorNode{Vector: stmt.RawVector, VectorName: denseVectorName, Limit: uint64(stmt.Limit) * 10, AsPrefetch: true})
				execPipeline.Add(&pipeline.SparseEmbedNode{Model: *sparseModel, VectorName: sparseVectorName, Limit: uint64(stmt.Limit) * 10, AsPrefetch: true})
				if stmt.FusionType != nil {
					execPipeline.Add(&pipeline.FusionNode{Mode: strings.ToLower(*stmt.FusionType)})
				} else {
					execPipeline.Add(&pipeline.FusionNode{Mode: "rrf"})
				}
			} else {
				execPipeline.Add(&pipeline.RawVectorNode{Vector: stmt.RawVector, VectorName: denseVectorName})
				if stmt.FusionType != nil {
					execPipeline.Add(&pipeline.FusionNode{Mode: strings.ToLower(*stmt.FusionType)})
					state.VectorName = ""
				}
			}
		} else if stmt.QueryID != nil {
			execPipeline.Add(&pipeline.RecommendNode{PositiveIDs: []any{stmt.QueryID}})
		} else {
			if stmt.QueryText != nil {
				state.QueryText = *stmt.QueryText
			}
			switch stmt.Type {
			case ast.QueryTypeHybrid:
				execPipeline.Add(&pipeline.DenseEmbedNode{Model: denseModel, VectorName: denseVectorName, Limit: uint64(stmt.Limit) * 10, AsPrefetch: true})
				execPipeline.Add(&pipeline.SparseEmbedNode{Model: *sparseModel, VectorName: sparseVectorName, Limit: uint64(stmt.Limit) * 10, AsPrefetch: true})
				if stmt.FusionType != nil {
					execPipeline.Add(&pipeline.FusionNode{Mode: strings.ToLower(*stmt.FusionType)})
				} else {
					execPipeline.Add(&pipeline.FusionNode{Mode: "rrf"})
				}
			case ast.QueryTypeSparse:
				execPipeline.Add(&pipeline.SparseEmbedNode{Model: e.resolveSparseModel(stmt.Model), VectorName: sparseVectorName, Limit: uint64(stmt.Limit)})
				state.VectorName = sparseVectorName
			default:
				if stmt.QueryText != nil {
					execPipeline.Add(&pipeline.DenseEmbedNode{Model: denseModel, VectorName: denseVectorName, Limit: uint64(stmt.Limit)})
					state.VectorName = denseVectorName
				}
			}
			if stmt.FusionType != nil && stmt.Type != ast.QueryTypeHybrid {
				execPipeline.Add(&pipeline.FusionNode{Mode: strings.ToLower(*stmt.FusionType)})
				state.VectorName = ""
			}
		}
	case ast.QueryModeRecommend:
		execPipeline.Add(&pipeline.RecommendNode{PositiveIDs: stmt.PositiveIDs, NegativeIDs: stmt.NegativeIDs, Strategy: stmt.Strategy})
		state.VectorName = denseVectorName
	case ast.QueryModeContext:
		pairs := make([]pipeline.ContextPair, len(stmt.ContextPairs))
		for i, p := range stmt.ContextPairs {
			pairs[i] = pipeline.ContextPair{Positive: p.Positive, Negative: p.Negative}
		}
		execPipeline.Add(&pipeline.ContextNode{Pairs: pairs})
		state.VectorName = denseVectorName
	case ast.QueryModeDiscover:
		pairs := make([]pipeline.ContextPair, len(stmt.ContextPairs))
		for i, p := range stmt.ContextPairs {
			pairs[i] = pipeline.ContextPair{Positive: p.Positive, Negative: p.Negative}
		}
		execPipeline.Add(&pipeline.DiscoverNode{Target: stmt.Target, Pairs: pairs})
		state.VectorName = denseVectorName
	}

	if stmt.Rerank {
		rerankModel := "default-reranker"
		if stmt.RerankModel != nil {
			rerankModel = *stmt.RerankModel
		}
		execPipeline.Add(&pipeline.RerankNode{Model: rerankModel})
	}

	if stmt.Formula != nil {
		execPipeline.Add(&pipeline.FormulaNode{Expr: stmt.Formula, Defaults: stmt.FormulaDefaults})
	}

	// Resolve CTEs into PrefetchQuery pointers (recursive)
	cteMap, err := e.resolveCTEs(ctx, stmt.CTEs)
	if err != nil {
		return nil, err
	}
	if len(stmt.PrefetchRefs) > 0 && len(cteMap) > 0 {
		for _, ref := range stmt.PrefetchRefs {
			pq, ok := cteMap[ref.CTEName]
			if !ok {
				return nil, fmt.Errorf("unknown CTE referenced in prefetch: '%s'", ref.CTEName)
			}
			if ref.Filter != nil || ref.ScoreThreshold != nil || ref.LookupFrom != "" {
				clone := &qdrant.PrefetchQuery{
					Prefetch:   pq.Prefetch,
					Query:      pq.Query,
					Using:      pq.Using,
					Filter:     pq.Filter,
					Params:     pq.Params,
					Limit:      pq.Limit,
					LookupFrom: pq.LookupFrom,
				}
				if ref.Filter != nil {
					f, err := filters.NewFilterConverter().BuildFilter(ref.Filter)
					if err != nil {
						return nil, fmt.Errorf("per-prefetch filter on '%s': %w", ref.CTEName, err)
					}
					clone.Filter = f
				}
				if ref.ScoreThreshold != nil {
					clone.ScoreThreshold = qdrant.PtrOf(float32(*ref.ScoreThreshold))
				}
				if ref.LookupFrom != "" {
					clone.LookupFrom = &qdrant.LookupLocation{
						CollectionName: ref.LookupFrom,
					}
					if ref.LookupVector != nil {
						clone.LookupFrom.VectorName = qdrant.PtrOf(*ref.LookupVector)
					}
				}
				state.ManualPrefetches = append(state.ManualPrefetches, clone)
			} else {
				state.ManualPrefetches = append(state.ManualPrefetches, pq)
			}
		}
	}

	return &queryBuildResult{state: state, pipeline: execPipeline, stmt: stmt}, nil
}

func (e *Executor) doQuery(stmt *ast.QueryStmt) (*ExecResponse, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()

	built, err := e.buildQueryStateAndPipeline(ctx, stmt)
	if err != nil {
		return nil, err
	}
	state := built.state
	execPipeline := built.pipeline

	if err := execPipeline.Execute(ctx, state); err != nil {
		return nil, err
	}

	if state.GroupBy != "" {
		return e.executeGroupedQuery(ctx, execPipeline, state)
	}
	return e.executeFlatQuery(ctx, execPipeline, state)
}

// BuildQueryPoints parses a QQL query and returns the QueryPoints request
// without executing it. Used by BatchQuery for native Qdrant batch operations.
func (e *Executor) BuildQueryPoints(ctx context.Context, query string) (*qdrant.QueryPoints, error) {
	node, err := parseQuery(query)
	if err != nil {
		return nil, err
	}
	return e.BuildQueryPointsNode(ctx, node)
}

// BuildQueryPointsNode builds QueryPoints directly from an AST node.
func (e *Executor) BuildQueryPointsNode(ctx context.Context, node ast.ASTNode) (*qdrant.QueryPoints, error) {
	stmt, ok := node.(*ast.QueryStmt)
	if !ok {
		return nil, fmt.Errorf("expected QUERY statement, got %T", node)
	}

	built, err := e.buildQueryStateAndPipeline(ctx, stmt)
	if err != nil {
		return nil, err
	}

	if err := built.pipeline.Execute(ctx, built.state); err != nil {
		return nil, err
	}

	return built.pipeline.BuildFlatRequest(built.state), nil
}

// resolveCTEs builds a map from CTE name to *qdrant.PrefetchQuery.
// CTEs are built in definition order and can only reference previously defined CTEs.
func (e *Executor) resolveCTEs(ctx context.Context, ctes []ast.CTE) (map[string]*qdrant.PrefetchQuery, error) {
	if len(ctes) == 0 {
		return nil, nil
	}

	built := make(map[string]*qdrant.PrefetchQuery, len(ctes))

	for _, cte := range ctes {
		pq, err := e.buildCTEPrefetch(ctx, cte.Stmt, built)
		if err != nil {
			return nil, fmt.Errorf("failed to build CTE '%s': %w", cte.Name, err)
		}
		built[cte.Name] = pq
	}
	return built, nil
}

// buildCTEPrefetch builds a single *qdrant.PrefetchQuery from a CTE's QueryStmt,
// resolving embedded PrefetchRefs against the (possibly partially built) CTE map.
func (e *Executor) buildCTEPrefetch(ctx context.Context, stmt *ast.QueryStmt, cteMap map[string]*qdrant.PrefetchQuery) (*qdrant.PrefetchQuery, error) {
	pq := &qdrant.PrefetchQuery{}

	denseModel := e.resolveDenseModel(stmt.Model)

	for _, ref := range stmt.PrefetchRefs {
		if nested, ok := cteMap[ref.CTEName]; ok {
			pq.Prefetch = append(pq.Prefetch, nested)
		} else {
			return nil, fmt.Errorf("unknown CTE referenced in prefetch: '%s'", ref.CTEName)
		}
	}

	if stmt.Using != nil {
		pq.Using = qdrant.PtrOf(*stmt.Using)
	}
	if stmt.Limit > 0 {
		pq.Limit = qdrant.PtrOf(uint64(stmt.Limit))
	}
	if stmt.ScoreThreshold != nil {
		pq.ScoreThreshold = qdrant.PtrOf(float32(*stmt.ScoreThreshold))
	}
	if stmt.LookupFrom != "" {
		pq.LookupFrom = &qdrant.LookupLocation{
			CollectionName: stmt.LookupFrom,
		}
		if stmt.LookupVector != nil {
			pq.LookupFrom.VectorName = qdrant.PtrOf(*stmt.LookupVector)
		}
	}
	if stmt.QueryFilter != nil {
		filter, err := filters.NewFilterConverter().BuildFilter(stmt.QueryFilter)
		if err != nil {
			return nil, err
		}
		pq.Filter = filter
	}
	if stmt.WithClause != nil {
		pq.Params = pipeline.BuildSearchParams(stmt.WithClause)
	}

	switch stmt.Mode {
	case ast.QueryModeRecommend:
		pos := make([]*qdrant.VectorInput, len(stmt.PositiveIDs))
		for i, id := range stmt.PositiveIDs {
			pid, _ := buildPointID(id)
			pos[i] = qdrant.NewVectorInputID(pid)
		}
		neg := make([]*qdrant.VectorInput, len(stmt.NegativeIDs))
		for i, id := range stmt.NegativeIDs {
			pid, _ := buildPointID(id)
			neg[i] = qdrant.NewVectorInputID(pid)
		}
		pq.Query = qdrant.NewQueryRecommend(&qdrant.RecommendInput{
			Positive: pos,
			Negative: neg,
		})

	case ast.QueryModeNearest:
		if stmt.Type == ast.QueryTypeHybrid {
			return nil, fmt.Errorf("USING HYBRID is not supported inside CTE prefetch queries; define separate sparse and dense CTEs and combine them via prefetch references")
		}

		if len(stmt.RawVector) > 0 {
			raw := make([]float32, len(stmt.RawVector))
			for i, v := range stmt.RawVector {
				raw[i] = float32(v)
			}
			pq.Query = qdrant.NewQueryDense(raw)
		} else if stmt.QueryText != nil {
			isSparse := stmt.Type == ast.QueryTypeSparse
			if isSparse {
				if e.usesLocalEmbeddings() {
					indices, values, err := e.EmbedSparse(ctx, *stmt.QueryText)
					if err != nil {
						return nil, err
					}
					pq.Query = qdrant.NewQuerySparse(indices, values)
				} else {
					pq.Query = qdrant.NewQueryDocument(&qdrant.Document{
						Text:    *stmt.QueryText,
						Model:   e.resolveSparseModel(stmt.Model),
						Options: buildDocumentOptionsFromMap(e.cloudModelOptions()),
					})
				}
			} else {
				if e.usesLocalEmbeddings() {
					dense, err := e.EmbedDense(ctx, *stmt.QueryText, denseModel)
					if err != nil {
						return nil, err
					}
					pq.Query = qdrant.NewQueryDense(dense)
				} else {
					pq.Query = qdrant.NewQueryDocument(&qdrant.Document{
						Text:    *stmt.QueryText,
						Model:   denseModel,
						Options: buildDocumentOptionsFromMap(e.cloudModelOptions()),
					})
				}
			}
		} else if stmt.QueryID != nil {
			pid, _ := buildPointID(stmt.QueryID)
			pq.Query = qdrant.NewQueryNearest(qdrant.NewVectorInputID(pid))
		}
	}

	return pq, nil
}

// buildPointID converts a point ID value (string or int) to *qdrant.PointId.
func buildPointID(val any) (*qdrant.PointId, error) {
	switch v := val.(type) {
	case string:
		return qdrant.NewIDUUID(v), nil
	case int:
		return qdrant.NewIDNum(uint64(v)), nil
	case uint64:
		return qdrant.NewIDNum(v), nil
	case float64:
		return qdrant.NewIDNum(uint64(v)), nil
	default:
		return nil, fmt.Errorf("unsupported point id type %T", val)
	}
}

func (e *Executor) executeFlatQuery(ctx context.Context, p *pipeline.QueryPipeline, state *pipeline.QueryState) (*ExecResponse, error) {
	req := p.BuildFlatRequest(state)
	// Always request payload — users want to see the data.
	if req.WithPayload == nil {
		req.WithPayload = qdrant.NewWithPayload(true)
	}
	results, err := e.client.Query(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query on qdrant: %w", err)
	}

	formatted := make([]SearchHit, len(results))
	for i, hit := range results {
		formatted[i] = SearchHit{
			ID:    pointIDString(hit.Id),
			Score: hit.Score,
		}
		if textVal, ok := hit.Payload["text"]; ok {
			formatted[i].Text = textVal.GetStringValue()
		}
		// Include full payload.
		if hit.Payload != nil {
			payload := make(map[string]any, len(hit.Payload))
			for k, v := range hit.Payload {
				payload[k] = qdrantValueToAny(v)
			}
			formatted[i].Payload = payload
		}
	}

	return &ExecResponse{
		OK:        true,
		Operation: "QUERY",
		Message:   fmt.Sprintf("Found %d hits", len(formatted)),
		Data:      formatted,
	}, nil
}

func (e *Executor) executeGroupedQuery(ctx context.Context, p *pipeline.QueryPipeline, state *pipeline.QueryState) (*ExecResponse, error) {
	req := p.BuildGroupedRequest(state)
	// Always request payload.
	if req.WithPayload == nil {
		req.WithPayload = qdrant.NewWithPayload(true)
	}
	groups, err := e.client.QueryGroups(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to query groups on qdrant: %w", err)
	}

	formatted := make([]GroupedSearchResult, len(groups))
	for i, g := range groups {
		hits := make([]SearchHit, len(g.Hits))
		for j, hit := range g.Hits {
			hits[j] = SearchHit{
				ID:    pointIDString(hit.Id),
				Score: hit.Score,
			}
			if textVal, ok := hit.Payload["text"]; ok {
				hits[j].Text = textVal.GetStringValue()
			}
			if hit.Payload != nil {
				payload := make(map[string]any, len(hit.Payload))
				for k, v := range hit.Payload {
					payload[k] = qdrantValueToAny(v)
				}
				hits[j].Payload = payload
			}
		}
		formatted[i] = GroupedSearchResult{
			GroupID: groupIDString(g.Id),
			Hits:    hits,
		}
	}

	return &ExecResponse{
		OK:        true,
		Operation: "QUERY_GROUPS",
		Message:   fmt.Sprintf("Found %d groups", len(formatted)),
		Data:      formatted,
	}, nil
}

// qdrantValueToAny converts a qdrant.Value to a plain Go value for JSON serialization.
func qdrantValueToAny(v *qdrant.Value) any {
	if v == nil {
		return nil
	}
	switch val := v.Kind.(type) {
	case *qdrant.Value_StringValue:
		return val.StringValue
	case *qdrant.Value_IntegerValue:
		return val.IntegerValue
	case *qdrant.Value_DoubleValue:
		return val.DoubleValue
	case *qdrant.Value_BoolValue:
		return val.BoolValue
	case *qdrant.Value_ListValue:
		arr := make([]any, len(val.ListValue.Values))
		for i, item := range val.ListValue.Values {
			arr[i] = qdrantValueToAny(item)
		}
		return arr
	case *qdrant.Value_StructValue:
		m := make(map[string]any, len(val.StructValue.Fields))
		for k, item := range val.StructValue.Fields {
			m[k] = qdrantValueToAny(item)
		}
		return m
	case *qdrant.Value_NullValue:
		return nil
	default:
		return fmt.Sprintf("%v", v)
	}
}

func buildWithPayload(sel *ast.PayloadSelector) *qdrant.WithPayloadSelector {
	if sel == nil {
		return nil
	}
	if sel.Enable != nil {
		return qdrant.NewWithPayload(*sel.Enable)
	}
	if len(sel.Include) > 0 {
		return qdrant.NewWithPayloadInclude(sel.Include...)
	}
	if len(sel.Exclude) > 0 {
		return qdrant.NewWithPayloadExclude(sel.Exclude...)
	}
	return nil
}

func buildWithVectors(sel *ast.VectorsSelector) *qdrant.WithVectorsSelector {
	if sel == nil {
		return nil
	}
	if sel.Enable != nil {
		return qdrant.NewWithVectors(*sel.Enable)
	}
	if len(sel.Vectors) > 0 {
		return qdrant.NewWithVectorsInclude(sel.Vectors...)
	}
	return nil
}
