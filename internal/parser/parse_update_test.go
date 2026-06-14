package parser

import (
	"testing"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestParseUpdate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, node ast.ASTNode)
	}{
		{
			name:  "update vector by id",
			input: "UPDATE articles SET VECTOR WHERE id = 42 [0.1, 0.2]",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.UpdateVectorStmt)
				require.True(t, ok)
				assert.Equal(t, "articles", stmt.Collection)
				assert.Equal(t, 42, stmt.PointID)
				assert.Equal(t, []float32{0.1, 0.2}, stmt.Vector)
			},
		},
		{
			name:  "update payload by filter",
			input: "UPDATE articles SET PAYLOAD WHERE category = 'draft' {'status': 'published'}",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.UpdatePayloadStmt)
				require.True(t, ok)
				assert.Equal(t, "articles", stmt.Collection)
				assert.Nil(t, stmt.PointID)
				assertFilterExprEqual(t, &ast.CompareExpr{Field: "category", Op: "=", Value: "draft"}, stmt.QueryFilter)
				assert.Equal(t, map[string]any{"status": "published"}, stmt.Payload)
			},
		},
		{
			name:  "update payload by id",
			input: "UPDATE articles SET PAYLOAD WHERE id = 'abc-123' {'year': 2025}",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.UpdatePayloadStmt)
				require.True(t, ok)
				assert.Equal(t, "abc-123", stmt.PointID)
				assert.Equal(t, map[string]any{"year": 2025}, stmt.Payload)
			},
		},
		{
			name:    "update vector rejects bools",
			input:   "UPDATE articles SET VECTOR WHERE id = 1 [true, 0.2]",
			wantErr: true,
		},
		{
			name:    "update rejects invalid target",
			input:   "UPDATE articles SET NAME WHERE id = 1 {'x': 1}",
			wantErr: true,
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
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.check(t, node)
		})
	}
}
