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

	var embedDirectives []ast.EmbedDirective
	if p.peek().Kind == lexer.TokenKindIdentifier && asciiEqual(p.peek().Value, "EMBED") {
		embedDirectives, err = p.parseEmbedClause()
		if err != nil {
			return nil, err
		}
	}

	return &ast.InsertStmt{
		Collection:      collection,
		ValuesList:      valuesList,
		Model:           model,
		Hybrid:          hybrid,
		SparseModel:     sparseModel,
		DenseVector:     denseVector,
		SparseVector:    sparseVector,
		EmbedDirectives: embedDirectives,
	}, nil
}

// parseEmbedClause parses: EMBED field INTO vector [, field INTO vector ...]
// Each directive optionally carries USING MODEL '...' or USING SPARSE [MODEL '...'].
func (p *Parser) parseEmbedClause() ([]ast.EmbedDirective, error) {
	p.advance() // consume EMBED

	var directives []ast.EmbedDirective
	for {
		sourceField, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindInto); err != nil {
			return nil, err
		}
		targetVector, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		dir := ast.EmbedDirective{
			SourceField:  sourceField,
			TargetVector: targetVector,
		}

		// Optional: USING MODEL '...' or USING SPARSE [MODEL '...']
		if p.peek().Kind == lexer.TokenKindUsing {
			p.advance()
			if p.peek().Kind == lexer.TokenKindSparse {
				p.advance()
				sm, _ := p.parseOptionalModelString()
				// Always set SparseModel pointer to mark this as a sparse directive,
				// even when no explicit MODEL is provided.
				if sm == nil {
					empty := ""
					dir.SparseModel = &empty
				} else {
					dir.SparseModel = sm
				}
			} else if p.peek().Kind == lexer.TokenKindModel {
				p.advance() // consume MODEL
				m, err := p.parseStringPtr()
				if err != nil {
					return nil, err
				}
				dir.Model = m
			}
		}

		directives = append(directives, dir)

		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
			continue
		}
		break
	}

	if len(directives) == 0 {
		return nil, errors.NewQQLSyntaxError("EMBED requires at least one directive", p.peek().Pos)
	}
	return directives, nil
}
