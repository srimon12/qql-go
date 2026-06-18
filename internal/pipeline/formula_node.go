package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/filters"
	"github.com/srimon12/qql-go/internal/parser"
)

type FormulaNode struct {
	Expr     ast.FormulaExpr
	Defaults map[string]any
}

func (n *FormulaNode) Execute(ctx context.Context, state *QueryState) error {
	// Build the Expression protobuf
	expr, err := BuildExpression(n.Expr)
	if err != nil {
		return err
	}

	// Build the Defaults
	var defs map[string]*qdrant.Value
	if len(n.Defaults) > 0 {
		defs = make(map[string]*qdrant.Value)
		for k, v := range n.Defaults {
			var qVal *qdrant.Value
			switch val := v.(type) {
			case float64:
				qVal = qdrant.NewValueDouble(val)
			case int:
				qVal = qdrant.NewValueInt(int64(val))
			case int64:
				qVal = qdrant.NewValueInt(val)
			case string:
				qVal = qdrant.NewValueString(val)
			case bool:
				qVal = qdrant.NewValueBool(val)
			default:
				// Fallback to json string if it's a map or something we don't handle natively
				if b, err := json.Marshal(val); err == nil {
					qVal = qdrant.NewValueString(string(b))
				} else {
					qVal = qdrant.NewValueString(fmt.Sprintf("%v", val))
				}
			}
			defs[k] = qVal
		}
	}

	formula := &qdrant.Formula{
		Expression: expr,
		Defaults:   defs,
	}

	if state.TargetQuery != nil {
		pq := &qdrant.PrefetchQuery{
			Query: state.TargetQuery,
		}
		if state.VectorName != "" {
			pq.Using = qdrant.PtrOf(state.VectorName)
		}
		state.Prefetches = append(state.Prefetches, pq)
	}

	state.TargetQuery = qdrant.NewQueryFormula(formula)
	return nil
}

func BuildExpression(expr ast.FormulaExpr) (*qdrant.Expression, error) {
	switch e := expr.(type) {
	case ast.FormulaDatetime:
		return qdrant.NewExpressionDatetime(e.Value), nil
	case ast.FormulaDatetimeKey:
		return qdrant.NewExpressionDatetimeKey(e.Key), nil
	case ast.FormulaConstant:
		return qdrant.NewExpressionConstant(float32(e.Value)), nil
	case ast.FormulaVariable:
		return qdrant.NewExpressionVariable(e.Name), nil
	case ast.FormulaSum:
		left, err := BuildExpression(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := BuildExpression(e.Right)
		if err != nil {
			return nil, err
		}
		return qdrant.NewExpressionSum(&qdrant.SumExpression{Sum: []*qdrant.Expression{left, right}}), nil
	case ast.FormulaSub:
		left, err := BuildExpression(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := BuildExpression(e.Right)
		if err != nil {
			return nil, err
		}
		return qdrant.NewExpressionSum(&qdrant.SumExpression{Sum: []*qdrant.Expression{left, qdrant.NewExpressionNeg(right)}}), nil
	case ast.FormulaMul:
		left, err := BuildExpression(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := BuildExpression(e.Right)
		if err != nil {
			return nil, err
		}
		return qdrant.NewExpressionMult(&qdrant.MultExpression{Mult: []*qdrant.Expression{left, right}}), nil
	case ast.FormulaDiv:
		left, err := BuildExpression(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := BuildExpression(e.Right)
		if err != nil {
			return nil, err
		}
		div := &qdrant.DivExpression{
			Left:  left,
			Right: right,
		}
		if e.ByZeroDefault != nil {
			f32 := float32(*e.ByZeroDefault)
			div.ByZeroDefault = &f32
		}
		return qdrant.NewExpressionDiv(div), nil
	case ast.FormulaNeg:
		op, err := BuildExpression(e.Operand)
		if err != nil {
			return nil, err
		}
		return qdrant.NewExpressionNeg(op), nil
	case ast.FormulaAbs:
		x, err := BuildExpression(e.X)
		if err != nil {
			return nil, err
		}
		return qdrant.NewExpressionAbs(x), nil
	case ast.FormulaSqrt:
		x, err := BuildExpression(e.X)
		if err != nil {
			return nil, err
		}
		return qdrant.NewExpressionSqrt(x), nil
	case ast.FormulaLog:
		x, err := BuildExpression(e.X)
		if err != nil {
			return nil, err
		}
		return qdrant.NewExpressionLog10(x), nil
	case ast.FormulaLn:
		x, err := BuildExpression(e.X)
		if err != nil {
			return nil, err
		}
		return qdrant.NewExpressionLn(x), nil
	case ast.FormulaExp:
		x, err := BuildExpression(e.X)
		if err != nil {
			return nil, err
		}
		return qdrant.NewExpressionExp(x), nil
	case ast.FormulaPow:
		base, err := BuildExpression(e.Base)
		if err != nil {
			return nil, err
		}
		exp, err := BuildExpression(e.Exponent)
		if err != nil {
			return nil, err
		}
		return qdrant.NewExpressionPow(&qdrant.PowExpression{Base: base, Exponent: exp}), nil
	case ast.FormulaGeoDistance:
		return qdrant.NewExpressionGeoDistance(&qdrant.GeoDistance{
			Origin: &qdrant.GeoPoint{Lat: e.Lat, Lon: e.Lon},
			To:     e.Field,
		}), nil
	case ast.FormulaDecay:
		x, err := BuildExpression(e.X)
		if err != nil {
			return nil, err
		}
		var target *qdrant.Expression
		if e.Target != nil {
			target, err = BuildExpression(*e.Target)
			if err != nil {
				return nil, err
			}
		}
		var scale, mid *float32
		if e.Scale != nil {
			s := float32(*e.Scale)
			scale = &s
		}
		if e.Midpoint != nil {
			m := float32(*e.Midpoint)
			mid = &m
		}
		p := &qdrant.DecayParamsExpression{
			X:        x,
			Target:   target,
			Scale:    scale,
			Midpoint: mid,
		}
		switch e.Kind {
		case "exp_decay":
			return qdrant.NewExpressionExpDecay(p), nil
		case "gauss_decay":
			return qdrant.NewExpressionGaussDecay(p), nil
		case "lin_decay":
			return qdrant.NewExpressionLinDecay(p), nil
		default:
			return nil, fmt.Errorf("unknown decay kind: %s", e.Kind)
		}
	case ast.FormulaCase:
		qFilter, err := filters.NewFilterConverter().BuildFilter(e.Cond)
		if err != nil {
			return nil, err
		}

		condExpr := qdrant.NewExpressionCondition(qdrant.NewFilterAsCondition(qFilter))

		notCondFilter := &qdrant.Filter{
			MustNot: []*qdrant.Condition{
				qdrant.NewFilterAsCondition(qFilter),
			},
		}
		notCondExpr := qdrant.NewExpressionCondition(qdrant.NewFilterAsCondition(notCondFilter))

		thenExpr, err := BuildExpression(e.Then_)
		if err != nil {
			return nil, err
		}
		elseExpr, err := BuildExpression(e.Else_)
		if err != nil {
			return nil, err
		}

		thenPart := qdrant.NewExpressionMult(&qdrant.MultExpression{
			Mult: []*qdrant.Expression{condExpr, thenExpr},
		})

		elsePart := qdrant.NewExpressionMult(&qdrant.MultExpression{
			Mult: []*qdrant.Expression{notCondExpr, elseExpr},
		})

		return qdrant.NewExpressionSum(&qdrant.SumExpression{
			Sum: []*qdrant.Expression{thenPart, elsePart},
		}), nil
	case ast.FormulaMatchCondition:
		return buildMatchConditionExpression(e.Field, e.Values)
	case ast.RawFormulaExpr:
		parsed, err := parser.ParseFormulaString(e.Expr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse formula expression: %w", err)
		}
		return BuildExpression(parsed)
	default:
		return nil, fmt.Errorf("unknown formula expression type %T", expr)
	}
}

func buildMatchConditionExpression(field string, values []any) (*qdrant.Expression, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("MATCH requires at least one value")
	}
	var condition *qdrant.Condition
	if len(values) == 1 {
		switch v := values[0].(type) {
		case string:
			condition = qdrant.NewMatchKeyword(field, v)
		case int:
			condition = qdrant.NewMatchInt(field, int64(v))
		case int64:
			condition = qdrant.NewMatchInt(field, v)
		case uint64:
			condition = qdrant.NewMatchInt(field, int64(v))
		case float64:
			condition = qdrant.NewMatchInt(field, int64(v))
		default:
			return nil, fmt.Errorf("MATCH value must be a string or number, got %T", v)
		}
	} else {
		first := values[0]
		switch first.(type) {
		case string:
			keywords := make([]string, len(values))
			for i, v := range values {
				s, ok := v.(string)
				if !ok {
					return nil, fmt.Errorf("MATCH: all values must be strings when first is a string")
				}
				keywords[i] = s
			}
			condition = qdrant.NewMatchKeywords(field, keywords...)
		case int, int64, uint64, float64:
			vals := make([]int64, len(values))
			for i, v := range values {
				switch n := v.(type) {
				case int:
					vals[i] = int64(n)
				case int64:
					vals[i] = n
				case uint64:
					vals[i] = int64(n)
				case float64:
					vals[i] = int64(n)
				default:
					return nil, fmt.Errorf("MATCH: all values must be numbers when first is a number")
				}
			}
			condition = qdrant.NewMatchInts(field, vals...)
		default:
			return nil, fmt.Errorf("MATCH values must be strings or numbers, got %T", first)
		}
	}
	return qdrant.NewExpressionCondition(condition), nil
}
