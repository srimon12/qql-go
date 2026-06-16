package qql

import (
	"fmt"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/srimon12/qql-go/internal/parser"
)

// Parse turns a QQL string into an AST node.
// No Qdrant client is needed — this is pure syntax analysis.
//
// The returned node is one of:
//   - *ast.QueryStmt
//   - *ast.CreateCollectionStmt
//   - *ast.InsertStmt
//   - *ast.DeleteStmt
//   - *ast.ShowCollectionsStmt
//   - *ast.ShowCollectionStmt
//   - *ast.SelectStmt
//   - *ast.ScrollStmt
//   - etc.
func Parse(input string) (ast.ASTNode, error) {
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	if err != nil {
		return nil, fmt.Errorf("tokenization error: %w", err)
	}

	p := parser.NewParser()
	node, err := p.Parse(tokens)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return node, nil
}
