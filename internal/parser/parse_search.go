package parser

import (
	"fmt"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/errors"
	"github.com/srimon12/qql-go/internal/lexer"
)

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

	offset := 0
	hasOffset := false
	if p.peek().Kind == lexer.TokenKindOffset {
		hasOffset = true
		p.advance()
		offsetTok := p.peek()
		offset, err = parseIntToken(p.advance())
		if err != nil {
			return nil, err
		}
		if offset < 0 {
			return nil, errors.NewQQLSyntaxError("OFFSET must be a non-negative integer", offsetTok.Pos)
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

	model, hybrid, sparseOnly, sparseModel, fusion, denseVector, sparseVector, err := p.parseSearchEmbeddingOptions()
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
			if hasOffset {
				return nil, errors.NewQQLSyntaxError("OFFSET cannot be used with GROUP BY", p.peek().Pos)
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
				Collection:     collection,
				QueryText:      queryText,
				Limit:          limit,
				Model:          model,
				Hybrid:         hybrid,
				Fusion:         fusion,
				SparseOnly:     sparseOnly,
				SparseModel:    sparseModel,
				QueryFilter:    queryFilter,
				Rerank:         rerank,
				RerankModel:    rerankModel,
				WithClause:     withClause,
				GroupBy:        groupBy,
				GroupSize:      groupSize,
				Offset:         offset,
				ScoreThreshold: scoreThreshold,
				LookupFrom:     lookupFrom,
				LookupVector:   lookupVector,
				DenseVector:    denseVector,
				SparseVector:   sparseVector,
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
	var negativeIDs []any
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
		if offset < 0 {
			return nil, errors.NewQQLSyntaxError("OFFSET must be a non-negative integer", offsetTok.Pos)
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
		upper := toUpper(tok.Value)
		if upper == "TRUE" {
			return true, nil
		}
		if upper == "FALSE" {
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

func (p *Parser) parseWithClause() (*ast.SearchWith, error) {
	if _, err := p.expect(lexer.TokenKindLbrace); err != nil {
		return nil, err
	}
	hnswEf := 0
	exact := false
	acorn := false
	indexedOnly := false
	var quantization *ast.QuantizationSearchWith
	var mmrDiversity *float64
	var mmrCandidates *int
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
		case "indexed_only":
			indexedOnly, err = p.parseBool()
			if err != nil {
				return nil, err
			}
		case "quantization":
			quantization, err = p.parseQuantizationSearchWith()
			if err != nil {
				return nil, err
			}
		case "mmr_diversity":
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
		case "mmr_candidates":
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
		default:
			return nil, errors.NewQQLSyntaxError("Unknown WITH parameter '"+key+"'. Expected: hnsw_ef, exact, acorn, indexed_only, quantization, mmr_diversity, mmr_candidates", keyTok.Pos)
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
	return &ast.SearchWith{
		HnswEf:        hnswEf,
		Exact:         exact,
		Acorn:         acorn,
		IndexedOnly:   indexedOnly,
		Quantization:  quantization,
		MmrDiversity:  mmrDiversity,
		MmrCandidates: mmrCandidates,
	}, nil
}

func (p *Parser) parseQuantizationSearchWith() (*ast.QuantizationSearchWith, error) {
	if _, err := p.expect(lexer.TokenKindLbrace); err != nil {
		return nil, err
	}

	var ignore *bool
	var rescore *bool
	var oversampling *float64

	for p.peek().Kind != lexer.TokenKindRbrace {
		keyTok := p.peek()
		if keyTok.Kind != lexer.TokenKindIdentifier {
			return nil, errors.NewQQLSyntaxError("Expected a quantization parameter name, got '"+keyTok.Value+"'", keyTok.Pos)
		}
		p.advance()
		key := toLower(keyTok.Value)
		if _, err := p.expect(lexer.TokenKindColon); err != nil {
			return nil, err
		}

		switch key {
		case "ignore":
			value, err := p.parseBool()
			if err != nil {
				return nil, err
			}
			ignore = &value
		case "rescore":
			value, err := p.parseBool()
			if err != nil {
				return nil, err
			}
			rescore = &value
		case "oversampling":
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
			return nil, errors.NewQQLSyntaxError("Unknown quantization parameter '"+key+"'. Expected: ignore, rescore, oversampling", keyTok.Pos)
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
}
