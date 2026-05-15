package parser

import (
	"fmt"
	"strconv"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/errors"
	"github.com/srimon12/qql-go/internal/lexer"
)

type Parser struct {
	tokens []lexer.Token
	pos    int
}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(tokens []lexer.Token) (ast.ASTNode, error) {
	p.tokens = tokens
	p.pos = 0

	tok := p.peek()
	var node ast.ASTNode
	var err error
	switch tok.Kind {
	case lexer.TokenKindInsert:
		node, err = p.parseInsert()
	case lexer.TokenKindCreate:
		node, err = p.parseCreate()
	case lexer.TokenKindDrop:
		node, err = p.parseDrop()
	case lexer.TokenKindShow:
		node, err = p.parseShow()
	case lexer.TokenKindSearch:
		node, err = p.parseSearch()
	case lexer.TokenKindSelect:
		node, err = p.parseSelect()
	case lexer.TokenKindScroll:
		node, err = p.parseScroll()
	case lexer.TokenKindRecommend:
		node, err = p.parseRecommend()
	case lexer.TokenKindDelete:
		node, err = p.parseDelete()
	case lexer.TokenKindUpdate:
		node, err = p.parseUpdate()
	default:
		return nil, errors.NewQQLSyntaxError("Unexpected token '"+tok.Value+"'; expected a QQL statement keyword", tok.Pos)
	}

	if err != nil {
		return nil, err
	}

	if tok := p.peek(); tok.Kind != lexer.TokenKindEof {
		return nil, errors.NewQQLSyntaxError("Unexpected token '"+tok.Value+"'; expected end of input", tok.Pos)
	}

	return node, nil
}

func (p *Parser) peek() lexer.Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return lexer.Token{Kind: lexer.TokenKindEof, Value: "", Pos: -1}
}

func (p *Parser) advance() lexer.Token {
	tok := p.tokens[p.pos]
	if tok.Kind != lexer.TokenKindEof {
		p.pos++
	}
	return tok
}

func (p *Parser) expect(kind lexer.TokenKind) (lexer.Token, error) {
	tok := p.peek()
	if tok.Kind != kind {
		return tok, errors.NewQQLSyntaxError("Expected "+kind.String()+" but got '"+tok.Value+"'", tok.Pos)
	}
	return p.advance(), nil
}

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
	model, hybrid, sparseModel, err := p.parseEmbeddingOptions()
	if err != nil {
		return nil, err
	}
	return &ast.InsertStmt{
		Collection:  collection,
		PointID:     pointID,
		Values:      values,
		Model:       model,
		Hybrid:      hybrid,
		SparseModel: sparseModel,
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
	valuesList := make([]map[string]interface{}, 0, len(rawItems))
	for idx, item := range rawItems {
		dict, ok := item.(map[string]interface{})
		if !ok {
			return nil, errors.NewQQLSyntaxError("INSERT BULK VALUES item at index "+strconv.Itoa(idx)+" must be a dict", p.peek().Pos)
		}
		valuesList = append(valuesList, dict)
	}
	model, hybrid, sparseModel, err := p.parseEmbeddingOptions()
	if err != nil {
		return nil, err
	}
	return &ast.InsertBulkStmt{
		Collection:  collection,
		ValuesList:  valuesList,
		Model:       model,
		Hybrid:      hybrid,
		SparseModel: sparseModel,
	}, nil
}

func (p *Parser) parseCreate() (ast.ASTNode, error) {
	p.advance()
	tok := p.peek()
	if tok.Kind == lexer.TokenKindIndex {
		return p.parseCreateIndex()
	}
	if _, err := p.expect(lexer.TokenKindCollection); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	hybrid := false
	rerank := false
	var model *string
	var quantization *ast.QuantizationConfig
	if p.peek().Kind == lexer.TokenKindHybrid {
		p.advance()
		hybrid = true
		if p.peek().Kind == lexer.TokenKindRerank {
			p.advance()
			rerank = true
		}
	} else if p.peek().Kind == lexer.TokenKindUsing {
		p.advance()
		if p.peek().Kind == lexer.TokenKindHybrid {
			p.advance()
			hybrid = true
			if p.peek().Kind == lexer.TokenKindDense {
				p.advance()
				var err error
				model, err = p.parseRequiredModelString()
				if err != nil {
					return nil, err
				}
			}
		} else {
			var err error
			model, err = p.parseRequiredModelString()
			if err != nil {
				return nil, err
			}
		}
	}
	if p.peek().Kind == lexer.TokenKindQuantize {
		p.advance()
		var err error
		quantization, err = p.parseQuantizeClause()
		if err != nil {
			return nil, err
		}
	}
	return &ast.CreateCollectionStmt{
		Collection:   collection,
		Hybrid:       hybrid,
		Rerank:       rerank,
		Model:        model,
		Quantization: quantization,
	}, nil
}

func (p *Parser) parseQuantizeClause() (*ast.QuantizationConfig, error) {
	tok := p.peek()

	switch tok.Kind {
	case lexer.TokenKindScalar:
		p.advance()
		var quantile *float64
		alwaysRAM := false
		if p.peek().Kind == lexer.TokenKindQuantile {
			p.advance()
			valueTok := p.peek()
			value, err := p.parseNumericLiteral()
			if err != nil {
				return nil, err
			}
			if value < 0 || value > 1 {
				return nil, errors.NewQQLSyntaxError("QUANTILE must be between 0 and 1 inclusive, got '"+valueTok.Value+"'", valueTok.Pos)
			}
			quantile = &value
		}
		if p.peek().Kind == lexer.TokenKindAlways {
			p.advance()
			if _, err := p.expect(lexer.TokenKindRam); err != nil {
				return nil, err
			}
			alwaysRAM = true
		}
		return &ast.QuantizationConfig{
			Type:      ast.QuantizationTypeScalar,
			Quantile:  quantile,
			AlwaysRAM: alwaysRAM,
		}, nil
	case lexer.TokenKindBinary:
		p.advance()
		alwaysRAM := false
		if p.peek().Kind == lexer.TokenKindAlways {
			p.advance()
			if _, err := p.expect(lexer.TokenKindRam); err != nil {
				return nil, err
			}
			alwaysRAM = true
		}
		return &ast.QuantizationConfig{
			Type:      ast.QuantizationTypeBinary,
			AlwaysRAM: alwaysRAM,
		}, nil
	case lexer.TokenKindProduct:
		p.advance()
		alwaysRAM := false
		if p.peek().Kind == lexer.TokenKindAlways {
			p.advance()
			if _, err := p.expect(lexer.TokenKindRam); err != nil {
				return nil, err
			}
			alwaysRAM = true
		}
		return &ast.QuantizationConfig{
			Type:      ast.QuantizationTypeProduct,
			AlwaysRAM: alwaysRAM,
		}, nil
	case lexer.TokenKindTurbo:
		p.advance()
		var turboBits *float64
		alwaysRAM := false
		if p.peek().Kind == lexer.TokenKindBits {
			p.advance()
			bitsTok := p.peek()
			raw, err := p.parseNumericLiteral()
			if err != nil {
				return nil, err
			}
			if raw != 1.0 && raw != 1.5 && raw != 2.0 && raw != 4.0 {
				return nil, errors.NewQQLSyntaxError("BITS must be one of 1, 1.5, 2, or 4 for TURBO quantization, got '"+bitsTok.Value+"'", bitsTok.Pos)
			}
			turboBits = &raw
		}
		if p.peek().Kind == lexer.TokenKindAlways {
			p.advance()
			if _, err := p.expect(lexer.TokenKindRam); err != nil {
				return nil, err
			}
			alwaysRAM = true
		}
		return &ast.QuantizationConfig{
			Type:      ast.QuantizationTypeTurbo,
			TurboBits: turboBits,
			AlwaysRAM: alwaysRAM,
		}, nil
	default:
		return nil, errors.NewQQLSyntaxError("Expected SCALAR, BINARY, PRODUCT, or TURBO after QUANTIZE, got '"+tok.Value+"'", tok.Pos)
	}
}

func (p *Parser) parseCreateIndex() (*ast.CreateIndexStmt, error) {
	p.advance()
	if _, err := p.expect(lexer.TokenKindOn); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindCollection); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindFor); err != nil {
		return nil, err
	}
	field, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	fieldType := "keyword"
	if p.peek().Kind == lexer.TokenKindType {
		p.advance()
		typeTok, err := p.expect(lexer.TokenKindIdentifier)
		if err != nil {
			return nil, err
		}
		fieldType = toLower(typeTok.Value)
	}
	return &ast.CreateIndexStmt{
		Collection: collection,
		Field:      field,
		FieldType:  fieldType,
	}, nil
}

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

func (p *Parser) parseSearch() (*ast.SearchStmt, error) {
	p.advance()
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindSimilar); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindTo); err != nil {
		return nil, err
	}
	queryTok, err := p.expect(lexer.TokenKindString)
	if err != nil {
		return nil, err
	}
	queryText := queryTok.Value
	if _, err := p.expect(lexer.TokenKindLimit); err != nil {
		return nil, err
	}
	limitTok, err := p.expect(lexer.TokenKindInteger)
	if err != nil {
		return nil, err
	}
	limit, err := parseIntToken(limitTok)
	if err != nil {
		return nil, err
	}

	model, hybrid, sparseOnly, sparseModel, fusion, err := p.parseSearchEmbeddingOptions()
	if err != nil {
		return nil, err
	}

	var queryFilter ast.FilterExpr
	rerank := false
	var rerankModel *string
	var withClause *ast.SearchWith
	groupBy := ""
	groupSize := 0
	seenWhere := false
	seenRerank := false
	seenWith := false
	seenGroup := false

	for {
		switch p.peek().Kind {
		case lexer.TokenKindWhere:
			if seenWhere {
				return nil, errors.NewQQLSyntaxError("Duplicate WHERE clause", p.peek().Pos)
			}
			seenWhere = true
			p.advance()
			queryFilter, err = p.parseFilterExpr()
			if err != nil {
				return nil, err
			}
		case lexer.TokenKindRerank:
			if seenRerank {
				return nil, errors.NewQQLSyntaxError("Duplicate RERANK clause", p.peek().Pos)
			}
			if seenGroup {
				return nil, errors.NewQQLSyntaxError("GROUP BY and RERANK cannot be combined in the same SEARCH statement", p.peek().Pos)
			}
			seenRerank = true
			p.advance()
			rerank = true
			rerankModel, err = p.parseOptionalModelString()
			if err != nil {
				return nil, err
			}
		case lexer.TokenKindExact:
			p.advance()
			ensureSearchWith(&withClause).Exact = true
		case lexer.TokenKindWith:
			if seenWith {
				return nil, errors.NewQQLSyntaxError("Duplicate WITH clause", p.peek().Pos)
			}
			seenWith = true
			p.advance()
			parsedWith, err := p.parseWithClause()
			if err != nil {
				return nil, err
			}
			mergeSearchWith(&withClause, parsedWith)
		case lexer.TokenKindGroup:
			if seenGroup {
				return nil, errors.NewQQLSyntaxError("Duplicate GROUP BY clause", p.peek().Pos)
			}
			if rerank {
				return nil, errors.NewQQLSyntaxError("GROUP BY and RERANK cannot be combined in the same SEARCH statement", p.peek().Pos)
			}
			seenGroup = true
			p.advance()
			if _, err := p.expect(lexer.TokenKindBy); err != nil {
				return nil, err
			}
			groupBy, err = p.parseFieldPath()
			if err != nil {
				return nil, err
			}
			groupSize = 3
			if p.peek().Kind == lexer.TokenKindGroupSize {
				p.advance()
				groupTok, err := p.expect(lexer.TokenKindInteger)
				if err != nil {
					return nil, err
				}
				groupSize, err = parseIntToken(groupTok)
				if err != nil {
					return nil, err
				}
				if groupSize <= 0 {
					return nil, errors.NewQQLSyntaxError("GROUP_SIZE must be a positive integer, got "+groupTok.Value, groupTok.Pos)
				}
			}
		default:
			return &ast.SearchStmt{
				Collection:  collection,
				QueryText:   queryText,
				Limit:       limit,
				Model:       model,
				Hybrid:      hybrid,
				Fusion:      fusion,
				SparseOnly:  sparseOnly,
				SparseModel: sparseModel,
				QueryFilter: queryFilter,
				Rerank:      rerank,
				RerankModel: rerankModel,
				WithClause:  withClause,
				GroupBy:     groupBy,
				GroupSize:   groupSize,
			}, nil
		}
	}
}

func (p *Parser) parseRecommend() (*ast.RecommendStmt, error) {
	p.advance()
	if _, err := p.expect(lexer.TokenKindFrom); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindPositive); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindIds); err != nil {
		return nil, err
	}
	positiveIDs, err := p.parsePointIDList()
	if err != nil {
		return nil, err
	}
	var negativeIDs []interface{}
	if p.peek().Kind == lexer.TokenKindNegative {
		p.advance()
		if _, err := p.expect(lexer.TokenKindIds); err != nil {
			return nil, err
		}
		negativeIDs, err = p.parsePointIDList()
		if err != nil {
			return nil, err
		}
	}
	var strategy *string
	if p.peek().Kind == lexer.TokenKindStrategy {
		p.advance()
		strategy, err = p.parseStringPtr()
		if err != nil {
			return nil, err
		}
	}
	var lookupFrom string
	var lookupVector *string
	if p.peek().Kind == lexer.TokenKindLookup {
		p.advance()
		if _, err := p.expect(lexer.TokenKindFrom); err != nil {
			return nil, err
		}
		lookupFrom, err = p.parseIdentifier()
		if err != nil {
			return nil, err
		}
		if p.peek().Kind == lexer.TokenKindVector || (p.peek().Kind == lexer.TokenKindIdentifier && toUpper(p.peek().Value) == "VECTOR") {
			p.advance()
			lookupVector, err = p.parseStringPtr()
			if err != nil {
				return nil, err
			}
		}
	}
	var using *string
	if p.peek().Kind == lexer.TokenKindUsing {
		p.advance()
		using, err = p.parseStringPtr()
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(lexer.TokenKindLimit); err != nil {
		return nil, err
	}
	limitTok, err := p.expect(lexer.TokenKindInteger)
	if err != nil {
		return nil, err
	}
	limit, err := parseIntToken(limitTok)
	if err != nil {
		return nil, err
	}
	var offset int
	if p.peek().Kind == lexer.TokenKindOffset {
		p.advance()
		offsetTok, err := p.expect(lexer.TokenKindInteger)
		if err != nil {
			return nil, err
		}
		offset, err = parseIntToken(offsetTok)
		if err != nil {
			return nil, err
		}
	}
	var scoreThreshold *float64
	if p.peek().Kind == lexer.TokenKindScore {
		p.advance()
		if _, err := p.expect(lexer.TokenKindThreshold); err != nil {
			return nil, err
		}
		scoreTok := p.peek()
		if scoreTok.Kind == lexer.TokenKindFloat {
			p.advance()
			f, err := parseFloatToken(scoreTok)
			if err != nil {
				return nil, err
			}
			scoreThreshold = &f
		} else if scoreTok.Kind == lexer.TokenKindInteger {
			p.advance()
			v, err := parseIntToken(scoreTok)
			if err != nil {
				return nil, err
			}
			f := float64(v)
			scoreThreshold = &f
		} else {
			return nil, errors.NewQQLSyntaxError("Expected float or integer for SCORE THRESHOLD, got '"+scoreTok.Value+"'", scoreTok.Pos)
		}
	}
	var queryFilter ast.FilterExpr
	if p.peek().Kind == lexer.TokenKindWhere {
		p.advance()
		queryFilter, err = p.parseFilterExpr()
		if err != nil {
			return nil, err
		}
	}
	var withClause *ast.SearchWith
	if p.peek().Kind == lexer.TokenKindWith {
		p.advance()
		withClause, err = p.parseWithClause()
		if err != nil {
			return nil, err
		}
	}
	return &ast.RecommendStmt{
		Collection:     collection,
		PositiveIDs:    positiveIDs,
		NegativeIDs:    negativeIDs,
		Limit:          limit,
		Strategy:       strategy,
		QueryFilter:    queryFilter,
		Offset:         offset,
		ScoreThreshold: scoreThreshold,
		WithClause:     withClause,
		LookupFrom:     lookupFrom,
		LookupVector:   lookupVector,
		Using:          using,
	}, nil
}

func (p *Parser) parseSelect() (*ast.SelectStmt, error) {
	p.advance()
	if _, err := p.expect(lexer.TokenKindStar); err != nil {
		return nil, err
	}
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
	if _, err := p.expect(lexer.TokenKindId); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindEquals); err != nil {
		return nil, err
	}
	pointID, err := p.parsePointIDValue("SELECT")
	if err != nil {
		return nil, err
	}
	return &ast.SelectStmt{Collection: collection, PointID: pointID}, nil
}

func (p *Parser) parseScroll() (*ast.ScrollStmt, error) {
	p.advance()
	if _, err := p.expect(lexer.TokenKindFrom); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	var queryFilter ast.FilterExpr
	if p.peek().Kind == lexer.TokenKindWhere {
		p.advance()
		queryFilter, err = p.parseFilterExpr()
		if err != nil {
			return nil, err
		}
	}
	var after interface{}
	if p.peek().Kind == lexer.TokenKindAfter {
		p.advance()
		after, err = p.parsePointIDValue("SCROLL AFTER")
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(lexer.TokenKindLimit); err != nil {
		return nil, err
	}
	limitTok, err := p.expect(lexer.TokenKindInteger)
	if err != nil {
		return nil, err
	}
	limit, err := parseIntToken(limitTok)
	if err != nil {
		return nil, err
	}
	return &ast.ScrollStmt{
		Collection:  collection,
		Limit:       limit,
		QueryFilter: queryFilter,
		After:       after,
	}, nil
}

func (p *Parser) parsePointIDValue(statement string) (interface{}, error) {
	tok := p.peek()
	if tok.Kind == lexer.TokenKindString {
		p.advance()
		return tok.Value, nil
	}
	if tok.Kind == lexer.TokenKindInteger {
		p.advance()
		return parseIntToken(tok)
	}
	return nil, errors.NewQQLSyntaxError(statement+" requires a string or integer point id, got '"+tok.Value+"'", tok.Pos)
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
		tok := p.peek()
		var pointID interface{}
		if tok.Kind == lexer.TokenKindString {
			p.advance()
			pointID = tok.Value
		} else if tok.Kind == lexer.TokenKindInteger {
			p.advance()
			v, err := parseIntToken(tok)
			if err != nil {
				return nil, err
			}
			pointID = v
		} else {
			return nil, errors.NewQQLSyntaxError("Expected string or integer for point id, got '"+tok.Value+"'", tok.Pos)
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
		rawVector, ok := vectorValue.([]interface{})
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

func (p *Parser) parseFilterExpr() (ast.FilterExpr, error) {
	left, err := p.parseFilterAnd()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind != lexer.TokenKindOr {
		return left, nil
	}
	operands := []ast.FilterExpr{left}
	for p.peek().Kind == lexer.TokenKindOr {
		p.advance()
		right, err := p.parseFilterAnd()
		if err != nil {
			return nil, err
		}
		operands = append(operands, right)
	}
	return &ast.OrExpr{Operands: operands}, nil
}

func (p *Parser) parseFilterAnd() (ast.FilterExpr, error) {
	left, err := p.parseFilterNot()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind != lexer.TokenKindAnd {
		return left, nil
	}
	operands := []ast.FilterExpr{left}
	for p.peek().Kind == lexer.TokenKindAnd {
		p.advance()
		right, err := p.parseFilterNot()
		if err != nil {
			return nil, err
		}
		operands = append(operands, right)
	}
	return &ast.AndExpr{Operands: operands}, nil
}

func (p *Parser) parseFilterNot() (ast.FilterExpr, error) {
	if p.peek().Kind == lexer.TokenKindNot {
		p.advance()
		operand, err := p.parseFilterNot()
		if err != nil {
			return nil, err
		}
		return &ast.NotExpr{Operand: operand}, nil
	}
	return p.parseFilterPrimary()
}

func (p *Parser) parseFilterPrimary() (ast.FilterExpr, error) {
	if p.peek().Kind == lexer.TokenKindLparen {
		p.advance()
		expr, err := p.parseFilterExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindRparen); err != nil {
			return nil, err
		}
		return expr, nil
	}
	return p.parsePredicate()
}

func (p *Parser) parsePredicate() (ast.FilterExpr, error) {
	field, err := p.parseFieldPath()
	if err != nil {
		return nil, err
	}
	tok := p.peek()

	if tok.Kind == lexer.TokenKindIs {
		p.advance()
		if p.peek().Kind == lexer.TokenKindNot {
			p.advance()
			if p.peek().Kind == lexer.TokenKindNull {
				p.advance()
				return &ast.IsNotNullExpr{Field: field}, nil
			}
			if p.peek().Kind == lexer.TokenKindEmpty {
				p.advance()
				return &ast.IsNotEmptyExpr{Field: field}, nil
			}
			return nil, errors.NewQQLSyntaxError("Expected NULL or EMPTY after IS NOT", p.peek().Pos)
		}
		if p.peek().Kind == lexer.TokenKindNull {
			p.advance()
			return &ast.IsNullExpr{Field: field}, nil
		}
		if p.peek().Kind == lexer.TokenKindEmpty {
			p.advance()
			return &ast.IsEmptyExpr{Field: field}, nil
		}
		return nil, errors.NewQQLSyntaxError("Expected NULL, NOT NULL, EMPTY, or NOT EMPTY after IS", p.peek().Pos)
	}

	if tok.Kind == lexer.TokenKindIn {
		p.advance()
		values, err := p.parseLiteralList()
		if err != nil {
			return nil, err
		}
		return &ast.InExpr{Field: field, Values: values}, nil
	}

	if tok.Kind == lexer.TokenKindNot {
		p.advance()
		if _, err := p.expect(lexer.TokenKindIn); err != nil {
			return nil, err
		}
		values, err := p.parseLiteralList()
		if err != nil {
			return nil, err
		}
		return &ast.NotInExpr{Field: field, Values: values}, nil
	}

	if tok.Kind == lexer.TokenKindBetween {
		p.advance()
		low, err := p.parseNumber()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindAnd); err != nil {
			return nil, err
		}
		high, err := p.parseNumber()
		if err != nil {
			return nil, err
		}
		return &ast.BetweenExpr{Field: field, Low: low, High: high}, nil
	}

	if tok.Kind == lexer.TokenKindMatch {
		p.advance()
		if p.peek().Kind == lexer.TokenKindAny {
			p.advance()
			textTok, err := p.expect(lexer.TokenKindString)
			if err != nil {
				return nil, err
			}
			return &ast.MatchAnyExpr{Field: field, Text: textTok.Value}, nil
		}
		if p.peek().Kind == lexer.TokenKindPhrase {
			p.advance()
			textTok, err := p.expect(lexer.TokenKindString)
			if err != nil {
				return nil, err
			}
			return &ast.MatchPhraseExpr{Field: field, Text: textTok.Value}, nil
		}
		textTok, err := p.expect(lexer.TokenKindString)
		if err != nil {
			return nil, err
		}
		return &ast.MatchTextExpr{Field: field, Text: textTok.Value}, nil
	}

	op := tokenKindToOp(tok.Kind)
	if op != "" {
		p.advance()
		value, err := p.parseLiteral()
		if err != nil {
			return nil, err
		}
		return &ast.CompareExpr{Field: field, Op: op, Value: value}, nil
	}

	return nil, errors.NewQQLSyntaxError("Expected a filter operator after field '"+field+"', got '"+tok.Value+"'", tok.Pos)
}

func (p *Parser) parseFieldPath() (string, error) {
	tok := p.peek()
	if tok.Kind != lexer.TokenKindIdentifier && !isContextualFieldName(tok.Kind) {
		return "", errors.NewQQLSyntaxError("Expected a field name, got '"+tok.Value+"'", tok.Pos)
	}
	p.advance()
	return tok.Value, nil
}

func (p *Parser) parseLiteral() (interface{}, error) {
	tok := p.peek()
	switch tok.Kind {
	case lexer.TokenKindString:
		p.advance()
		return tok.Value, nil
	case lexer.TokenKindInteger:
		p.advance()
		return parseIntToken(tok)
	case lexer.TokenKindFloat:
		p.advance()
		return parseFloatToken(tok)
	}
	return nil, errors.NewQQLSyntaxError("Expected a literal value (string, integer, or float), got '"+tok.Value+"'", tok.Pos)
}

func (p *Parser) parseNumber() (interface{}, error) {
	tok := p.peek()
	switch tok.Kind {
	case lexer.TokenKindInteger:
		p.advance()
		return parseIntToken(tok)
	case lexer.TokenKindFloat:
		p.advance()
		return parseFloatToken(tok)
	}
	return nil, errors.NewQQLSyntaxError("Expected a number, got '"+tok.Value+"'", tok.Pos)
}

func (p *Parser) parseLiteralList() ([]interface{}, error) {
	if _, err := p.expect(lexer.TokenKindLparen); err != nil {
		return nil, err
	}
	var items []interface{}
	if p.peek().Kind == lexer.TokenKindRparen {
		p.advance()
		return items, nil
	}
	for {
		val, err := p.parseLiteral()
		if err != nil {
			return nil, err
		}
		items = append(items, val)
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
			if p.peek().Kind == lexer.TokenKindRparen {
				break
			}
		} else {
			break
		}
	}
	if _, err := p.expect(lexer.TokenKindRparen); err != nil {
		return nil, err
	}
	return items, nil
}

func (p *Parser) parsePointIDList() ([]interface{}, error) {
	values, err := p.parseLiteralList()
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		switch value.(type) {
		case string, int:
		default:
			return nil, errors.NewQQLSyntaxError("Point ids must be strings or integers", p.peek().Pos)
		}
	}
	return values, nil
}

func (p *Parser) parseIdentifier() (string, error) {
	tok := p.peek()
	if tok.Kind == lexer.TokenKindIdentifier || tok.Kind == lexer.TokenKindString || isContextualIdentifier(tok.Kind) {
		p.advance()
		return tok.Value, nil
	}
	return "", errors.NewQQLSyntaxError("Expected identifier or quoted name, got '"+tok.Value+"'", tok.Pos)
}

func isContextualIdentifier(kind lexer.TokenKind) bool {
	switch kind {
	case lexer.TokenKindOffset, lexer.TokenKindScore, lexer.TokenKindThreshold, lexer.TokenKindLookup:
		return true
	}
	return false
}

func isContextualFieldName(kind lexer.TokenKind) bool {
	return isContextualIdentifier(kind)
}

func (p *Parser) parseDict() (map[string]interface{}, error) {
	if _, err := p.expect(lexer.TokenKindLbrace); err != nil {
		return nil, err
	}
	result := make(map[string]interface{})
	if p.peek().Kind == lexer.TokenKindRbrace {
		p.advance()
		return result, nil
	}
	for {
		keyTok := p.peek()
		if keyTok.Kind != lexer.TokenKindString && keyTok.Kind != lexer.TokenKindIdentifier && keyTok.Kind != lexer.TokenKindId {
			return nil, errors.NewQQLSyntaxError("Expected string key in dict, got '"+keyTok.Value+"'", keyTok.Pos)
		}
		p.advance()
		key := keyTok.Value
		if _, err := p.expect(lexer.TokenKindColon); err != nil {
			return nil, err
		}
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		result[key] = value
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
			if p.peek().Kind == lexer.TokenKindRbrace {
				break
			}
		} else {
			break
		}
	}
	if _, err := p.expect(lexer.TokenKindRbrace); err != nil {
		return nil, err
	}
	return result, nil
}

func extractInsertPointID(values map[string]interface{}) (interface{}, map[string]interface{}) {
	for key, value := range values {
		if toLower(key) == "id" {
			delete(values, key)
			return value, values
		}
	}
	return nil, values
}

func (p *Parser) parseList() ([]interface{}, error) {
	if _, err := p.expect(lexer.TokenKindLbracket); err != nil {
		return nil, err
	}
	var items []interface{}
	if p.peek().Kind == lexer.TokenKindRbracket {
		p.advance()
		return items, nil
	}
	for {
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		items = append(items, value)
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
			if p.peek().Kind == lexer.TokenKindRbracket {
				break
			}
		} else {
			break
		}
	}
	if _, err := p.expect(lexer.TokenKindRbracket); err != nil {
		return nil, err
	}
	return items, nil
}

func (p *Parser) parseValue() (interface{}, error) {
	tok := p.peek()
	switch tok.Kind {
	case lexer.TokenKindString:
		p.advance()
		return tok.Value, nil
	case lexer.TokenKindFloat:
		p.advance()
		return parseFloatToken(tok)
	case lexer.TokenKindInteger:
		p.advance()
		return parseIntToken(tok)
	case lexer.TokenKindNull:
		p.advance()
		return nil, nil
	case lexer.TokenKindIdentifier:
		p.advance()
		upper := toUpper(tok.Value)
		if upper == "TRUE" {
			return true, nil
		}
		if upper == "FALSE" {
			return false, nil
		}
		if upper == "NULL" {
			return nil, nil
		}
		return tok.Value, nil
	case lexer.TokenKindLbrace:
		return p.parseDict()
	case lexer.TokenKindLbracket:
		return p.parseList()
	}
	return nil, errors.NewQQLSyntaxError("Unexpected value token '"+tok.Value+"'", tok.Pos)
}

func (p *Parser) parseWithClause() (*ast.SearchWith, error) {
	if _, err := p.expect(lexer.TokenKindLbrace); err != nil {
		return nil, err
	}
	hnswEf := 0
	exact := false
	acorn := false
	var err error
	for p.peek().Kind != lexer.TokenKindRbrace {
		keyTok := p.peek()
		if keyTok.Kind != lexer.TokenKindIdentifier && keyTok.Kind != lexer.TokenKindExact && keyTok.Kind != lexer.TokenKindAcorn {
			return nil, errors.NewQQLSyntaxError("Expected a WITH parameter name, got '"+keyTok.Value+"'", keyTok.Pos)
		}
		p.advance()
		key := toLower(keyTok.Value)
		if _, err := p.expect(lexer.TokenKindColon); err != nil {
			return nil, err
		}
		switch key {
		case "hnsw_ef":
			intTok, err := p.expect(lexer.TokenKindInteger)
			if err != nil {
				return nil, err
			}
			hnswEf, err = parseIntToken(intTok)
			if err != nil {
				return nil, err
			}
		case "exact":
			exact, err = p.parseBool()
			if err != nil {
				return nil, err
			}
		case "acorn":
			acorn, err = p.parseBool()
			if err != nil {
				return nil, err
			}
		default:
			return nil, errors.NewQQLSyntaxError("Unknown WITH parameter '"+key+"'. Expected: hnsw_ef, exact, acorn", keyTok.Pos)
		}
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
			if p.peek().Kind == lexer.TokenKindRbrace {
				break
			}
		} else {
			break
		}
	}
	if _, err := p.expect(lexer.TokenKindRbrace); err != nil {
		return nil, err
	}
	return &ast.SearchWith{HnswEf: hnswEf, Exact: exact, Acorn: acorn}, nil
}

func (p *Parser) parseBool() (bool, error) {
	tok := p.peek()
	if tok.Kind == lexer.TokenKindIdentifier {
		p.advance()
		upper := toUpper(tok.Value)
		if upper == "TRUE" {
			return true, nil
		}
		if upper == "FALSE" {
			return false, nil
		}
	}
	return false, errors.NewQQLSyntaxError("Expected TRUE or FALSE, got '"+tok.Value+"'", tok.Pos)
}

func tokenKindToOp(kind lexer.TokenKind) string {
	switch kind {
	case lexer.TokenKindEquals:
		return "="
	case lexer.TokenKindNotEquals:
		return "!="
	case lexer.TokenKindGt:
		return ">"
	case lexer.TokenKindGte:
		return ">="
	case lexer.TokenKindLt:
		return "<"
	case lexer.TokenKindLte:
		return "<="
	}
	return ""
}

func toUpper(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		result[i] = c
	}
	return string(result)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func (p *Parser) parseEmbeddingOptions() (*string, bool, *string, error) {
	if p.peek().Kind != lexer.TokenKindUsing {
		return nil, false, nil, nil
	}

	p.advance()
	if p.peek().Kind != lexer.TokenKindHybrid {
		model, err := p.parseRequiredModelString()
		return model, false, nil, err
	}

	p.advance()
	var model *string
	var sparseModel *string
	for p.peek().Kind == lexer.TokenKindDense || p.peek().Kind == lexer.TokenKindSparse {
		mode := p.advance().Kind
		currentModel, err := p.parseRequiredModelString()
		if err != nil {
			return nil, false, nil, err
		}
		if mode == lexer.TokenKindDense {
			model = currentModel
		} else {
			sparseModel = currentModel
		}
	}

	return model, true, sparseModel, nil
}

func (p *Parser) parseSearchEmbeddingOptions() (*string, bool, bool, *string, *string, error) {
	if p.peek().Kind != lexer.TokenKindUsing {
		return nil, false, false, nil, nil, nil
	}

	p.advance()
	if p.peek().Kind == lexer.TokenKindSparse {
		p.advance()
		var sparseModel *string
		var err error
		if p.peek().Kind == lexer.TokenKindModel {
			sparseModel, err = p.parseRequiredModelString()
			if err != nil {
				return nil, false, false, nil, nil, err
			}
		}
		return nil, false, true, sparseModel, nil, nil
	}

	model, hybrid, sparseModel, fusion, err := p.parseEmbeddingOptionsAfterUsingWithFusion()
	return model, hybrid, false, sparseModel, fusion, err
}

func (p *Parser) parseEmbeddingOptionsAfterUsingWithFusion() (*string, bool, *string, *string, error) {
	if p.peek().Kind != lexer.TokenKindHybrid {
		model, err := p.parseRequiredModelString()
		return model, false, nil, nil, err
	}

	p.advance()
	var model *string
	var sparseModel *string
	var fusion *string
	for p.peek().Kind == lexer.TokenKindDense || p.peek().Kind == lexer.TokenKindSparse || p.peek().Kind == lexer.TokenKindFusion {
		if p.peek().Kind == lexer.TokenKindFusion {
			p.advance()
			fusionTok, err := p.expect(lexer.TokenKindString)
			if err != nil {
				return nil, false, nil, nil, err
			}
			fusionVal := toLower(fusionTok.Value)
			if fusionVal != "rrf" && fusionVal != "dbsf" {
				return nil, false, nil, nil, errors.NewQQLSyntaxError("Unsupported hybrid fusion '"+fusionTok.Value+"'; expected 'rrf' or 'dbsf'", fusionTok.Pos)
			}
			fusion = &fusionVal
			continue
		}
		mode := p.advance().Kind
		currentModel, err := p.parseRequiredModelString()
		if err != nil {
			return nil, false, nil, nil, err
		}
		if mode == lexer.TokenKindDense {
			model = currentModel
		} else {
			sparseModel = currentModel
		}
	}

	return model, true, sparseModel, fusion, nil
}

func (p *Parser) parseRequiredModelString() (*string, error) {
	if _, err := p.expect(lexer.TokenKindModel); err != nil {
		return nil, err
	}
	return p.parseStringPtr()
}

func (p *Parser) parseOptionalModelString() (*string, error) {
	if p.peek().Kind != lexer.TokenKindModel {
		return nil, nil
	}
	p.advance()
	return p.parseStringPtr()
}

func (p *Parser) parseStringPtr() (*string, error) {
	tok, err := p.expect(lexer.TokenKindString)
	if err != nil {
		return nil, err
	}
	value := tok.Value
	return &value, nil
}

func ensureSearchWith(with **ast.SearchWith) *ast.SearchWith {
	if *with == nil {
		*with = &ast.SearchWith{}
	}
	return *with
}

func mergeSearchWith(dst **ast.SearchWith, src *ast.SearchWith) {
	if src == nil {
		return
	}
	current := ensureSearchWith(dst)
	if src.HnswEf != 0 {
		current.HnswEf = src.HnswEf
	}
	if src.Exact {
		current.Exact = true
	}
	if src.Acorn {
		current.Acorn = true
	}
}

func parseIntToken(tok lexer.Token) (int, error) {
	value, err := strconv.Atoi(tok.Value)
	if err != nil {
		return 0, errors.NewQQLSyntaxError("Invalid integer literal '"+tok.Value+"'", tok.Pos)
	}
	return value, nil
}

func parseFloatToken(tok lexer.Token) (float64, error) {
	value, err := strconv.ParseFloat(tok.Value, 64)
	if err != nil {
		return 0, errors.NewQQLSyntaxError("Invalid float literal '"+tok.Value+"'", tok.Pos)
	}
	return value, nil
}

func coerceVectorValues(values []interface{}) ([]float32, error) {
	vector := make([]float32, 0, len(values))
	for _, value := range values {
		switch v := value.(type) {
		case bool:
			return nil, errors.NewQQLSyntaxError("Vector elements must be numeric; got invalid value: bool", -1)
		case int:
			vector = append(vector, float32(v))
		case float64:
			vector = append(vector, float32(v))
		default:
			return nil, errors.NewQQLSyntaxError("Vector elements must be numeric; got invalid value: "+fmt.Sprintf("%v", v), -1)
		}
	}
	return vector, nil
}

func (p *Parser) parseNumericLiteral() (float64, error) {
	tok := p.peek()
	switch tok.Kind {
	case lexer.TokenKindInteger:
		p.advance()
		value, err := parseIntToken(tok)
		if err != nil {
			return 0, err
		}
		return float64(value), nil
	case lexer.TokenKindFloat:
		p.advance()
		return parseFloatToken(tok)
	default:
		return 0, errors.NewQQLSyntaxError("Expected a number, got '"+tok.Value+"'", tok.Pos)
	}
}
