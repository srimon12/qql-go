package parser

import (
	"strings"
	"testing"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInsert(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.InsertStmt
		wantErr bool
	}{
		{
			name:  "simple insert",
			input: `INSERT INTO COLLECTION test VALUES {"text": "hello"}`,
			want: &ast.InsertStmt{
				Collection: "test",
				Values:     map[string]interface{}{"text": "hello"},
			},
		},
		{
			name:  "insert with bare keys",
			input: `INSERT INTO COLLECTION test VALUES {text: 'hello', topic: 'search'}`,
			want: &ast.InsertStmt{
				Collection: "test",
				Values:     map[string]interface{}{"text": "hello", "topic": "search"},
			},
		},
		{
			name:  "insert with model",
			input: `INSERT INTO COLLECTION test VALUES {"text": "hello"} USING MODEL 'model-name'`,
			want: &ast.InsertStmt{
				Collection: "test",
				Values:     map[string]interface{}{"text": "hello"},
				Model:      strPtr("model-name"),
			},
		},
		{
			name:  "insert with hybrid",
			input: `INSERT INTO COLLECTION test VALUES {"text": "hello"} USING HYBRID`,
			want: &ast.InsertStmt{
				Collection: "test",
				Values:     map[string]interface{}{"text": "hello"},
				Hybrid:     true,
			},
		},
		{
			name:  "insert with hybrid and models",
			input: `INSERT INTO COLLECTION test VALUES {"text": "hello"} USING HYBRID DENSE MODEL 'dense-model' SPARSE MODEL 'sparse-model'`,
			want: &ast.InsertStmt{
				Collection:  "test",
				Values:      map[string]interface{}{"text": "hello"},
				Hybrid:      true,
				Model:       strPtr("dense-model"),
				SparseModel: strPtr("sparse-model"),
			},
		},
		{
			name:  "insert with sparse model only",
			input: `INSERT INTO COLLECTION test VALUES {"text": "hello"} USING HYBRID SPARSE MODEL 'sparse-model'`,
			want: &ast.InsertStmt{
				Collection:  "test",
				Values:      map[string]interface{}{"text": "hello"},
				Hybrid:      true,
				SparseModel: strPtr("sparse-model"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			stmt, ok := node.(*ast.InsertStmt)
			require.True(t, ok, "expected InsertStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.Values, stmt.Values)
			assert.Equal(t, tt.want.Hybrid, stmt.Hybrid)
			if tt.want.Model != nil {
				assert.Equal(t, *tt.want.Model, *stmt.Model)
			}
			if tt.want.SparseModel != nil {
				assert.Equal(t, *tt.want.SparseModel, *stmt.SparseModel)
			}
		})
	}
}

func TestParseCreate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *ast.CreateCollectionStmt
	}{
		{
			name:  "simple create",
			input: "CREATE COLLECTION mycollection",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
			},
		},
		{
			name:  "create with hybrid",
			input: "CREATE COLLECTION mycollection HYBRID",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
				Hybrid:     true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			require.NoError(t, err)

			stmt, ok := node.(*ast.CreateCollectionStmt)
			require.True(t, ok, "expected CreateCollectionStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.Hybrid, stmt.Hybrid)
		})
	}
}

func TestParseDrop(t *testing.T) {
	input := "DROP COLLECTION mycollection"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.DropCollectionStmt)
	require.True(t, ok, "expected DropCollectionStmt")
	assert.Equal(t, "mycollection", stmt.Collection)
}

func TestParseCreateIndex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.CreateIndexStmt
		wantErr bool
	}{
		{
			name:  "simple create index",
			input: "CREATE INDEX ON COLLECTION mycollection FOR field",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "field",
				FieldType:  "keyword",
			},
		},
		{
			name:  "create index with type keyword",
			input: "CREATE INDEX ON COLLECTION mycollection FOR field TYPE keyword",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "field",
				FieldType:  "keyword",
			},
		},
		{
			name:  "create index with type integer",
			input: "CREATE INDEX ON COLLECTION mycollection FOR patient_id TYPE integer",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "patient_id",
				FieldType:  "integer",
			},
		},
		{
			name:  "create index with type float",
			input: "CREATE INDEX ON COLLECTION mycollection FOR score TYPE float",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "score",
				FieldType:  "float",
			},
		},
		{
			name:  "create index with type bool",
			input: "CREATE INDEX ON COLLECTION mycollection FOR is_active TYPE bool",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "is_active",
				FieldType:  "bool",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			stmt, ok := node.(*ast.CreateIndexStmt)
			require.True(t, ok, "expected CreateIndexStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.Field, stmt.Field)
			assert.Equal(t, tt.want.FieldType, stmt.FieldType)
		})
	}
}

func TestParseShow(t *testing.T) {
	input := "SHOW COLLECTIONS"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	_, ok := node.(*ast.ShowCollectionsStmt)
	assert.True(t, ok, "expected ShowCollectionsStmt")
}

func TestParseSearch(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.SearchStmt
		wantErr bool
	}{
		{
			name:  "simple search",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
			},
		},
		{
			name:  "search with exact",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 EXACT",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{Exact: true},
			},
		},
		{
			name:  "search with with clause hnsw_ef",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WITH {hnsw_ef: 128}",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{HnswEf: 128},
			},
		},
		{
			name:  "search with with clause exact true",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WITH {exact: true}",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{Exact: true},
			},
		},
		{
			name:  "search with with clause exact false",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WITH {exact: false}",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{Exact: false},
			},
		},
		{
			name:  "search with with clause acorn",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WITH {acorn: true}",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{Acorn: true},
			},
		},
		{
			name:  "search with model",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 USING MODEL 'my-model'",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				Model:      strPtr("my-model"),
			},
		},
		{
			name:  "search with hybrid",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 USING HYBRID",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				Hybrid:     true,
			},
		},
		{
			name:  "search with hybrid and models",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 USING HYBRID DENSE MODEL 'dense' SPARSE MODEL 'sparse'",
			want: &ast.SearchStmt{
				Collection:  "mycollection",
				QueryText:   "query text",
				Limit:       10,
				Hybrid:      true,
				Model:       strPtr("dense"),
				SparseModel: strPtr("sparse"),
			},
		},
		{
			name:  "search with where clause",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WHERE tags = 'important'",
			want: &ast.SearchStmt{
				Collection:  "mycollection",
				QueryText:   "query text",
				Limit:       10,
				QueryFilter: &ast.CompareExpr{Field: "tags", Op: "=", Value: "important"},
			},
		},
		{
			name:  "search with rerank",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 RERANK",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				Rerank:     true,
			},
		},
		{
			name:  "search with rerank model",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 RERANK MODEL 'cross-encoder'",
			want: &ast.SearchStmt{
				Collection:  "mycollection",
				QueryText:   "query text",
				Limit:       10,
				Rerank:      true,
				RerankModel: strPtr("cross-encoder"),
			},
		},
		{
			name:  "search with hybrid and rerank",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 USING HYBRID RERANK",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				Hybrid:     true,
				Rerank:     true,
			},
		},
		{
			name:  "search with reordered modifiers",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 EXACT WITH {hnsw_ef: 64, acorn: true} WHERE tags = 'important' RERANK MODEL 'cross-encoder'",
			want: &ast.SearchStmt{
				Collection:  "mycollection",
				QueryText:   "query text",
				Limit:       10,
				QueryFilter: &ast.CompareExpr{Field: "tags", Op: "=", Value: "important"},
				Rerank:      true,
				RerankModel: strPtr("cross-encoder"),
				WithClause:  &ast.SearchWith{HnswEf: 64, Exact: true, Acorn: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			stmt, ok := node.(*ast.SearchStmt)
			require.True(t, ok, "expected SearchStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.QueryText, stmt.QueryText)
			assert.Equal(t, tt.want.Limit, stmt.Limit)
			assert.Equal(t, tt.want.Hybrid, stmt.Hybrid)
			if tt.want.Model != nil {
				require.NotNil(t, stmt.Model)
				assert.Equal(t, *tt.want.Model, *stmt.Model)
			}
			if tt.want.SparseModel != nil {
				require.NotNil(t, stmt.SparseModel)
				assert.Equal(t, *tt.want.SparseModel, *stmt.SparseModel)
			}
			if tt.want.WithClause != nil {
				require.NotNil(t, stmt.WithClause)
				assert.Equal(t, tt.want.WithClause.HnswEf, stmt.WithClause.HnswEf)
				assert.Equal(t, tt.want.WithClause.Exact, stmt.WithClause.Exact)
				assert.Equal(t, tt.want.WithClause.Acorn, stmt.WithClause.Acorn)
			}
			if tt.want.QueryFilter != nil {
				require.NotNil(t, stmt.QueryFilter)
				assertFilterExprEqual(t, tt.want.QueryFilter, stmt.QueryFilter)
			}
			assert.Equal(t, tt.want.Rerank, stmt.Rerank)
			if tt.want.RerankModel != nil {
				require.NotNil(t, stmt.RerankModel)
				assert.Equal(t, *tt.want.RerankModel, *stmt.RerankModel)
			}
		})
	}
}

func TestParseDelete(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.DeleteStmt
		wantErr bool
	}{
		{
			name:  "delete with string id",
			input: "DELETE FROM mycollection WHERE id = 'point-123'",
			want: &ast.DeleteStmt{
				Collection: "mycollection",
				PointID:    "point-123",
			},
		},
		{
			name:  "delete with integer id",
			input: "DELETE FROM mycollection WHERE id = 42",
			want: &ast.DeleteStmt{
				Collection: "mycollection",
				PointID:    42,
			},
		},
		{
			name:  "delete by field",
			input: "DELETE FROM mycollection WHERE status = 'archived'",
			want: &ast.DeleteStmt{
				Collection: "mycollection",
				Field:      "status",
				Value:      "archived",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			stmt, ok := node.(*ast.DeleteStmt)
			require.True(t, ok, "expected DeleteStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.PointID, stmt.PointID)
			assert.Equal(t, tt.want.Field, stmt.Field)
			assert.Equal(t, tt.want.Value, stmt.Value)
		})
	}
}

func TestParseDocumentedExamples(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		check   func(t *testing.T, node ast.ASTNode)
		wantErr bool
	}{
		{
			name:  "readme create hybrid collection",
			input: "CREATE COLLECTION docs HYBRID",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.CreateCollectionStmt)
				require.True(t, ok, "expected CreateCollectionStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.True(t, stmt.Hybrid)
			},
		},
		{
			name:  "readme create hybrid rerank collection",
			input: "CREATE COLLECTION docs HYBRID RERANK",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.CreateCollectionStmt)
				require.True(t, ok, "expected CreateCollectionStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.True(t, stmt.Hybrid)
				assert.True(t, stmt.Rerank)
			},
		},
		{
			name:  "readme hybrid insert",
			input: "INSERT INTO COLLECTION docs VALUES {'text': 'Qdrant stores vectors', 'topic': 'search'} USING HYBRID",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.InsertStmt)
				require.True(t, ok, "expected InsertStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.True(t, stmt.Hybrid)
				assert.Equal(t, map[string]interface{}{"text": "Qdrant stores vectors", "topic": "search"}, stmt.Values)
			},
		},
		{
			name:  "readme hybrid search",
			input: "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 USING HYBRID",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.SearchStmt)
				require.True(t, ok, "expected SearchStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.Equal(t, "vector database", stmt.QueryText)
				assert.Equal(t, 5, stmt.Limit)
				assert.True(t, stmt.Hybrid)
			},
		},
		{
			name:  "readme hybrid search with filter",
			input: "SEARCH notes SIMILAR TO 'vector search' LIMIT 5 USING HYBRID WHERE topic = 'search'",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.SearchStmt)
				require.True(t, ok, "expected SearchStmt")
				assert.Equal(t, "notes", stmt.Collection)
				assert.True(t, stmt.Hybrid)
				require.NotNil(t, stmt.QueryFilter)
				assertFilterExprEqual(t, &ast.CompareExpr{Field: "topic", Op: "=", Value: "search"}, stmt.QueryFilter)
			},
		},
		{
			name:  "readme hybrid rerank search",
			input: "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 USING HYBRID RERANK",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.SearchStmt)
				require.True(t, ok, "expected SearchStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.True(t, stmt.Hybrid)
				assert.True(t, stmt.Rerank)
			},
		},
		{
			name:  "readme delete by id",
			input: "DELETE FROM notes WHERE id = 'uuid'",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.DeleteStmt)
				require.True(t, ok, "expected DeleteStmt")
				assert.Equal(t, "notes", stmt.Collection)
				assert.Equal(t, "uuid", stmt.PointID)
			},
		},
		{
			name:  "readme delete by field",
			input: "DELETE FROM notes WHERE specialty = 'search'",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.DeleteStmt)
				require.True(t, ok, "expected DeleteStmt")
				assert.Equal(t, "notes", stmt.Collection)
				assert.Equal(t, "specialty", stmt.Field)
				assert.Equal(t, "search", stmt.Value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, tt.check)
			tt.check(t, node)
		})
	}
}

func TestParseFilterComparison(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ast.FilterExpr
	}{
		{
			name:     "equals",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field = 'value'",
			expected: &ast.CompareExpr{Field: "field", Op: "=", Value: "value"},
		},
		{
			name:     "not equals",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field != 'value'",
			expected: &ast.CompareExpr{Field: "field", Op: "!=", Value: "value"},
		},
		{
			name:     "greater than",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE count > 5",
			expected: &ast.CompareExpr{Field: "count", Op: ">", Value: 5},
		},
		{
			name:     "greater than or equals",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE count >= 5",
			expected: &ast.CompareExpr{Field: "count", Op: ">=", Value: 5},
		},
		{
			name:     "less than",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE count < 10",
			expected: &ast.CompareExpr{Field: "count", Op: "<", Value: 10},
		},
		{
			name:     "less than or equals",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE count <= 10",
			expected: &ast.CompareExpr{Field: "count", Op: "<=", Value: 10},
		},
		{
			name:     "equals integer",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE count = 42",
			expected: &ast.CompareExpr{Field: "count", Op: "=", Value: 42},
		},
		{
			name:     "equals float",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE score = 3.14",
			expected: &ast.CompareExpr{Field: "score", Op: "=", Value: 3.14},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			require.NoError(t, err)

			stmt, ok := node.(*ast.SearchStmt)
			require.True(t, ok)
			require.NotNil(t, stmt.QueryFilter)
			assertFilterExprEqual(t, tt.expected, stmt.QueryFilter)
		})
	}
}

func TestParseFilterBetween(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE age BETWEEN 18 AND 65"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	between, ok := stmt.QueryFilter.(*ast.BetweenExpr)
	require.True(t, ok)
	assert.Equal(t, "age", between.Field)
	assert.Equal(t, 18, between.Low)
	assert.Equal(t, 65, between.High)
}

func TestParseFilterIn(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE status IN ('active', 'pending')"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	inExpr, ok := stmt.QueryFilter.(*ast.InExpr)
	require.True(t, ok)
	assert.Equal(t, "status", inExpr.Field)
	assert.Equal(t, []interface{}{"active", "pending"}, inExpr.Values)
}

func TestParseFilterNotIn(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE status NOT IN ('deleted', 'archived')"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	notIn, ok := stmt.QueryFilter.(*ast.NotInExpr)
	require.True(t, ok)
	assert.Equal(t, "status", notIn.Field)
	assert.Equal(t, []interface{}{"deleted", "archived"}, notIn.Values)
}

func TestParseFilterIsNull(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field IS NULL"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isNull, ok := stmt.QueryFilter.(*ast.IsNullExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isNull.Field)
}

func TestParseFilterIsNotNull(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field IS NOT NULL"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isNotNull, ok := stmt.QueryFilter.(*ast.IsNotNullExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isNotNull.Field)
}

func TestParseFilterIsEmpty(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field IS EMPTY"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isEmpty, ok := stmt.QueryFilter.(*ast.IsEmptyExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isEmpty.Field)
}

func TestParseFilterIsNotEmpty(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field IS NOT EMPTY"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isNotEmpty, ok := stmt.QueryFilter.(*ast.IsNotEmptyExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isNotEmpty.Field)
}

func TestParseFilterMatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ast.FilterExpr
	}{
		{
			name:     "match text",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE content MATCH 'hello world'",
			expected: &ast.MatchTextExpr{Field: "content", Text: "hello world"},
		},
		{
			name:     "match any",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE content MATCH ANY 'hello world'",
			expected: &ast.MatchAnyExpr{Field: "content", Text: "hello world"},
		},
		{
			name:     "match phrase",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE content MATCH PHRASE 'hello world'",
			expected: &ast.MatchPhraseExpr{Field: "content", Text: "hello world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			require.NoError(t, err)

			stmt, ok := node.(*ast.SearchStmt)
			require.True(t, ok)
			require.NotNil(t, stmt.QueryFilter)
			assertFilterExprEqual(t, tt.expected, stmt.QueryFilter)
		})
	}
}

func TestParseFilterAnd(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE a = 1 AND b = 2"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	andExpr, ok := stmt.QueryFilter.(*ast.AndExpr)
	require.True(t, ok)
	require.Len(t, andExpr.Operands, 2)
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "a", Op: "=", Value: 1}, andExpr.Operands[0])
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "b", Op: "=", Value: 2}, andExpr.Operands[1])
}

func TestParseFilterOr(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE a = 1 OR b = 2"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	orExpr, ok := stmt.QueryFilter.(*ast.OrExpr)
	require.True(t, ok)
	require.Len(t, orExpr.Operands, 2)
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "a", Op: "=", Value: 1}, orExpr.Operands[0])
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "b", Op: "=", Value: 2}, orExpr.Operands[1])
}

func TestParseFilterNot(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE NOT a = 1"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	notExpr, ok := stmt.QueryFilter.(*ast.NotExpr)
	require.True(t, ok)
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "a", Op: "=", Value: 1}, notExpr.Operand)
}

func TestParseFilterComplex(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE (a = 1 AND b = 2) OR (c = 3 AND NOT d = 4)"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	orExpr, ok := stmt.QueryFilter.(*ast.OrExpr)
	require.True(t, ok)
	assertFilterExprEqual(t, &ast.OrExpr{
		Operands: []ast.FilterExpr{
			&ast.AndExpr{
				Operands: []ast.FilterExpr{
					&ast.CompareExpr{Field: "a", Op: "=", Value: 1},
					&ast.CompareExpr{Field: "b", Op: "=", Value: 2},
				},
			},
			&ast.AndExpr{
				Operands: []ast.FilterExpr{
					&ast.CompareExpr{Field: "c", Op: "=", Value: 3},
					&ast.NotExpr{
						Operand: &ast.CompareExpr{Field: "d", Op: "=", Value: 4},
					},
				},
			},
		},
	}, orExpr)
}

func TestParseFilterPrecedence(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE a = 1 AND b = 2 OR c = 3"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	orExpr, ok := stmt.QueryFilter.(*ast.OrExpr)
	require.True(t, ok)
	assertFilterExprEqual(t, &ast.OrExpr{
		Operands: []ast.FilterExpr{
			&ast.AndExpr{
				Operands: []ast.FilterExpr{
					&ast.CompareExpr{Field: "a", Op: "=", Value: 1},
					&ast.CompareExpr{Field: "b", Op: "=", Value: 2},
				},
			},
			&ast.CompareExpr{Field: "c", Op: "=", Value: 3},
		},
	}, orExpr)
}

func TestParseError(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "invalid statement",
			input:   "INVALID KEYWORD",
			wantErr: true,
		},
		{
			name:    "insert missing values",
			input:   "INSERT INTO COLLECTION test",
			wantErr: true,
		},
		{
			name:    "search missing limit",
			input:   "SEARCH test SIMILAR TO 'text'",
			wantErr: true,
		},
		{
			name:    "search missing query text",
			input:   "SEARCH test SIMILAR TO LIMIT 10",
			wantErr: true,
		},
		{
			name:    "search with invalid with boolean",
			input:   "SEARCH test SIMILAR TO 'text' LIMIT 10 WITH {exact: maybe}",
			wantErr: true,
		},
		{
			name:    "reject trailing tokens",
			input:   "INSERT INTO COLLECTION test VALUES {\"text\": \"hello\"} EXTRA",
			wantErr: true,
		},
		{
			name:    "reject explain in parser",
			input:   "EXPLAIN SEARCH test SIMILAR TO 'text' LIMIT 10",
			wantErr: true,
		},
		{
			name:    "reject overflowing limit",
			input:   "SEARCH test SIMILAR TO 'text' LIMIT 999999999999999999999999999",
			wantErr: true,
		},
		{
			name:    "reject overflowing integer literal",
			input:   "DELETE FROM test WHERE id = 999999999999999999999999999",
			wantErr: true,
		},
		{
			name:    "reject overflowing float literal",
			input:   "SEARCH test SIMILAR TO 'text' LIMIT 10 WHERE score = " + strings.Repeat("9", 400) + ".0",
			wantErr: true,
		},
		{
			name:    "reject duplicate where clause",
			input:   "SEARCH test SIMILAR TO 'text' LIMIT 10 WHERE a = 1 WHERE b = 2",
			wantErr: true,
		},
		{
			name:    "reject duplicate with clause",
			input:   "SEARCH test SIMILAR TO 'text' LIMIT 10 WITH {exact: true} WITH {acorn: true}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			if err != nil {
				if tt.wantErr {
					return
				}
				t.Fatalf("lexer error: %v", err)
			}

			p := NewParser()
			_, err = p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func assertFilterExprEqual(t *testing.T, expected, actual ast.FilterExpr) {
	t.Helper()

	switch e := expected.(type) {
	case *ast.CompareExpr:
		a, ok := actual.(*ast.CompareExpr)
		require.True(t, ok, "expected CompareExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Op, a.Op)
		assert.Equal(t, e.Value, a.Value)
	case *ast.BetweenExpr:
		a, ok := actual.(*ast.BetweenExpr)
		require.True(t, ok, "expected BetweenExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Low, a.Low)
		assert.Equal(t, e.High, a.High)
	case *ast.InExpr:
		a, ok := actual.(*ast.InExpr)
		require.True(t, ok, "expected InExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Values, a.Values)
	case *ast.NotInExpr:
		a, ok := actual.(*ast.NotInExpr)
		require.True(t, ok, "expected NotInExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Values, a.Values)
	case *ast.IsNullExpr:
		a, ok := actual.(*ast.IsNullExpr)
		require.True(t, ok, "expected IsNullExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.IsNotNullExpr:
		a, ok := actual.(*ast.IsNotNullExpr)
		require.True(t, ok, "expected IsNotNullExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.IsEmptyExpr:
		a, ok := actual.(*ast.IsEmptyExpr)
		require.True(t, ok, "expected IsEmptyExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.IsNotEmptyExpr:
		a, ok := actual.(*ast.IsNotEmptyExpr)
		require.True(t, ok, "expected IsNotEmptyExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.MatchTextExpr:
		a, ok := actual.(*ast.MatchTextExpr)
		require.True(t, ok, "expected MatchTextExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Text, a.Text)
	case *ast.MatchAnyExpr:
		a, ok := actual.(*ast.MatchAnyExpr)
		require.True(t, ok, "expected MatchAnyExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Text, a.Text)
	case *ast.MatchPhraseExpr:
		a, ok := actual.(*ast.MatchPhraseExpr)
		require.True(t, ok, "expected MatchPhraseExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Text, a.Text)
	case *ast.AndExpr:
		a, ok := actual.(*ast.AndExpr)
		require.True(t, ok, "expected AndExpr")
		require.Len(t, a.Operands, len(e.Operands))
		for i := range e.Operands {
			assertFilterExprEqual(t, e.Operands[i], a.Operands[i])
		}
	case *ast.OrExpr:
		a, ok := actual.(*ast.OrExpr)
		require.True(t, ok, "expected OrExpr")
		require.Len(t, a.Operands, len(e.Operands))
		for i := range e.Operands {
			assertFilterExprEqual(t, e.Operands[i], a.Operands[i])
		}
	case *ast.NotExpr:
		a, ok := actual.(*ast.NotExpr)
		require.True(t, ok, "expected NotExpr")
		assertFilterExprEqual(t, e.Operand, a.Operand)
	default:
		t.Fatalf("unexpected type %T", expected)
	}
}
