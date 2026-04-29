package lexer

import (
	"testing"

	"github.com/srimon12/qql-go/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenizeKeywords(t *testing.T) {
	lexer := &Lexer{}
	tests := []struct {
		name     string
		input    string
		expected []Token
	}{
		{
			name:  "INSERT",
			input: "INSERT",
			expected: []Token{
				{Kind: TokenKindInsert, Value: "INSERT", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "BULK",
			input: "BULK",
			expected: []Token{
				{Kind: TokenKindBulk, Value: "BULK", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 4},
			},
		},
		{
			name:  "INTO",
			input: "INTO",
			expected: []Token{
				{Kind: TokenKindInto, Value: "INTO", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 4},
			},
		},
		{
			name:  "COLLECTION",
			input: "COLLECTION",
			expected: []Token{
				{Kind: TokenKindCollection, Value: "COLLECTION", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 10},
			},
		},
		{
			name:  "VALUES",
			input: "VALUES",
			expected: []Token{
				{Kind: TokenKindValues, Value: "VALUES", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "USING",
			input: "USING",
			expected: []Token{
				{Kind: TokenKindUsing, Value: "USING", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 5},
			},
		},
		{
			name:  "MODEL",
			input: "MODEL",
			expected: []Token{
				{Kind: TokenKindModel, Value: "MODEL", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 5},
			},
		},
		{
			name:  "HYBRID",
			input: "HYBRID",
			expected: []Token{
				{Kind: TokenKindHybrid, Value: "HYBRID", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "DENSE",
			input: "DENSE",
			expected: []Token{
				{Kind: TokenKindDense, Value: "DENSE", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 5},
			},
		},
		{
			name:  "SPARSE",
			input: "SPARSE",
			expected: []Token{
				{Kind: TokenKindSparse, Value: "SPARSE", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "RERANK",
			input: "RERANK",
			expected: []Token{
				{Kind: TokenKindRerank, Value: "RERANK", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "EXACT",
			input: "EXACT",
			expected: []Token{
				{Kind: TokenKindExact, Value: "EXACT", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 5},
			},
		},
		{
			name:  "WITH",
			input: "WITH",
			expected: []Token{
				{Kind: TokenKindWith, Value: "WITH", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 4},
			},
		},
		{
			name:  "ACORN",
			input: "ACORN",
			expected: []Token{
				{Kind: TokenKindAcorn, Value: "ACORN", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 5},
			},
		},
		{
			name:  "QUANTIZE",
			input: "QUANTIZE",
			expected: []Token{
				{Kind: TokenKindQuantize, Value: "QUANTIZE", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 8},
			},
		},
		{
			name:  "SCALAR",
			input: "SCALAR",
			expected: []Token{
				{Kind: TokenKindScalar, Value: "SCALAR", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "BINARY",
			input: "BINARY",
			expected: []Token{
				{Kind: TokenKindBinary, Value: "BINARY", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "PRODUCT",
			input: "PRODUCT",
			expected: []Token{
				{Kind: TokenKindProduct, Value: "PRODUCT", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 7},
			},
		},
		{
			name:  "QUANTILE",
			input: "QUANTILE",
			expected: []Token{
				{Kind: TokenKindQuantile, Value: "QUANTILE", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 8},
			},
		},
		{
			name:  "ALWAYS",
			input: "ALWAYS",
			expected: []Token{
				{Kind: TokenKindAlways, Value: "ALWAYS", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "RAM",
			input: "RAM",
			expected: []Token{
				{Kind: TokenKindRam, Value: "RAM", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 3},
			},
		},
		{
			name:  "CREATE",
			input: "CREATE",
			expected: []Token{
				{Kind: TokenKindCreate, Value: "CREATE", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "DROP",
			input: "DROP",
			expected: []Token{
				{Kind: TokenKindDrop, Value: "DROP", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 4},
			},
		},
		{
			name:  "SHOW",
			input: "SHOW",
			expected: []Token{
				{Kind: TokenKindShow, Value: "SHOW", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 4},
			},
		},
		{
			name:  "COLLECTIONS",
			input: "COLLECTIONS",
			expected: []Token{
				{Kind: TokenKindCollections, Value: "COLLECTIONS", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 11},
			},
		},
		{
			name:  "SEARCH",
			input: "SEARCH",
			expected: []Token{
				{Kind: TokenKindSearch, Value: "SEARCH", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "RECOMMEND",
			input: "RECOMMEND",
			expected: []Token{
				{Kind: TokenKindRecommend, Value: "RECOMMEND", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 9},
			},
		},
		{
			name:  "SIMILAR",
			input: "SIMILAR",
			expected: []Token{
				{Kind: TokenKindSimilar, Value: "SIMILAR", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 7},
			},
		},
		{
			name:  "TO",
			input: "TO",
			expected: []Token{
				{Kind: TokenKindTo, Value: "TO", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 2},
			},
		},
		{
			name:  "LIMIT",
			input: "LIMIT",
			expected: []Token{
				{Kind: TokenKindLimit, Value: "LIMIT", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 5},
			},
		},
		{
			name:  "POSITIVE",
			input: "POSITIVE",
			expected: []Token{
				{Kind: TokenKindPositive, Value: "POSITIVE", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 8},
			},
		},
		{
			name:  "NEGATIVE",
			input: "NEGATIVE",
			expected: []Token{
				{Kind: TokenKindNegative, Value: "NEGATIVE", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 8},
			},
		},
		{
			name:  "IDS",
			input: "IDS",
			expected: []Token{
				{Kind: TokenKindIds, Value: "IDS", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 3},
			},
		},
		{
			name:  "STRATEGY",
			input: "STRATEGY",
			expected: []Token{
				{Kind: TokenKindStrategy, Value: "STRATEGY", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 8},
			},
		},
		{
			name:  "DELETE",
			input: "DELETE",
			expected: []Token{
				{Kind: TokenKindDelete, Value: "DELETE", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "FROM",
			input: "FROM",
			expected: []Token{
				{Kind: TokenKindFrom, Value: "FROM", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 4},
			},
		},
		{
			name:  "WHERE",
			input: "WHERE",
			expected: []Token{
				{Kind: TokenKindWhere, Value: "WHERE", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 5},
			},
		},
		{
			name:  "ID",
			input: "ID",
			expected: []Token{
				{Kind: TokenKindId, Value: "ID", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 2},
			},
		},
		{
			name:  "AND",
			input: "AND",
			expected: []Token{
				{Kind: TokenKindAnd, Value: "AND", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 3},
			},
		},
		{
			name:  "OR",
			input: "OR",
			expected: []Token{
				{Kind: TokenKindOr, Value: "OR", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 2},
			},
		},
		{
			name:  "NOT",
			input: "NOT",
			expected: []Token{
				{Kind: TokenKindNot, Value: "NOT", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 3},
			},
		},
		{
			name:  "IN",
			input: "IN",
			expected: []Token{
				{Kind: TokenKindIn, Value: "IN", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 2},
			},
		},
		{
			name:  "BETWEEN",
			input: "BETWEEN",
			expected: []Token{
				{Kind: TokenKindBetween, Value: "BETWEEN", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 7},
			},
		},
		{
			name:  "IS",
			input: "IS",
			expected: []Token{
				{Kind: TokenKindIs, Value: "IS", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 2},
			},
		},
		{
			name:  "NULL",
			input: "NULL",
			expected: []Token{
				{Kind: TokenKindNull, Value: "NULL", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 4},
			},
		},
		{
			name:  "EMPTY",
			input: "EMPTY",
			expected: []Token{
				{Kind: TokenKindEmpty, Value: "EMPTY", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 5},
			},
		},
		{
			name:  "MATCH",
			input: "MATCH",
			expected: []Token{
				{Kind: TokenKindMatch, Value: "MATCH", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 5},
			},
		},
		{
			name:  "ANY",
			input: "ANY",
			expected: []Token{
				{Kind: TokenKindAny, Value: "ANY", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 3},
			},
		},
		{
			name:  "PHRASE",
			input: "PHRASE",
			expected: []Token{
				{Kind: TokenKindPhrase, Value: "PHRASE", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.Tokenize(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, tokens)
		})
	}
}

func TestTokenizeKeywordsCaseInsensitive(t *testing.T) {
	lexer := &Lexer{}
	tests := []struct {
		name     string
		input    string
		expected TokenKind
	}{
		{"lowercase insert", "insert", TokenKindInsert},
		{"mixed case Insert", "Insert", TokenKindInsert},
		{"lowercase where", "where", TokenKindWhere},
		{"mixed case Where", "WhErE", TokenKindWhere},
		{"lowercase from", "from", TokenKindFrom},
		{"uppercase FROM", "FROM", TokenKindFrom},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.Tokenize(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, tokens[0].Kind)
			assert.Equal(t, tt.input, tokens[0].Value)
		})
	}
}

func TestTokenizeStringLiterals(t *testing.T) {
	lexer := &Lexer{}
	tests := []struct {
		name     string
		input    string
		expected []Token
	}{
		{
			name:  "double quoted string",
			input: `"hello"`,
			expected: []Token{
				{Kind: TokenKindString, Value: "hello", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 7},
			},
		},
		{
			name:  "single quoted string",
			input: "'world'",
			expected: []Token{
				{Kind: TokenKindString, Value: "world", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 7},
			},
		},
		{
			name:  "string with escaped quote",
			input: `"hello\"world"`,
			expected: []Token{
				{Kind: TokenKindString, Value: `hello"world`, Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 14},
			},
		},
		{
			name:  "string with escaped single quote",
			input: `'hello\'world'`,
			expected: []Token{
				{Kind: TokenKindString, Value: `hello'world`, Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 14},
			},
		},
		{
			name:  "string with escaped backslash",
			input: `"hello\\world"`,
			expected: []Token{
				{Kind: TokenKindString, Value: `hello\world`, Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 14},
			},
		},
		{
			name:  "empty double quoted string",
			input: `""`,
			expected: []Token{
				{Kind: TokenKindString, Value: "", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 2},
			},
		},
		{
			name:  "empty single quoted string",
			input: "''",
			expected: []Token{
				{Kind: TokenKindString, Value: "", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.Tokenize(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, tokens)
		})
	}
}

func TestTokenizeUnterminatedString(t *testing.T) {
	lexer := &Lexer{}
	tests := []struct {
		name  string
		input string
	}{
		{"unterminated double quote", `"hello`},
		{"unterminated single quote", `'world`},
		{"unterminated with escape", `"hello\n`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lexer.Tokenize(tt.input)
			require.Error(t, err)
			assert.IsType(t, &errors.QQLSyntaxError{}, err)
		})
	}
}

func TestTokenizeNumbers(t *testing.T) {
	lexer := &Lexer{}
	tests := []struct {
		name     string
		input    string
		expected []Token
	}{
		{
			name:  "integer",
			input: "123",
			expected: []Token{
				{Kind: TokenKindInteger, Value: "123", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 3},
			},
		},
		{
			name:  "float",
			input: "123.456",
			expected: []Token{
				{Kind: TokenKindFloat, Value: "123.456", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 7},
			},
		},
		{
			name:  "negative integer",
			input: "-123",
			expected: []Token{
				{Kind: TokenKindInteger, Value: "-123", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 4},
			},
		},
		{
			name:  "negative float",
			input: "-123.456",
			expected: []Token{
				{Kind: TokenKindFloat, Value: "-123.456", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 8},
			},
		},
		{
			name:  "zero",
			input: "0",
			expected: []Token{
				{Kind: TokenKindInteger, Value: "0", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
		{
			name:  "negative zero",
			input: "-0",
			expected: []Token{
				{Kind: TokenKindInteger, Value: "-0", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 2},
			},
		},
		{
			name:  "float with leading digit and decimal",
			input: "1.5",
			expected: []Token{
				{Kind: TokenKindFloat, Value: "1.5", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.Tokenize(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, tokens)
		})
	}
}

func TestTokenizeIdentifiers(t *testing.T) {
	lexer := &Lexer{}
	tests := []struct {
		name     string
		input    string
		expected []Token
	}{
		{
			name:  "simple identifier",
			input: "foo",
			expected: []Token{
				{Kind: TokenKindIdentifier, Value: "foo", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 3},
			},
		},
		{
			name:  "identifier with underscore",
			input: "foo_bar",
			expected: []Token{
				{Kind: TokenKindIdentifier, Value: "foo_bar", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 7},
			},
		},
		{
			name:  "identifier with numbers",
			input: "foo123",
			expected: []Token{
				{Kind: TokenKindIdentifier, Value: "foo123", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 6},
			},
		},
		{
			name:  "dotted field path",
			input: "meta.source",
			expected: []Token{
				{Kind: TokenKindIdentifier, Value: "meta.source", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 11},
			},
		},
		{
			name:  "deep dotted path",
			input: "country.cities.population",
			expected: []Token{
				{Kind: TokenKindIdentifier, Value: "country.cities.population", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 25},
			},
		},
		{
			name:  "dotted path with array marker",
			input: "country.cities[].population",
			expected: []Token{
				{Kind: TokenKindIdentifier, Value: "country.cities[].population", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 27},
			},
		},
		{
			name:  "dotted keyword should be identifier",
			input: "meta.from",
			expected: []Token{
				{Kind: TokenKindIdentifier, Value: "meta.from", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 9},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.Tokenize(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, tokens)
		})
	}
}

func TestTokenizeOperators(t *testing.T) {
	lexer := &Lexer{}
	tests := []struct {
		name     string
		input    string
		expected []Token
	}{
		{
			name:  "equals",
			input: "=",
			expected: []Token{
				{Kind: TokenKindEquals, Value: "=", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
		{
			name:  "not equals",
			input: "!=",
			expected: []Token{
				{Kind: TokenKindNotEquals, Value: "!=", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 2},
			},
		},
		{
			name:  "greater than",
			input: ">",
			expected: []Token{
				{Kind: TokenKindGt, Value: ">", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
		{
			name:  "greater than or equal",
			input: ">=",
			expected: []Token{
				{Kind: TokenKindGte, Value: ">=", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 2},
			},
		},
		{
			name:  "less than",
			input: "<",
			expected: []Token{
				{Kind: TokenKindLt, Value: "<", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
		{
			name:  "less than or equal",
			input: "<=",
			expected: []Token{
				{Kind: TokenKindLte, Value: "<=", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.Tokenize(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, tokens)
		})
	}
}

func TestTokenizePunctuation(t *testing.T) {
	lexer := &Lexer{}
	tests := []struct {
		name     string
		input    string
		expected []Token
	}{
		{
			name:  "left brace",
			input: "{",
			expected: []Token{
				{Kind: TokenKindLbrace, Value: "{", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
		{
			name:  "right brace",
			input: "}",
			expected: []Token{
				{Kind: TokenKindRbrace, Value: "}", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
		{
			name:  "left bracket",
			input: "[",
			expected: []Token{
				{Kind: TokenKindLbracket, Value: "[", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
		{
			name:  "right bracket",
			input: "]",
			expected: []Token{
				{Kind: TokenKindRbracket, Value: "]", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
		{
			name:  "left paren",
			input: "(",
			expected: []Token{
				{Kind: TokenKindLparen, Value: "(", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
		{
			name:  "right paren",
			input: ")",
			expected: []Token{
				{Kind: TokenKindRparen, Value: ")", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
		{
			name:  "colon",
			input: ":",
			expected: []Token{
				{Kind: TokenKindColon, Value: ":", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
		{
			name:  "comma",
			input: ",",
			expected: []Token{
				{Kind: TokenKindComma, Value: ",", Pos: 0},
				{Kind: TokenKindEof, Value: "", Pos: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.Tokenize(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, tokens)
		})
	}
}

func TestTokenizeFullQuery(t *testing.T) {
	lexer := &Lexer{}
	input := `INSERT INTO COLLECTION mycol VALUES {"text": "hello", "vector": [0.1, 0.2]}`
	tokens, err := lexer.Tokenize(input)
	require.NoError(t, err)

	assert.Equal(t, TokenKindInsert, tokens[0].Kind)
	assert.Equal(t, TokenKindInto, tokens[1].Kind)
	assert.Equal(t, TokenKindCollection, tokens[2].Kind)
	assert.Equal(t, TokenKindIdentifier, tokens[3].Kind)
	assert.Equal(t, "mycol", tokens[3].Value)
	assert.Equal(t, TokenKindValues, tokens[4].Kind)
	assert.Equal(t, TokenKindLbrace, tokens[5].Kind)
	assert.Equal(t, TokenKindString, tokens[6].Kind)
	assert.Equal(t, "text", tokens[6].Value)
	assert.Equal(t, TokenKindColon, tokens[7].Kind)
	assert.Equal(t, TokenKindString, tokens[8].Kind)
	assert.Equal(t, "hello", tokens[8].Value)
	assert.Equal(t, TokenKindComma, tokens[9].Kind)
	assert.Equal(t, TokenKindString, tokens[10].Kind)
	assert.Equal(t, "vector", tokens[10].Value)
	assert.Equal(t, TokenKindColon, tokens[11].Kind)
	assert.Equal(t, TokenKindLbracket, tokens[12].Kind)
	assert.Equal(t, TokenKindFloat, tokens[13].Kind)
	assert.Equal(t, TokenKindComma, tokens[14].Kind)
	assert.Equal(t, TokenKindFloat, tokens[15].Kind)
	assert.Equal(t, TokenKindRbracket, tokens[16].Kind)
	assert.Equal(t, TokenKindRbrace, tokens[17].Kind)
	assert.Equal(t, TokenKindEof, tokens[18].Kind)
}

func TestTokenizeSearchQuery(t *testing.T) {
	lexer := &Lexer{}
	input := `SEARCH mycol SIMILAR TO 'query text' LIMIT 10`
	tokens, err := lexer.Tokenize(input)
	require.NoError(t, err)

	assert.Equal(t, TokenKindSearch, tokens[0].Kind)
	assert.Equal(t, TokenKindIdentifier, tokens[1].Kind)
	assert.Equal(t, "mycol", tokens[1].Value)
	assert.Equal(t, TokenKindSimilar, tokens[2].Kind)
	assert.Equal(t, TokenKindTo, tokens[3].Kind)
	assert.Equal(t, TokenKindString, tokens[4].Kind)
	assert.Equal(t, "query text", tokens[4].Value)
	assert.Equal(t, TokenKindLimit, tokens[5].Kind)
	assert.Equal(t, TokenKindInteger, tokens[6].Kind)
	assert.Equal(t, "10", tokens[6].Value)
	assert.Equal(t, TokenKindEof, tokens[7].Kind)
}

func TestTokenizeWhereClause(t *testing.T) {
	lexer := &Lexer{}
	input := `WHERE id = '123' AND score >= 0.5`
	tokens, err := lexer.Tokenize(input)
	require.NoError(t, err)

	assert.Equal(t, TokenKindWhere, tokens[0].Kind)
	assert.Equal(t, TokenKindId, tokens[1].Kind)
	assert.Equal(t, "id", tokens[1].Value)
	assert.Equal(t, TokenKindEquals, tokens[2].Kind)
	assert.Equal(t, TokenKindString, tokens[3].Kind)
	assert.Equal(t, "123", tokens[3].Value)
	assert.Equal(t, TokenKindAnd, tokens[4].Kind)
	assert.Equal(t, TokenKindScore, tokens[5].Kind)
	assert.Equal(t, "score", tokens[5].Value)
	assert.Equal(t, TokenKindGte, tokens[6].Kind)
	assert.Equal(t, TokenKindFloat, tokens[7].Kind)
	assert.Equal(t, "0.5", tokens[7].Value)
	assert.Equal(t, TokenKindEof, tokens[8].Kind)
}

func TestTokenizeUnexpectedCharacter(t *testing.T) {
	lexer := &Lexer{}
	input := "@"
	_, err := lexer.Tokenize(input)
	require.Error(t, err)
	assert.IsType(t, &errors.QQLSyntaxError{}, err)
}

func TestTokenizeUnexpectedBang(t *testing.T) {
	lexer := &Lexer{}
	input := "!"
	_, err := lexer.Tokenize(input)
	require.Error(t, err)
	assert.IsType(t, &errors.QQLSyntaxError{}, err)
}

func TestTokenizeWhitespace(t *testing.T) {
	lexer := &Lexer{}
	input := "  INSERT   INTO   COLLECTION  "
	tokens, err := lexer.Tokenize(input)
	require.NoError(t, err)

	assert.Equal(t, TokenKindInsert, tokens[0].Kind)
	assert.Equal(t, 2, tokens[0].Pos)
	assert.Equal(t, TokenKindInto, tokens[1].Kind)
	assert.Equal(t, TokenKindCollection, tokens[2].Kind)
	assert.Equal(t, TokenKindEof, tokens[3].Kind)
}

func TestTokenizeTabsAndNewlines(t *testing.T) {
	lexer := &Lexer{}
	input := "INSERT\tINTO\nCOLLECTION"
	tokens, err := lexer.Tokenize(input)
	require.NoError(t, err)

	assert.Equal(t, TokenKindInsert, tokens[0].Kind)
	assert.Equal(t, TokenKindInto, tokens[1].Kind)
	assert.Equal(t, TokenKindCollection, tokens[2].Kind)
	assert.Equal(t, TokenKindEof, tokens[3].Kind)
}

func TestTokenKindString(t *testing.T) {
	tests := []struct {
		kind TokenKind
		str  string
	}{
		{TokenKindInsert, "INSERT"},
		{TokenKindEof, "EOF"},
		{TokenKindIdentifier, "IDENTIFIER"},
		{TokenKindString, "STRING"},
		{TokenKindInteger, "INTEGER"},
		{TokenKindFloat, "FLOAT"},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			assert.Equal(t, tt.str, tt.kind.String())
		})
	}
}

func TestTokenString(t *testing.T) {
	token := Token{Kind: TokenKindInsert, Value: "INSERT", Pos: 0}
	assert.Equal(t, "INSERT(INSERT)", token.String())
}
