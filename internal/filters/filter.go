package filters

import (
	"reflect"

	"github.com/qdrant/go-client/qdrant"
	"github.com/qdrant/qql-go/internal/ast"
	"github.com/qdrant/qql-go/internal/errors"
)

type FilterConverter struct{}

func NewFilterConverter() *FilterConverter {
	return &FilterConverter{}
}

func (fc *FilterConverter) BuildFilter(expr ast.FilterExpr) (*qdrant.Filter, error) {
	if expr == nil {
		return nil, nil
	}

	filter, err := fc.buildCondition(expr)
	if err != nil {
		return nil, err
	}

	return fc.wrapAsFilter(filter), nil
}

func (fc *FilterConverter) buildCondition(expr ast.FilterExpr) (*qdrant.Condition, error) {
	expr, err := normalizeFilterExpr(expr)
	if err != nil {
		return nil, err
	}

	switch e := expr.(type) {
	case ast.CompareExpr:
		return fc.buildCompareExpr(e)
	case ast.BetweenExpr:
		return fc.buildBetweenExpr(e)
	case ast.InExpr:
		return fc.buildInExpr(e)
	case ast.NotInExpr:
		return fc.buildNotInExpr(e)
	case ast.IsNullExpr:
		return fc.buildIsNullExpr(e)
	case ast.IsNotNullExpr:
		return fc.buildIsNotNullExpr(e)
	case ast.IsEmptyExpr:
		return fc.buildIsEmptyExpr(e)
	case ast.IsNotEmptyExpr:
		return fc.buildIsNotEmptyExpr(e)
	case ast.MatchTextExpr:
		return fc.buildMatchTextExpr(e)
	case ast.MatchAnyExpr:
		return fc.buildMatchAnyExpr(e)
	case ast.MatchPhraseExpr:
		return fc.buildMatchPhraseExpr(e)
	case ast.AndExpr:
		return fc.buildAndExpr(e)
	case ast.OrExpr:
		return fc.buildOrExpr(e)
	case ast.NotExpr:
		return fc.buildNotExpr(e)
	default:
		return nil, errors.NewQQLRuntimeError("unknown filter expression type")
	}
}

func (fc *FilterConverter) buildCompareExpr(expr ast.CompareExpr) (*qdrant.Condition, error) {
	switch expr.Op {
	case "=":
		return fc.buildEqualCondition(expr.Field, expr.Value)
	case "!=":
		return fc.buildNotEqualCondition(expr.Field, expr.Value)
	case ">":
		return qdrant.NewRange(expr.Field, &qdrant.Range{
			Gt: toFloat64(expr.Value),
		}), nil
	case ">=":
		return qdrant.NewRange(expr.Field, &qdrant.Range{
			Gte: toFloat64(expr.Value),
		}), nil
	case "<":
		return qdrant.NewRange(expr.Field, &qdrant.Range{
			Lt: toFloat64(expr.Value),
		}), nil
	case "<=":
		return qdrant.NewRange(expr.Field, &qdrant.Range{
			Lte: toFloat64(expr.Value),
		}), nil
	default:
		return nil, errors.NewQQLRuntimeError("unknown comparison operator: " + expr.Op)
	}
}

func normalizeFilterExpr(expr ast.FilterExpr) (ast.FilterExpr, error) {
	if expr == nil {
		return nil, errors.NewQQLRuntimeError("unknown filter expression type")
	}

	value := reflect.ValueOf(expr)
	if value.Kind() != reflect.Ptr {
		return expr, nil
	}
	if value.IsNil() {
		return nil, errors.NewQQLRuntimeError("unknown filter expression type")
	}

	normalized, ok := value.Elem().Interface().(ast.FilterExpr)
	if !ok {
		return nil, errors.NewQQLRuntimeError("unknown filter expression type")
	}
	return normalized, nil
}

func (fc *FilterConverter) buildBetweenExpr(expr ast.BetweenExpr) (*qdrant.Condition, error) {
	return qdrant.NewRange(expr.Field, &qdrant.Range{
		Gte: toFloat64(expr.Low),
		Lte: toFloat64(expr.High),
	}), nil
}

func (fc *FilterConverter) buildInExpr(expr ast.InExpr) (*qdrant.Condition, error) {
	return fc.buildSetCondition(expr.Field, expr.Values, false)
}

func (fc *FilterConverter) buildNotInExpr(expr ast.NotInExpr) (*qdrant.Condition, error) {
	return fc.buildSetCondition(expr.Field, expr.Values, true)
}

func (fc *FilterConverter) buildIsNullExpr(expr ast.IsNullExpr) (*qdrant.Condition, error) {
	return qdrant.NewIsNull(expr.Field), nil
}

func (fc *FilterConverter) buildIsNotNullExpr(expr ast.IsNotNullExpr) (*qdrant.Condition, error) {
	return qdrant.NewFilterAsCondition(&qdrant.Filter{
		MustNot: []*qdrant.Condition{
			qdrant.NewIsNull(expr.Field),
		},
	}), nil
}

func (fc *FilterConverter) buildIsEmptyExpr(expr ast.IsEmptyExpr) (*qdrant.Condition, error) {
	return qdrant.NewIsEmpty(expr.Field), nil
}

func (fc *FilterConverter) buildIsNotEmptyExpr(expr ast.IsNotEmptyExpr) (*qdrant.Condition, error) {
	return qdrant.NewFilterAsCondition(&qdrant.Filter{
		MustNot: []*qdrant.Condition{
			qdrant.NewIsEmpty(expr.Field),
		},
	}), nil
}

func (fc *FilterConverter) buildMatchTextExpr(expr ast.MatchTextExpr) (*qdrant.Condition, error) {
	return qdrant.NewMatchText(expr.Field, expr.Text), nil
}

func (fc *FilterConverter) buildMatchAnyExpr(expr ast.MatchAnyExpr) (*qdrant.Condition, error) {
	return qdrant.NewMatchTextAny(expr.Field, expr.Text), nil
}

func (fc *FilterConverter) buildMatchPhraseExpr(expr ast.MatchPhraseExpr) (*qdrant.Condition, error) {
	return qdrant.NewMatchPhrase(expr.Field, expr.Text), nil
}

func (fc *FilterConverter) buildAndExpr(expr ast.AndExpr) (*qdrant.Condition, error) {
	must := make([]*qdrant.Condition, len(expr.Operands))
	for i, operand := range expr.Operands {
		cond, err := fc.buildCondition(operand)
		if err != nil {
			return nil, err
		}
		must[i] = cond
	}

	return qdrant.NewFilterAsCondition(&qdrant.Filter{
		Must: must,
	}), nil
}

func (fc *FilterConverter) buildOrExpr(expr ast.OrExpr) (*qdrant.Condition, error) {
	should := make([]*qdrant.Condition, len(expr.Operands))
	for i, operand := range expr.Operands {
		cond, err := fc.buildCondition(operand)
		if err != nil {
			return nil, err
		}
		should[i] = cond
	}

	return qdrant.NewFilterAsCondition(&qdrant.Filter{
		Should:  should,
		MustNot: []*qdrant.Condition{},
	}), nil
}

func (fc *FilterConverter) buildNotExpr(expr ast.NotExpr) (*qdrant.Condition, error) {
	cond, err := fc.buildCondition(expr.Operand)
	if err != nil {
		return nil, err
	}

	return qdrant.NewFilterAsCondition(&qdrant.Filter{
		MustNot: []*qdrant.Condition{cond},
	}), nil
}

func (fc *FilterConverter) wrapAsFilter(condition *qdrant.Condition) *qdrant.Filter {
	if condition == nil {
		return nil
	}

	if filterCond := condition.GetFilter(); filterCond != nil {
		return filterCond
	}

	return &qdrant.Filter{
		Must: []*qdrant.Condition{condition},
	}
}

func (fc *FilterConverter) buildEqualCondition(field string, value interface{}) (*qdrant.Condition, error) {
	switch v := value.(type) {
	case string:
		return qdrant.NewMatch(field, v), nil
	case int:
		return qdrant.NewMatchInt(field, int64(v)), nil
	case int64:
		return qdrant.NewMatchInt(field, v), nil
	case bool:
		return qdrant.NewMatchBool(field, v), nil
	case float32:
		f := float64(v)
		return exactFloatCondition(field, f), nil
	case float64:
		return exactFloatCondition(field, v), nil
	default:
		return nil, errors.NewQQLRuntimeError("unsupported value type for equality match")
	}
}

func (fc *FilterConverter) buildNotEqualCondition(field string, value interface{}) (*qdrant.Condition, error) {
	switch v := value.(type) {
	case string:
		return qdrant.NewMatchExcept(field, v), nil
	case int:
		return qdrant.NewMatchExceptInts(field, int64(v)), nil
	case int64:
		return qdrant.NewMatchExceptInts(field, v), nil
	case bool:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{
			MustNot: []*qdrant.Condition{qdrant.NewMatchBool(field, v)},
		}), nil
	case float32:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{
			MustNot: []*qdrant.Condition{exactFloatCondition(field, float64(v))},
		}), nil
	case float64:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{
			MustNot: []*qdrant.Condition{exactFloatCondition(field, v)},
		}), nil
	default:
		return nil, errors.NewQQLRuntimeError("unsupported value type for inequality match")
	}
}

func (fc *FilterConverter) buildSetCondition(field string, values []interface{}, negate bool) (*qdrant.Condition, error) {
	if len(values) == 0 {
		if negate {
			return qdrant.NewMatchExceptKeywords(field), nil
		}
		return qdrant.NewMatchKeywords(field), nil
	}

	kind, err := literalKindOf(values[0])
	if err != nil {
		return nil, err
	}

	for _, value := range values[1:] {
		nextKind, err := literalKindOf(value)
		if err != nil {
			return nil, err
		}
		if nextKind != kind {
			return nil, errors.NewQQLRuntimeError("mixed literal types are not supported in IN/NOT IN")
		}
	}

	switch kind {
	case literalKindString:
		strValues := make([]string, len(values))
		for i, value := range values {
			strValues[i] = value.(string)
		}
		if negate {
			return qdrant.NewMatchExceptKeywords(field, strValues...), nil
		}
		return qdrant.NewMatchKeywords(field, strValues...), nil
	case literalKindInt:
		intValues := make([]int64, len(values))
		for i, value := range values {
			intValues[i] = toInt64(value)
		}
		if negate {
			return qdrant.NewMatchExceptInts(field, intValues...), nil
		}
		return qdrant.NewMatchInts(field, intValues...), nil
	case literalKindBool:
		conds := make([]*qdrant.Condition, len(values))
		for i, value := range values {
			conds[i] = qdrant.NewMatchBool(field, value.(bool))
		}
		if negate {
			return qdrant.NewFilterAsCondition(&qdrant.Filter{MustNot: conds}), nil
		}
		return combineConditions(conds), nil
	case literalKindFloat:
		conds := make([]*qdrant.Condition, len(values))
		for i, value := range values {
			conds[i] = exactFloatCondition(field, toFloat64Value(value))
		}
		if negate {
			return qdrant.NewFilterAsCondition(&qdrant.Filter{MustNot: conds}), nil
		}
		return combineConditions(conds), nil
	default:
		return nil, errors.NewQQLRuntimeError("unsupported literal type in IN/NOT IN")
	}
}

func exactFloatCondition(field string, value float64) *qdrant.Condition {
	gte := value
	lte := value
	return qdrant.NewRange(field, &qdrant.Range{
		Gte: &gte,
		Lte: &lte,
	})
}

func combineConditions(conds []*qdrant.Condition) *qdrant.Condition {
	if len(conds) == 1 {
		return conds[0]
	}
	return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: conds})
}

type literalKind int

const (
	literalKindUnknown literalKind = iota
	literalKindString
	literalKindInt
	literalKindFloat
	literalKindBool
)

func literalKindOf(v interface{}) (literalKind, error) {
	switch v.(type) {
	case string:
		return literalKindString, nil
	case int, int64:
		return literalKindInt, nil
	case float32, float64:
		return literalKindFloat, nil
	case bool:
		return literalKindBool, nil
	default:
		return literalKindUnknown, errors.NewQQLRuntimeError("unsupported literal type")
	}
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	}
	return 0
}

func toFloat64Value(v interface{}) float64 {
	switch val := v.(type) {
	case float32:
		return float64(val)
	case float64:
		return val
	}
	return 0
}

func toFloat64(v interface{}) *float64 {
	switch val := v.(type) {
	case float64:
		return &val
	case float32:
		f := float64(val)
		return &f
	case int:
		f := float64(val)
		return &f
	case int64:
		f := float64(val)
		return &f
	default:
		return nil
	}
}
