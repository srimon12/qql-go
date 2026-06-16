package lexer

type TokenKind int

const (
	TokenKindInsert TokenKind = iota
	TokenKindInto
	TokenKindCollection
	TokenKindValues
	TokenKindUsing
	TokenKindModel
	TokenKindHybrid
	TokenKindDense
	TokenKindSparse
	TokenKindRerank
	TokenKindExact
	TokenKindWith
	TokenKindAs
	TokenKindAcorn
	TokenKindQuantize
	TokenKindScalar
	TokenKindBinary
	TokenKindProduct
	TokenKindTurbo
	TokenKindBits
	TokenKindQuantile
	TokenKindAlways
	TokenKindRam
	TokenKindHnsw
	TokenKindVectors
	TokenKindOptimizers
	TokenKindParams
	TokenKindDisabled
	TokenKindCreate
	TokenKindAlter
	TokenKindDrop
	TokenKindShow
	TokenKindCollections
	TokenKindSelect
	TokenKindScroll
	TokenKindStar
	TokenKindAfter
	TokenKindRecommend
	TokenKindLimit
	TokenKindGroup
	TokenKindBy
	TokenKindGroupSize
	TokenKindStrategy
	TokenKindDelete
	TokenKindUpdate
	TokenKindSet
	TokenKindVector
	TokenKindPayload
	TokenKindFrom
	TokenKindWhere
	TokenKindId
	TokenKindIndex
	TokenKindOn
	TokenKindFor
	TokenKindType
	TokenKindAnd
	TokenKindOr
	TokenKindNot
	TokenKindIn
	TokenKindBetween
	TokenKindIs
	TokenKindNull
	TokenKindEmpty
	TokenKindMatch
	TokenKindAny
	TokenKindPhrase
	TokenKindOffset
	TokenKindScore
	TokenKindThreshold
	TokenKindLookup
	TokenKindCosine
	TokenKindDot
	TokenKindEuclid
	TokenKindManhattan
	TokenKindOrder
	TokenKindAsc
	TokenKindDesc
	TokenKindQuery
	TokenKindNearest
	TokenKindContext
	TokenKindDiscover
	TokenKindPairs
	TokenKindTarget
	TokenKindIdentifier
	TokenKindString
	TokenKindInteger
	TokenKindFloat
	TokenKindLbrace
	TokenKindRbrace
	TokenKindLbracket
	TokenKindRbracket
	TokenKindLparen
	TokenKindRparen
	TokenKindColon
	TokenKindComma
	TokenKindEquals
	TokenKindNotEquals
	TokenKindGt
	TokenKindGte
	TokenKindLt
	TokenKindLte
	TokenKindPrefetch
	TokenKindFusion
	TokenKindSample
	TokenKindBoost
	TokenKindDefaults
	TokenKindCase
	TokenKindWhen
	TokenKindThen
	TokenKindElse
	TokenKindEnd
	TokenKindPlus
	TokenKindMinus
	TokenKindSlash
	TokenKindEof
)

var tokenKindStrings = map[TokenKind]string{
	TokenKindInsert:      "INSERT",
	TokenKindInto:        "INTO",
	TokenKindCollection:  "COLLECTION",
	TokenKindValues:      "VALUES",
	TokenKindUsing:       "USING",
	TokenKindModel:       "MODEL",
	TokenKindHybrid:      "HYBRID",
	TokenKindDense:       "DENSE",
	TokenKindSparse:      "SPARSE",
	TokenKindRerank:      "RERANK",
	TokenKindExact:       "EXACT",
	TokenKindWith:        "WITH",
	TokenKindAs:          "AS",
	TokenKindAcorn:       "ACORN",
	TokenKindQuantize:    "QUANTIZE",
	TokenKindScalar:      "SCALAR",
	TokenKindBinary:      "BINARY",
	TokenKindProduct:     "PRODUCT",
	TokenKindTurbo:       "TURBO",
	TokenKindBits:        "BITS",
	TokenKindQuantile:    "QUANTILE",
	TokenKindAlways:      "ALWAYS",
	TokenKindRam:         "RAM",
	TokenKindHnsw:        "HNSW",
	TokenKindVectors:     "VECTORS",
	TokenKindOptimizers:  "OPTIMIZERS",
	TokenKindParams:      "PARAMS",
	TokenKindDisabled:    "DISABLED",
	TokenKindCreate:      "CREATE",
	TokenKindAlter:       "ALTER",
	TokenKindDrop:        "DROP",
	TokenKindShow:        "SHOW",
	TokenKindCollections: "COLLECTIONS",
	TokenKindSelect:      "SELECT",
	TokenKindScroll:      "SCROLL",
	TokenKindStar:        "STAR",
	TokenKindAfter:       "AFTER",
	TokenKindRecommend:   "RECOMMEND",
	TokenKindLimit:       "LIMIT",
	TokenKindGroup:       "GROUP",
	TokenKindBy:          "BY",
	TokenKindGroupSize:   "GROUP_SIZE",
	TokenKindStrategy:    "STRATEGY",
	TokenKindDelete:      "DELETE",
	TokenKindUpdate:      "UPDATE",
	TokenKindSet:         "SET",
	TokenKindVector:      "VECTOR",
	TokenKindPayload:     "PAYLOAD",
	TokenKindFrom:        "FROM",
	TokenKindWhere:       "WHERE",
	TokenKindId:          "ID",
	TokenKindIndex:       "INDEX",
	TokenKindOn:          "ON",
	TokenKindFor:         "FOR",
	TokenKindType:        "TYPE",
	TokenKindAnd:         "AND",
	TokenKindOr:          "OR",
	TokenKindNot:         "NOT",
	TokenKindIn:          "IN",
	TokenKindBetween:     "BETWEEN",
	TokenKindIs:          "IS",
	TokenKindNull:        "NULL",
	TokenKindEmpty:       "EMPTY",
	TokenKindMatch:       "MATCH",
	TokenKindAny:         "ANY",
	TokenKindPhrase:      "PHRASE",
	TokenKindOffset:      "OFFSET",
	TokenKindScore:       "SCORE",
	TokenKindThreshold:   "THRESHOLD",
	TokenKindLookup:      "LOOKUP",
	TokenKindCosine:      "COSINE",
	TokenKindDot:         "DOT",
	TokenKindEuclid:      "EUCLID",
	TokenKindManhattan:   "MANHATTAN",
	TokenKindOrder:       "ORDER",
	TokenKindAsc:         "ASC",
	TokenKindDesc:        "DESC",
	TokenKindQuery:       "QUERY",
	TokenKindNearest:     "NEAREST",
	TokenKindContext:     "CONTEXT",
	TokenKindDiscover:    "DISCOVER",
	TokenKindPairs:       "PAIRS",
	TokenKindTarget:      "TARGET",
	TokenKindIdentifier:  "IDENTIFIER",
	TokenKindString:      "STRING",
	TokenKindInteger:     "INTEGER",
	TokenKindFloat:       "FLOAT",
	TokenKindLbrace:      "LBRACE",
	TokenKindRbrace:      "RBRACE",
	TokenKindLbracket:    "LBRACKET",
	TokenKindRbracket:    "RBRACKET",
	TokenKindLparen:      "LPAREN",
	TokenKindRparen:      "RPAREN",
	TokenKindColon:       "COLON",
	TokenKindComma:       "COMMA",
	TokenKindEquals:      "EQUALS",
	TokenKindNotEquals:   "NOT_EQUALS",
	TokenKindGt:          "GT",
	TokenKindGte:         "GTE",
	TokenKindLt:          "LT",
	TokenKindLte:         "LTE",
	TokenKindPrefetch:    "PREFETCH",
	TokenKindFusion:      "FUSION",
	TokenKindSample:      "SAMPLE",
	TokenKindBoost:       "BOOST",
	TokenKindDefaults:    "DEFAULTS",
	TokenKindCase:        "CASE",
	TokenKindWhen:        "WHEN",
	TokenKindThen:        "THEN",
	TokenKindElse:        "ELSE",
	TokenKindEnd:         "END",
	TokenKindPlus:        "PLUS",
	TokenKindMinus:       "MINUS",
	TokenKindSlash:       "SLASH",
	TokenKindEof:         "EOF",
}

func (k TokenKind) String() string {
	if s, ok := tokenKindStrings[k]; ok {
		return s
	}
	return "UNKNOWN"
}
