package parser

import (
	"strconv"

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
		switch tok.Kind {
		case lexer.TokenKindAsc:
			p.advance()
			asc := true
			stmt.OrderByAsc = &asc
		case lexer.TokenKindDesc:
			p.advance()
			asc := false
			stmt.OrderByAsc = &asc
		}

	case lexer.TokenKindSample:
		stmt.Mode = ast.QueryModeSample
		p.advance()

	case lexer.TokenKindRelevance:
		stmt.Mode = ast.QueryModeRelevanceFeedback
		p.advance()
		if _, err := p.expect(lexer.TokenKindFeedback); err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindTarget); err != nil {
			return nil, err
		}
		targetVal, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		stmt.FeedbackTarget = targetVal
		if _, err := p.expect(lexer.TokenKindFeedback); err != nil {
			return nil, err
		}
		items, err := p.parseFeedbackItems()
		if err != nil {
			return nil, err
		}
		stmt.FeedbackItems = items

	default:
		stmt.Mode = ast.QueryModeNearest
		switch tok.Kind {
		case lexer.TokenKindString:
			text := tok.Value
			stmt.QueryText = &text
			p.advance()
		case lexer.TokenKindInteger:
			id, err := p.parsePointIDValue("QUERY")
			if err != nil {
				return nil, err
			}
			stmt.QueryID = id
		default:
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
	case lexer.TokenKindSample:
		stmt.Mode = ast.QueryModeSample
		p.advance()
	default:
		stmt.Mode = ast.QueryModeNearest
		switch tok.Kind {
		case lexer.TokenKindString:
			text := tok.Value
			stmt.QueryText = &text
			p.advance()
		case lexer.TokenKindInteger:
			id, err := p.parsePointIDValue("QUERY")
			if err != nil {
				return nil, err
			}
			stmt.QueryID = id
		default:
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
		key := keyTok.Value
		if _, err := p.expect(lexer.TokenKindEquals); err != nil {
			return
		}
		switch {
		case asciiEqualLower(key, "positive"):
			ids, err := p.parsePointIDList()
			if err != nil {
				return
			}
			stmt.PositiveIDs = ids
		case asciiEqualLower(key, "negative"):
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

// parseFeedbackItems parses ((id, score), (id, score), ...) for RELEVANCE FEEDBACK.
func (p *Parser) parseFeedbackItems() ([]ast.FeedbackItem, error) {
	if _, err := p.expect(lexer.TokenKindLparen); err != nil {
		return nil, err
	}
	var items []ast.FeedbackItem
	for {
		if _, err := p.expect(lexer.TokenKindLparen); err != nil {
			return nil, err
		}
		exampleVal, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindComma); err != nil {
			return nil, err
		}
		scoreTok, err := p.parseNumericLiteral()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKindRparen); err != nil {
			return nil, err
		}
		items = append(items, ast.FeedbackItem{Example: exampleVal, Score: scoreTok})
		if p.peek().Kind == lexer.TokenKindComma {
			p.advance()
			if p.peek().Kind == lexer.TokenKindRparen {
				break
			}
			continue
		}
		break
	}
	if _, err := p.expect(lexer.TokenKindRparen); err != nil {
		return nil, err
	}
	return items, nil
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

	seenWhere, seenRerank, seenGroup, seenGroupSize := false, false, false, false
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
			if p.peek().Kind == lexer.TokenKindVector || (p.peek().Kind == lexer.TokenKindIdentifier && asciiEqual(p.peek().Value, "VECTOR")) {
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
				if p.peek().Kind != lexer.TokenKindIdentifier {
					break
				}
				ref := ast.PrefetchRef{CTEName: p.peek().Value}
				p.advance()

				// Optional per-prefetch WHERE
				if p.peek().Kind == lexer.TokenKindWhere {
					p.advance()
					filter, err := p.parseFilterExpr()
					if err != nil {
						return
					}
					ref.Filter = filter
				}

				// Optional per-prefetch SCORE THRESHOLD
				if p.peek().Kind == lexer.TokenKindScore {
					p.advance()
					if _, err := p.expect(lexer.TokenKindThreshold); err != nil {
						return
					}
					scoreTok := p.peek()
					if scoreTok.Kind == lexer.TokenKindFloat || scoreTok.Kind == lexer.TokenKindInteger {
						p.advance()
						f, err := parseFloatToken(scoreTok)
						if err != nil {
							return
						}
						ref.ScoreThreshold = &f
					}
				}

				// Optional per-prefetch LOOKUP FROM
				if p.peek().Kind == lexer.TokenKindLookup {
					p.advance()
					if _, err := p.expect(lexer.TokenKindFrom); err != nil {
						return
					}
					lookupFrom, err := p.parseIdentifier()
					if err != nil {
						return
					}
					ref.LookupFrom = lookupFrom
					if p.peek().Kind == lexer.TokenKindVector || (p.peek().Kind == lexer.TokenKindIdentifier && asciiEqual(p.peek().Value, "VECTOR")) {
						p.advance()
						lv, _ := p.parseStringPtr()
						ref.LookupVector = lv
					}
				}

				stmt.PrefetchRefs = append(stmt.PrefetchRefs, ref)

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
			if fusionTok.Kind != lexer.TokenKindIdentifier || (!asciiEqual(fusionTok.Value, "RRF") && !asciiEqual(fusionTok.Value, "DBSF")) {
				return
			}
			p.advance()
			upper := fusionTok.Value
			if asciiEqual(fusionTok.Value, "RRF") {
				upper = "RRF"
			} else {
				upper = "DBSF"
			}
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
			} else if p.peek().Kind == lexer.TokenKindVectors {
				p.advance()
				parsed, err := p.parseWithVectors()
				if err != nil {
					return
				}
				stmt.WithVectors = parsed
			} else if p.peek().Kind == lexer.TokenKindLookup {
				p.advance()
				if _, err := p.expect(lexer.TokenKindFrom); err != nil {
					return
				}
				collection, err := p.parseIdentifier()
				if err != nil {
					return
				}
				stmt.WithLookupCollection = &collection
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
			// Check for NAIVE strategy with params: STRATEGY NAIVE (a = ..., b = ..., c = ...)
			if p.peek().Kind == lexer.TokenKindIdentifier && asciiEqualLower(p.peek().Value, "naive") {
				p.advance()
				if _, err := p.expect(lexer.TokenKindLparen); err != nil {
					return
				}
				strat := &ast.FeedbackStrategy{Type: ast.FeedbackStrategyNaive}
				for p.peek().Kind != lexer.TokenKindRparen {
					key, err := p.parseIdentifier()
					if err != nil {
						return
					}
					if _, err := p.expect(lexer.TokenKindEquals); err != nil {
						return
					}
					val, err := p.parseNumericLiteral()
					if err != nil {
						return
					}
					switch {
					case asciiEqualLower(key, "a"):
						strat.A = val
					case asciiEqualLower(key, "b"):
						strat.B = val
					case asciiEqualLower(key, "c"):
						strat.C = val
					}
					if p.peek().Kind == lexer.TokenKindComma {
						p.advance()
					}
				}
				p.advance() // consume )
				stmt.FeedbackStrategy = strat
			} else {
				strategy, err := p.parseStringPtr()
				if err != nil {
					return
				}
				stmt.Strategy = strategy
			}

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

		case lexer.TokenKindBoost:
			p.advance()
			expr, err := p.parseFormulaExpr(precedenceLowest)
			if err != nil {
				return
			}
			stmt.Formula = expr

		case lexer.TokenKindDefaults:
			p.advance()
			if _, err := p.expect(lexer.TokenKindLparen); err != nil {
				return
			}
			defaults := make(map[string]any)
			for p.peek().Kind != lexer.TokenKindRparen {
				key, err := p.parseIdentifier()
				if err != nil {
					return
				}
				if _, err := p.expect(lexer.TokenKindEquals); err != nil {
					return
				}
				valTok := p.peek()
				if valTok.Kind != lexer.TokenKindFloat && valTok.Kind != lexer.TokenKindInteger {
					return
				}
				p.advance()
				f, err := strconv.ParseFloat(valTok.Value, 64)
				if err != nil {
					return
				}
				defaults[key] = f
				if p.peek().Kind == lexer.TokenKindComma {
					p.advance()
				} else {
					break
				}
			}
			p.expect(lexer.TokenKindRparen)
			stmt.FormulaDefaults = defaults

		default:
			return
		}
	}
}
