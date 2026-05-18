package ast

type InsertStmt struct {
	Collection  string
	PointID     interface{}
	Values      map[string]interface{}
	Model       *string
	Hybrid      bool
	SparseModel *string
}

type InsertBulkStmt struct {
	Collection  string
	ValuesList  []map[string]interface{}
	Model       *string
	Hybrid      bool
	SparseModel *string
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
	DeletedThreshold          *float64
	VacuumMinVectorNumber     *uint64
	DefaultSegmentNumber      *uint64
	MaxSegmentSize            *uint64
	MemmapThreshold           *uint64
	IndexingThreshold         *uint64
	FlushIntervalSec          *uint64
	MaxOptimizationThreads    interface{} // int or "auto" string
	PreventUnoptimized        *bool
}

type CollectionParamsConfig struct {
	ReplicationFactor       *uint64
	WriteConsistencyFactor  *uint64
	ReadFanOutFactor        *uint64
	ReadFanOutDelayMs       *uint64
	OnDiskPayload           *bool
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

type CreateCollectionStmt struct {
	Collection   string
	Hybrid       bool
	Rerank       bool
	Model        *string
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
	PointID    interface{}
}

type ScrollStmt struct {
	Collection  string
	Limit       int
	QueryFilter FilterExpr
	After       interface{}
}

type SearchStmt struct {
	Collection  string
	QueryText   string
	Limit       int
	Model       *string
	Hybrid      bool
	Fusion      *string
	SparseOnly  bool
	SparseModel *string
	QueryFilter FilterExpr
	Rerank      bool
	RerankModel *string
	WithClause  *SearchWith
	GroupBy     string
	GroupSize   int
}

type QuantizationSearchWith struct {
	Ignore       *bool
	Rescore      *bool
	Oversampling *float64
}

type RecommendStmt struct {
	Collection     string
	PositiveIDs    []interface{}
	NegativeIDs    []interface{}
	Limit          int
	Strategy       *string
	QueryFilter    FilterExpr
	Offset         int
	ScoreThreshold *float64
	WithClause     *SearchWith
	LookupFrom     string
	LookupVector   *string
	Using          *string
}

type DeleteStmt struct {
	Collection string
	PointID    interface{}
	Field      string
	Value      interface{}
}

type UpdateVectorStmt struct {
	Collection string
	PointID    interface{}
	Vector     []float32
}

type UpdatePayloadStmt struct {
	Collection  string
	PointID     interface{}
	QueryFilter FilterExpr
	Payload     map[string]interface{}
}

type CreateIndexStmt struct {
	Collection string
	Field      string
	FieldType  string
	Options    map[string]interface{}
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
func (SearchStmt) isASTNode()           {}
func (RecommendStmt) isASTNode()        {}
func (DeleteStmt) isASTNode()           {}
func (UpdateVectorStmt) isASTNode()     {}
func (UpdatePayloadStmt) isASTNode()    {}
func (CreateIndexStmt) isASTNode()      {}
