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

type CreateCollectionStmt struct {
	Collection   string
	Hybrid       bool
	Rerank       bool
	Model        *string
	Quantization *QuantizationConfig
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
}

type ASTNode interface {
	isASTNode()
}

func (InsertStmt) isASTNode()           {}
func (InsertBulkStmt) isASTNode()       {}
func (CreateCollectionStmt) isASTNode() {}
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
