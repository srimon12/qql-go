package parser

import (
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
			input: `INSERT INTO test VALUES {"text": "hello"}`,
			want: &ast.InsertStmt{
				Collection: "test",
				ValuesList: []map[string]any{{"text": "hello"}},
			},
		},
		{
			name:  "insert with bare keys",
			input: `INSERT INTO test VALUES {text: 'hello', topic: 'search'}`,
			want: &ast.InsertStmt{
				Collection: "test",
				ValuesList: []map[string]any{{"text": "hello", "topic": "search"}},
			},
		},
		{
			name:  "insert with explicit id",
			input: `INSERT INTO test VALUES {id: 'point-123', text: 'hello'}`,
			want: &ast.InsertStmt{
				Collection: "test",
				ValuesList: []map[string]any{{"id": "point-123", "text": "hello"}},
			},
		},
		{
			name:  "insert with model",
			input: `INSERT INTO test VALUES {"text": "hello"} USING MODEL 'model-name'`,
			want: &ast.InsertStmt{
				Collection: "test",
				ValuesList: []map[string]any{{"text": "hello"}},
				Model:      strPtr("model-name"),
			},
		},
		{
			name:  "insert with hybrid",
			input: `INSERT INTO test VALUES {"text": "hello"} USING HYBRID`,
			want: &ast.InsertStmt{
				Collection: "test",
				ValuesList: []map[string]any{{"text": "hello"}},
				Hybrid:     true,
			},
		},
		{
			name:  "insert with hybrid and models",
			input: `INSERT INTO test VALUES {"text": "hello"} USING HYBRID DENSE MODEL 'dense-model' SPARSE MODEL 'sparse-model'`,
			want: &ast.InsertStmt{
				Collection:  "test",
				ValuesList:  []map[string]any{{"text": "hello"}},
				Hybrid:      true,
				Model:       strPtr("dense-model"),
				SparseModel: strPtr("sparse-model"),
			},
		},
		{
			name:  "insert with sparse model only",
			input: `INSERT INTO test VALUES {"text": "hello"} USING HYBRID SPARSE MODEL 'sparse-model'`,
			want: &ast.InsertStmt{
				Collection:  "test",
				ValuesList:  []map[string]any{{"text": "hello"}},
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
			assert.Equal(t, tt.want.ValuesList, stmt.ValuesList)
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

func TestParseInsertBulk(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *ast.InsertStmt
	}{
		{
			name:  "comma separated bulk insert",
			input: `INSERT INTO test VALUES {"text": "hello"}, {"text": "world"}`,
			want: &ast.InsertStmt{
				Collection: "test",
				ValuesList: []map[string]any{
					{"text": "hello"},
					{"text": "world"},
				},
			},
		},
		{
			name:  "bulk insert with hybrid models",
			input: `INSERT INTO test VALUES {"text": "hello"} USING HYBRID DENSE MODEL 'dense-model' SPARSE MODEL 'sparse-model'`,
			want: &ast.InsertStmt{
				Collection: "test",
				ValuesList: []map[string]any{
					{"text": "hello"},
				},
				Hybrid:      true,
				Model:       strPtr("dense-model"),
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
			require.NoError(t, err)

			stmt, ok := node.(*ast.InsertStmt)
			require.True(t, ok, "expected InsertStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.ValuesList, stmt.ValuesList)
			assert.Equal(t, tt.want.Hybrid, stmt.Hybrid)
			if tt.want.Model != nil {
				require.NotNil(t, stmt.Model)
				assert.Equal(t, *tt.want.Model, *stmt.Model)
			}
			if tt.want.SparseModel != nil {
				require.NotNil(t, stmt.SparseModel)
				assert.Equal(t, *tt.want.SparseModel, *stmt.SparseModel)
			}
		})
	}
}
