package ast

type SearchWith struct {
	HnswEf        int
	Exact         bool
	Acorn         bool
	IndexedOnly   bool
	Quantization  *QuantizationSearchWith
	MmrDiversity  *float64
	MmrCandidates *int
	RrfK          *int
	RrfWeights    []float32
}

type FilterExpr interface {
	isFilterExpr()
}

type CompareExpr struct {
	Field string
	Op    string
	Value any
}

func (CompareExpr) isFilterExpr() {}

type BetweenExpr struct {
	Field string
	Low   any
	High  any
}

func (BetweenExpr) isFilterExpr() {}

type InExpr struct {
	Field  string
	Values []any
}

func (InExpr) isFilterExpr() {}

type NotInExpr struct {
	Field  string
	Values []any
}

func (NotInExpr) isFilterExpr() {}

type IsNullExpr struct {
	Field string
}

func (IsNullExpr) isFilterExpr() {}

type IsNotNullExpr struct {
	Field string
}

func (IsNotNullExpr) isFilterExpr() {}

type IsEmptyExpr struct {
	Field string
}

func (IsEmptyExpr) isFilterExpr() {}

type IsNotEmptyExpr struct {
	Field string
}

func (IsNotEmptyExpr) isFilterExpr() {}

type MatchTextExpr struct {
	Field string
	Text  string
}

func (MatchTextExpr) isFilterExpr() {}

type MatchAnyExpr struct {
	Field string
	Text  string
}

func (MatchAnyExpr) isFilterExpr() {}

type MatchPhraseExpr struct {
	Field string
	Text  string
}

func (MatchPhraseExpr) isFilterExpr() {}

type AndExpr struct {
	Operands []FilterExpr
}

func (AndExpr) isFilterExpr() {}

type OrExpr struct {
	Operands []FilterExpr
}

func (OrExpr) isFilterExpr() {}

type NotExpr struct {
	Operand FilterExpr
}

func (NotExpr) isFilterExpr() {}
