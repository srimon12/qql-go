package ast

import (
	"fmt"
	"strings"
)

// FormulaExpr represents any node in the formula AST.
type FormulaExpr interface {
	isFormulaExpr()
}

type FormulaDatetime struct {
	Value string
}

func (FormulaDatetime) isFormulaExpr() {}

type FormulaDatetimeKey struct {
	Key string
}

func (FormulaDatetimeKey) isFormulaExpr() {}

type FormulaConstant struct {
	Value float64
}

func (FormulaConstant) isFormulaExpr() {}

type FormulaVariable struct {
	Name string
}

func (FormulaVariable) isFormulaExpr() {}

type FormulaSum struct {
	Left  FormulaExpr
	Right FormulaExpr
}

func (FormulaSum) isFormulaExpr() {}

type FormulaSub struct {
	Left  FormulaExpr
	Right FormulaExpr
}

func (FormulaSub) isFormulaExpr() {}

type FormulaMul struct {
	Left  FormulaExpr
	Right FormulaExpr
}

func (FormulaMul) isFormulaExpr() {}

type FormulaDiv struct {
	Left          FormulaExpr
	Right         FormulaExpr
	ByZeroDefault *float64
}

func (FormulaDiv) isFormulaExpr() {}

type FormulaNeg struct {
	Operand FormulaExpr
}

func (FormulaNeg) isFormulaExpr() {}

type FormulaAbs struct {
	X FormulaExpr
}

func (FormulaAbs) isFormulaExpr() {}

type FormulaSqrt struct {
	X FormulaExpr
}

func (FormulaSqrt) isFormulaExpr() {}

type FormulaLog struct {
	X FormulaExpr
}

func (FormulaLog) isFormulaExpr() {}

type FormulaLn struct {
	X FormulaExpr
}

func (FormulaLn) isFormulaExpr() {}

type FormulaExp struct {
	X FormulaExpr
}

func (FormulaExp) isFormulaExpr() {}

type FormulaPow struct {
	Base     FormulaExpr
	Exponent FormulaExpr
}

func (FormulaPow) isFormulaExpr() {}

type FormulaGeoDistance struct {
	Lat   float64
	Lon   float64
	Field string
}

func (FormulaGeoDistance) isFormulaExpr() {}

type FormulaDecay struct {
	Kind     string // exp_decay, gauss_decay, lin_decay
	X        FormulaExpr
	Target   *FormulaExpr
	Scale    *float64
	Midpoint *float64
}

func (FormulaDecay) isFormulaExpr() {}

type FormulaCase struct {
	Cond  FilterExpr
	Then_ FormulaExpr
	Else_ FormulaExpr
}

func (FormulaCase) isFormulaExpr() {}

// FormulaExprString renders a FormulaExpr as a human-readable string.
func FormulaExprString(expr FormulaExpr) string {
	if expr == nil {
		return "<nil>"
	}
	switch e := expr.(type) {
	case FormulaDatetime:
		return fmt.Sprintf("DATETIME('%s')", e.Value)
	case FormulaDatetimeKey:
		return fmt.Sprintf("DATETIME_KEY('%s')", e.Key)
	case FormulaConstant:
		if e.Value == float64(int(e.Value)) {
			return fmt.Sprintf("%d", int(e.Value))
		}
		return fmt.Sprintf("%g", e.Value)
	case FormulaVariable:
		return e.Name
	case FormulaSum:
		return fmt.Sprintf("(%s + %s)", FormulaExprString(e.Left), FormulaExprString(e.Right))
	case FormulaSub:
		return fmt.Sprintf("(%s - %s)", FormulaExprString(e.Left), FormulaExprString(e.Right))
	case FormulaMul:
		return fmt.Sprintf("(%s * %s)", FormulaExprString(e.Left), FormulaExprString(e.Right))
	case FormulaDiv:
		s := fmt.Sprintf("(%s / %s)", FormulaExprString(e.Left), FormulaExprString(e.Right))
		if e.ByZeroDefault != nil {
			s += fmt.Sprintf(" [default=%g]", *e.ByZeroDefault)
		}
		return s
	case FormulaNeg:
		return fmt.Sprintf("(-%s)", FormulaExprString(e.Operand))
	case FormulaAbs:
		return fmt.Sprintf("ABS(%s)", FormulaExprString(e.X))
	case FormulaSqrt:
		return fmt.Sprintf("SQRT(%s)", FormulaExprString(e.X))
	case FormulaLog:
		return fmt.Sprintf("LOG(%s)", FormulaExprString(e.X))
	case FormulaLn:
		return fmt.Sprintf("LN(%s)", FormulaExprString(e.X))
	case FormulaExp:
		return fmt.Sprintf("EXP(%s)", FormulaExprString(e.X))
	case FormulaPow:
		return fmt.Sprintf("POW(%s, %s)", FormulaExprString(e.Base), FormulaExprString(e.Exponent))
	case FormulaGeoDistance:
		return fmt.Sprintf("GEO_DISTANCE(%g, %g, %s)", e.Lat, e.Lon, e.Field)
	case FormulaDecay:
		parts := []string{FormulaExprString(e.X)}
		if e.Target != nil {
			parts = append(parts, FormulaExprString(*e.Target))
		}
		if e.Scale != nil {
			parts = append(parts, fmt.Sprintf("%g", *e.Scale))
		}
		if e.Midpoint != nil {
			parts = append(parts, fmt.Sprintf("%g", *e.Midpoint))
		}
		return fmt.Sprintf("%s(%s)", strings.ToUpper(e.Kind), strings.Join(parts, ", "))
	case FormulaCase:
		return fmt.Sprintf("CASE WHEN %s THEN %s ELSE %s END", "<filter>", FormulaExprString(e.Then_), FormulaExprString(e.Else_))
	default:
		return fmt.Sprintf("<unknown:%T>", expr)
	}
}
