package ast

type InsertStmt struct {
	Collection   string
	PointID      any
	Values       map[string]any
	Model        *string
	Hybrid       bool
	DenseVector  *string
	SparseModel  *string
	SparseVector *string
}

type InsertBulkStmt struct {
	Collection   string
	ValuesList   []map[string]any
	Model        *string
	Hybrid       bool
	DenseVector  *string
	SparseModel  *string
	SparseVector *string
}

type VectorsConfig struct {
	OnDisk *bool
}

type HnswRuntimeConfig struct {
	M                  *uint64
	EfConstruct        *uint64
	FullScanThreshold  *uint64
	MaxIndexingThreads *uint64
	OnDisk             *bool
	PayloadM           *uint64
	InlineStorage      *bool
}

type OptimizersRuntimeConfig struct {
	DeletedThreshold       *float64
	VacuumMinVectorNumber  *uint64
	DefaultSegmentNumber   *uint64
	MaxSegmentSize         *uint64
	MemmapThreshold        *uint64
	IndexingThreshold      *uint64
	FlushIntervalSec       *uint64
	MaxOptimizationThreads *OptimizationThreads
	PreventUnoptimized     *bool
}

type OptimizationThreads struct {
	Auto  bool
	Value uint64
}

type CollectionParamsConfig struct {
	ReplicationFactor      *uint64
	WriteConsistencyFactor *uint64
	ReadFanOutFactor       *uint64
	ReadFanOutDelayMs      *uint64
	OnDiskPayload          *bool
}

type CollectionConfig struct {
	Vectors    *VectorsConfig
	Hnsw       *HnswRuntimeConfig
	Optimizers *OptimizersRuntimeConfig
	Params     *CollectionParamsConfig
}

type QuantizationUpdate struct {
	Disabled bool
	Config   *QuantizationConfig
}

type VectorDistance string

const (
	DistanceCosine    VectorDistance = "COSINE"
	DistanceDot       VectorDistance = "DOT"
	DistanceEuclid    VectorDistance = "EUCLID"
	DistanceManhattan VectorDistance = "MANHATTAN"
)

type VectorDef struct {
	Name     string
	Size     uint64
	Distance VectorDistance
}

type SparseVectorDef struct {
	Name string
}

type CreateCollectionStmt struct {
	Collection   string
	Hybrid       bool
	Rerank       bool
	Model        *string
	DenseVector  *string
	SparseVector *string

	Vectors       []VectorDef
	SparseVectors []SparseVectorDef

	Quantization *QuantizationConfig
	Config       *CollectionConfig
}

type AlterCollectionStmt struct {
	Collection   string
	Config       *CollectionConfig
	Quantization *QuantizationUpdate
}

type QuantizationType string

const (
	QuantizationTypeScalar  QuantizationType = "scalar"
	QuantizationTypeBinary  QuantizationType = "binary"
	QuantizationTypeProduct QuantizationType = "product"
	QuantizationTypeTurbo   QuantizationType = "turbo"
)

type QuantizationConfig struct {
	Type      QuantizationType
	Quantile  *float64
	AlwaysRAM bool
	TurboBits *float64
}

type DropCollectionStmt struct {
	Collection string
}

type ShowCollectionsStmt struct {
}

type ShowCollectionStmt struct {
	Collection string
}

type SelectStmt struct {
	Collection string
	PointID    any
}

type ScrollStmt struct {
	Collection  string
	Limit       int
	QueryFilter FilterExpr
	After       any
}

type QuantizationSearchWith struct {
	Ignore       *bool
	Rescore      *bool
	Oversampling *float64
}

type QueryMode string

const (
	QueryModeNearest   QueryMode = "NEAREST"
	QueryModeRecommend QueryMode = "RECOMMEND"
	QueryModeDiscover  QueryMode = "DISCOVER"
	QueryModeContext   QueryMode = "CONTEXT"
)

type ContextPair struct {
	Positive any
	Negative any
}

type QueryType int

const (
	QueryTypeDense QueryType = iota
	QueryTypeSparse
	QueryTypeHybrid
)

type Prefetch struct {
	Prefetches     []*Prefetch
	Type           QueryType
	QueryText      *string
	QueryID        any
	Mode           QueryMode
	PositiveIDs    []any
	NegativeIDs    []any
	ContextPairs   []ContextPair
	Target         any
	Limit          int
	Strategy       *string
	QueryFilter    FilterExpr
	ScoreThreshold *float64
	GroupBy        *string
	GroupSize      *int
	WithClause     *SearchWith
	LookupFrom     string
	LookupVector   *string
	Using          *string
}

type QueryStmt struct {
	Collection string
	Mode       QueryMode
	Type       QueryType

	// For NEAREST
	QueryText *string
	QueryID   any

	// For RECOMMEND
	PositiveIDs []any
	NegativeIDs []any

	// For CONTEXT
	ContextPairs []ContextPair

	// For DISCOVER
	Target any // ID or string

	Limit          int
	Strategy       *string
	QueryFilter    FilterExpr
	Offset         int
	ScoreThreshold *float64
	GroupBy        *string
	GroupSize      *int
	WithClause     *SearchWith
	LookupFrom     string
	LookupVector   *string
	Using          *string
	Model          *string

	// Prefetch DAG
	Prefetches []*Prefetch
	FusionType *string

	// Legacy flags mapped to pipeline logic
	Hybrid      bool
	Rerank      bool
	RerankModel *string
}

type DeleteStmt struct {
	Collection string
	PointID    any
	Field      string
	Value      any
}

type UpdateVectorStmt struct {
	Collection string
	PointID    any
	Vector     []float32
	VectorName *string
}

type UpdatePayloadStmt struct {
	Collection  string
	PointID     any
	QueryFilter FilterExpr
	Payload     map[string]any
}

type CreateIndexStmt struct {
	Collection string
	Field      string
	FieldType  string
	Options    map[string]any
}

type ASTNode interface {
	isASTNode()
}

func (InsertStmt) isASTNode()           {}
func (InsertBulkStmt) isASTNode()       {}
func (CreateCollectionStmt) isASTNode() {}
func (AlterCollectionStmt) isASTNode()  {}
func (DropCollectionStmt) isASTNode()   {}
func (ShowCollectionsStmt) isASTNode()  {}
func (ShowCollectionStmt) isASTNode()   {}
func (SelectStmt) isASTNode()           {}
func (ScrollStmt) isASTNode()           {}

// Removed SearchStmt and RecommendStmt methods
func (QueryStmt) isASTNode()         {}
func (DeleteStmt) isASTNode()        {}
func (UpdateVectorStmt) isASTNode()  {}
func (UpdatePayloadStmt) isASTNode() {}
func (CreateIndexStmt) isASTNode()   {}
