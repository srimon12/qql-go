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
		if tok2 := p.peek(); tok2.Kind == lexer.TokenKindPositive {
			p.advance()
			if _, err := p.expect(lexer.TokenKindIds); err != nil {
				return nil, err
			}
			ids, err := p.parsePointIDList()
			if err != nil {
				return nil, err
			}
			stmt.PositiveIDs = ids
		} else {
			return nil, errors.NewQQLSyntaxError("Expected POSITIVE IDS after QUERY RECOMMEND", tok2.Pos)
		}
		if tok3 := p.peek(); tok3.Kind == lexer.TokenKindNegative {
			p.advance()
			if _, err := p.expect(lexer.TokenKindIds); err != nil {
				return nil, err
			}
			ids, err := p.parsePointIDList()
			if err != nil {
				return nil, err
			}
			stmt.NegativeIDs = ids
		}
	case lexer.TokenKindContext:
		stmt.Mode = ast.QueryModeContext
		p.advance()
		if _, err := p.expect(lexer.TokenKindPairs); err != nil {
			return nil, err
		}
		for {
			if _, err := p.expect(lexer.TokenKindLparen); err != nil {
				return nil, err
			}
			posId, err := p.parsePointIDValue("CONTEXT POSITIVE")
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokenKindComma); err != nil {
				return nil, err
			}
			negId, err := p.parsePointIDValue("CONTEXT NEGATIVE")
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokenKindRparen); err != nil {
				return nil, err
			}

			stmt.ContextPairs = append(stmt.ContextPairs, ast.ContextPair{Positive: posId, Negative: negId})

			if p.peek().Kind == lexer.TokenKindComma {
				p.advance()
			} else {
				break
			}
		}
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
		if tok2 := p.peek(); tok2.Kind == lexer.TokenKindContext {
			p.advance()
			if _, err := p.expect(lexer.TokenKindPairs); err != nil {
				return nil, err
			}
			for {
				if _, err := p.expect(lexer.TokenKindLparen); err != nil {
					return nil, err
				}
				posId, err := p.parsePointIDValue("DISCOVER CONTEXT")
				if err != nil {
					return nil, err
				}
				if _, err := p.expect(lexer.TokenKindComma); err != nil {
					return nil, err
				}
				negId, err := p.parsePointIDValue("DISCOVER CONTEXT")
				if err != nil {
					return nil, err
				}
				if _, err := p.expect(lexer.TokenKindRparen); err != nil {
					return nil, err
				}

				stmt.ContextPairs = append(stmt.ContextPairs, ast.ContextPair{Positive: posId, Negative: negId})

				if p.peek().Kind == lexer.TokenKindComma {
					p.advance()
				} else {
					break
				}
			}
		}
	default:
		// Assume NEAREST mode for any string or ID
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

	if p.peek().Kind == lexer.TokenKindLimit {
		p.advance()
		limitTok, err := p.expect(lexer.TokenKindInteger)
		if err != nil {
			return nil, err
		}
		limit, err := parseIntToken(limitTok)
		if err != nil {
			return nil, err
		}
		stmt.Limit = limit
	} else {
		stmt.Limit = 10
	}

	if p.peek().Kind == lexer.TokenKindOffset {
		p.advance()
		offsetTok := p.peek()
		offset, err := parseIntToken(p.advance())
		if err != nil {
			return nil, err
		}
		if offset < 0 {
			return nil, errors.NewQQLSyntaxError("OFFSET must be a non-negative integer", offsetTok.Pos)
		}
		stmt.Offset = offset
	}

	if p.peek().Kind == lexer.TokenKindScore {
		p.advance()
		if _, err := p.expect(lexer.TokenKindThreshold); err != nil {
			return nil, err
		}
		scoreTok := p.peek()
		switch scoreTok.Kind {
		case lexer.TokenKindFloat:
			p.advance()
			f, err := parseFloatToken(scoreTok)
			if err != nil {
				return nil, err
			}
			stmt.ScoreThreshold = &f
		case lexer.TokenKindInteger:
			p.advance()
			v, err := parseIntToken(scoreTok)
			if err != nil {
				return nil, err
			}
			f := float64(v)
			stmt.ScoreThreshold = &f
		default:
			return nil, errors.NewQQLSyntaxError("Expected float or integer for SCORE THRESHOLD", scoreTok.Pos)
		}
	}

	if p.peek().Kind == lexer.TokenKindLookup {
		p.advance()
		if _, err := p.expect(lexer.TokenKindFrom); err != nil {
			return nil, err
		}
		lookupFrom, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}
		stmt.LookupFrom = lookupFrom
		if p.peek().Kind == lexer.TokenKindVector || (p.peek().Kind == lexer.TokenKindIdentifier && strings.ToUpper(p.peek().Value) == "VECTOR") {
			p.advance()
			lookupVector, err := p.parseStringPtr()
			if err != nil {
				return nil, err
			}
			stmt.LookupVector = lookupVector
		}
	}

	if p.peek().Kind == lexer.TokenKindUsing {
		p.advance()
		if p.peek().Kind == lexer.TokenKindHybrid {
			p.advance()
			stmt.Type = ast.QueryTypeHybrid
		} else if p.peek().Kind == lexer.TokenKindSparse {
			p.advance()
			stmt.Type = ast.QueryTypeSparse
			if p.peek().Kind == lexer.TokenKindString {
				vecNameTok := p.peek()
				p.advance()
				stmt.Using = &vecNameTok.Value
			}
		} else if p.peek().Kind == lexer.TokenKindString {
			vecNameTok := p.peek()
			p.advance()
			stmt.Using = &vecNameTok.Value
			stmt.Type = ast.QueryTypeDense
		} else {
			return nil, errors.NewQQLSyntaxError("Expected HYBRID, SPARSE, or a vector name string after USING", p.peek().Pos)
		}
	} else {
		stmt.Type = ast.QueryTypeDense
	}

	if p.peek().Kind == lexer.TokenKindPrefetch {
		if stmt.Type == ast.QueryTypeHybrid {
			return nil, errors.NewQQLSyntaxError("Cannot combine USING HYBRID with manual PREFETCH blocks", p.peek().Pos)
		}
		prefetches, err := p.parsePrefetchClause()
		if err != nil {
			return nil, err
		}
		stmt.Prefetches = prefetches
	}

	seenWith := false
	if p.peek().Kind == lexer.TokenKindWith {
		// Lookahead to check if it's WITH MODEL
		seenWith = true
		p.advance() // Consume WITH
		if p.peek().Kind == lexer.TokenKindIdentifier && strings.ToUpper(p.peek().Value) == "MODEL" {
			p.advance() // Consume MODEL
			modelTok, err := p.expect(lexer.TokenKindString)
			if err != nil {
				return nil, err
			}
			stmt.Model = &modelTok.Value
		} else {
			// It's a WITH { ... } clause
			parsedWith, err := p.parseWithClause()
			if err != nil {
				return nil, err
			}
			mergeSearchWith(&stmt.WithClause, parsedWith)
		}
	}

	// For now, let's keep it simple.
	seenWhere := false
	seenRerank := false
	seenGroup := false
	seenExact := false
	seenGroupSize := false
	seenStrategy := false
	seenFusion := false

	for {
		switch p.peek().Kind {
		case lexer.TokenKindWhere:
			if seenWhere {
				return nil, errors.NewQQLSyntaxError("Duplicate WHERE clause", p.peek().Pos)
			}
			seenWhere = true
			p.advance()
			filter, err := p.parseFilterExpr()
			if err != nil {
				return nil, err
			}
			stmt.QueryFilter = filter
		case lexer.TokenKindRerank:
			if seenRerank {
				return nil, errors.NewQQLSyntaxError("Duplicate RERANK clause", p.peek().Pos)
			}
			seenRerank = true
			p.advance()
			stmt.Rerank = true
			rerankModel, err := p.parseOptionalModelString()
			if err != nil {
				return nil, err
			}
			stmt.RerankModel = rerankModel
		case lexer.TokenKindExact:
			if seenExact {
				return nil, errors.NewQQLSyntaxError("Duplicate EXACT clause", p.peek().Pos)
			}
			seenExact = true
			p.advance()
			mergeSearchWith(&stmt.WithClause, &ast.SearchWith{Exact: true})
		case lexer.TokenKindFusion:
			if seenFusion {
				return nil, errors.NewQQLSyntaxError("Duplicate FUSION clause", p.peek().Pos)
			}
			seenFusion = true
			p.advance()
			fusionTok := p.peek()
			if fusionTok.Kind != lexer.TokenKindIdentifier || (strings.ToUpper(fusionTok.Value) != "RRF" && strings.ToUpper(fusionTok.Value) != "DBSF") {
				return nil, errors.NewQQLSyntaxError("Expected RRF or DBSF after FUSION", fusionTok.Pos)
			}
			p.advance()
			if stmt.Type != ast.QueryTypeHybrid && len(stmt.Prefetches) == 0 {
				return nil, errors.NewQQLSyntaxError("FUSION requires PREFETCH blocks or USING HYBRID", fusionTok.Pos)
			}
			upperFusion := strings.ToUpper(fusionTok.Value)
			stmt.FusionType = &upperFusion
		case lexer.TokenKindWith:
			if seenWith {
				return nil, errors.NewQQLSyntaxError("Duplicate WITH clause", p.peek().Pos)
			}
			seenWith = true
			p.advance()
			if p.peek().Kind == lexer.TokenKindIdentifier && strings.ToUpper(p.peek().Value) == "MODEL" {
				p.advance()
				modelTok, err := p.expect(lexer.TokenKindString)
				if err != nil {
					return nil, err
				}
				stmt.Model = &modelTok.Value
			} else {
				parsedWith, err := p.parseWithClause()
				if err != nil {
					return nil, err
				}
				mergeSearchWith(&stmt.WithClause, parsedWith)
			}
		case lexer.TokenKindGroup:
			if seenGroup {
				return nil, errors.NewQQLSyntaxError("Duplicate GROUP BY clause", p.peek().Pos)
			}
			seenGroup = true
			p.advance()
			if _, err := p.expect(lexer.TokenKindBy); err != nil {
				return nil, err
			}
			groupField, err := p.parseStringPtr()
			if err != nil {
				return nil, err
			}
			stmt.GroupBy = groupField
		case lexer.TokenKindGroupSize:
			if seenGroupSize {
				return nil, errors.NewQQLSyntaxError("Duplicate GROUP_SIZE clause", p.peek().Pos)
			}
			seenGroupSize = true
			p.advance()
			groupSizeTok := p.peek()
			groupSizeVal, err := p.parseNumericLiteral()
			if err != nil {
				return nil, err
			}
			if groupSizeVal <= 0 || float64(uint64(groupSizeVal)) != groupSizeVal {
				return nil, errors.NewQQLSyntaxError("GROUP_SIZE must be a positive integer", groupSizeTok.Pos)
			}
			sizeInt := int(groupSizeVal)
			stmt.GroupSize = &sizeInt
		case lexer.TokenKindStrategy:
			if seenStrategy {
				return nil, errors.NewQQLSyntaxError("Duplicate STRATEGY clause", p.peek().Pos)
			}
			seenStrategy = true
			p.advance()
			strategy, err := p.parseStringPtr()
			if err != nil {
				return nil, err
			}
			stmt.Strategy = strategy
		case lexer.TokenKindLimit:
			p.advance()
			limitTok, err := p.expect(lexer.TokenKindInteger)
			if err != nil {
				return nil, err
			}
			limit, err := parseIntToken(limitTok)
			if err != nil {
				return nil, err
			}
			stmt.Limit = limit
		case lexer.TokenKindOffset:
			p.advance()
			offsetTok := p.peek()
			offset, err := parseIntToken(p.advance())
			if err != nil {
				return nil, err
			}
			if offset < 0 {
				return nil, errors.NewQQLSyntaxError("OFFSET must be a non-negative integer", offsetTok.Pos)
			}
			stmt.Offset = offset
		default:
			return stmt, nil
		}
	}
}

func (p *Parser) parsePrefetchClause() ([]*ast.Prefetch, error) {
	if _, err := p.expect(lexer.TokenKindPrefetch); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindLparen); err != nil {
		return nil, err
	}

	var prefetches []*ast.Prefetch
	if p.peek().Kind == lexer.TokenKindRparen {
		return nil, errors.NewQQLSyntaxError("Expected at least one PREFETCH block inside ()", p.peek().Pos)
	}

	for {
		prefetch, err := p.parseSinglePrefetch()
		if err != nil {
			return nil, err
		}
		prefetches = append(prefetches, prefetch)

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
	return prefetches, nil
}

func (p *Parser) parseSinglePrefetch() (*ast.Prefetch, error) {
	prefetch := &ast.Prefetch{}

	if p.peek().Kind == lexer.TokenKindPrefetch {
		nested, err := p.parsePrefetchClause()
		if err != nil {
			return nil, err
		}
		prefetch.Prefetches = nested
	} else if p.peek().Kind == lexer.TokenKindQuery {
		p.advance()
		tok := p.peek()
		if tok.Kind == lexer.TokenKindNearest {
			p.advance()
			tok = p.peek()
		}

		switch tok.Kind {
		case lexer.TokenKindRecommend:
			prefetch.Mode = ast.QueryModeRecommend
			p.advance()
			if tok2 := p.peek(); tok2.Kind == lexer.TokenKindPositive {
				p.advance()
				if _, err := p.expect(lexer.TokenKindIds); err != nil {
					return nil, err
				}
				ids, err := p.parsePointIDList()
				if err != nil {
					return nil, err
				}
				prefetch.PositiveIDs = ids
			} else {
				return nil, errors.NewQQLSyntaxError("Expected POSITIVE IDS after QUERY RECOMMEND", tok2.Pos)
			}
			if tok3 := p.peek(); tok3.Kind == lexer.TokenKindNegative {
				p.advance()
				if _, err := p.expect(lexer.TokenKindIds); err != nil {
					return nil, err
				}
				ids, err := p.parsePointIDList()
				if err != nil {
					return nil, err
				}
				prefetch.NegativeIDs = ids
			}
		case lexer.TokenKindContext:
			prefetch.Mode = ast.QueryModeContext
			p.advance()
			if _, err := p.expect(lexer.TokenKindPairs); err != nil {
				return nil, err
			}
			for {
				if _, err := p.expect(lexer.TokenKindLparen); err != nil {
					return nil, err
				}
				posId, err := p.parsePointIDValue("CONTEXT POSITIVE")
				if err != nil {
					return nil, err
				}
				if _, err := p.expect(lexer.TokenKindComma); err != nil {
					return nil, err
				}
				negId, err := p.parsePointIDValue("CONTEXT NEGATIVE")
				if err != nil {
					return nil, err
				}
				if _, err := p.expect(lexer.TokenKindRparen); err != nil {
					return nil, err
				}

				prefetch.ContextPairs = append(prefetch.ContextPairs, ast.ContextPair{Positive: posId, Negative: negId})

				if p.peek().Kind == lexer.TokenKindComma {
					p.advance()
				} else {
					break
				}
			}
		case lexer.TokenKindDiscover:
			prefetch.Mode = ast.QueryModeDiscover
			p.advance()
			if _, err := p.expect(lexer.TokenKindTarget); err != nil {
				return nil, err
			}
			targetId, err := p.parsePointIDValue("DISCOVER TARGET")
			if err != nil {
				return nil, err
			}
			prefetch.Target = targetId
			if tok2 := p.peek(); tok2.Kind == lexer.TokenKindContext {
				p.advance()
				if _, err := p.expect(lexer.TokenKindPairs); err != nil {
					return nil, err
				}
				for {
					if _, err := p.expect(lexer.TokenKindLparen); err != nil {
						return nil, err
					}
					posId, err := p.parsePointIDValue("DISCOVER CONTEXT")
					if err != nil {
						return nil, err
					}
					if _, err := p.expect(lexer.TokenKindComma); err != nil {
						return nil, err
					}
					negId, err := p.parsePointIDValue("DISCOVER CONTEXT")
					if err != nil {
						return nil, err
					}
					if _, err := p.expect(lexer.TokenKindRparen); err != nil {
						return nil, err
					}

					prefetch.ContextPairs = append(prefetch.ContextPairs, ast.ContextPair{Positive: posId, Negative: negId})

					if p.peek().Kind == lexer.TokenKindComma {
						p.advance()
					} else {
						break
					}
				}
			}
		default:
			prefetch.Mode = ast.QueryModeNearest
			if tok.Kind == lexer.TokenKindString {
				p.advance()
				prefetch.QueryText = &tok.Value
			} else if tok.Kind == lexer.TokenKindInteger {
				p.advance()
				val, err := parseIntToken(tok)
				if err != nil {
					return nil, err
				}
				prefetch.QueryID = val
			} else {
				return nil, errors.NewQQLSyntaxError("Expected string, integer, RECOMMEND, DISCOVER, or CONTEXT for PREFETCH QUERY", tok.Pos)
			}
		}
	}

	for {
		tok := p.peek()
		switch tok.Kind {
		case lexer.TokenKindUsing:
			p.advance()
			if p.peek().Kind == lexer.TokenKindString {
				vecTok := p.peek()
				p.advance()
				prefetch.Using = &vecTok.Value
			} else if p.peek().Kind == lexer.TokenKindDense {
				p.advance()
				dense := "dense"
				prefetch.Using = &dense
				prefetch.Type = ast.QueryTypeDense
			} else if p.peek().Kind == lexer.TokenKindSparse {
				p.advance()
				prefetch.Type = ast.QueryTypeSparse
				if p.peek().Kind == lexer.TokenKindString {
					vecTok := p.peek()
					p.advance()
					prefetch.Using = &vecTok.Value
				} else {
					sparse := "sparse"
					prefetch.Using = &sparse
				}
			} else {
				return nil, errors.NewQQLSyntaxError("Expected string vector name, DENSE, or SPARSE after USING in PREFETCH", p.peek().Pos)
			}
		case lexer.TokenKindLimit:
			p.advance()
			intTok, err := p.expect(lexer.TokenKindInteger)
			if err != nil {
				return nil, err
			}
			limit, err := parseIntToken(intTok)
			if err != nil {
				return nil, err
			}
			prefetch.Limit = limit
		case lexer.TokenKindScore:
			p.advance()
			if _, err := p.expect(lexer.TokenKindThreshold); err != nil {
				return nil, err
			}
			scoreTok := p.peek()
			if scoreTok.Kind != lexer.TokenKindFloat && scoreTok.Kind != lexer.TokenKindInteger {
				return nil, errors.NewQQLSyntaxError("Expected float or integer for SCORE THRESHOLD", scoreTok.Pos)
			}
			p.advance()
			val, _ := parseFloatToken(scoreTok)
			prefetch.ScoreThreshold = &val
		case lexer.TokenKindWhere:
			p.advance()
			filter, err := p.parseFilterExpr()
			if err != nil {
				return nil, err
			}
			prefetch.QueryFilter = filter
		case lexer.TokenKindLookup:
			p.advance()
			if _, err := p.expect(lexer.TokenKindFrom); err != nil {
				return nil, err
			}
			lookupFrom, err := p.parseIdentifier()
			if err != nil {
				return nil, err
			}
			prefetch.LookupFrom = lookupFrom
			if p.peek().Kind == lexer.TokenKindVector || (p.peek().Kind == lexer.TokenKindIdentifier && strings.ToUpper(p.peek().Value) == "VECTOR") {
				p.advance()
				lookupVector, err := p.parseStringPtr()
				if err != nil {
					return nil, err
				}
				prefetch.LookupVector = lookupVector
			}
		case lexer.TokenKindWith:
			p.advance()
			parsedWith, err := p.parseWithClause()
			if err != nil {
				return nil, err
			}
			mergeSearchWith(&prefetch.WithClause, parsedWith)
		default:
			return prefetch, nil
		}
	}
}
