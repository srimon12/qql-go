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
	err    error
}

func (p *Parser) setError(err error) error {
	if err != nil && p.err == nil {
		p.err = err
	}
	return err
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
	case lexer.TokenKindAlter:
		node, err = p.parseAlter()
	case lexer.TokenKindDrop:
		node, err = p.parseDrop()
	case lexer.TokenKindShow:
		node, err = p.parseShow()
	case lexer.TokenKindSelect:
		node, err = p.parseSelect()
	case lexer.TokenKindScroll:
		node, err = p.parseScroll()
	case lexer.TokenKindQuery:
		node, err = p.parseQuery()
	case lexer.TokenKindWith:
		node, err = p.parseQueryWithCTE()
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

	if p.err != nil {
		return nil, p.err
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

func (p *Parser) peekNext() lexer.Token {
	if p.pos+1 < len(p.tokens) {
		return p.tokens[p.pos+1]
	}
	return lexer.Token{Kind: lexer.TokenKindEof, Value: "", Pos: -1}
}

func (p *Parser) advance() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Kind: lexer.TokenKindEof, Value: "", Pos: -1}
	}
	tok := p.tokens[p.pos]
	if tok.Kind != lexer.TokenKindEof {
		p.pos++
	}
	return tok
}

func (p *Parser) expect(kind lexer.TokenKind) (lexer.Token, error) {
	tok := p.peek()
	if tok.Kind != kind {
		return tok, p.setError(errors.NewQQLSyntaxError("Expected "+kind.String()+" but got '"+tok.Value+"'", tok.Pos))
	}
	return p.advance(), nil
}

func (p *Parser) parseIdentifier() (string, error) {
	tok := p.peek()
	if tok.Kind == lexer.TokenKindIdentifier || tok.Kind == lexer.TokenKindString || isContextualIdentifier(tok.Kind) {
		p.advance()
		return tok.Value, nil
	}
	return "", p.setError(errors.NewQQLSyntaxError("Expected identifier or quoted name, got '"+tok.Value+"'", tok.Pos))
}

func isContextualIdentifier(kind lexer.TokenKind) bool {
	switch kind {
	case lexer.TokenKindOffset, lexer.TokenKindScore, lexer.TokenKindThreshold, lexer.TokenKindLookup, lexer.TokenKindId, lexer.TokenKindDense, lexer.TokenKindSparse, lexer.TokenKindVector:
		return true
	}
	return false
}

func isContextualFieldName(kind lexer.TokenKind) bool {
	return isContextualIdentifier(kind)
}

func (p *Parser) parsePayloadDict() (map[string]any, error) {
	if _, err := p.expect(lexer.TokenKindLbrace); err != nil {
		return nil, err
	}
	result := make(map[string]any)
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

func (p *Parser) parseConfigBlock() (map[string]any, error) {
	if _, err := p.expect(lexer.TokenKindLparen); err != nil {
		return nil, err
	}
	result := make(map[string]any)
	if p.peek().Kind == lexer.TokenKindRparen {
		p.advance()
		return result, nil
	}
	for {
		keyTok := p.peek()
		switch keyTok.Kind {
		case lexer.TokenKindLparen, lexer.TokenKindRparen, lexer.TokenKindEquals, lexer.TokenKindComma, lexer.TokenKindEof:
			return nil, errors.NewQQLSyntaxError("Expected configuration key, got '"+keyTok.Value+"'", keyTok.Pos)
		}
		p.advance()
		key := keyTok.Value

		if _, err := p.expect(lexer.TokenKindEquals); err != nil {
			return nil, err
		}
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		result[key] = value

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
	return result, nil
}

func (p *Parser) parseList() ([]any, error) {
	if _, err := p.expect(lexer.TokenKindLbracket); err != nil {
		return nil, err
	}
	var items []any
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

func (p *Parser) parseValue() (any, error) {
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
		if asciiEqual(tok.Value, "TRUE") {
			return true, nil
		}
		if asciiEqual(tok.Value, "FALSE") {
			return false, nil
		}
		if asciiEqual(tok.Value, "NULL") {
			return nil, nil
		}
		return tok.Value, nil
	case lexer.TokenKindLbrace:
		return p.parsePayloadDict()
	case lexer.TokenKindLbracket:
		return p.parseList()
	}
	return nil, errors.NewQQLSyntaxError("Unexpected value token '"+tok.Value+"'", tok.Pos)
}

func (p *Parser) parseBool() (bool, error) {
	tok := p.peek()
	if tok.Kind == lexer.TokenKindIdentifier {
		p.advance()
		if asciiEqual(tok.Value, "TRUE") {
			return true, nil
		}
		if asciiEqual(tok.Value, "FALSE") {
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
	case lexer.TokenKindGeoBbox:
		return "GEO_BBOX"
	case lexer.TokenKindGeoRadius:
		return "GEO_RADIUS"
	case lexer.TokenKindValuesCount:
		return "VALUES_COUNT"
	case lexer.TokenKindHasVector:
		return "HAS_VECTOR"
	}
	return ""
}

func (p *Parser) parseEmbeddingOptions() (*string, bool, *string, *string, *string, error) {
	if p.peek().Kind != lexer.TokenKindUsing {
		return nil, false, nil, nil, nil, nil
	}
	p.advance()
	if p.peek().Kind != lexer.TokenKindHybrid {
		if p.peek().Kind == lexer.TokenKindDense {
			p.advance()
		}
		denseVector, err := p.parseOptionalVectorString()
		if err != nil {
			return nil, false, nil, nil, nil, err
		}
		model, err := p.parseOptionalModelString()
		if err != nil {
			return nil, false, nil, nil, nil, err
		}
		if denseVector == nil {
			denseVector, err = p.parseOptionalVectorString()
			if err != nil {
				return nil, false, nil, nil, nil, err
			}
		}
		if model == nil && denseVector == nil {
			model, err = p.parseRequiredModelString()
			if err != nil {
				return nil, false, nil, nil, nil, err
			}
		}
		return model, false, nil, denseVector, nil, nil
	}
	p.advance()
	var model, sparseModel, denseVector, sparseVector *string
	for p.peek().Kind == lexer.TokenKindDense || p.peek().Kind == lexer.TokenKindSparse {
		mode := p.advance().Kind
		currentVector, err := p.parseOptionalVectorString()
		if err != nil {
			return nil, false, nil, nil, nil, err
		}
		currentModel, err := p.parseOptionalModelString()
		if err != nil {
			return nil, false, nil, nil, nil, err
		}
		if currentVector == nil {
			currentVector, err = p.parseOptionalVectorString()
			if err != nil {
				return nil, false, nil, nil, nil, err
			}
		}
		if currentModel == nil && currentVector == nil {
			return nil, false, nil, nil, nil, errors.NewQQLSyntaxError("Expected MODEL or VECTOR after DENSE/SPARSE", p.peek().Pos)
		}
		if mode == lexer.TokenKindDense {
			model, denseVector = currentModel, currentVector
		} else {
			sparseModel, sparseVector = currentModel, currentVector
		}
	}
	return model, true, sparseModel, denseVector, sparseVector, nil
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

func (p *Parser) parseRawVector() ([]float64, error) {
	if _, err := p.expect(lexer.TokenKindLbracket); err != nil {
		return nil, err
	}
	var vec []float64
	for p.peek().Kind != lexer.TokenKindRbracket && p.peek().Kind != lexer.TokenKindEof {
		tok := p.peek()
		if tok.Kind != lexer.TokenKindFloat && tok.Kind != lexer.TokenKindInteger {
			return nil, errors.NewQQLSyntaxError("Expected numeric value in raw vector, got '"+tok.Value+"'", tok.Pos)
		}
		f, err := parseFloatToken(p.advance())
		if err != nil {
			return nil, err
		}
		vec = append(vec, f)
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
		}
	}
	if _, err := p.expect(lexer.TokenKindRbracket); err != nil {
		return nil, err
	}
	return vec, nil
}

func coerceVectorValues(values []any) ([]float32, error) {
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

func (p *Parser) parseOptionalVectorString() (*string, error) {
	tok := p.peek()
	if tok.Kind == lexer.TokenKindVector || (tok.Kind == lexer.TokenKindIdentifier && asciiEqual(tok.Value, "VECTOR")) {
		p.advance()
		return p.parseStringPtr()
	}
	return nil, nil
}

// asciiEqual performs case-insensitive ASCII comparison without allocation.
func asciiEqual(s, upper string) bool {
	if len(s) != len(upper) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		if c != upper[i] {
			return false
		}
	}
	return true
}

// asciiEqualLower performs case-insensitive ASCII comparison against a lowercase string.
func asciiEqualLower(s, lower string) bool {
	if len(s) != len(lower) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		if c != lower[i] {
			return false
		}
	}
	return true
}
