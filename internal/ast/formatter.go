package ast

import (
	"fmt"
	"strings"
)

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "\\'"))
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case int, int32, int64, uint, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []any:
		var items []string
		for _, item := range val {
			items = append(items, formatValue(item))
		}
		return "(" + strings.Join(items, ", ") + ")"
	case map[string]interface{}:
		// Format as {lat: 48.1, lon: 11.5}
		if lat, ok := val["lat"]; ok {
			if lon, ok := val["lon"]; ok {
				return fmt.Sprintf("{lat: %v, lon: %v}", lat, lon)
			}
		}
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func FormatFilterExpr(expr FilterExpr) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case CompareExpr:
		return fmt.Sprintf("%s %s %s", e.Field, e.Op, formatValue(e.Value))
	case BetweenExpr:
		return fmt.Sprintf("%s BETWEEN %s AND %s", e.Field, formatValue(e.Low), formatValue(e.High))
	case InExpr:
		return fmt.Sprintf("%s IN %s", e.Field, formatValue(e.Values))
	case NotInExpr:
		return fmt.Sprintf("%s NOT IN %s", e.Field, formatValue(e.Values))
	case IsNullExpr:
		return fmt.Sprintf("%s IS NULL", e.Field)
	case IsNotNullExpr:
		return fmt.Sprintf("%s IS NOT NULL", e.Field)
	case IsEmptyExpr:
		return fmt.Sprintf("%s IS EMPTY", e.Field)
	case IsNotEmptyExpr:
		return fmt.Sprintf("%s IS NOT EMPTY", e.Field)
	case MatchTextExpr:
		return fmt.Sprintf("%s MATCH %s", e.Field, formatValue(e.Text))
	case MatchAnyExpr:
		return fmt.Sprintf("%s MATCH ANY %s", e.Field, formatValue(e.Text))
	case MatchPhraseExpr:
		return fmt.Sprintf("%s MATCH PHRASE %s", e.Field, formatValue(e.Text))
	case AndExpr:
		var parts []string
		for _, op := range e.Operands {
			parts = append(parts, FormatFilterExpr(op))
		}
		return strings.Join(parts, " AND ")
	case OrExpr:
		var parts []string
		for _, op := range e.Operands {
			parts = append(parts, FormatFilterExpr(op))
		}
		return "(" + strings.Join(parts, " OR ") + ")"
	case NotExpr:
		return fmt.Sprintf("NOT %s", FormatFilterExpr(e.Operand))
	default:
		return "<unknown_filter>"
	}
}

func FormatQueryStmt(q *QueryStmt) string {
	var parts []string

	// CTEs
	if len(q.CTEs) > 0 {
		var ctes []string
		for _, cte := range q.CTEs {
			ctes = append(ctes, fmt.Sprintf("%s AS (%s)", cte.Name, FormatQueryStmt(cte.Stmt)))
		}
		parts = append(parts, "WITH "+strings.Join(ctes, ", "))
	}

	// QUERY type
	switch q.Mode {
	case QueryModeNearest:
		if q.QueryText != nil {
			parts = append(parts, fmt.Sprintf("QUERY %s", formatValue(*q.QueryText)))
		} else if q.QueryID != nil {
			parts = append(parts, fmt.Sprintf("QUERY %s", formatValue(q.QueryID)))
		} else {
			parts = append(parts, "QUERY '<query_text>'")
		}
	case QueryModeRecommend:
		parts = append(parts, "QUERY RECOMMEND")
		if len(q.PositiveIDs) > 0 || len(q.NegativeIDs) > 0 {
			var withParts []string
			if len(q.PositiveIDs) > 0 {
				withParts = append(withParts, fmt.Sprintf("positive = %s", formatValue(q.PositiveIDs)))
			}
			if len(q.NegativeIDs) > 0 {
				withParts = append(withParts, fmt.Sprintf("negative = %s", formatValue(q.NegativeIDs)))
			}
			parts = append(parts, fmt.Sprintf("WITH (%s)", strings.Join(withParts, ", ")))
		}
	case QueryModeDiscover:
		parts = append(parts, fmt.Sprintf("QUERY DISCOVER TARGET %s", formatValue(q.Target)))
	case QueryModeContext:
		parts = append(parts, "QUERY CONTEXT")
	case QueryModeRelevanceFeedback:
		parts = append(parts, fmt.Sprintf("QUERY RELEVANCE FEEDBACK TARGET %s", formatValue(q.FeedbackTarget)))
		if len(q.FeedbackItems) > 0 {
			var fbParts []string
			for _, item := range q.FeedbackItems {
				fbParts = append(fbParts, fmt.Sprintf("(%s, %g)", formatValue(item.Example), item.Score))
			}
			parts = append(parts, fmt.Sprintf("FEEDBACK (%s)", strings.Join(fbParts, ", ")))
		}
	default:
		parts = append(parts, "QUERY")
	}

	if q.Collection != "" {
		parts = append(parts, "FROM "+q.Collection)
	}

	// Strategy
	if q.FeedbackStrategy != nil {
		if q.FeedbackStrategy.Type == FeedbackStrategyNaive {
			parts = append(parts, fmt.Sprintf("STRATEGY NAIVE (a = %g, b = %g, c = %g)",
				q.FeedbackStrategy.A, q.FeedbackStrategy.B, q.FeedbackStrategy.C))
		}
	} else if q.Strategy != nil {
		parts = append(parts, "STRATEGY "+*q.Strategy)
	}

	if q.WithClause != nil {
		var withParts []string
		if q.WithClause.MmrDiversity != nil {
			withParts = append(withParts, fmt.Sprintf("mmr_diversity = %g", *q.WithClause.MmrDiversity))
		}
		if q.WithClause.MmrCandidates != nil {
			withParts = append(withParts, fmt.Sprintf("mmr_candidates = %d", *q.WithClause.MmrCandidates))
		}
		if q.WithClause.Exact {
			withParts = append(withParts, "exact = true")
		}
		if q.WithClause.IndexedOnly {
			withParts = append(withParts, "indexed_only = true")
		}
		if q.WithClause.HnswEf > 0 {
			withParts = append(withParts, fmt.Sprintf("hnsw_ef = %d", q.WithClause.HnswEf))
		}
		if len(withParts) > 0 {
			parts = append(parts, fmt.Sprintf("WITH (%s)", strings.Join(withParts, ", ")))
		}
	}

	if q.QueryFilter != nil {
		parts = append(parts, "WHERE "+FormatFilterExpr(q.QueryFilter))
	}

	if q.Formula != nil {
		parts = append(parts, fmt.Sprintf("BOOST (%s)", FormulaExprString(q.Formula)))
	}

	if len(q.FormulaDefaults) > 0 {
		var defs []string
		for k, v := range q.FormulaDefaults {
			defs = append(defs, fmt.Sprintf("%s = %s", k, formatValue(v)))
		}
		parts = append(parts, fmt.Sprintf("DEFAULTS (%s)", strings.Join(defs, ", ")))
	}

	if q.Limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", q.Limit))
	}

	if len(q.PrefetchRefs) > 0 {
		var prefetches []string
		for _, p := range q.PrefetchRefs {
			prefetches = append(prefetches, p.CTEName)
		}
		parts = append(parts, fmt.Sprintf("PREFETCH (%s)", strings.Join(prefetches, ", ")))
	}

	return strings.Join(parts, " ")
}
