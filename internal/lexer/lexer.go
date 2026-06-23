//go:generate go run gen.go

package lexer

import (
	"github.com/srimon12/qql-go/internal/errors"
)

var keywords = map[string]TokenKind{
	"GEO_BBOX":     TokenKindGeoBbox,
	"GEO_RADIUS":   TokenKindGeoRadius,
	"VALUES_COUNT": TokenKindValuesCount,
	"HAS_VECTOR":   TokenKindHasVector,
	"BOOST":        TokenKindBoost,
	"DEFAULTS":     TokenKindDefaults,
	"CASE":         TokenKindCase,
	"WHEN":         TokenKindWhen,
	"THEN":         TokenKindThen,
	"ELSE":         TokenKindElse,
	"END":          TokenKindEnd,
	"INSERT":       TokenKindInsert,
	"INTO":         TokenKindInto,
	"COLLECTION":   TokenKindCollection,
	"VALUES":       TokenKindValues,
	"USING":        TokenKindUsing,
	"MODEL":        TokenKindModel,
	"HYBRID":       TokenKindHybrid,
	"DENSE":        TokenKindDense,
	"SPARSE":       TokenKindSparse,
	"RERANK":       TokenKindRerank,
	"EXACT":        TokenKindExact,
	"WITH":         TokenKindWith,
	"AS":           TokenKindAs,
	"ACORN":        TokenKindAcorn,
	"QUANTIZE":     TokenKindQuantize,
	"SCALAR":       TokenKindScalar,
	"BINARY":       TokenKindBinary,
	"PRODUCT":      TokenKindProduct,
	"TURBO":        TokenKindTurbo,
	"BITS":         TokenKindBits,
	"QUANTILE":     TokenKindQuantile,
	"ALWAYS":       TokenKindAlways,
	"RAM":          TokenKindRam,
	"HNSW":         TokenKindHnsw,
	"VECTORS":      TokenKindVectors,
	"OPTIMIZERS":   TokenKindOptimizers,
	"PARAMS":       TokenKindParams,
	"DISABLED":     TokenKindDisabled,
	"CREATE":       TokenKindCreate,
	"ALTER":        TokenKindAlter,
	"DROP":         TokenKindDrop,
	"SHOW":         TokenKindShow,
	"COLLECTIONS":  TokenKindCollections,
	"SELECT":       TokenKindSelect,
	"SCROLL":       TokenKindScroll,
	"AFTER":        TokenKindAfter,
	"RECOMMEND":    TokenKindRecommend,
	"QUERY":        TokenKindQuery,
	"NEAREST":      TokenKindNearest,
	"CONTEXT":      TokenKindContext,
	"DISCOVER":     TokenKindDiscover,
	"PAIRS":        TokenKindPairs,
	"TARGET":       TokenKindTarget,
	"ORDER":        TokenKindOrder,
	"ASC":          TokenKindAsc,
	"DESC":         TokenKindDesc,
	"LIMIT":        TokenKindLimit,
	"GROUP":        TokenKindGroup,
	"BY":           TokenKindBy,
	"GROUP_SIZE":   TokenKindGroupSize,
	"STRATEGY":     TokenKindStrategy,
	"DELETE":       TokenKindDelete,
	"UPDATE":       TokenKindUpdate,
	"SET":          TokenKindSet,
	"VECTOR":       TokenKindVector,
	"PAYLOAD":      TokenKindPayload,
	"FROM":         TokenKindFrom,
	"WHERE":        TokenKindWhere,
	"ID":           TokenKindId,
	"INDEX":        TokenKindIndex,
	"ON":           TokenKindOn,
	"FOR":          TokenKindFor,
	"TYPE":         TokenKindType,
	"AND":          TokenKindAnd,
	"OR":           TokenKindOr,
	"NOT":          TokenKindNot,
	"IN":           TokenKindIn,
	"BETWEEN":      TokenKindBetween,
	"IS":           TokenKindIs,
	"NULL":         TokenKindNull,
	"EMPTY":        TokenKindEmpty,
	"MATCH":        TokenKindMatch,
	"ANY":          TokenKindAny,
	"PHRASE":       TokenKindPhrase,
	"OFFSET":       TokenKindOffset,
	"SCORE":        TokenKindScore,
	"THRESHOLD":    TokenKindThreshold,
	"LOOKUP":       TokenKindLookup,
	"COSINE":       TokenKindCosine,
	"DOT":          TokenKindDot,
	"EUCLID":       TokenKindEuclid,
	"MANHATTAN":    TokenKindManhattan,
	"PREFETCH":     TokenKindPrefetch,
	"FUSION":       TokenKindFusion,
	"SAMPLE":       TokenKindSample,
	"RELEVANCE":    TokenKindRelevance,
	"FEEDBACK":     TokenKindFeedback,
}

type Lexer struct{}

func (l *Lexer) Tokenize(query string) ([]Token, error) {
	tokens := make([]Token, 0, len(query)/8)
	i := 0
	n := len(query)

	for {
		if i >= n {
			break
		}
		ch := query[i]

		if isWhitespace(ch) {
			i++
			continue
		}

		switch ch {
		case '{':
			tokens = append(tokens, Token{Kind: TokenKindLbrace, Value: "{", Pos: i})
			i++
		case '}':
			tokens = append(tokens, Token{Kind: TokenKindRbrace, Value: "}", Pos: i})
			i++
		case '[':
			tokens = append(tokens, Token{Kind: TokenKindLbracket, Value: "[", Pos: i})
			i++
		case ']':
			tokens = append(tokens, Token{Kind: TokenKindRbracket, Value: "]", Pos: i})
			i++
		case '(':
			tokens = append(tokens, Token{Kind: TokenKindLparen, Value: "(", Pos: i})
			i++
		case ')':
			tokens = append(tokens, Token{Kind: TokenKindRparen, Value: ")", Pos: i})
			i++
		case '*':
			tokens = append(tokens, Token{Kind: TokenKindStar, Value: "*", Pos: i})
			i++
		case ':':
			tokens = append(tokens, Token{Kind: TokenKindColon, Value: ":", Pos: i})
			i++
		case ',':
			tokens = append(tokens, Token{Kind: TokenKindComma, Value: ",", Pos: i})
			i++
		case '=':
			tokens = append(tokens, Token{Kind: TokenKindEquals, Value: "=", Pos: i})
			i++
		case '!':
			if i+1 < n && query[i+1] == '=' {
				tokens = append(tokens, Token{Kind: TokenKindNotEquals, Value: "!=", Pos: i})
				i += 2
			} else {
				return nil, errors.NewQQLSyntaxError("Unexpected character '!'", i)
			}
		case '>':
			if i+1 < n && query[i+1] == '=' {
				tokens = append(tokens, Token{Kind: TokenKindGte, Value: ">=", Pos: i})
				i += 2
			} else {
				tokens = append(tokens, Token{Kind: TokenKindGt, Value: ">", Pos: i})
				i++
			}
		case '<':
			if i+1 < n && query[i+1] == '=' {
				tokens = append(tokens, Token{Kind: TokenKindLte, Value: "<=", Pos: i})
				i += 2
			} else {
				tokens = append(tokens, Token{Kind: TokenKindLt, Value: "<", Pos: i})
				i++
			}
		case '"', '\'':
			token, endPos, err := l.readString(query, i, ch)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
			i = endPos
		case '+':
			tokens = append(tokens, Token{Kind: TokenKindPlus, Value: "+", Pos: i})
			i++
		case '/':
			tokens = append(tokens, Token{Kind: TokenKindSlash, Value: "/", Pos: i})
			i++
		case '-':
			if i+1 < n && isDigit(query[i+1]) {
				token := l.readNumber(query, i)
				tokens = append(tokens, token)
				i = token.Pos + len(token.Value)
			} else {
				tokens = append(tokens, Token{Kind: TokenKindMinus, Value: "-", Pos: i})
				i++
			}
		default:
			if isDigit(ch) {
				token := l.readNumber(query, i)
				tokens = append(tokens, token)
				i = token.Pos + len(token.Value)
			} else if isAlpha(ch) || ch == '_' || ch == '$' {
				token := l.readIdentifier(query, i)
				tokens = append(tokens, token)
				i = token.Pos + len(token.Value)
			} else {
				return nil, errors.NewQQLSyntaxError("Unexpected character '"+string(ch)+"'", i)
			}
		}
	}

	tokens = append(tokens, Token{Kind: TokenKindEof, Value: "", Pos: n})
	return tokens, nil
}

func (l *Lexer) readString(query string, start int, quote byte) (Token, int, error) {
	i := start + 1
	n := len(query)

	for i < n {
		ch := query[i]
		if ch == '\\' {
			break
		}
		if ch == quote {
			return Token{Kind: TokenKindString, Value: query[start+1 : i], Pos: start}, i + 1, nil
		}
		i++
	}

	buf := make([]byte, 0, i-start-1)
	buf = append(buf, query[start+1:i]...)

	for i < n {
		ch := query[i]
		if ch == '\\' && i+1 < n {
			nextCh := query[i+1]
			switch nextCh {
			case 'n':
				buf = append(buf, '\n')
			case 't':
				buf = append(buf, '\t')
			case '"', '\'', '\\':
				buf = append(buf, nextCh)
			default:
				buf = append(buf, '\\')
				buf = append(buf, nextCh)
			}
			i += 2
		} else if ch == quote {
			return Token{Kind: TokenKindString, Value: string(buf), Pos: start}, i + 1, nil
		} else {
			buf = append(buf, ch)
			i++
		}
	}

	return Token{}, 0, errors.NewQQLSyntaxError("Unterminated string literal", start)
}

func (l *Lexer) readNumber(query string, start int) Token {
	i := start
	n := len(query)

	if query[i] == '-' {
		i++
	}

	for i < n && isDigit(query[i]) {
		i++
	}

	if i < n && query[i] == '.' && i+1 < n && isDigit(query[i+1]) {
		i++
		for i < n && isDigit(query[i]) {
			i++
		}
		return Token{Kind: TokenKindFloat, Value: query[start:i], Pos: start}
	}

	return Token{Kind: TokenKindInteger, Value: query[start:i], Pos: start}
}

func (l *Lexer) readIdentifier(query string, start int) Token {
	i := start
	n := len(query)

	for i < n && (isAlnum(query[i]) || query[i] == '_') {
		i++
	}

	for i < n {
		if query[i] == '.' && i+1 < n && (isAlpha(query[i+1]) || query[i+1] == '_') {
			i++
			for i < n && (isAlnum(query[i]) || query[i] == '_') {
				i++
			}
		} else if i+3 < n && query[i:i+3] == "[]." && (isAlpha(query[i+3]) || query[i+3] == '_') {
			i += 3
			for i < n && (isAlnum(query[i]) || query[i] == '_') {
				i++
			}
		} else {
			break
		}
	}

	word := query[start:i]
	segLen := findDot(word)
	if segLen > 0 && segLen == len(word) {
		if kind, ok := lookupKeywordFast(word[:segLen]); ok {
			return Token{Kind: kind, Value: word, Pos: start}
		}
	}

	return Token{Kind: TokenKindIdentifier, Value: word, Pos: start}
}

func findDot(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return i
		}
	}
	return len(s)
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isAlpha(ch byte) bool {
	if ch == '$' {
		return true
	}
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isAlnum(ch byte) bool {
	return isAlpha(ch) || isDigit(ch)
}
