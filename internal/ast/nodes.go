package ast

type InsertStmt struct {
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
	Vectors            *VectorsConfig
	Hnsw               *HnswRuntimeConfig
	Optimizers         *OptimizersRuntimeConfig
	Params             *CollectionParamsConfig
	Quantization       *QuantizationConfig
	QuantizationUpdate *QuantizationUpdate
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
	Name         string
	Size         uint64
	Distance     VectorDistance
	Hnsw         *HnswRuntimeConfig
	Quantization *QuantizationConfig
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

	Config *CollectionConfig
}

type AlterCollectionStmt struct {
	Collection string
	Config     *CollectionConfig
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
	QueryModeNearest           QueryMode = "NEAREST"
	QueryModeRecommend         QueryMode = "RECOMMEND"
	QueryModeDiscover          QueryMode = "DISCOVER"
	QueryModeContext            QueryMode = "CONTEXT"
	QueryModeOrderBy           QueryMode = "ORDER_BY"
	QueryModeSample            QueryMode = "SAMPLE"
	QueryModeRelevanceFeedback QueryMode = "RELEVANCE_FEEDBACK"
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

// CTE represents a named sub-query defined in a WITH clause.
type CTE struct {
	Name string
	Stmt *QueryStmt
}

type PayloadSelector struct {
	Enable  *bool
	Include []string
	Exclude []string
}

type VectorsSelector struct {
	Enable  *bool
	Vectors []string
}

// PrefetchRef is a reference to a CTE by name, used in PREFETCH clauses.
// Optional Filter and ScoreThreshold allow per-prefetch overrides.
type PrefetchRef struct {
	CTEName        string
	Filter         FilterExpr // per-prefetch WHERE clause
	ScoreThreshold *float64   // per-prefetch SCORE THRESHOLD
	LookupFrom     string     // per-prefetch LOOKUP FROM <collection>
	LookupVector   *string    // per-prefetch LOOKUP FROM ... VECTOR <name>
}

// FeedbackItem represents a single scored example for relevance feedback.
type FeedbackItem struct {
	Example any     // point ID or vector
	Score   float64
}

// FeedbackStrategyType identifies the feedback strategy algorithm.
type FeedbackStrategyType string

const (
	FeedbackStrategyNaive FeedbackStrategyType = "naive"
)

// FeedbackStrategy holds the parameters for a feedback strategy.
type FeedbackStrategy struct {
	Type FeedbackStrategyType
	A    float64
	B    float64
	C    float64
}

type QueryStmt struct {
	Collection string
	Mode       QueryMode
	Type       QueryType

	// For NEAREST
	QueryText *string
	QueryID   any
	RawVector []float64

	// For RECOMMEND
	PositiveIDs []any
	NegativeIDs []any

	// For CONTEXT
	ContextPairs []ContextPair

	// For DISCOVER
	Target any

	// For ORDER BY
	OrderByField *string
	OrderByAsc   *bool

	Limit                int
	Strategy             *string
	QueryFilter          FilterExpr
	Offset               int
	ScoreThreshold       *float64
	GroupBy              *string
	GroupSize            *int
	WithClause           *SearchWith
	WithPayload          *PayloadSelector
	WithVectors          *VectorsSelector
	LookupFrom           string
	LookupVector         *string
	WithLookupCollection *string // cross-collection group ID lookup (WITH LOOKUP FROM <collection>)
	Using                *string
	Model                *string

	// CTEs defined at the top level or within a CTE's body
	CTEs []CTE

	// Prefetch DAG — references to CTE names or inline queries
	PrefetchRefs []PrefetchRef
	FusionType   *string

	Rerank      bool
	RerankModel *string

	Formula         FormulaExpr
	FormulaDefaults map[string]any

	// For RELEVANCE FEEDBACK
	FeedbackTarget   any             // point ID or vector
	FeedbackItems    []FeedbackItem
	FeedbackStrategy *FeedbackStrategy
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
func (CreateCollectionStmt) isASTNode() {}
func (AlterCollectionStmt) isASTNode()  {}
func (DropCollectionStmt) isASTNode()   {}
func (ShowCollectionsStmt) isASTNode()  {}
func (ShowCollectionStmt) isASTNode()   {}
func (SelectStmt) isASTNode()           {}
func (ScrollStmt) isASTNode()           {}
func (QueryStmt) isASTNode()            {}
func (DeleteStmt) isASTNode()           {}
func (UpdateVectorStmt) isASTNode()     {}
func (UpdatePayloadStmt) isASTNode()    {}
func (CreateIndexStmt) isASTNode()      {}
