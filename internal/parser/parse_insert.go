package parser

import (
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/errors"
	"github.com/srimon12/qql-go/internal/lexer"
)

func (p *Parser) parseInsert() (ast.ASTNode, error) {
	p.advance()
	if _, err := p.expect(lexer.TokenKindInto); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindValues); err != nil {
		return nil, err
	}

	// Parse comma-separated dicts: {'a': 1}, {'b': 2}
	var valuesList []map[string]any
	for {
		dict, err := p.parsePayloadDict()
		if err != nil {
			return nil, err
		}
		valuesList = append(valuesList, dict)
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
			continue
		}
		break
	}
	if len(valuesList) == 0 {
		return nil, errors.NewQQLSyntaxError("INSERT VALUES requires at least one row", p.peek().Pos)
	}

	model, hybrid, sparseModel, denseVector, sparseVector, err := p.parseEmbeddingOptions()
	if err != nil {
		return nil, err
	}
	return &ast.InsertStmt{
		Collection:   collection,
		ValuesList:   valuesList,
		Model:        model,
		Hybrid:       hybrid,
		SparseModel:  sparseModel,
		DenseVector:  denseVector,
		SparseVector: sparseVector,
	}, nil
}
