package ast

type InsertStmt struct {
	Collection  string
	Values      map[string]interface{}
	Model       *string
	Hybrid      bool
	SparseModel *string
}

type CreateCollectionStmt struct {
	Collection string
	Hybrid     bool
	Rerank     bool
}

type DropCollectionStmt struct {
	Collection string
}

type ShowCollectionsStmt struct {
}

type SearchStmt struct {
	Collection  string
	QueryText   string
	Limit       int
	Model       *string
	Hybrid      bool
	SparseModel *string
	QueryFilter FilterExpr
	Rerank      bool
	RerankModel *string
	WithClause  *SearchWith
}

type DeleteStmt struct {
	Collection string
	PointID    interface{}
	Field      string
	Value      interface{}
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
func (CreateCollectionStmt) isASTNode() {}
func (DropCollectionStmt) isASTNode()   {}
func (ShowCollectionsStmt) isASTNode()  {}
func (SearchStmt) isASTNode()           {}
func (DeleteStmt) isASTNode()           {}
func (CreateIndexStmt) isASTNode()      {}
