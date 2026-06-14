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

	// 1. Resolve Embedding Options
	denseVectorName := ""
	if stmt.Using != nil {
		denseVectorName = *stmt.Using
	}
	sparseVectorName := ""
	if stmt.Hybrid {
		sparseVectorName = denseVectorName
	}
	
	denseModel := ""
	if stmt.Model != nil {
		denseModel = *stmt.Model
	}
	var sparseModel *string
	if stmt.Type == ast.QueryTypeHybrid {
		sparseModelStr := denseModel
		if denseModel != "" {
			sparseModelStr += "-sparse" // Default inference convention
		}
		sparseModel = &sparseModelStr
	}

	state := &pipeline.QueryState{
		Embedder:          e,
		LocalEmbed:        e.usesLocalEmbeddings(),
		Params:            searchParamsFromWithClause(stmt.WithClause),
		HasMMR:            hasMMR(stmt.WithClause),
		CloudModelOptions: e.cloudModelOptions(),
	}
	if state.HasMMR {
		if stmt.WithClause.MmrDiversity != nil {
			state.MmrDiversity = float32(*stmt.WithClause.MmrDiversity)
		}
		if stmt.WithClause.MmrCandidates != nil {
			state.MmrCandidates = uint32(*stmt.WithClause.MmrCandidates)
		}
	}

	// 2. Build Pipeline
	execPipeline := pipeline.NewQueryPipeline()

	switch stmt.Mode {
	case ast.QueryModeNearest:
		if stmt.QueryID != nil {
			// For ID-based NEAREST, we use RecommendNode with single positive
			execPipeline.Add(&pipeline.RecommendNode{
				PositiveIDs: []any{stmt.QueryID},
			})
		} else {
			if stmt.QueryText != nil {
				state.QueryText = *stmt.QueryText
			}

			if stmt.Type == ast.QueryTypeHybrid {
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
			} else if stmt.Type == ast.QueryTypeSparse {
				execPipeline.Add(&pipeline.SparseEmbedNode{
					Model:      denseModel,
					VectorName: sparseVectorName,
					Limit:      uint64(stmt.Limit),
					AsPrefetch: false,
				})
			} else {
				execPipeline.Add(&pipeline.DenseEmbedNode{
					Model:      denseModel,
					VectorName: denseVectorName,
					Limit:      uint64(stmt.Limit),
					AsPrefetch: false,
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
			pairs[i] = pipeline.ContextPair{
				Positive: p.Positive,
				Negative: p.Negative,
			}
		}
		execPipeline.Add(&pipeline.ContextNode{Pairs: pairs})

	case ast.QueryModeDiscover:
		pairs := make([]pipeline.ContextPair, len(stmt.ContextPairs))
		for i, p := range stmt.ContextPairs {
			pairs[i] = pipeline.ContextPair{
				Positive: p.Positive,
				Negative: p.Negative,
			}
		}
		execPipeline.Add(&pipeline.DiscoverNode{
			Target: stmt.Target,
			Pairs:  pairs,
		})
	}

	if stmt.Rerank {
		rerankModel := "default-reranker"
		if stmt.RerankModel != nil {
			rerankModel = *stmt.RerankModel
		}
		execPipeline.Add(&pipeline.RerankNode{
			Model: rerankModel,
			Limit: uint64(stmt.Limit),
		})
	}

	// 3. Execute Pipeline
	if err := execPipeline.Execute(ctx, state); err != nil {
		return nil, err
	}

	// 4. Build Request
	req := &qdrant.QueryPoints{
		CollectionName: stmt.Collection,
		Query:          state.TargetQuery,
		Prefetch:       state.Prefetches,
		Limit:          qdrant.PtrOf(uint64(stmt.Limit)),
		Offset:         qdrant.PtrOf(uint64(stmt.Offset)),
		Params:         state.Params,
	}
	
	if stmt.QueryFilter != nil {
		filter, err := filters.NewFilterConverter().BuildFilter(stmt.QueryFilter)
		if err != nil {
			return nil, err
		}
		req.Filter = filter
	}
	
	if stmt.ScoreThreshold != nil {
		req.ScoreThreshold = qdrant.PtrOf(float32(*stmt.ScoreThreshold))
	}
	
	if stmt.LookupFrom != "" {
		req.LookupFrom = &qdrant.LookupLocation{
			CollectionName: stmt.LookupFrom,
			VectorName:     stmt.LookupVector,
		}
	}

	if stmt.GroupBy != nil {
		groupReq := &qdrant.QueryPointGroups{
			CollectionName: stmt.Collection,
			Query:          state.TargetQuery,
			Prefetch:       state.Prefetches,
			Limit:          qdrant.PtrOf(uint64(stmt.Limit)),
			GroupBy:        *stmt.GroupBy,
			Filter:         req.Filter,
			ScoreThreshold: req.ScoreThreshold,
			LookupFrom:     req.LookupFrom,
			Params:         req.Params,
		}
		if stmt.GroupSize != nil {
			groupReq.GroupSize = qdrant.PtrOf(uint64(*stmt.GroupSize))
		}
		
		groups, err := e.client.QueryGroups(ctx, groupReq)
		if err != nil {
			return nil, fmt.Errorf("failed to query groups from qdrant: %w", err)
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
