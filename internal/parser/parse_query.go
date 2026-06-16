package parser

import (
	"strings"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/errors"
	"github.com/srimon12/qql-go/internal/lexer"
)

func (p *Parser) parseQuery() (*ast.QueryStmt, error) {
	if _, err := p.expect(lexer.TokenKindQuery); err != nil {
		return nil, err
	}

	return p.parseQueryBody()
}

// parseQueryWithCTE handles statements starting with WITH (CTE block first, then QUERY).
func (p *Parser) parseQueryWithCTE() (*ast.QueryStmt, error) {
	if _, err := p.expect(lexer.TokenKindWith); err != nil {
		return nil, err
	}
	ctes, err := p.parseCTEList()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindQuery); err != nil {
		return nil, err
	}
	stmt, err := p.parseQueryBody()
	if err != nil {
		return nil, err
	}
	stmt.CTEs = ctes
	return stmt, nil
}

// parseQueryBody parses a QUERY statement (after the QUERY keyword, with optional preceding CTEs).
func (p *Parser) parseQueryBody() (*ast.QueryStmt, error) {
	stmt := &ast.QueryStmt{}

	tok := p.peek()
	if tok.Kind == lexer.TokenKindNearest {
		p.advance()
		tok = p.peek()
	}

	switch tok.Kind {
	case lexer.TokenKindRecommend:
		stmt.Mode = ast.QueryModeRecommend
		p.advance()
		// RECOMMEND WITH (positive = [...], negative = [...])
		if p.peek().Kind == lexer.TokenKindWith {
			p.parseRecommendWith(stmt)
		}

	case lexer.TokenKindContext:
		stmt.Mode = ast.QueryModeContext
		p.advance()
		if _, err := p.expect(lexer.TokenKindPairs); err != nil {
			return nil, err
		}
		stmt.ContextPairs = p.parseContextPairs("CONTEXT")

	case lexer.TokenKindDiscover:
		stmt.Mode = ast.QueryModeDiscover
		p.advance()
		if _, err := p.expect(lexer.TokenKindTarget); err != nil {
			return nil, err
		}
		targetId, err := p.parsePointIDValue("DISCOVER TARGET")
		if err != nil {
			return nil, err
		}
		stmt.Target = targetId
		if p.peek().Kind == lexer.TokenKindContext {
			p.advance()
			if _, err := p.expect(lexer.TokenKindPairs); err != nil {
				return nil, err
			}
			stmt.ContextPairs = p.parseContextPairs("DISCOVER CONTEXT")
		}

	case lexer.TokenKindOrder:
		stmt.Mode = ast.QueryModeOrderBy
		p.advance()
		if _, err := p.expect(lexer.TokenKindBy); err != nil {
			return nil, err
		}
		fieldTok, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}
		stmt.OrderByField = &fieldTok

		tok := p.peek()
		if tok.Kind == lexer.TokenKindAsc {
			p.advance()
			asc := true
			stmt.OrderByAsc = &asc
		} else if tok.Kind == lexer.TokenKindDesc {
			p.advance()
			asc := false
			stmt.OrderByAsc = &asc
		}

	default:
		stmt.Mode = ast.QueryModeNearest
		if tok.Kind == lexer.TokenKindString {
			text := tok.Value
			stmt.QueryText = &text
			p.advance()
		} else if tok.Kind == lexer.TokenKindInteger {
			id, err := p.parsePointIDValue("QUERY")
			if err != nil {
				return nil, err
			}
			stmt.QueryID = id
		} else {
			return nil, errors.NewQQLSyntaxError("Expected a string query, a point ID, or a query mode (RECOMMEND/DISCOVER/CONTEXT) after QUERY", tok.Pos)
		}
	}

	if _, err := p.expect(lexer.TokenKindFrom); err != nil {
		return nil, err
	}
	coll, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	stmt.Collection = coll

	// Parse trailing clauses in any order (with duplicate detection)
	p.parseQueryClauses(stmt)

	return stmt, nil
}

// parseCTEList parses a comma-separated list of name AS (subquery) definitions.
func (p *Parser) parseCTEList() ([]ast.CTE, error) {
	var ctes []ast.CTE
	for {
		nameTok, err := p.expect(lexer.TokenKindIdentifier)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindAs); err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindLparen); err != nil {
			return nil, err
		}
		subStmt, err := p.parseCTEQuery()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindRparen); err != nil {
			return nil, err
		}
		ctes = append(ctes, ast.CTE{Name: nameTok.Value, Stmt: subStmt})
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
			continue
		}
		return ctes, nil
	}
}

// parseCTEQuery parses a QUERY statement inside a CTE body.
func (p *Parser) parseCTEQuery() (*ast.QueryStmt, error) {
	if _, err := p.expect(lexer.TokenKindQuery); err != nil {
		return nil, err
	}

	stmt := &ast.QueryStmt{}
	tok := p.peek()
	if tok.Kind == lexer.TokenKindNearest {
		p.advance()
		tok = p.peek()
	}

	switch tok.Kind {
	case lexer.TokenKindRecommend:
		stmt.Mode = ast.QueryModeRecommend
		p.advance()
		if p.peek().Kind == lexer.TokenKindWith {
			p.parseRecommendWith(stmt)
		}
	case lexer.TokenKindContext:
		stmt.Mode = ast.QueryModeContext
		p.advance()
		if _, err := p.expect(lexer.TokenKindPairs); err != nil {
			return nil, err
		}
		stmt.ContextPairs = p.parseContextPairs("CONTEXT")
	case lexer.TokenKindDiscover:
		stmt.Mode = ast.QueryModeDiscover
		p.advance()
		if _, err := p.expect(lexer.TokenKindTarget); err != nil {
			return nil, err
		}
		targetId, err := p.parsePointIDValue("DISCOVER TARGET")
		if err != nil {
			return nil, err
		}
		stmt.Target = targetId
		if p.peek().Kind == lexer.TokenKindContext {
			p.advance()
			if _, err := p.expect(lexer.TokenKindPairs); err != nil {
				return nil, err
			}
			stmt.ContextPairs = p.parseContextPairs("DISCOVER")
		}
	default:
		stmt.Mode = ast.QueryModeNearest
		if tok.Kind == lexer.TokenKindString {
			text := tok.Value
			stmt.QueryText = &text
			p.advance()
		} else if tok.Kind == lexer.TokenKindInteger {
			id, err := p.parsePointIDValue("QUERY")
			if err != nil {
				return nil, err
			}
			stmt.QueryID = id
		} else {
			return nil, errors.NewQQLSyntaxError("Expected string, integer, or query mode for CTE QUERY", tok.Pos)
		}
	}

	p.parseQueryClauses(stmt)
	return stmt, nil
}

// parseRecommendWith parses WITH (positive = [...], negative = [...]) after RECOMMEND.
func (p *Parser) parseRecommendWith(stmt *ast.QueryStmt) {
	p.advance() // consume WITH
	if _, err := p.expect(lexer.TokenKindLparen); err != nil {
		return
	}
	for p.peek().Kind != lexer.TokenKindRparen {
		keyTok := p.peek()
		if keyTok.Kind != lexer.TokenKindIdentifier {
			return
		}
		p.advance()
		key := strings.ToLower(keyTok.Value)
		if _, err := p.expect(lexer.TokenKindEquals); err != nil {
			return
		}
		switch key {
		case "positive":
			ids, err := p.parsePointIDList()
			if err != nil {
				return
			}
			stmt.PositiveIDs = ids
		case "negative":
			ids, err := p.parsePointIDList()
			if err != nil {
				return
			}
			stmt.NegativeIDs = ids
		}
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
		} else {
			break
		}
	}
	p.expect(lexer.TokenKindRparen)
}

// parseContextPairs parses (pos_id, neg_id), (pos_id, neg_id) lists.
func (p *Parser) parseContextPairs(label string) []ast.ContextPair {
	var pairs []ast.ContextPair
	for {
		if _, err := p.expect(lexer.TokenKindLparen); err != nil {
			return pairs
		}
		posId, err := p.parsePointIDValue(label + " POSITIVE")
		if err != nil {
			return pairs
		}
		if _, err := p.expect(lexer.TokenKindComma); err != nil {
			return pairs
		}
		negId, err := p.parsePointIDValue(label + " NEGATIVE")
		if err != nil {
			return pairs
		}
		if _, err := p.expect(lexer.TokenKindRparen); err != nil {
			return pairs
		}
		pairs = append(pairs, ast.ContextPair{Positive: posId, Negative: negId})
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
			continue
		}
		return pairs
	}
}

// parseQueryClauses parses all trailing clauses after FROM <collection>.
func (p *Parser) parseQueryClauses(stmt *ast.QueryStmt) {
	if p.peek().Kind == lexer.TokenKindLimit {
		p.advance()
		limitTok, err := p.expect(lexer.TokenKindInteger)
		if err != nil {
			return
		}
		limit, err := parseIntToken(limitTok)
		if err != nil {
			return
		}
		stmt.Limit = limit
	} else {
		stmt.Limit = 10
	}

	seenWhere, seenRerank, seenWith, seenGroup, seenGroupSize := false, false, false, false, false
	seenExact, seenFusion, seenStrategy := false, false, false

	for {
		switch p.peek().Kind {
		case lexer.TokenKindOffset:
			p.advance()
			offsetTok := p.peek()
			offset, err := parseIntToken(p.advance())
			if err != nil {
				return
			}
			if offset < 0 {
				_ = offsetTok
				return
			}
			stmt.Offset = offset

		case lexer.TokenKindScore:
			p.advance()
			if _, err := p.expect(lexer.TokenKindThreshold); err != nil {
				return
			}
			scoreTok := p.peek()
			if scoreTok.Kind == lexer.TokenKindFloat || scoreTok.Kind == lexer.TokenKindInteger {
				p.advance()
				f, _ := parseFloatToken(scoreTok)
				stmt.ScoreThreshold = &f
			}

		case lexer.TokenKindLookup:
			p.advance()
			if _, err := p.expect(lexer.TokenKindFrom); err != nil {
				return
			}
			lookupFrom, err := p.parseIdentifier()
			if err != nil {
				return
			}
			stmt.LookupFrom = lookupFrom
			if p.peek().Kind == lexer.TokenKindVector || (p.peek().Kind == lexer.TokenKindIdentifier && strings.ToUpper(p.peek().Value) == "VECTOR") {
				p.advance()
				lv, _ := p.parseStringPtr()
				stmt.LookupVector = lv
			}

		case lexer.TokenKindUsing:
			p.advance()
			if p.peek().Kind == lexer.TokenKindHybrid {
				p.advance()
				stmt.Type = ast.QueryTypeHybrid
			} else if p.peek().Kind == lexer.TokenKindSparse {
				p.advance()
				stmt.Type = ast.QueryTypeSparse
				sparse := "sparse"
				stmt.Using = &sparse
				if p.peek().Kind == lexer.TokenKindString {
					vec := p.peek().Value
					p.advance()
					stmt.Using = &vec
				}
			} else if p.peek().Kind == lexer.TokenKindDense {
				p.advance()
				stmt.Type = ast.QueryTypeDense
				dense := "dense"
				stmt.Using = &dense
				if p.peek().Kind == lexer.TokenKindString {
					vec := p.peek().Value
					p.advance()
					stmt.Using = &vec
				}
			} else if p.peek().Kind == lexer.TokenKindString {
				vec := p.peek().Value
				p.advance()
				stmt.Using = &vec
				stmt.Type = ast.QueryTypeDense
			}

		case lexer.TokenKindPrefetch:
		p.advance()
		if _, err := p.expect(lexer.TokenKindLparen); err != nil {
			return
		}
		for p.peek().Kind != lexer.TokenKindRparen {
			if p.peek().Kind == lexer.TokenKindIdentifier {
				stmt.PrefetchRefs = append(stmt.PrefetchRefs, ast.PrefetchRef{CTEName: p.peek().Value})
				p.advance()
			} else {
				break
			}
			if p.peek().Kind == lexer.TokenKindComma {
				p.advance()
			} else {
				break
			}
		}
		p.expect(lexer.TokenKindRparen)

	case lexer.TokenKindFusion:
		if seenFusion {
			return
		}
		seenFusion = true
		p.advance()
		fusionTok := p.peek()
		if fusionTok.Kind != lexer.TokenKindIdentifier || (strings.ToUpper(fusionTok.Value) != "RRF" && strings.ToUpper(fusionTok.Value) != "DBSF") {
			return
		}
		p.advance()
		upper := strings.ToUpper(fusionTok.Value)
		stmt.FusionType = &upper

	case lexer.TokenKindWhere:
		if seenWhere {
			return
		}
		seenWhere = true
		p.advance()
		filter, err := p.parseFilterExpr()
		if err != nil {
			return
		}
		stmt.QueryFilter = filter

	case lexer.TokenKindRerank:
		if seenRerank {
			return
		}
		seenRerank = true
		p.advance()
		stmt.Rerank = true
		if p.peek().Kind == lexer.TokenKindModel {
			p.advance()
			stmt.RerankModel, _ = p.parseStringPtr()
		}

	case lexer.TokenKindExact:
		if seenExact {
			return
		}
		seenExact = true
		p.advance()
		mergeSearchWith(&stmt.WithClause, &ast.SearchWith{Exact: true})

	case lexer.TokenKindWith:
		if seenWith {
			return
		}
		seenWith = true
		p.advance()
		if p.peek().Kind == lexer.TokenKindModel {
			p.advance()
			modelTok, err := p.expect(lexer.TokenKindString)
			if err != nil {
				return
			}
			stmt.Model = &modelTok.Value
		} else if p.peek().Kind == lexer.TokenKindPayload {
			p.advance()
			parsed, err := p.parseWithPayload()
			if err != nil {
				return
			}
			stmt.WithPayload = parsed
			seenWith = false // allow other WITH clauses
		} else if p.peek().Kind == lexer.TokenKindVectors {
			p.advance()
			parsed, err := p.parseWithVectors()
			if err != nil {
				return
			}
			stmt.WithVectors = parsed
			seenWith = false // allow other WITH clauses
		} else {
			parsed, err := p.parseWithClause()
			if err != nil {
				return
			}
			mergeSearchWith(&stmt.WithClause, parsed)
		}

	case lexer.TokenKindGroup:
		if seenGroup {
			return
		}
		seenGroup = true
		p.advance()
		if _, err := p.expect(lexer.TokenKindBy); err != nil {
			return
		}
		groupField, err := p.parseStringPtr()
		if err != nil {
			return
		}
		stmt.GroupBy = groupField

	case lexer.TokenKindGroupSize:
		if seenGroupSize {
			return
		}
		seenGroupSize = true
		p.advance()
		val, err := p.parseNumericLiteral()
		if err != nil {
			return
		}
		if val <= 0 || float64(uint64(val)) != val {
			return
		}
		size := int(val)
		stmt.GroupSize = &size

	case lexer.TokenKindStrategy:
		if seenStrategy {
			return
		}
		seenStrategy = true
		p.advance()
		strategy, err := p.parseStringPtr()
		if err != nil {
			return
		}
		stmt.Strategy = strategy

	case lexer.TokenKindLimit:
		p.advance()
		limitTok, err := p.expect(lexer.TokenKindInteger)
		if err != nil {
			return
		}
		limit, err := parseIntToken(limitTok)
		if err != nil {
			return
		}
		stmt.Limit = limit

	default:
		return
	}
	}
}

// nextIsWithClauseBody returns true if the WITH token is followed by a { or ( (a search-params block or model assignment),
// i.e. NOT a CTE definition.
func (p *Parser) nextIsWithClauseBody() bool {
	pos := p.pos + 1
	if pos >= len(p.tokens) {
		return false
	}
	// WITH MODEL '...'
	if p.tokens[pos].Kind == lexer.TokenKindModel {
		return true
	}
	// WITH { ... } or WITH ( ... )
	if p.tokens[pos].Kind == lexer.TokenKindLbrace || p.tokens[pos].Kind == lexer.TokenKindLparen {
		return true
	}
	// WITH followed by an identifier that isn't AS → likely a CTE name
	return false
}
