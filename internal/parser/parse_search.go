package parser

import (
	"fmt"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/errors"
	"github.com/srimon12/qql-go/internal/lexer"
)

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
	var after any
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
	if p.peek().Kind == lexer.TokenKindIdentifier && asciiEqual(p.peek().Value, "NESTED") {
		return p.parseNestedFunction()
	}
	return p.parsePredicate()
}

// parseNestedFunction parses: NESTED('path', inner_filter)
func (p *Parser) parseNestedFunction() (ast.FilterExpr, error) {
	p.advance() // consume NESTED
	if _, err := p.expect(lexer.TokenKindLparen); err != nil {
		return nil, err
	}
	pathTok, err := p.expect(lexer.TokenKindString)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindComma); err != nil {
		return nil, err
	}
	inner, err := p.parseFilterExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindRparen); err != nil {
		return nil, err
	}
	return &ast.NestedExpr{Path: pathTok.Value, Filter: inner}, nil
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
		low, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindAnd); err != nil {
			return nil, err
		}
		high, err := p.parseValue()
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
		value, err := p.parseValue()
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

func (p *Parser) parseLiteral() (any, error) {
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
	case lexer.TokenKindIdentifier:
		p.advance()
		if asciiEqual(tok.Value, "TRUE") {
			return true, nil
		}
		if asciiEqual(tok.Value, "FALSE") {
			return false, nil
		}
	}
	return nil, errors.NewQQLSyntaxError("Expected a literal value (string, integer, float, or boolean), got '"+tok.Value+"'", tok.Pos)
}

func (p *Parser) parseNumber() (any, error) {
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

func (p *Parser) parseLiteralList() ([]any, error) {
	if _, err := p.expect(lexer.TokenKindLparen); err != nil {
		return nil, err
	}
	var items []any
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

func (p *Parser) parsePointIDList() ([]any, error) {
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

func (p *Parser) parsePointIDValue(statement string) (any, error) {
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

// parseWithClause parses WITH (key = value, ...) — SQL-native syntax.
func (p *Parser) parseWithClause() (*ast.SearchWith, error) {
	if _, err := p.expect(lexer.TokenKindLparen); err != nil {
		return nil, err
	}
	hnswEf := 0
	exact := false
	acorn := false
	indexedOnly := false
	var quantization *ast.QuantizationSearchWith
	var mmrDiversity *float64
	var mmrCandidates *int
	var rrfK *int
	var rrfWeights []float32
	var err error
	for p.peek().Kind != lexer.TokenKindRparen {
		keyTok := p.peek()
		if keyTok.Kind != lexer.TokenKindIdentifier && keyTok.Kind != lexer.TokenKindExact && keyTok.Kind != lexer.TokenKindAcorn {
			return nil, errors.NewQQLSyntaxError("Expected a WITH parameter name, got '"+keyTok.Value+"'", keyTok.Pos)
		}
		p.advance()
		if _, err := p.expect(lexer.TokenKindEquals); err != nil {
			return nil, err
		}
		switch {
		case asciiEqualLower(keyTok.Value, "hnsw_ef"):
			intTok, err := p.expect(lexer.TokenKindInteger)
			if err != nil {
				return nil, err
			}
			hnswEf, err = parseIntToken(intTok)
			if err != nil {
				return nil, err
			}
		case asciiEqualLower(keyTok.Value, "exact"):
			exact, err = p.parseBool()
			if err != nil {
				return nil, err
			}
		case asciiEqualLower(keyTok.Value, "acorn"):
			acorn, err = p.parseBool()
			if err != nil {
				return nil, err
			}
		case asciiEqualLower(keyTok.Value, "indexed_only"):
			indexedOnly, err = p.parseBool()
			if err != nil {
				return nil, err
			}
		case asciiEqualLower(keyTok.Value, "quantization"):
			quantization, err = p.parseQuantizationSearchWith()
			if err != nil {
				return nil, err
			}
		case asciiEqualLower(keyTok.Value, "mmr_diversity"):
			value, err := p.parseNumber()
			if err != nil {
				return nil, err
			}
			var diversity float64
			switch typed := value.(type) {
			case int:
				diversity = float64(typed)
			case float64:
				diversity = typed
			default:
				return nil, errors.NewQQLSyntaxError("mmr_diversity must be numeric", keyTok.Pos)
			}
			if diversity < 0 || diversity > 1 {
				return nil, errors.NewQQLSyntaxError("mmr_diversity must be between 0 and 1, got '"+fmt.Sprintf("%v", diversity)+"'", keyTok.Pos)
			}
			mmrDiversity = &diversity
		case asciiEqualLower(keyTok.Value, "mmr_candidates"):
			intTok, err := p.expect(lexer.TokenKindInteger)
			if err != nil {
				return nil, err
			}
			candidates, err := parseIntToken(intTok)
			if err != nil {
				return nil, err
			}
			if candidates <= 0 {
				return nil, errors.NewQQLSyntaxError("mmr_candidates must be a positive integer, got '"+intTok.Value+"'", intTok.Pos)
			}
			mmrCandidates = &candidates
		case asciiEqualLower(keyTok.Value, "rrf_k"):
			intTok, err := p.expect(lexer.TokenKindInteger)
			if err != nil {
				return nil, err
			}
			k, err := parseIntToken(intTok)
			if err != nil {
				return nil, err
			}
			if k <= 0 {
				return nil, errors.NewQQLSyntaxError("rrf_k must be a positive integer, got '"+intTok.Value+"'", intTok.Pos)
			}
			rrfK = &k
		case asciiEqualLower(keyTok.Value, "rrf_weights"):
			if _, err := p.expect(lexer.TokenKindLbracket); err != nil {
				return nil, err
			}
			for p.peek().Kind != lexer.TokenKindRbracket {
				valTok, err := p.parseNumber()
				if err != nil {
					return nil, err
				}
				switch typed := valTok.(type) {
				case int:
					rrfWeights = append(rrfWeights, float32(typed))
				case float64:
					rrfWeights = append(rrfWeights, float32(typed))
				default:
					return nil, errors.NewQQLSyntaxError("rrf_weights must contain numeric values", keyTok.Pos)
				}
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
		case asciiEqualLower(keyTok.Value, "model"):
			tok, err := p.expect(lexer.TokenKindString)
			if err != nil {
				return nil, err
			}
			// model is handled elsewhere; skip for WITH params
			_ = tok
		default:
			return nil, errors.NewQQLSyntaxError("Unknown WITH parameter '"+keyTok.Value+"'. Expected: hnsw_ef, exact, acorn, indexed_only, quantization, mmr_diversity, mmr_candidates, rrf_k, rrf_weights", keyTok.Pos)
		}
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
	return &ast.SearchWith{
		HnswEf:        hnswEf,
		Exact:         exact,
		Acorn:         acorn,
		IndexedOnly:   indexedOnly,
		Quantization:  quantization,
		MmrDiversity:  mmrDiversity,
		MmrCandidates: mmrCandidates,
		RrfK:          rrfK,
		RrfWeights:    rrfWeights,
	}, nil
}

func (p *Parser) parseQuantizationSearchWith() (*ast.QuantizationSearchWith, error) {
	if _, err := p.expect(lexer.TokenKindLparen); err != nil {
		return nil, err
	}

	var ignore *bool
	var rescore *bool
	var oversampling *float64

	for p.peek().Kind != lexer.TokenKindRparen {
		keyTok := p.peek()
		if keyTok.Kind != lexer.TokenKindIdentifier {
			return nil, errors.NewQQLSyntaxError("Expected a quantization parameter name, got '"+keyTok.Value+"'", keyTok.Pos)
		}
		p.advance()
		if _, err := p.expect(lexer.TokenKindEquals); err != nil {
			return nil, err
		}

		switch {
		case asciiEqualLower(keyTok.Value, "ignore"):
			value, err := p.parseBool()
			if err != nil {
				return nil, err
			}
			ignore = &value
		case asciiEqualLower(keyTok.Value, "rescore"):
			value, err := p.parseBool()
			if err != nil {
				return nil, err
			}
			rescore = &value
		case asciiEqualLower(keyTok.Value, "oversampling"):
			value, err := p.parseNumber()
			if err != nil {
				return nil, err
			}
			switch typed := value.(type) {
			case int:
				v := float64(typed)
				oversampling = &v
			case float64:
				v := typed
				oversampling = &v
			default:
				return nil, errors.NewQQLSyntaxError("oversampling must be numeric", keyTok.Pos)
			}
		default:
			return nil, errors.NewQQLSyntaxError("Unknown quantization parameter '"+keyTok.Value+"'. Expected: ignore, rescore, oversampling", keyTok.Pos)
		}

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

	return &ast.QuantizationSearchWith{
		Ignore:       ignore,
		Rescore:      rescore,
		Oversampling: oversampling,
	}, nil
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
	if src.IndexedOnly {
		current.IndexedOnly = true
	}
	if src.Quantization != nil {
		current.Quantization = src.Quantization
	}
	if src.MmrDiversity != nil {
		current.MmrDiversity = src.MmrDiversity
	}
	if src.MmrCandidates != nil {
		current.MmrCandidates = src.MmrCandidates
	}
	if src.RrfK != nil {
		current.RrfK = src.RrfK
	}
	if len(src.RrfWeights) > 0 {
		current.RrfWeights = src.RrfWeights
	}
}

func (p *Parser) parseWithPayload() (*ast.PayloadSelector, error) {
	if p.peek().Kind == lexer.TokenKindIdentifier && (asciiEqual(p.peek().Value, "TRUE") || asciiEqual(p.peek().Value, "FALSE")) {
		tok := p.peek()
		p.advance()
		val := asciiEqual(tok.Value, "TRUE")
		return &ast.PayloadSelector{Enable: &val}, nil
	}
	if _, err := p.expect(lexer.TokenKindLparen); err != nil {
		return nil, err
	}
	var include, exclude []string
	for p.peek().Kind != lexer.TokenKindRparen {
		keyTok, err := p.expect(lexer.TokenKindIdentifier)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindEquals); err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindLbracket); err != nil {
			return nil, err
		}
		var fields []string
		for p.peek().Kind != lexer.TokenKindRbracket {
			valTok, err := p.expect(lexer.TokenKindString)
			if err != nil {
				return nil, err
			}
			fields = append(fields, valTok.Value)
			if p.peek().Kind == lexer.TokenKindComma {
				p.advance()
			} else {
				break
			}
		}
		if _, err := p.expect(lexer.TokenKindRbracket); err != nil {
			return nil, err
		}
		if asciiEqualLower(keyTok.Value, "include") {
			include = fields
		} else if asciiEqualLower(keyTok.Value, "exclude") {
			exclude = fields
		} else {
			return nil, errors.NewQQLSyntaxError("Expected 'include' or 'exclude', got '"+keyTok.Value+"'", keyTok.Pos)
		}
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
		} else {
			break
		}
	}
	if _, err := p.expect(lexer.TokenKindRparen); err != nil {
		return nil, err
	}
	return &ast.PayloadSelector{Include: include, Exclude: exclude}, nil
}

func (p *Parser) parseWithVectors() (*ast.VectorsSelector, error) {
	if p.peek().Kind == lexer.TokenKindIdentifier && (asciiEqual(p.peek().Value, "TRUE") || asciiEqual(p.peek().Value, "FALSE")) {
		tok := p.peek()
		p.advance()
		val := asciiEqual(tok.Value, "TRUE")
		return &ast.VectorsSelector{Enable: &val}, nil
	}
	if _, err := p.expect(lexer.TokenKindLparen); err != nil {
		return nil, err
	}
	var vectors []string
	for p.peek().Kind != lexer.TokenKindRparen {
		valTok, err := p.expect(lexer.TokenKindString)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, valTok.Value)
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
		} else {
			break
		}
	}
	if _, err := p.expect(lexer.TokenKindRparen); err != nil {
		return nil, err
	}
	return &ast.VectorsSelector{Vectors: vectors}, nil
}
