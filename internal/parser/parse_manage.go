package parser

import (
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/errors"
	"github.com/srimon12/qql-go/internal/lexer"
)

func (p *Parser) parseDrop() (*ast.DropCollectionStmt, error) {
	p.advance()
	if _, err := p.expect(lexer.TokenKindCollection); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	return &ast.DropCollectionStmt{Collection: collection}, nil
}

func (p *Parser) parseShow() (ast.ASTNode, error) {
	p.advance()
	if p.peek().Kind == lexer.TokenKindCollections {
		p.advance()
		return &ast.ShowCollectionsStmt{}, nil
	}
	if _, err := p.expect(lexer.TokenKindCollection); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	return &ast.ShowCollectionStmt{Collection: collection}, nil
}

func (p *Parser) parseAlter() (*ast.AlterCollectionStmt, error) {
	p.advance()
	if _, err := p.expect(lexer.TokenKindCollection); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	config, err := p.parseCollectionConfigBlocks(true)
	if err != nil {
		return nil, err
	}
	quantization, err := p.parseOptionalAlterQuantization()
	if err != nil {
		return nil, err
	}
	if config == nil && quantization == nil {
		return nil, errors.NewQQLSyntaxError(
			"ALTER COLLECTION requires at least one WITH HNSW/VECTORS/OPTIMIZERS/PARAMS clause or QUANTIZE clause",
			p.peek().Pos,
		)
	}
	return &ast.AlterCollectionStmt{
		Collection:   collection,
		Config:       config,
		Quantization: quantization,
	}, nil
}
