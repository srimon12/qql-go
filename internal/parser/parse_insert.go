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
		dict, err := p.parseDict()
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

func extractInsertPointID(values map[string]any) (any, map[string]any) {
	var matches []string
	for key := range values {
		if toLowerEqual(key, "id") {
			matches = append(matches, key)
		}
	}
	if len(matches) == 0 {
		return nil, values
	}
	chosen := matches[0]
	value := values[chosen]
	delete(values, chosen)
	return value, values
}

func toLowerEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca := a[i]
		cb := b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}


