package parser

import (
	"strconv"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/errors"
	"github.com/srimon12/qql-go/internal/lexer"
)

func (p *Parser) parseInsert() (ast.ASTNode, error) {
	p.advance()
	if p.peek().Kind == lexer.TokenKindBulk {
		p.advance()
		return p.parseInsertBulk()
	}
	if _, err := p.expect(lexer.TokenKindInto); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindCollection); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindValues); err != nil {
		return nil, err
	}
	values, err := p.parseDict()
	if err != nil {
		return nil, err
	}
	pointID, values := extractInsertPointID(values)
	model, hybrid, sparseModel, denseVector, sparseVector, err := p.parseEmbeddingOptions()
	if err != nil {
		return nil, err
	}
	return &ast.InsertStmt{
		Collection:   collection,
		PointID:      pointID,
		Values:       values,
		Model:        model,
		Hybrid:       hybrid,
		SparseModel:  sparseModel,
		DenseVector:  denseVector,
		SparseVector: sparseVector,
	}, nil
}

func (p *Parser) parseInsertBulk() (*ast.InsertBulkStmt, error) {
	if _, err := p.expect(lexer.TokenKindInto); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindCollection); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindValues); err != nil {
		return nil, err
	}
	rawItems, err := p.parseList()
	if err != nil {
		return nil, err
	}
	valuesList := make([]map[string]any, 0, len(rawItems))
	for idx, item := range rawItems {
		dict, ok := item.(map[string]any)
		if !ok {
			return nil, errors.NewQQLSyntaxError("INSERT BULK VALUES item at index "+strconv.Itoa(idx)+" must be a dict", p.peek().Pos)
		}
		valuesList = append(valuesList, dict)
	}
	model, hybrid, sparseModel, denseVector, sparseVector, err := p.parseEmbeddingOptions()
	if err != nil {
		return nil, err
	}
	return &ast.InsertBulkStmt{
		Collection:   collection,
		ValuesList:   valuesList,
		Model:        model,
		Hybrid:       hybrid,
		SparseModel:  sparseModel,
		DenseVector:  denseVector,
		SparseVector: sparseVector,
	}, nil
}

func extractInsertPointID(values map[string]any) (any, map[string]any) {
	for key, value := range values {
		if toLower(key) == "id" {
			delete(values, key)
			return value, values
		}
	}
	return nil, values
}
