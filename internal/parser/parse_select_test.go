package parser

import (
	"testing"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSelect(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.SelectStmt
		wantErr bool
	}{
		{
			name:  "select with string id",
			input: `SELECT * FROM docs WHERE id = 'point-123'`,
			want: &ast.SelectStmt{
				Collection: "docs",
				PointID:    "point-123",
			},
		},
		{
			name:  "select with integer id",
			input: `SELECT * FROM docs WHERE id = 42`,
			want: &ast.SelectStmt{
				Collection: "docs",
				PointID:    42,
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
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, ok := node.(*ast.SelectStmt)
			require.True(t, ok, "expected SelectStmt, got %T", node)
			assert.Equal(t, tt.want.Collection, got.Collection)
			assert.Equal(t, tt.want.PointID, got.PointID)
		})
	}
}

func TestParseScroll(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.ScrollStmt
		wantErr bool
	}{
		{
			name:  "basic scroll",
			input: `SCROLL FROM docs LIMIT 10`,
			want: &ast.ScrollStmt{
				Collection: "docs",
				Limit:      10,
			},
		},
		{
			name:  "scroll with where",
			input: `SCROLL FROM docs WHERE status = 'active' LIMIT 5`,
			want: &ast.ScrollStmt{
				Collection:  "docs",
				Limit:       5,
				QueryFilter: &ast.CompareExpr{Field: "status", Op: "=", Value: "active"},
			},
		},
		{
			name:  "scroll with after",
			input: `SCROLL FROM docs AFTER 'point-123' LIMIT 10`,
			want: &ast.ScrollStmt{
				Collection: "docs",
				Limit:      10,
				After:      "point-123",
			},
		},
		{
			name:  "scroll with after integer",
			input: `SCROLL FROM docs AFTER 42 LIMIT 10`,
			want: &ast.ScrollStmt{
				Collection: "docs",
				Limit:      10,
				After:      42,
			},
		},
		{
			name:  "scroll with where and after",
			input: `SCROLL FROM docs WHERE status = 'active' AFTER 'point-50' LIMIT 20`,
			want: &ast.ScrollStmt{
				Collection:  "docs",
				Limit:       20,
				QueryFilter: &ast.CompareExpr{Field: "status", Op: "=", Value: "active"},
				After:       "point-50",
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
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, ok := node.(*ast.ScrollStmt)
			require.True(t, ok, "expected ScrollStmt, got %T", node)
			assert.Equal(t, tt.want.Collection, got.Collection)
			assert.Equal(t, tt.want.Limit, got.Limit)
			assert.Equal(t, tt.want.After, got.After)
			if tt.want.QueryFilter != nil {
				assertFilterExprEqual(t, tt.want.QueryFilter, got.QueryFilter)
			}
		})
	}
}
