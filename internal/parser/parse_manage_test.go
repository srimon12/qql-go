package parser

import (
	"testing"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCreateRejectsAlterOnlyParamsCaseInsensitive(t *testing.T) {
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize("CREATE COLLECTION docs WITH PARAMS { Read_Fan_Out_Factor: 4 }")
	require.NoError(t, err)

	p := NewParser()
	_, err = p.Parse(tokens)
	require.Error(t, err)
	require.Contains(t, err.Error(), "supported only for ALTER COLLECTION")
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

func TestParseShowCollection(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "simple",
			input: "SHOW COLLECTION docs",
			want:  "docs",
		},
		{
			name:  "case insensitive",
			input: "show collection MY_COL",
			want:  "MY_COL",
		},
		{
			name:    "error without collection name",
			input:   "SHOW COLLECTION",
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
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			stmt, ok := node.(*ast.ShowCollectionStmt)
			assert.True(t, ok, "expected ShowCollectionStmt")
			assert.Equal(t, tt.want, stmt.Collection)
		})
	}
}
