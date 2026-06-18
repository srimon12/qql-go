package convert

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/srimon12/qql-go/internal/ast"
)

func convertFormulaQuery(input, collection string) ([]string, error) {
	var req RESTQueryRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid formula query JSON: %w", err)
	}

	stmt, err := convertRESTQueryToAST(&req, collection)
	if err != nil {
		return nil, err
	}

	// Extract query text if present
	if req.Query.Document != nil {
		if docMap, ok := req.Query.Document.(map[string]any); ok {
			if text, ok := docMap["text"]; ok {
				if s, ok := text.(string); ok {
					stmt.QueryText = &s
				}
			}
		}
	} else if req.Query.Nearest != nil {
		if nearestMap, ok := req.Query.Nearest.(map[string]any); ok {
			if doc, ok := nearestMap["document"]; ok {
				if docMap, ok := doc.(map[string]any); ok {
					if text, ok := docMap["text"]; ok {
						if s, ok := text.(string); ok {
							stmt.QueryText = &s
						}
					}
				}
			}
		}
	}

	// Formula conversion
	if req.Query.Formula != nil {
		var formulaObj any
		if err := json.Unmarshal(req.Query.Formula, &formulaObj); err == nil {
			formulaStr, err := convertFormulaExpression(formulaObj)
			if err == nil && formulaStr != "" {
				stmt.Formula = ast.RawFormulaExpr{Expr: formulaStr}
			}
		}
	}

	return []string{ast.FormatQueryStmt(stmt)}, nil
}

func convertFormulaExpression(input any) (string, error) {
	switch expr := input.(type) {
	case string:
		// Variable reference: "$score", "field_name"
		return expr, nil
	case float64:
		return fmt.Sprintf("%v", expr), nil
	case map[string]any:
		return convertFormulaObject(expr)
	default:
		return fmt.Sprintf("%v", input), nil
	}
}

func convertFormulaObject(expr map[string]any) (string, error) {
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
		if arr, ok := div.([]any); ok && len(arr) == 2 {
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
		if arr, ok := pow.([]any); ok && len(arr) == 2 {
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
	// Condition (match/key) - Qdrant formula inline condition
	if key, ok := expr["key"]; ok {
		if match, ok := expr["match"]; ok {
			matchMap, ok := match.(map[string]any)
			if ok {
				if any, ok := matchMap["any"]; ok {
					return convertMatchFormulaCondition(fmt.Sprintf("%v", key), any), nil
				}
				if value, ok := matchMap["value"]; ok {
					return fmt.Sprintf("MATCH(%s, %s)", key, formatFormulaValue(value)), nil
				}
				if keyword, ok := matchMap["keyword"]; ok {
					return fmt.Sprintf("MATCH(%s, %s)", key, formatFormulaValue(keyword)), nil
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

func convertNaryOp(op string, input any) (string, error) {
	arr, ok := input.([]any)
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

func convertMatchFormulaCondition(field string, any any) string {
	values, ok := any.([]interface{})
	if !ok {
		return fmt.Sprintf("MATCH(%s, %s)", field, formatFormulaValue(any))
	}
	var vals []string
	for _, v := range values {
		vals = append(vals, formatFormulaValue(v))
	}
	return fmt.Sprintf("MATCH(%s, [%s])", field, strings.Join(vals, ", "))
}

func convertGeoDistanceExpr(input any) (string, error) {
	geoMap, ok := input.(map[string]any)
	if !ok {
		return "GEO_DISTANCE(...)", nil
	}
	origin, _ := geoMap["origin"].(map[string]any)
	to, _ := geoMap["to"].(string)
	if origin != nil {
		lat, _ := origin["lat"].(float64)
		lon, _ := origin["lon"].(float64)
		return fmt.Sprintf("GEO_DISTANCE({lat: %v, lon: %v}, %s)", lat, lon, to), nil
	}
	return fmt.Sprintf("GEO_DISTANCE(origin, %s)", to), nil
}

func convertDecayExpr(name string, input any) (string, error) {
	decayMap, ok := input.(map[string]any)
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
			Nearest any `json:"nearest"`
			MMR     struct {
				Diversity       float64 `json:"diversity"`
				CandidatesLimit int     `json:"candidates_limit"`
			} `json:"mmr"`
		} `json:"query"`
		Limit          int         `json:"limit"`
		Offset         int         `json:"offset"`
		Filter         *RESTFilter `json:"filter"`
		WithPayload    any         `json:"with_payload"`
		WithVector     any         `json:"with_vector"`
		ScoreThreshold *float64    `json:"score_threshold"`
		Using          string      `json:"using"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid MMR query JSON: %w", err)
	}

	stmt := &ast.QueryStmt{
		Collection:     collection,
		Mode:           ast.QueryModeNearest,
		Limit:          req.Limit,
		Offset:         req.Offset,
		ScoreThreshold: req.ScoreThreshold,
	}

	// Nearest Vector or Text
	if vec, ok := req.Query.Nearest.([]any); ok && len(vec) > 0 {
		raw := make([]float64, len(vec))
		for i, v := range vec {
			switch f := v.(type) {
			case float64:
				raw[i] = f
			case int:
				raw[i] = float64(f)
			}
		}
		stmt.RawVector = raw
	} else if docMap, ok := req.Query.Nearest.(map[string]any); ok {
		if text, ok := docMap["text"]; ok {
			if s, ok := text.(string); ok {
				stmt.QueryText = &s
			}
		}
	} else if req.Query.Nearest != nil {
		str := "<query_text>"
		stmt.QueryText = &str
	}

	// Using
	switch strings.ToLower(req.Using) {
	case "hybrid":
		stmt.Type = ast.QueryTypeHybrid
	case "sparse":
		stmt.Type = ast.QueryTypeSparse
	default:
		if req.Using != "" {
			using := req.Using
			stmt.Using = &using
		}
	}

	// Filter
	if req.Filter != nil {
		stmt.QueryFilter = convertRESTFilter(req.Filter)
	}

	// MMR Params
	stmt.WithClause = &ast.SearchWith{
		MmrDiversity: &req.Query.MMR.Diversity,
	}
	if req.Query.MMR.CandidatesLimit > 0 {
		stmt.WithClause.MmrCandidates = &req.Query.MMR.CandidatesLimit
	}

	// WithPayload and WithVectors
	stmt.WithPayload = buildPayloadSelector(req.WithPayload)
	stmt.WithVectors = buildVectorsSelector(req.WithVector)

	return []string{ast.FormatQueryStmt(stmt)}, nil
}

func convertRelevanceFeedback(input, collection string) ([]string, error) {
	var req struct {
		Query struct {
			RelevanceFeedback struct {
				Target   any `json:"target"`
				Feedback []struct {
					Example any     `json:"example"`
					Score   float64 `json:"score"`
				} `json:"feedback"`
				Strategy any `json:"strategy"`
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
		if stratMap, ok := rf.Strategy.(map[string]any); ok {
			if naive, ok := stratMap["naive"].(map[string]any); ok {
				a, _ := naive["a"].(float64)
				b, _ := naive["b"].(float64)
				c, _ := naive["c"].(float64)
				parts = append(parts, fmt.Sprintf("STRATEGY NAIVE (a = %g, b = %g, c = %g)", a, b, c))
			}
		}
	}

	return []string{strings.Join(parts, " ")}, nil
}

func formatFormulaValue(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", val)
	case []any:
		var items []string
		for _, item := range val {
			items = append(items, formatFormulaValue(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case map[string]any:
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
