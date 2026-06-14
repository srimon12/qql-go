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
	QueryText     string
	Prefetches    []*qdrant.PrefetchQuery
	TargetQuery   *qdrant.Query
	Params        *qdrant.SearchParams
	HasMMR        bool
	MmrCandidates uint32
	MmrDiversity  float32
	LocalEmbed    bool
	Embedder      Embedder
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
