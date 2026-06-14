package pipeline

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/qdrant/go-client/qdrant"
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
			Options: buildDocumentOptions(state.CloudModelOptions),
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
			Options: buildDocumentOptions(state.CloudModelOptions),
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
		Options: buildDocumentOptions(state.CloudModelOptions),
	})
	return nil
}

// buildDocumentOptions converts the string-map from config into the *qdrant.Value map
// expected by the qdrant-go gRPC client for cloud inference provider API keys.
func buildDocumentOptions(opts map[string]string) map[string]*qdrant.Value {
	if len(opts) == 0 {
		return nil
	}
	out := make(map[string]*qdrant.Value, len(opts))
	for k, v := range opts {
		out[k] = qdrant.NewValueString(v)
	}
	return out
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

func buildVectorInput(ctx context.Context, state *QueryState, val any) (*qdrant.VectorInput, error) {
	switch v := val.(type) {
	case string:
		if num, err := strconv.ParseUint(v, 10, 64); err == nil {
			return qdrant.NewVectorInputID(qdrant.NewIDNum(num)), nil
		}
		if isUUID(v) {
			return qdrant.NewVectorInputID(qdrant.NewIDUUID(v)), nil
		}
		if state.LocalEmbed {
			if state.Embedder == nil {
				return nil, fmt.Errorf("local embedding requested but no Embedder provided")
			}
			denseVector, err := state.Embedder.EmbedDense(ctx, v, state.DenseModel)
			if err != nil {
				return nil, fmt.Errorf("failed to embed target query: %w", err)
			}
			return qdrant.NewVectorInputDense(denseVector), nil
		}
		return qdrant.NewVectorInputDocument(&qdrant.Document{
			Text:    v,
			Model:   state.DenseModel,
			Options: buildDocumentOptions(state.CloudModelOptions),
		}), nil
	case int:
		if v < 0 {
			return nil, fmt.Errorf("unsupported vector input type: negative integer")
		}
		return qdrant.NewVectorInputID(qdrant.NewIDNum(uint64(v))), nil
	case int64:
		if v < 0 {
			return nil, fmt.Errorf("unsupported vector input type: negative integer")
		}
		return qdrant.NewVectorInputID(qdrant.NewIDNum(uint64(v))), nil
	case uint64:
		return qdrant.NewVectorInputID(qdrant.NewIDNum(v)), nil
	case float64:
		// Prevent precision loss silently passing the check
		if v < 0 || v > float64(1<<53) || v != float64(uint64(v)) {
			return nil, fmt.Errorf("unsupported vector input type: non-integer or oversized float")
		}
		return qdrant.NewVectorInputID(qdrant.NewIDNum(uint64(v))), nil
	case []float32: // In case of raw vectors
		return qdrant.NewVectorInputDense(v), nil
	default:
		return nil, fmt.Errorf("unsupported vector input type: %T", val)
	}
}
