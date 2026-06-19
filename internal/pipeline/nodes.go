package pipeline

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
)

type DenseEmbedNode struct {
	Model      string
	VectorName string
	Limit      uint64
	AsPrefetch bool
}

func (n *DenseEmbedNode) Execute(ctx context.Context, state *QueryState) error {
	var query *qdrant.Query
	var mmrNearest *qdrant.VectorInput

	if state.LocalEmbed {
		if state.Embedder == nil {
			return fmt.Errorf("local embedding requested but no Embedder provided")
		}
		denseVector, err := state.Embedder.EmbedDense(ctx, state.QueryText, n.Model)
		if err != nil {
			return fmt.Errorf("failed to embed dense search query: %w", err)
		}
		query = qdrant.NewQueryDense(denseVector)
		if state.HasMMR {
			mmrNearest = qdrant.NewVectorInputDense(denseVector)
		}
	} else {
		doc := &qdrant.Document{
			Text:    state.QueryText,
			Model:   n.Model,
			Options: state.GetDocOptions(),
		}
		query = qdrant.NewQueryDocument(doc)
		if state.HasMMR {
			mmrNearest = qdrant.NewVectorInputDocument(doc)
		}
	}

	if state.HasMMR {
		query = qdrant.NewQueryMMR(mmrNearest, &qdrant.Mmr{
			Diversity:       qdrant.PtrOf(state.MmrDiversity),
			CandidatesLimit: qdrant.PtrOf(state.MmrCandidates),
		})
	}

	if n.AsPrefetch {
		state.Prefetches = append(state.Prefetches, &qdrant.PrefetchQuery{
			Query:  query,
			Using:  qdrant.PtrOf(n.VectorName),
			Limit:  qdrant.PtrOf(n.Limit),
			Params: state.Params,
		})
	} else {
		state.TargetQuery = query
	}
	return nil
}

type RawVectorNode struct {
	Vector     []float64
	VectorName string
	AsPrefetch bool
	Limit      uint64
}

func (n *RawVectorNode) Execute(_ context.Context, state *QueryState) error {
	raw := make([]float32, len(n.Vector))
	for i, v := range n.Vector {
		raw[i] = float32(v)
	}
	query := qdrant.NewQueryDense(raw)

	if n.AsPrefetch {
		state.Prefetches = append(state.Prefetches, &qdrant.PrefetchQuery{
			Query:  query,
			Using:  qdrant.PtrOf(n.VectorName),
			Limit:  qdrant.PtrOf(n.Limit),
			Params: state.Params,
		})
	} else {
		state.TargetQuery = query
		if n.VectorName != "" {
			state.VectorName = n.VectorName
		}
	}
	return nil
}

type SparseEmbedNode struct {
	Model      string
	VectorName string
	Limit      uint64
	AsPrefetch bool
}

func (n *SparseEmbedNode) Execute(ctx context.Context, state *QueryState) error {
	if state.HasMMR && !n.AsPrefetch {
		return fmt.Errorf("MMR is supported only for standard NEAREST queries, not sparse-only queries")
	}

	var query *qdrant.Query

	if state.LocalEmbed {
		if state.Embedder == nil {
			return fmt.Errorf("local embedding requested but no Embedder provided")
		}
		indices, values, err := state.Embedder.EmbedSparse(ctx, state.QueryText)
		if err != nil {
			return fmt.Errorf("failed to embed sparse search query: %w", err)
		}
		query = qdrant.NewQuerySparse(indices, values)
	} else {
		doc := &qdrant.Document{
			Text:    state.QueryText,
			Model:   n.Model,
			Options: state.GetDocOptions(),
		}
		query = qdrant.NewQueryDocument(doc)
	}

	if n.AsPrefetch {
		state.Prefetches = append(state.Prefetches, &qdrant.PrefetchQuery{
			Query:  query,
			Using:  qdrant.PtrOf(n.VectorName),
			Limit:  qdrant.PtrOf(n.Limit),
			Params: state.Params,
		})
	} else {
		state.TargetQuery = query
	}
	return nil
}

type FusionNode struct {
	Mode string // rrf or dbsf
}

func (n *FusionNode) Execute(ctx context.Context, state *QueryState) error {
	if n.Mode == "rrf" && state.FusionConfig != nil {
		state.TargetQuery = qdrant.NewQueryRRF(state.FusionConfig)
		return nil
	}

	switch n.Mode {
	case "rrf":
		state.TargetQuery = qdrant.NewQueryFusion(qdrant.Fusion_RRF)
	case "dbsf":
		state.TargetQuery = qdrant.NewQueryFusion(qdrant.Fusion_DBSF)
	default:
		return fmt.Errorf("unknown fusion mode '%s'; expected 'rrf' or 'dbsf'", n.Mode)
	}
	return nil
}

type RerankNode struct {
	Model string
}

func (n *RerankNode) Execute(ctx context.Context, state *QueryState) error {
	if state.LocalEmbed {
		return fmt.Errorf("RERANK is currently only available in cloud inference mode")
	}

	state.TargetQuery = qdrant.NewQueryDocument(&qdrant.Document{
		Text:    state.QueryText,
		Model:   n.Model,
		Options: state.GetDocOptions(),
	})
	return nil
}

// RecommendNode handles building a QueryRecommend
type RecommendNode struct {
	PositiveIDs []any
	NegativeIDs []any
	Strategy    *string
}

func (n *RecommendNode) Execute(ctx context.Context, state *QueryState) error {
	if state.HasMMR {
		return fmt.Errorf("MMR is supported only for standard NEAREST queries")
	}
	if len(n.PositiveIDs) == 0 && len(n.NegativeIDs) == 0 {
		return fmt.Errorf("RECOMMEND requires at least one POSITIVE or NEGATIVE ID")
	}

	var pos []*qdrant.VectorInput
	for _, id := range n.PositiveIDs {
		vi, err := buildVectorInput(ctx, state, id)
		if err != nil {
			return err
		}
		pos = append(pos, vi)
	}
	var neg []*qdrant.VectorInput
	for _, id := range n.NegativeIDs {
		vi, err := buildVectorInput(ctx, state, id)
		if err != nil {
			return err
		}
		neg = append(neg, vi)
	}

	rec := &qdrant.RecommendInput{
		Positive: pos,
		Negative: neg,
	}
	if n.Strategy != nil && *n.Strategy != "" {
		strategy, ok := RecommendStrategy(*n.Strategy)
		if !ok {
			return fmt.Errorf("unknown recommend strategy '%s'", *n.Strategy)
		}
		rec.Strategy = strategy.Enum()
	}
	state.TargetQuery = qdrant.NewQueryRecommend(rec)
	return nil
}

func RecommendStrategy(value string) (qdrant.RecommendStrategy, bool) {
	switch strings.ToLower(value) {
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

// ContextPair represents a single positive/negative pair for Context & Discover queries
type ContextPair struct {
	Positive any
	Negative any
}

// ContextNode handles building a QueryContext
type ContextNode struct {
	Pairs []ContextPair
}

func (n *ContextNode) buildPairs(ctx context.Context, state *QueryState) ([]*qdrant.ContextInputPair, error) {
	var pairs []*qdrant.ContextInputPair
	for _, p := range n.Pairs {
		pair := &qdrant.ContextInputPair{}
		if p.Positive != nil {
			vi, err := buildVectorInput(ctx, state, p.Positive)
			if err != nil {
				return nil, err
			}
			pair.Positive = vi
		}
		if p.Negative != nil {
			vi, err := buildVectorInput(ctx, state, p.Negative)
			if err != nil {
				return nil, err
			}
			pair.Negative = vi
		}
		pairs = append(pairs, pair)
	}
	return pairs, nil
}

func (n *ContextNode) Execute(ctx context.Context, state *QueryState) error {
	pairs, err := n.buildPairs(ctx, state)
	if err != nil {
		return err
	}
	state.TargetQuery = qdrant.NewQueryContext(&qdrant.ContextInput{Pairs: pairs})
	return nil
}

// DiscoverNode handles building a QueryDiscover
type DiscoverNode struct {
	Target any
	Pairs  []ContextPair
}

func (n *DiscoverNode) Execute(ctx context.Context, state *QueryState) error {
	target, err := buildVectorInput(ctx, state, n.Target)
	if err != nil {
		return err
	}
	// We can reuse ContextNode logic for pairs
	ctxNode := &ContextNode{Pairs: n.Pairs}
	pairs, err := ctxNode.buildPairs(ctx, state)
	if err != nil {
		return err
	}

	state.TargetQuery = qdrant.NewQueryDiscover(&qdrant.DiscoverInput{
		Target:  target,
		Context: &qdrant.ContextInput{Pairs: pairs},
	})
	return nil
}

func isUUID(u string) bool {
	if len(u) != 36 {
		return false
	}
	for i, c := range u {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// ToPointID converts any point ID value (string, int, float, etc.) to *qdrant.PointId.
func ToPointID(val any) (*qdrant.PointId, error) {
	switch v := val.(type) {
	case string:
		if num, err := strconv.ParseUint(v, 10, 64); err == nil {
			return qdrant.NewIDNum(num), nil
		}
		return qdrant.NewIDUUID(v), nil
	case int:
		if v < 0 {
			return nil, fmt.Errorf("unsupported vector input type: negative integer")
		}
		return qdrant.NewIDNum(uint64(v)), nil
	case int64:
		if v < 0 {
			return nil, fmt.Errorf("unsupported vector input type: negative integer")
		}
		return qdrant.NewIDNum(uint64(v)), nil
	case uint64:
		return qdrant.NewIDNum(v), nil
	case float64:
		// Prevent precision loss silently passing the check
		if v < 0 || v > float64(1<<53) || v != float64(uint64(v)) {
			return nil, fmt.Errorf("unsupported vector input type: non-integer or oversized float")
		}
		return qdrant.NewIDNum(uint64(v)), nil
	case []float32: // In case of raw vectors
		return nil, fmt.Errorf("cannot use raw vector as PointId")
	default:
		return nil, fmt.Errorf("unsupported vector input type: %T", val)
	}
}

func buildVectorInput(ctx context.Context, state *QueryState, val any) (*qdrant.VectorInput, error) {
	if s, ok := val.(string); ok && !isUUID(s) {
		if _, err := strconv.ParseUint(s, 10, 64); err != nil {
			if state.LocalEmbed {
				if state.Embedder == nil {
					return nil, fmt.Errorf("local embedding requested but no Embedder provided")
				}
				denseVector, err := state.Embedder.EmbedDense(ctx, s, state.DenseModel)
				if err != nil {
					return nil, fmt.Errorf("failed to embed target query: %w", err)
				}
				return qdrant.NewVectorInputDense(denseVector), nil
			}
			return qdrant.NewVectorInputDocument(&qdrant.Document{
				Text:    s,
				Model:   state.DenseModel,
				Options: state.GetDocOptions(),
			}), nil
		}
	}
	if v, ok := val.([]float32); ok {
		return qdrant.NewVectorInputDense(v), nil
	}
	pid, err := ToPointID(val)
	if err != nil {
		return nil, err
	}
	return qdrant.NewVectorInputID(pid), nil
}

func BuildSearchParams(withClause *ast.SearchWith) *qdrant.SearchParams {
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

type OrderByNode struct {
	Field string
	Asc   bool
}

func (n *OrderByNode) Execute(ctx context.Context, state *QueryState) error {
	dir := qdrant.Direction_Asc
	if !n.Asc {
		dir = qdrant.Direction_Desc
	}
	state.TargetQuery = qdrant.NewQueryOrderBy(&qdrant.OrderBy{
		Key:       n.Field,
		Direction: &dir,
	})
	return nil
}

type SampleNode struct{}

func (n *SampleNode) Execute(ctx context.Context, state *QueryState) error {
	state.TargetQuery = qdrant.NewQuerySample(qdrant.Sample_Random)
	return nil
}

type RelevanceFeedbackNode struct {
	Target   any
	Feedback []struct {
		Example any
		Score   float64
	}
	Strategy *struct {
		A, B, C float64
	}
}

func (n *RelevanceFeedbackNode) Execute(ctx context.Context, state *QueryState) error {
	targetInput, err := buildVectorInputFromValue(ctx, state, n.Target)
	if err != nil {
		return fmt.Errorf("relevance feedback target: %w", err)
	}

	feedbackItems := make([]*qdrant.FeedbackItem, len(n.Feedback))
	for i, f := range n.Feedback {
		exampleInput, err := buildVectorInputFromValue(ctx, state, f.Example)
		if err != nil {
			return fmt.Errorf("relevance feedback example %d: %w", i, err)
		}
		feedbackItems[i] = &qdrant.FeedbackItem{
			Example: exampleInput,
			Score:   float32(f.Score),
		}
	}

	rf := &qdrant.RelevanceFeedbackInput{
		Target:   targetInput,
		Feedback: feedbackItems,
	}

	if n.Strategy != nil {
		rf.Strategy = qdrant.NewFeedbackStrategyNaive(&qdrant.NaiveFeedbackStrategy{
			A: float32(n.Strategy.A),
			B: float32(n.Strategy.B),
			C: float32(n.Strategy.C),
		})
	}

	state.TargetQuery = qdrant.NewQueryRelevanceFeedback(rf)
	return nil
}

func buildVectorInputFromValue(_ context.Context, _ *QueryState, val any) (*qdrant.VectorInput, error) {
	switch v := val.(type) {
	case []float32:
		return qdrant.NewVectorInputDense(v), nil
	case []any:
		vec := make([]float32, len(v))
		for i, item := range v {
			f, ok := item.(float64)
			if !ok {
				return nil, fmt.Errorf("vector element %d is not a number", i)
			}
			vec[i] = float32(f)
		}
		return qdrant.NewVectorInputDense(vec), nil
	default:
		// Treat as point ID
		pid, err := ToPointID(val)
		if err != nil {
			return nil, err
		}
		return qdrant.NewVectorInputID(pid), nil
	}
}
