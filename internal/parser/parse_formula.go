package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/errors"
	"github.com/srimon12/qql-go/internal/lexer"
)

const (
	_ int = iota
	precedenceLowest
	precedenceSum     // + -
	precedenceProduct // * /
	precedencePrefix  // -X
	precedenceCall    // func(X)
)

var precedences = map[lexer.TokenKind]int{
	lexer.TokenKindPlus:  precedenceSum,
	lexer.TokenKindMinus: precedenceSum,
	lexer.TokenKindStar:  precedenceProduct,
	lexer.TokenKindSlash: precedenceProduct,
}

func (p *Parser) currentPrecedence() int {
	if p, ok := precedences[p.peek().Kind]; ok {
		return p
	}
	return precedenceLowest
}

type prefixParseFn func() (ast.FormulaExpr, error)
type infixParseFn func(ast.FormulaExpr) (ast.FormulaExpr, error)

func (p *Parser) parseFormulaExpr(precedence int) (ast.FormulaExpr, error) {
	prefix := p.formulaPrefixParseFn(p.peek().Kind)
	if prefix == nil {
		return nil, errors.NewQQLSyntaxError("Unexpected token in formula: "+p.peek().Value, p.peek().Pos)
	}

	leftExp, err := prefix()
	if err != nil {
		return nil, err
	}

	for p.peek().Kind != lexer.TokenKindEof && precedence < p.currentPrecedence() {
		infix := p.formulaInfixParseFn(p.peek().Kind)
		if infix == nil {
			return leftExp, nil
		}

		leftExp, err = infix(leftExp)
		if err != nil {
			return nil, err
		}
	}

	return leftExp, nil
}

func (p *Parser) formulaPrefixParseFn(kind lexer.TokenKind) prefixParseFn {
	switch kind {
	case lexer.TokenKindIdentifier, lexer.TokenKindScore, lexer.TokenKindOffset, lexer.TokenKindThreshold, lexer.TokenKindLookup, lexer.TokenKindMatch:
		return p.parseFormulaIdentifierOrFunc
	case lexer.TokenKindInteger, lexer.TokenKindFloat:
		return p.parseFormulaConstant
	case lexer.TokenKindMinus:
		return p.parseFormulaPrefixExpression
	case lexer.TokenKindLparen:
		return p.parseFormulaGroupedExpression
	case lexer.TokenKindCase:
		return p.parseFormulaCaseExpression
	default:
		return nil
	}
}

func (p *Parser) formulaInfixParseFn(kind lexer.TokenKind) infixParseFn {
	switch kind {
	case lexer.TokenKindPlus, lexer.TokenKindMinus, lexer.TokenKindStar, lexer.TokenKindSlash:
		return p.parseFormulaInfixExpression
	default:
		return nil
	}
}

func (p *Parser) parseFormulaIdentifierOrFunc() (ast.FormulaExpr, error) {
	tok := p.advance()
	val := tok.Value
	lower := strings.ToLower(val)

	// Check if it's a function call (followed by '(')
	if p.peek().Kind == lexer.TokenKindLparen {
		p.advance() // consume '('
		return p.parseFormulaFunctionCall(lower, tok.Pos)
	}

	return ast.FormulaVariable{Name: val}, nil
}

func (p *Parser) parseFormulaConstant() (ast.FormulaExpr, error) {
	tok := p.advance()
	v, err := strconv.ParseFloat(tok.Value, 64)
	if err != nil {
		return nil, errors.NewQQLSyntaxError("Invalid number format in formula", tok.Pos)
	}
	return ast.FormulaConstant{Value: v}, nil
}

func (p *Parser) parseFormulaPrefixExpression() (ast.FormulaExpr, error) {
	p.advance() // consume operator (currently only '-')
	right, err := p.parseFormulaExpr(precedencePrefix)
	if err != nil {
		return nil, err
	}
	return ast.FormulaNeg{Operand: right}, nil
}

func (p *Parser) parseFormulaInfixExpression(left ast.FormulaExpr) (ast.FormulaExpr, error) {
	tok := p.advance()
	precedence := precedences[tok.Kind]
	right, err := p.parseFormulaExpr(precedence)
	if err != nil {
		return nil, err
	}

	switch tok.Kind {
	case lexer.TokenKindPlus:
		return ast.FormulaSum{Left: left, Right: right}, nil
	case lexer.TokenKindMinus:
		return ast.FormulaSub{Left: left, Right: right}, nil
	case lexer.TokenKindStar:
		return ast.FormulaMul{Left: left, Right: right}, nil
	case lexer.TokenKindSlash:
		return ast.FormulaDiv{Left: left, Right: right}, nil
	default:
		return nil, errors.NewQQLSyntaxError("Unknown formula operator: "+tok.Value, tok.Pos)
	}
}

func (p *Parser) parseFormulaGroupedExpression() (ast.FormulaExpr, error) {
	p.advance() // consume '('
	expr, err := p.parseFormulaExpr(precedenceLowest)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindRparen); err != nil {
		return nil, err
	}
	return expr, nil
}

func (p *Parser) parseFormulaCaseExpression() (ast.FormulaExpr, error) {
	p.advance() // consume CASE
	if _, err := p.expect(lexer.TokenKindWhen); err != nil {
		return nil, err
	}

	// parseFilterExpr parses until THEN, AND, OR, etc.
	cond, err := p.parseFilterExpr()
	if err != nil {
		return nil, err
	}

	if _, err := p.expect(lexer.TokenKindThen); err != nil {
		return nil, err
	}

	thenExpr, err := p.parseFormulaExpr(precedenceLowest)
	if err != nil {
		return nil, err
	}

	if _, err := p.expect(lexer.TokenKindElse); err != nil {
		return nil, err
	}

	elseExpr, err := p.parseFormulaExpr(precedenceLowest)
	if err != nil {
		return nil, err
	}

	if _, err := p.expect(lexer.TokenKindEnd); err != nil {
		return nil, err
	}

	return ast.FormulaCase{Cond: cond, Then_: thenExpr, Else_: elseExpr}, nil
}

func (p *Parser) parseFormulaFunctionCall(funcName string, pos int) (ast.FormulaExpr, error) {
	// Special cases that don't follow generic expression arguments
	switch funcName {
	case "match", "match_any":
		fieldTok, err := p.expect(lexer.TokenKindIdentifier)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindComma); err != nil {
			return nil, err
		}
		var values []any
		if p.peek().Kind == lexer.TokenKindLbracket {
			vals, err := p.parseList()
			if err != nil {
				return nil, err
			}
			values = vals
		} else {
			single, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			values = []any{single}
		}
		if _, err := p.expect(lexer.TokenKindRparen); err != nil {
			return nil, err
		}
		return ast.FormulaMatchCondition{Field: fieldTok.Value, Values: values}, nil
	case "datetime":
		tok, err := p.expect(lexer.TokenKindString)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindRparen); err != nil {
			return nil, err
		}
		return ast.FormulaDatetime{Value: tok.Value}, nil
	case "datetime_key":
		tok, err := p.expect(lexer.TokenKindString)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindRparen); err != nil {
			return nil, err
		}
		return ast.FormulaDatetimeKey{Key: tok.Value}, nil
	case "geo_distance":
		// Check if the first argument is a dictionary
		if p.peek().Kind == lexer.TokenKindLbrace {
			dict, err := p.parsePayloadDict() // This consumes { 'lat': x, 'lon': y }
			if err != nil {
				return nil, err
			}
			if p.peek().Kind == lexer.TokenKindComma {
				p.advance()
			}
			fieldTok, err := p.expect(lexer.TokenKindIdentifier)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokenKindRparen); err != nil {
				return nil, err
			}

			latVal, hasLat := dict["lat"]
			lonVal, hasLon := dict["lon"]
			if !hasLat || !hasLon {
				return nil, errors.NewQQLSyntaxError("geo_distance dict must have 'lat' and 'lon' keys", pos)
			}

			var lat, lon float64
			switch v := latVal.(type) {
			case float64:
				lat = v
			case int:
				lat = float64(v)
			default:
				return nil, errors.NewQQLSyntaxError("geo_distance lat must be a number", pos)
			}

			switch v := lonVal.(type) {
			case float64:
				lon = v
			case int:
				lon = float64(v)
			default:
				return nil, errors.NewQQLSyntaxError("geo_distance lon must be a number", pos)
			}

			return ast.FormulaGeoDistance{Lat: lat, Lon: lon, Field: fieldTok.Value}, nil
		}
	}

	args, kwargs, err := p.parseFormulaCallArgumentsAndKwargs()
	if err != nil {
		return nil, err
	}

	switch funcName {
	case "abs":
		if len(args) != 1 {
			return nil, errors.NewQQLSyntaxError("ABS() expects 1 argument", pos)
		}
		return ast.FormulaAbs{X: args[0]}, nil
	case "sqrt":
		if len(args) != 1 {
			return nil, errors.NewQQLSyntaxError("SQRT() expects 1 argument", pos)
		}
		return ast.FormulaSqrt{X: args[0]}, nil
	case "log":
		if len(args) != 1 {
			return nil, errors.NewQQLSyntaxError("LOG() expects 1 argument", pos)
		}
		return ast.FormulaLog{X: args[0]}, nil
	case "ln":
		if len(args) != 1 {
			return nil, errors.NewQQLSyntaxError("LN() expects 1 argument", pos)
		}
		return ast.FormulaLn{X: args[0]}, nil
	case "exp":
		if len(args) != 1 {
			return nil, errors.NewQQLSyntaxError("EXP() expects 1 argument", pos)
		}
		return ast.FormulaExp{X: args[0]}, nil
	case "pow":
		if len(args) != 2 {
			return nil, errors.NewQQLSyntaxError("POW() expects 2 arguments", pos)
		}
		return ast.FormulaPow{Base: args[0], Exponent: args[1]}, nil
	case "geo_distance":
		if len(args) != 3 {
			return nil, errors.NewQQLSyntaxError("GEO_DISTANCE() expects 3 arguments (lat, lon, field_name)", pos)
		}
		latNode, ok1 := args[0].(ast.FormulaConstant)
		lonNode, ok2 := args[1].(ast.FormulaConstant)
		fieldNode, ok3 := args[2].(ast.FormulaVariable)
		if !ok1 || !ok2 || !ok3 {
			return nil, errors.NewQQLSyntaxError("GEO_DISTANCE() arguments must be (float lat, float lon, field_name)", pos)
		}
		return ast.FormulaGeoDistance{Lat: latNode.Value, Lon: lonNode.Value, Field: fieldNode.Name}, nil
	case "exp_decay", "gauss_decay", "lin_decay":
		if len(args)+len(kwargs) < 1 {
			return nil, errors.NewQQLSyntaxError(strings.ToUpper(funcName)+"() expects at least 1 argument (x)", pos)
		}

		var x ast.FormulaExpr
		var target *ast.FormulaExpr
		var scale, midpoint *float64

		// x is always the first argument, either positional or kwargs
		if len(args) > 0 {
			x = args[0]
		} else if val, ok := kwargs["x"]; ok {
			x = val
		} else {
			return nil, errors.NewQQLSyntaxError(strings.ToUpper(funcName)+"() requires 'x' argument", pos)
		}

		// target
		if len(args) > 1 {
			target = &args[1]
		} else if val, ok := kwargs["target"]; ok {
			target = &val
		}

		// scale
		if len(args) > 2 {
			if c, ok := args[2].(ast.FormulaConstant); ok {
				scale = &c.Value
			}
		} else if val, ok := kwargs["scale"]; ok {
			if c, ok := val.(ast.FormulaConstant); ok {
				scale = &c.Value
			}
		}

		// midpoint
		if len(args) > 3 {
			if c, ok := args[3].(ast.FormulaConstant); ok {
				midpoint = &c.Value
			}
		} else if val, ok := kwargs["midpoint"]; ok {
			if c, ok := val.(ast.FormulaConstant); ok {
				midpoint = &c.Value
			}
		}

		return ast.FormulaDecay{
			Kind:     funcName,
			X:        x,
			Target:   target,
			Scale:    scale,
			Midpoint: midpoint,
		}, nil
	default:
		return nil, errors.NewQQLSyntaxError("Unknown formula function: "+funcName, pos)
	}
}

func (p *Parser) parseFormulaCallArgumentsAndKwargs() ([]ast.FormulaExpr, map[string]ast.FormulaExpr, error) {
	var args []ast.FormulaExpr
	kwargs := make(map[string]ast.FormulaExpr)

	if p.peek().Kind == lexer.TokenKindRparen {
		p.advance()
		return args, kwargs, nil
	}

	for {
		// Check for kwarg: Anything followed by Equals (allows keywords like 'target', 'scale')
		if p.peekNext().Kind == lexer.TokenKindEquals {
			keyTok := p.advance() // Identifier or Keyword
			p.advance()           // Equals
			arg, err := p.parseFormulaExpr(precedenceLowest)
			if err != nil {
				return nil, nil, err
			}
			kwargs[keyTok.Value] = arg
		} else {
			// Positional argument
			if len(kwargs) > 0 {
				return nil, nil, errors.NewQQLSyntaxError("Positional argument cannot follow keyword argument", p.peek().Pos)
			}
			arg, err := p.parseFormulaExpr(precedenceLowest)
			if err != nil {
				return nil, nil, err
			}
			args = append(args, arg)
		}

		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
		} else {
			break
		}
	}

	if _, err := p.expect(lexer.TokenKindRparen); err != nil {
		return nil, nil, err
	}

	return args, kwargs, nil
}

// ParseFormulaString parses a standalone formula expression string into an AST FormulaExpr.
// The input should be the body of a BOOST clause, e.g. "$score * 2.0 + MATCH(tag, ['h1'])"
func ParseFormulaString(input string) (ast.FormulaExpr, error) {
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	if err != nil {
		return nil, fmt.Errorf("failed to tokenize formula: %w", err)
	}
	p := NewParser()
	p.tokens = tokens
	p.pos = 0
	return p.parseFormulaExpr(precedenceLowest)
}
