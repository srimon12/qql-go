package parser

import (
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/errors"
	"github.com/srimon12/qql-go/internal/lexer"
)

func (p *Parser) parseUpdate() (ast.ASTNode, error) {
	p.advance()
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindSet); err != nil {
		return nil, err
	}

	switch p.peek().Kind {
	case lexer.TokenKindVector:
		p.advance()
		if _, err := p.expect(lexer.TokenKindWhere); err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindId); err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindEquals); err != nil {
			return nil, err
		}
		pointID, err := p.parsePointIDValue("UPDATE SET VECTOR")
		if err != nil {
			return nil, err
		}
		vectorValue, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		rawVector, ok := vectorValue.([]any)
		if !ok {
			return nil, errors.NewQQLSyntaxError("Expected a vector list [...] after point ID in UPDATE SET VECTOR", p.peek().Pos)
		}
		vector, err := coerceVectorValues(rawVector)
		if err != nil {
			return nil, err
		}
		return &ast.UpdateVectorStmt{
			Collection: collection,
			PointID:    pointID,
			Vector:     vector,
		}, nil
	case lexer.TokenKindPayload:
		p.advance()
		if _, err := p.expect(lexer.TokenKindWhere); err != nil {
			return nil, err
		}
		if p.peek().Kind == lexer.TokenKindId {
			p.advance()
			if _, err := p.expect(lexer.TokenKindEquals); err != nil {
				return nil, err
			}
			pointID, err := p.parsePointIDValue("UPDATE SET PAYLOAD")
			if err != nil {
				return nil, err
			}
			payload, err := p.parseDict()
			if err != nil {
				return nil, err
			}
			return &ast.UpdatePayloadStmt{
				Collection: collection,
				PointID:    pointID,
				Payload:    payload,
			}, nil
		}
		queryFilter, err := p.parseFilterExpr()
		if err != nil {
			return nil, err
		}
		payload, err := p.parseDict()
		if err != nil {
			return nil, err
		}
		return &ast.UpdatePayloadStmt{
			Collection:  collection,
			QueryFilter: queryFilter,
			Payload:     payload,
		}, nil
	default:
		tok := p.peek()
		return nil, errors.NewQQLSyntaxError("Expected VECTOR or PAYLOAD after SET, got '"+tok.Value+"'", tok.Pos)
	}
}

func (p *Parser) parseDelete() (*ast.DeleteStmt, error) {
	p.advance()
	if _, err := p.expect(lexer.TokenKindFrom); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindWhere); err != nil {
		return nil, err
	}
	tok := p.peek()
	if tok.Kind == lexer.TokenKindId {
		p.advance()
		if _, err := p.expect(lexer.TokenKindEquals); err != nil {
			return nil, err
		}
		pointID, err := p.parsePointIDValue("DELETE")
		if err != nil {
			return nil, err
		}
		return &ast.DeleteStmt{Collection: collection, PointID: pointID}, nil
	}
	field, err := p.parseFieldPath()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindEquals); err != nil {
		return nil, err
	}
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	return &ast.DeleteStmt{Collection: collection, Field: field, Value: value}, nil
}
