package lexer

import (
	"github.com/srimon12/qql-go/internal/errors"
)

var keywords = map[string]TokenKind{
	"INSERT":      TokenKindInsert,
	"BULK":        TokenKindBulk,
	"INTO":        TokenKindInto,
	"COLLECTION":  TokenKindCollection,
	"VALUES":      TokenKindValues,
	"USING":       TokenKindUsing,
	"MODEL":       TokenKindModel,
	"HYBRID":      TokenKindHybrid,
	"DENSE":       TokenKindDense,
	"SPARSE":      TokenKindSparse,
	"RERANK":      TokenKindRerank,
	"EXACT":       TokenKindExact,
	"WITH":        TokenKindWith,
	"ACORN":       TokenKindAcorn,
	"QUANTIZE":    TokenKindQuantize,
	"SCALAR":      TokenKindScalar,
	"BINARY":      TokenKindBinary,
	"PRODUCT":     TokenKindProduct,
	"TURBO":       TokenKindTurbo,
	"BITS":        TokenKindBits,
	"QUANTILE":    TokenKindQuantile,
	"ALWAYS":      TokenKindAlways,
	"RAM":         TokenKindRam,
	"HNSW":        TokenKindHnsw,
	"VECTORS":     TokenKindVectors,
	"OPTIMIZERS":  TokenKindOptimizers,
	"PARAMS":      TokenKindParams,
	"DISABLED":    TokenKindDisabled,
	"CREATE":      TokenKindCreate,
	"ALTER":       TokenKindAlter,
	"DROP":        TokenKindDrop,
	"SHOW":        TokenKindShow,
	"COLLECTIONS": TokenKindCollections,
	"SEARCH":      TokenKindSearch,
	"SELECT":      TokenKindSelect,
	"SCROLL":      TokenKindScroll,
	"FUSION":      TokenKindFusion,
	"AFTER":       TokenKindAfter,
	"RECOMMEND":   TokenKindRecommend,
	"QUERY":       TokenKindQuery,
	"NEAREST":     TokenKindNearest,
	"CONTEXT":     TokenKindContext,
	"DISCOVER":    TokenKindDiscover,
	"PAIRS":       TokenKindPairs,
	"TARGET":      TokenKindTarget,
	"SIMILAR":     TokenKindSimilar,
	"TO":          TokenKindTo,
	"LIMIT":       TokenKindLimit,
	"GROUP":       TokenKindGroup,
	"BY":          TokenKindBy,
	"GROUP_SIZE":  TokenKindGroupSize,
	"POSITIVE":    TokenKindPositive,
	"NEGATIVE":    TokenKindNegative,
	"IDS":         TokenKindIds,
	"STRATEGY":    TokenKindStrategy,
	"DELETE":      TokenKindDelete,
	"UPDATE":      TokenKindUpdate,
	"SET":         TokenKindSet,
	"VECTOR":      TokenKindVector,
	"PAYLOAD":     TokenKindPayload,
	"FROM":        TokenKindFrom,
	"WHERE":       TokenKindWhere,
	"ID":          TokenKindId,
	"INDEX":       TokenKindIndex,
	"ON":          TokenKindOn,
	"FOR":         TokenKindFor,
	"TYPE":        TokenKindType,
	"AND":         TokenKindAnd,
	"OR":          TokenKindOr,
	"NOT":         TokenKindNot,
	"IN":          TokenKindIn,
	"BETWEEN":     TokenKindBetween,
	"IS":          TokenKindIs,
	"NULL":        TokenKindNull,
	"EMPTY":       TokenKindEmpty,
	"MATCH":       TokenKindMatch,
	"ANY":         TokenKindAny,
	"PHRASE":      TokenKindPhrase,
	"OFFSET":      TokenKindOffset,
	"SCORE":       TokenKindScore,
	"THRESHOLD":   TokenKindThreshold,
	"LOOKUP":      TokenKindLookup,
	"COSINE":      TokenKindCosine,
	"DOT":         TokenKindDot,
	"EUCLID":      TokenKindEuclid,
	"MANHATTAN":   TokenKindManhattan,
}

type Lexer struct{}

func (l *Lexer) Tokenize(query string) ([]Token, error) {
	tokens := make([]Token, 0)
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
		case '-':
			if i+1 < n && isDigit(query[i+1]) {
				token := l.readNumber(query, i)
				tokens = append(tokens, token)
				i = token.Pos + len(token.Value)
			} else {
				return nil, errors.NewQQLSyntaxError("Unexpected character '-'", i)
			}
		default:
			if isDigit(ch) {
				token := l.readNumber(query, i)
				tokens = append(tokens, token)
				i = token.Pos + len(token.Value)
			} else if isAlpha(ch) || ch == '_' {
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
	buf := make([]byte, 0)

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
	firstSegment := word[:findDot(word)]
	if len(firstSegment) > 0 {
		upperFirst := toUpper(firstSegment)
		if _, ok := keywords[upperFirst]; ok && !containsDot(word) {
			return Token{Kind: keywords[upperFirst], Value: word, Pos: start}
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

func containsDot(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return true
		}
	}
	return false
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

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isAlnum(ch byte) bool {
	return isAlpha(ch) || isDigit(ch)
}
