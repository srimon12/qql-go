package commands

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/filters"
	"github.com/srimon12/qql-go/internal/pipeline"
)

func (e *Executor) doQuery(stmt *ast.QueryStmt) (*ExecResponse, error) {
	ctx := context.Background()

	// 1. Resolve embedding options
	denseVectorName := ""
	sparseVectorName := ""
	if stmt.Using != nil {
		denseVectorName = *stmt.Using
		sparseVectorName = *stmt.Using
	}

	denseModel := ""
	if stmt.Model != nil {
		denseModel = *stmt.Model
	}
	var sparseModel *string
	if stmt.Type == ast.QueryTypeHybrid {
		sparseModelStr := denseModel + "-sparse"
		if denseModel == "" {
			sparseModelStr = ""
		}
		sparseModel = &sparseModelStr
	}

	// 2. Build filter (pure AST→protobuf conversion, no I/O)
	var qdrantFilter *qdrant.Filter
	if stmt.QueryFilter != nil {
		var err error
		qdrantFilter, err = filters.NewFilterConverter().BuildFilter(stmt.QueryFilter)
		if err != nil {
			return nil, err
		}
	}

	// 3. Populate QueryState with ALL request-level fields
	state := &pipeline.QueryState{
		Embedder:          e,
		LocalEmbed:        e.usesLocalEmbeddings(),
		Params:            searchParamsFromWithClause(stmt.WithClause),
		HasMMR:            hasMMR(stmt.WithClause),
		CloudModelOptions: e.cloudModelOptions(),
		CollectionName:    stmt.Collection,
		Limit:             uint64(stmt.Limit),
		Offset:            uint64(stmt.Offset),
		QdrantFilter:      qdrantFilter,
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
	if stmt.GroupBy != nil {
		state.GroupBy = *stmt.GroupBy
	}
	if stmt.GroupSize != nil {
		state.GroupSize = uint64(*stmt.GroupSize)
	}
	if state.HasMMR {
		if stmt.WithClause.MmrDiversity != nil {
			state.MmrDiversity = float32(*stmt.WithClause.MmrDiversity)
		}
		if stmt.WithClause.MmrCandidates != nil {
			state.MmrCandidates = uint32(*stmt.WithClause.MmrCandidates)
		}
	}

	// 4. Build & execute pipeline (only embeds transform; request assembly is in BuildXxxRequest)
	execPipeline := pipeline.NewQueryPipeline()

	switch stmt.Mode {
	case ast.QueryModeNearest:
		if stmt.QueryID != nil {
			execPipeline.Add(&pipeline.RecommendNode{PositiveIDs: []any{stmt.QueryID}})
		} else {
			if stmt.QueryText != nil {
				state.QueryText = *stmt.QueryText
			}
			switch stmt.Type {
			case ast.QueryTypeHybrid:
				execPipeline.Add(&pipeline.DenseEmbedNode{
					Model:      denseModel,
					VectorName: denseVectorName,
					Limit:      uint64(stmt.Limit) * 10,
					AsPrefetch: true,
				})
				execPipeline.Add(&pipeline.SparseEmbedNode{
					Model:      *sparseModel,
					VectorName: sparseVectorName,
					Limit:      uint64(stmt.Limit) * 10,
					AsPrefetch: true,
				})
				fusionMode := "rrf"
				if stmt.Fusion != nil {
					fusionMode = *stmt.Fusion
				}
				execPipeline.Add(&pipeline.FusionNode{Mode: fusionMode})
			case ast.QueryTypeSparse:
				execPipeline.Add(&pipeline.SparseEmbedNode{
					Model:      denseModel,
					VectorName: sparseVectorName,
					Limit:      uint64(stmt.Limit),
				})
			default:
				execPipeline.Add(&pipeline.DenseEmbedNode{
					Model:      denseModel,
					VectorName: denseVectorName,
					Limit:      uint64(stmt.Limit),
				})
			}
		}

	case ast.QueryModeRecommend:
		execPipeline.Add(&pipeline.RecommendNode{
			PositiveIDs: stmt.PositiveIDs,
			NegativeIDs: stmt.NegativeIDs,
			Strategy:    stmt.Strategy,
		})

	case ast.QueryModeContext:
		pairs := make([]pipeline.ContextPair, len(stmt.ContextPairs))
		for i, p := range stmt.ContextPairs {
			pairs[i] = pipeline.ContextPair{Positive: p.Positive, Negative: p.Negative}
		}
		execPipeline.Add(&pipeline.ContextNode{Pairs: pairs})

	case ast.QueryModeDiscover:
		pairs := make([]pipeline.ContextPair, len(stmt.ContextPairs))
		for i, p := range stmt.ContextPairs {
			pairs[i] = pipeline.ContextPair{Positive: p.Positive, Negative: p.Negative}
		}
		execPipeline.Add(&pipeline.DiscoverNode{Target: stmt.Target, Pairs: pairs})
	}

	if stmt.Rerank {
		rerankModel := "default-reranker"
		if stmt.RerankModel != nil {
			rerankModel = *stmt.RerankModel
		}
		execPipeline.Add(&pipeline.RerankNode{Model: rerankModel})
	}

	if err := execPipeline.Execute(ctx, state); err != nil {
		return nil, err
	}

	// 5. Pipeline owns request construction — executor just calls Qdrant
	if state.GroupBy != "" {
		return e.executeGroupedQuery(ctx, execPipeline, state)
	}
	return e.executeFlatQuery(ctx, execPipeline, state)
}

func (e *Executor) executeFlatQuery(ctx context.Context, p *pipeline.QueryPipeline, state *pipeline.QueryState) (*ExecResponse, error) {
	req := p.BuildFlatRequest(state)
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
