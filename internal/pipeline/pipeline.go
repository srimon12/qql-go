package pipeline

import (
	"context"

	"github.com/qdrant/go-client/qdrant"
)

// Embedder abstracts the local or cloud embedding capability away from the execution pipeline.
type Embedder interface {
	EmbedDense(ctx context.Context, text string, model string) ([]float32, error)
	EmbedSparse(ctx context.Context, text string) ([]uint32, []float32, error)
}

// QueryState represents the transient state as a query traverses the execution DAG.
type QueryState struct {
	// --- Query construction (set by embed nodes) ---
	QueryText        string
	Prefetches       []*qdrant.PrefetchQuery
	ManualPrefetches []*qdrant.PrefetchQuery
	TargetQuery      *qdrant.Query
	Params           *qdrant.SearchParams
	FusionConfig     *qdrant.Rrf

	// --- Embedding strategy ---
	HasMMR            bool
	MmrCandidates     uint32
	MmrDiversity      float32
	LocalEmbed        bool
	Embedder          Embedder
	CloudModelOptions map[string]string
	DenseModel        string

	// --- Cached computed values ---
	DocOptions     map[string]*qdrant.Value
	RequestTimeout *uint64

	// --- Request assembly (set by executor before pipeline runs) ---
	CollectionName string
	VectorName     string
	Limit          uint64
	Offset         uint64
	QdrantFilter   *qdrant.Filter
	ScoreThreshold *float32
	LookupFrom     *qdrant.LookupLocation
	WithPayload    *qdrant.WithPayloadSelector
	WithVectors    *qdrant.WithVectorsSelector

	// --- GroupBy ---
	GroupBy    string
	GroupSize  uint64
	WithLookup *qdrant.WithLookup
}

// ExecutionNode defines a single step in the QQL Query Planner DAG.
type ExecutionNode interface {
	Execute(ctx context.Context, state *QueryState) error
}

// QueryPipeline orchestrates a sequence of execution nodes.
type QueryPipeline struct {
	nodes []ExecutionNode
}

func NewQueryPipeline() *QueryPipeline {
	return &QueryPipeline{}
}

func (p *QueryPipeline) Add(node ExecutionNode) *QueryPipeline {
	p.nodes = append(p.nodes, node)
	return p
}

func (p *QueryPipeline) Execute(ctx context.Context, state *QueryState) error {
	for _, node := range p.nodes {
		if err := node.Execute(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

// BuildFlatRequest assembles a complete QueryPoints request from the accumulated state.
// Call this after Execute().
func (p *QueryPipeline) BuildFlatRequest(state *QueryState) *qdrant.QueryPoints {
	prefetches := state.Prefetches
	if len(state.ManualPrefetches) > 0 {
		prefetches = append(prefetches, state.ManualPrefetches...)
	}

	req := &qdrant.QueryPoints{
		CollectionName: state.CollectionName,
		Query:          state.TargetQuery,
		Prefetch:       prefetches,
		Limit:          &state.Limit,
		Params:         state.Params,
		Filter:         state.QdrantFilter,
		WithPayload:    state.WithPayload,
		WithVectors:    state.WithVectors,
		Timeout:        state.RequestTimeout,
	}
	if state.VectorName != "" {
		req.Using = qdrant.PtrOf(state.VectorName)
	}
	if state.Offset > 0 {
		req.Offset = &state.Offset
	}
	if state.ScoreThreshold != nil {
		req.ScoreThreshold = state.ScoreThreshold
	}
	if state.LookupFrom != nil {
		req.LookupFrom = state.LookupFrom
	}
	return req
}

// BuildGroupedRequest assembles a complete QueryPointGroups request from the accumulated state.
// Call this after Execute().
func (p *QueryPipeline) BuildGroupedRequest(state *QueryState) *qdrant.QueryPointGroups {
	flat := p.BuildFlatRequest(state)
	return &qdrant.QueryPointGroups{
		CollectionName: flat.CollectionName,
		Query:          flat.Query,
		Prefetch:       flat.Prefetch,
		Using:          flat.Using,
		Limit:          flat.Limit,
		GroupBy:        state.GroupBy,
		GroupSize:      groupSizePtr(state.GroupSize),
		Filter:         flat.Filter,
		ScoreThreshold: flat.ScoreThreshold,
		LookupFrom:     flat.LookupFrom,
		Params:         flat.Params,
		WithPayload:    flat.WithPayload,
		WithVectors:    flat.WithVectors,
		WithLookup:     state.WithLookup,
	}
}

func groupSizePtr(n uint64) *uint64 {
	if n == 0 {
		return nil
	}
	return &n
}

// GetDocOptions returns the cached cloud model document options, computing them once if needed.
func (s *QueryState) GetDocOptions() map[string]*qdrant.Value {
	if s.DocOptions == nil && len(s.CloudModelOptions) > 0 {
		s.DocOptions = buildDocumentOptions(s.CloudModelOptions)
	}
	return s.DocOptions
}

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
