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

func TestParseInsertEmbed(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.InsertStmt
		wantErr bool
	}{
		{
			name:  "single embed directive",
			input: `INSERT INTO arxiv VALUES {id: 'p1', text: 'chunk', title: 'Paper'} EMBED text INTO dense_chunk`,
			want: &ast.InsertStmt{
				Collection: "arxiv",
				ValuesList: []map[string]any{{"id": "p1", "text": "chunk", "title": "Paper"}},
				EmbedDirectives: []ast.EmbedDirective{
					{SourceField: "text", TargetVector: "dense_chunk"},
				},
			},
		},
		{
			name:  "multiple embed directives",
			input: `INSERT INTO arxiv VALUES {id: 'p1', text: 'chunk', title: 'Paper', abstract: 'Full abstract'} EMBED text INTO dense_chunk, title INTO dense_title, abstract INTO dense_abstract`,
			want: &ast.InsertStmt{
				Collection: "arxiv",
				ValuesList: []map[string]any{{"id": "p1", "text": "chunk", "title": "Paper", "abstract": "Full abstract"}},
				EmbedDirectives: []ast.EmbedDirective{
					{SourceField: "text", TargetVector: "dense_chunk"},
					{SourceField: "title", TargetVector: "dense_title"},
					{SourceField: "abstract", TargetVector: "dense_abstract"},
				},
			},
		},
		{
			name:  "embed with sparse model",
			input: `INSERT INTO arxiv VALUES {id: 'p1', title: 'Paper'} EMBED title INTO sparse_title USING SPARSE`,
			want: &ast.InsertStmt{
				Collection: "arxiv",
				ValuesList: []map[string]any{{"id": "p1", "title": "Paper"}},
				EmbedDirectives: []ast.EmbedDirective{
					{SourceField: "title", TargetVector: "sparse_title", SparseModel: strPtr("")},
				},
			},
		},
		{
			name:  "embed with explicit model",
			input: `INSERT INTO arxiv VALUES {id: 'p1', title: 'Paper'} EMBED title INTO dense_title USING MODEL 'custom-model'`,
			want: &ast.InsertStmt{
				Collection: "arxiv",
				ValuesList: []map[string]any{{"id": "p1", "title": "Paper"}},
				EmbedDirectives: []ast.EmbedDirective{
					{SourceField: "title", TargetVector: "dense_title", Model: strPtr("custom-model")},
				},
			},
		},
		{
			name:  "embed mixed dense and sparse",
			input: `INSERT INTO arxiv VALUES {id: 'p1', text: 'chunk', title: 'Paper'} EMBED text INTO dense_chunk, title INTO sparse_title USING SPARSE`,
			want: &ast.InsertStmt{
				Collection: "arxiv",
				ValuesList: []map[string]any{{"id": "p1", "text": "chunk", "title": "Paper"}},
				EmbedDirectives: []ast.EmbedDirective{
					{SourceField: "text", TargetVector: "dense_chunk"},
					{SourceField: "title", TargetVector: "sparse_title", SparseModel: strPtr("")},
				},
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
			require.Len(t, stmt.EmbedDirectives, len(tt.want.EmbedDirectives))
			for i, want := range tt.want.EmbedDirectives {
				assert.Equal(t, want.SourceField, stmt.EmbedDirectives[i].SourceField)
				assert.Equal(t, want.TargetVector, stmt.EmbedDirectives[i].TargetVector)
				if want.Model != nil {
					require.NotNil(t, stmt.EmbedDirectives[i].Model)
					assert.Equal(t, *want.Model, *stmt.EmbedDirectives[i].Model)
				}
				if want.SparseModel != nil {
					require.NotNil(t, stmt.EmbedDirectives[i].SparseModel)
				}
			}
		})
	}
}
