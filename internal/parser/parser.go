package parser

import (
	"github.com/qdrant/qql-go/internal/ast"
	"github.com/qdrant/qql-go/internal/errors"
	"github.com/qdrant/qql-go/internal/lexer"
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
	case lexer.TokenKindDelete:
		node, err = p.parseDelete()
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

func (p *Parser) parseInsert() (*ast.InsertStmt, error) {
	p.advance()
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
	var model *string
	hybrid := false
	var sparseModel *string

	tok := p.peek()
	if tok.Kind == lexer.TokenKindUsing {
		p.advance()
		if p.peek().Kind == lexer.TokenKindHybrid {
			p.advance()
			hybrid = true
			for p.peek().Kind == lexer.TokenKindDense || p.peek().Kind == lexer.TokenKindSparse {
				sub := p.advance()
				if _, err := p.expect(lexer.TokenKindModel); err != nil {
					return nil, err
				}
				mTok, err := p.expect(lexer.TokenKindString)
				if err != nil {
					return nil, err
				}
				if sub.Kind == lexer.TokenKindDense {
					modelVal := mTok.Value
					model = &modelVal
				} else {
					sparseVal := mTok.Value
					sparseModel = &sparseVal
				}
			}
		} else {
			if _, err := p.expect(lexer.TokenKindModel); err != nil {
				return nil, err
			}
			mTok, err := p.expect(lexer.TokenKindString)
			if err != nil {
				return nil, err
			}
			modelVal := mTok.Value
			model = &modelVal
		}
	}
	return &ast.InsertStmt{
		Collection:  collection,
		Values:      values,
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
	if p.peek().Kind == lexer.TokenKindHybrid {
		p.advance()
		hybrid = true
		if p.peek().Kind == lexer.TokenKindRerank {
			p.advance()
			rerank = true
		}
	}
	return &ast.CreateCollectionStmt{Collection: collection, Hybrid: hybrid, Rerank: rerank}, nil
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

func (p *Parser) parseShow() (*ast.ShowCollectionsStmt, error) {
	p.advance()
	if _, err := p.expect(lexer.TokenKindCollections); err != nil {
		return nil, err
	}
	return &ast.ShowCollectionsStmt{}, nil
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
	var limit int
	parseInt(limitTok.Value, &limit)

	var withClause *ast.SearchWith
	tok := p.peek()
	if tok.Kind == lexer.TokenKindExact {
		p.advance()
		withClause = &ast.SearchWith{Exact: true}
	}

	var model *string
	hybrid := false
	var sparseModel *string

	tok = p.peek()
	if tok.Kind == lexer.TokenKindUsing {
		p.advance()
		if p.peek().Kind == lexer.TokenKindHybrid {
			p.advance()
			hybrid = true
			for p.peek().Kind == lexer.TokenKindDense || p.peek().Kind == lexer.TokenKindSparse {
				sub := p.advance()
				if _, err := p.expect(lexer.TokenKindModel); err != nil {
					return nil, err
				}
				mTok, err := p.expect(lexer.TokenKindString)
				if err != nil {
					return nil, err
				}
				if sub.Kind == lexer.TokenKindDense {
					modelVal := mTok.Value
					model = &modelVal
				} else {
					sparseVal := mTok.Value
					sparseModel = &sparseVal
				}
			}
		} else {
			if _, err := p.expect(lexer.TokenKindModel); err != nil {
				return nil, err
			}
			mTok, err := p.expect(lexer.TokenKindString)
			if err != nil {
				return nil, err
			}
			modelVal := mTok.Value
			model = &modelVal
		}
	}

	var queryFilter ast.FilterExpr
	tok = p.peek()
	if tok.Kind == lexer.TokenKindWhere {
		p.advance()
		queryFilter, err = p.parseFilterExpr()
		if err != nil {
			return nil, err
		}
	}

	rerank := false
	var rerankModel *string
	tok = p.peek()
	if tok.Kind == lexer.TokenKindRerank {
		p.advance()
		rerank = true
		if p.peek().Kind == lexer.TokenKindModel {
			p.advance()
			rmTok, err := p.expect(lexer.TokenKindString)
			if err != nil {
				return nil, err
			}
			rmVal := rmTok.Value
			rerankModel = &rmVal
		}
	}

	tok = p.peek()
	if tok.Kind == lexer.TokenKindExact {
		p.advance()
		if withClause == nil {
			withClause = &ast.SearchWith{Exact: true}
		} else {
			withClause.Exact = true
		}
	}

	tok = p.peek()
	if tok.Kind == lexer.TokenKindWith {
		p.advance()
		parsedWith, err := p.parseWithClause()
		if err != nil {
			return nil, err
		}
		if withClause == nil {
			withClause = parsedWith
		} else {
			if parsedWith.HnswEf != 0 {
				withClause.HnswEf = parsedWith.HnswEf
			}
			if parsedWith.Exact {
				withClause.Exact = true
			}
			if parsedWith.Acorn {
				withClause.Acorn = true
			}
		}
	}

	return &ast.SearchStmt{
		Collection:  collection,
		QueryText:   queryText,
		Limit:       limit,
		Model:       model,
		Hybrid:      hybrid,
		SparseModel: sparseModel,
		QueryFilter: queryFilter,
		Rerank:      rerank,
		RerankModel: rerankModel,
		WithClause:  withClause,
	}, nil
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
			var v int
			parseInt(tok.Value, &v)
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
	if tok.Kind != lexer.TokenKindIdentifier {
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
		var v int
		parseInt(tok.Value, &v)
		return v, nil
	case lexer.TokenKindFloat:
		p.advance()
		var v float64
		parseFloat(tok.Value, &v)
		return v, nil
	}
	return nil, errors.NewQQLSyntaxError("Expected a literal value (string, integer, or float), got '"+tok.Value+"'", tok.Pos)
}

func (p *Parser) parseNumber() (interface{}, error) {
	tok := p.peek()
	switch tok.Kind {
	case lexer.TokenKindInteger:
		p.advance()
		var v int
		parseInt(tok.Value, &v)
		return v, nil
	case lexer.TokenKindFloat:
		p.advance()
		var v float64
		parseFloat(tok.Value, &v)
		return v, nil
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

func (p *Parser) parseIdentifier() (string, error) {
	tok := p.peek()
	if tok.Kind == lexer.TokenKindIdentifier || tok.Kind == lexer.TokenKindString {
		p.advance()
		return tok.Value, nil
	}
	return "", errors.NewQQLSyntaxError("Expected identifier or quoted name, got '"+tok.Value+"'", tok.Pos)
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
		if keyTok.Kind != lexer.TokenKindString && keyTok.Kind != lexer.TokenKindIdentifier {
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
		var v float64
		parseFloat(tok.Value, &v)
		return v, nil
	case lexer.TokenKindInteger:
		p.advance()
		var v int
		parseInt(tok.Value, &v)
		return v, nil
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
			parseInt(intTok.Value, &hnswEf)
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

func parseInt(s string, result *int) {
	*result = 0
	sign := 1
	i := 0
	if len(s) > 0 && s[0] == '-' {
		sign = -1
		i = 1
	}
	for ; i < len(s); i++ {
		*result = *result*10 + int(s[i]-'0')
	}
	*result *= sign
}

func parseFloat(s string, result *float64) {
	*result = 0
	negative := false
	i := 0
	if len(s) > 0 && s[0] == '-' {
		negative = true
		i = 1
	}

	dotIdx := -1
	for j := i; j < len(s); j++ {
		if s[j] == '.' {
			dotIdx = j
			break
		}
	}

	intPart := 0.0
	for j := i; j < len(s) && s[j] != '.'; j++ {
		intPart = intPart*10 + float64(s[j]-'0')
	}
	*result = intPart

	if dotIdx != -1 && dotIdx+1 < len(s) {
		fracPart := 0.0
		divisor := 1.0
		for j := dotIdx + 1; j < len(s); j++ {
			fracPart = fracPart*10 + float64(s[j]-'0')
			divisor *= 10
		}
		*result += fracPart / divisor
	}

	if negative {
		*result = -*result
	}
}
