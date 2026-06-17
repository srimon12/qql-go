package convert

import (
	"encoding/json"
	"fmt"
	"strings"
)

func convertFormulaQuery(input, collection string) ([]string, error) {
	var req struct {
		Prefetch interface{} `json:"prefetch"`
		Query    struct {
			Formula  interface{} `json:"formula"`
			Nearest  interface{} `json:"nearest"`
			Document interface{} `json:"document"`
		} `json:"query"`
		Limit    int         `json:"limit"`
		Filter   interface{} `json:"filter"`
		Defaults interface{} `json:"defaults"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid formula query JSON: %w", err)
	}

	var stmts []string
	var cteNames []string

	// Handle prefetches (single object or array)
	if req.Prefetch != nil {
		prefetches := normalizePrefetchArray(req.Prefetch)
		for i, pf := range prefetches {
			cteName := fmt.Sprintf("_pf%d", i)
			cteNames = append(cteNames, cteName)
			cteQQL, err := convertSinglePrefetch(pf, cteName, collection)
			if err == nil {
				stmts = append(stmts, cteQQL)
			}
		}
	}

	// Build main query
	var parts []string
	parts = append(parts, fmt.Sprintf("QUERY '<query_text>' FROM %s", collection))

	if req.Limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", req.Limit))
	}

	// Convert formula expression
	if req.Query.Formula != nil {
		formulaStr, err := convertFormulaExpression(req.Query.Formula)
		if err == nil && formulaStr != "" {
			parts = append(parts, fmt.Sprintf("BOOST (%s)", formulaStr))
		}
	}

	// Convert defaults
	if req.Defaults != nil {
		if defs, ok := req.Defaults.(map[string]interface{}); ok && len(defs) > 0 {
			var entries []string
			for k, v := range defs {
				entries = append(entries, fmt.Sprintf("%s = %s", k, formatFormulaValue(v)))
			}
			parts = append(parts, fmt.Sprintf("DEFAULTS (%s)", strings.Join(entries, ", ")))
		}
	}

	mainQQL := strings.Join(parts, " ")

	// If we have CTEs, build WITH ... QUERY ... PREFETCH (...) syntax
	if len(cteNames) > 0 {
		var cteDefs []string
		for i, name := range cteNames {
			cteDefs = append(cteDefs, fmt.Sprintf("%s AS (%s)", name, stmts[i]))
		}
		withClause := "WITH " + strings.Join(cteDefs, ", ")
		prefetchClause := fmt.Sprintf("PREFETCH (%s)", strings.Join(cteNames, ", "))
		return []string{fmt.Sprintf("%s %s %s", withClause, mainQQL, prefetchClause)}, nil
	}

	return []string{mainQQL}, nil
}

func normalizePrefetchArray(input interface{}) []interface{} {
	switch pf := input.(type) {
	case []interface{}:
		return pf
	case map[string]interface{}:
		return []interface{}{pf}
	default:
		return nil
	}
}

func convertSinglePrefetch(pf interface{}, _, _ string) (string, error) {
	pfMap, ok := pf.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid prefetch object")
	}

	// CTE bodies don't support FROM — use simplified QUERY syntax
	var parts []string
	parts = append(parts, "QUERY '<query_text>'")

	if limit, ok := pfMap["limit"].(float64); ok && limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", int(limit)))
	}

	if pfMap["filter"] != nil {
		filterStr, err := convertFilter(pfMap["filter"])
		if err == nil && filterStr != "" {
			parts = append(parts, "WHERE "+filterStr)
		}
	}

	return strings.Join(parts, " "), nil
}

func convertFormulaExpression(input interface{}) (string, error) {
	switch expr := input.(type) {
	case string:
		// Variable reference: "$score", "field_name"
		return expr, nil
	case float64:
		return fmt.Sprintf("%v", expr), nil
	case map[string]interface{}:
		return convertFormulaObject(expr)
	default:
		return fmt.Sprintf("%v", input), nil
	}
}

func convertFormulaObject(expr map[string]interface{}) (string, error) {
	// Sum
	if sum, ok := expr["sum"]; ok {
		return convertNaryOp(" + ", sum)
	}
	// Mult
	if mult, ok := expr["mult"]; ok {
		return convertNaryOp(" * ", mult)
	}
	// Div
	if div, ok := expr["div"]; ok {
		if arr, ok := div.([]interface{}); ok && len(arr) == 2 {
			left, _ := convertFormulaExpression(arr[0])
			right, _ := convertFormulaExpression(arr[1])
			return fmt.Sprintf("(%s / %s)", left, right), nil
		}
	}
	// Abs
	if abs, ok := expr["abs"]; ok {
		inner, _ := convertFormulaExpression(abs)
		return fmt.Sprintf("ABS(%s)", inner), nil
	}
	// Sqrt
	if sqrt, ok := expr["sqrt"]; ok {
		inner, _ := convertFormulaExpression(sqrt)
		return fmt.Sprintf("SQRT(%s)", inner), nil
	}
	// Pow
	if pow, ok := expr["pow"]; ok {
		if arr, ok := pow.([]interface{}); ok && len(arr) == 2 {
			base, _ := convertFormulaExpression(arr[0])
			exp, _ := convertFormulaExpression(arr[1])
			return fmt.Sprintf("POW(%s, %s)", base, exp), nil
		}
	}
	// Log10
	if log, ok := expr["log10"]; ok {
		inner, _ := convertFormulaExpression(log)
		return fmt.Sprintf("LOG(%s)", inner), nil
	}
	// Ln
	if ln, ok := expr["ln"]; ok {
		inner, _ := convertFormulaExpression(ln)
		return fmt.Sprintf("LN(%s)", inner), nil
	}
	// Exp
	if exp, ok := expr["exp"]; ok {
		inner, _ := convertFormulaExpression(exp)
		return fmt.Sprintf("EXP(%s)", inner), nil
	}
	// Condition (match/key)
	if key, ok := expr["key"]; ok {
		if match, ok := expr["match"]; ok {
			matchMap, ok := match.(map[string]interface{})
			if ok {
				if any, ok := matchMap["any"]; ok {
					return convertMatchAnyCondition(fmt.Sprintf("%v", key), any), nil
				}
				if value, ok := matchMap["value"]; ok {
					return fmt.Sprintf("%s = %s", key, formatValue(value)), nil
				}
				if keyword, ok := matchMap["keyword"]; ok {
					return fmt.Sprintf("%s = %s", key, formatValue(keyword)), nil
				}
			}
		}
	}
	// Geo distance
	if geoDist, ok := expr["geo_distance"]; ok {
		return convertGeoDistanceExpr(geoDist)
	}
	// Gauss decay
	if decay, ok := expr["gauss_decay"]; ok {
		return convertDecayExpr("GAUSS_DECAY", decay)
	}
	// Exp decay
	if decay, ok := expr["exp_decay"]; ok {
		return convertDecayExpr("EXP_DECAY", decay)
	}
	// Lin decay
	if decay, ok := expr["lin_decay"]; ok {
		return convertDecayExpr("LIN_DECAY", decay)
	}
	// Datetime
	if dt, ok := expr["datetime"]; ok {
		return fmt.Sprintf("datetime('%v')", dt), nil
	}
	// Datetime key
	if dk, ok := expr["datetime_key"]; ok {
		return fmt.Sprintf("datetime_key('%v')", dk), nil
	}

	return fmt.Sprintf("%v", expr), nil
}

func convertNaryOp(op string, input interface{}) (string, error) {
	arr, ok := input.([]interface{})
	if !ok {
		return "", fmt.Errorf("expected array")
	}
	var parts []string
	for _, item := range arr {
		s, _ := convertFormulaExpression(item)
		parts = append(parts, s)
	}
	return "(" + strings.Join(parts, op) + ")", nil
}

func convertMatchAnyCondition(key string, any interface{}) string {
	values, ok := any.([]interface{})
	if !ok {
		return fmt.Sprintf("%s = %v", key, any)
	}
	var vals []string
	for _, v := range values {
		vals = append(vals, formatValue(v))
	}
	return fmt.Sprintf("%s IN (%s)", key, strings.Join(vals, ", "))
}

func convertGeoDistanceExpr(input interface{}) (string, error) {
	geoMap, ok := input.(map[string]interface{})
	if !ok {
		return "GEO_DISTANCE(...)", nil
	}
	origin, _ := geoMap["origin"].(map[string]interface{})
	to, _ := geoMap["to"].(string)
	if origin != nil {
		lat, _ := origin["lat"].(float64)
		lon, _ := origin["lon"].(float64)
		return fmt.Sprintf("GEO_DISTANCE({lat: %v, lon: %v}, %s)", lat, lon, to), nil
	}
	return fmt.Sprintf("GEO_DISTANCE(origin, %s)", to), nil
}

func convertDecayExpr(name string, input interface{}) (string, error) {
	decayMap, ok := input.(map[string]interface{})
	if !ok {
		return fmt.Sprintf("%s(...)", name), nil
	}

	var args []string

	// x parameter
	if x, ok := decayMap["x"]; ok {
		xStr, _ := convertFormulaExpression(x)
		args = append(args, xStr)
	}

	// target
	if target, ok := decayMap["target"]; ok {
		targetStr, _ := convertFormulaExpression(target)
		args = append(args, fmt.Sprintf("target: %s", targetStr))
	}

	// scale
	if scale, ok := decayMap["scale"]; ok {
		args = append(args, fmt.Sprintf("scale: %v", scale))
	}

	// midpoint
	if midpoint, ok := decayMap["midpoint"]; ok {
		args = append(args, fmt.Sprintf("midpoint: %v", midpoint))
	}

	return fmt.Sprintf("%s(%s)", name, strings.Join(args, ", ")), nil
}

func convertMMRQuery(input, collection string) ([]string, error) {
	var req struct {
		Query struct {
			Nearest interface{} `json:"nearest"`
			MMR     struct {
				Diversity       float64 `json:"diversity"`
				CandidatesLimit int     `json:"candidates_limit"`
			} `json:"mmr"`
		} `json:"query"`
		Limit  int         `json:"limit"`
		Filter interface{} `json:"filter"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid MMR query JSON: %w", err)
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("QUERY '<query_text>' FROM %s", collection))

	if req.Limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", req.Limit))
	}

	mmrParams := []string{
		fmt.Sprintf("mmr_diversity = %g", req.Query.MMR.Diversity),
	}
	if req.Query.MMR.CandidatesLimit > 0 {
		mmrParams = append(mmrParams, fmt.Sprintf("mmr_candidates = %d", req.Query.MMR.CandidatesLimit))
	}
	parts = append(parts, fmt.Sprintf("WITH (%s)", strings.Join(mmrParams, ", ")))

	if req.Filter != nil {
		filterStr, err := convertFilter(req.Filter)
		if err == nil && filterStr != "" {
			parts = append(parts, "WHERE "+filterStr)
		}
	}

	return []string{strings.Join(parts, " ")}, nil
}

func convertRelevanceFeedback(input, collection string) ([]string, error) {
	var req struct {
		Query struct {
			RelevanceFeedback struct {
				Target   interface{} `json:"target"`
				Feedback []struct {
					Example interface{} `json:"example"`
					Score   float64     `json:"score"`
				} `json:"feedback"`
				Strategy interface{} `json:"strategy"`
			} `json:"relevance_feedback"`
		} `json:"query"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid relevance feedback JSON: %w", err)
	}

	rf := req.Query.RelevanceFeedback

	var feedbackItems []string
	for _, f := range rf.Feedback {
		feedbackItems = append(feedbackItems, fmt.Sprintf("(%s, %g)", formatID(f.Example), f.Score))
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("QUERY RELEVANCE FEEDBACK TARGET %s", formatFormulaValue(rf.Target)))
	if len(feedbackItems) > 0 {
		parts = append(parts, fmt.Sprintf("FEEDBACK (%s)", strings.Join(feedbackItems, ", ")))
	}
	parts = append(parts, fmt.Sprintf("FROM %s", collection))

	// Strategy
	if rf.Strategy != nil {
		if stratMap, ok := rf.Strategy.(map[string]interface{}); ok {
			if naive, ok := stratMap["naive"].(map[string]interface{}); ok {
				a, _ := naive["a"].(float64)
				b, _ := naive["b"].(float64)
				c, _ := naive["c"].(float64)
				parts = append(parts, fmt.Sprintf("STRATEGY NAIVE (a = %g, b = %g, c = %g)", a, b, c))
			}
		}
	}

	return []string{strings.Join(parts, " ")}, nil
}

func formatFormulaValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", val)
	case []interface{}:
		var items []string
		for _, item := range val {
			items = append(items, formatFormulaValue(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case map[string]interface{}:
		// Geo point
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

// --- Filter conversion ---
