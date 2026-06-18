package convert

import (
	"encoding/json"
	"fmt"
	"github.com/srimon12/qql-go/internal/ast"
	"maps"
)

func convertRESTQueryToAST(req *RESTQueryRequest, collection string) (*ast.QueryStmt, error) {
	stmt := &ast.QueryStmt{
		Collection: collection,
		Mode:       ast.QueryModeNearest,
	}

	if req.Limit != nil {
		stmt.Limit = *req.Limit
	}

	if req.Offset != nil {
		stmt.Offset = *req.Offset
	}

	if req.Filter != nil {
		f := convertRESTFilter(req.Filter)
		stmt.QueryFilter = f
	}

	// Using
	if req.Using != "" {
		using := req.Using
		stmt.Using = &using
	}

	// WithPayload
	if req.WithPayload != nil {
		stmt.WithPayload = buildPayloadSelector(req.WithPayload)
	}

	if len(req.Defaults) > 0 {
		stmt.FormulaDefaults = make(map[string]any)
		maps.Copy(stmt.FormulaDefaults, req.Defaults)
	}

	// Prefetches
	if len(req.Prefetch) > 0 {
		for i, pf := range req.Prefetch {
			cteName := fmt.Sprintf("_pf%d", i)
			pfStmt, err := convertRESTPrefetchToAST(&pf, collection, cteName)
			if err == nil {
				stmt.CTEs = append(stmt.CTEs, ast.CTE{Name: cteName, Stmt: pfStmt})
				stmt.PrefetchRefs = append(stmt.PrefetchRefs, ast.PrefetchRef{CTEName: cteName})
			}
		}
	}

	return stmt, nil
}

func convertRESTPrefetchToAST(pf *RESTPrefetch, collection, prefix string) (*ast.QueryStmt, error) {
	stmt := &ast.QueryStmt{
		Mode: ast.QueryModeNearest,
	}
	if pf.Limit != nil {
		stmt.Limit = *pf.Limit
	}
	if pf.Filter != nil {
		stmt.QueryFilter = convertRESTFilter(pf.Filter)
	}

	// Using
	if pf.Using != "" {
		using := pf.Using
		stmt.Using = &using
	}

	// ScoreThreshold
	if pf.ScoreThreshold != nil {
		stmt.ScoreThreshold = pf.ScoreThreshold
	}

	// Handle Nested Prefetches
	if len(pf.Prefetch) > 0 {
		for i, childPf := range pf.Prefetch {
			cteName := fmt.Sprintf("%s_%d", prefix, i)
			pfStmt, err := convertRESTPrefetchToAST(&childPf, collection, cteName)
			if err == nil {
				// Add the child's own CTEs if any, then the child itself
				stmt.CTEs = append(stmt.CTEs, pfStmt.CTEs...)
				pfStmt.CTEs = nil // Clear child CTEs as they're bubbled up

				stmt.CTEs = append(stmt.CTEs, ast.CTE{Name: cteName, Stmt: pfStmt})
				stmt.PrefetchRefs = append(stmt.PrefetchRefs, ast.PrefetchRef{CTEName: cteName})
			}
		}
	}

	if pf.Query.Text != "" {
		stmt.QueryText = &pf.Query.Text
		if pf.Query.Model != "" {
			stmt.Model = &pf.Query.Model
		}
	} else if pf.Document != nil {
		if docMap, ok := pf.Document.(map[string]any); ok {
			if text, ok := docMap["text"]; ok {
				if s, ok := text.(string); ok {
					stmt.QueryText = &s
				}
			}
		}
	} else if pf.Query.Document != nil {
		if docMap, ok := pf.Query.Document.(map[string]any); ok {
			if text, ok := docMap["text"]; ok {
				if s, ok := text.(string); ok {
					stmt.QueryText = &s
				}
			}
		}
	} else if pf.Query.Nearest != nil {
		if nearestMap, ok := pf.Query.Nearest.(map[string]any); ok {
			if doc, ok := nearestMap["document"]; ok {
				if docMap, ok := doc.(map[string]any); ok {
					if text, ok := docMap["text"]; ok {
						if s, ok := text.(string); ok {
							stmt.QueryText = &s
						}
					}
				}
			}
		} else if vec, ok := pf.Query.Nearest.([]any); ok && len(vec) > 0 {
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
		}
	} else if pf.Vector != nil {
		if vec, ok := pf.Vector.([]any); ok && len(vec) > 0 {
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
		}
	}

	return stmt, nil
}

func convertRESTFilter(f *RESTFilter) ast.FilterExpr {
	if f == nil {
		return nil
	}
	var operands []ast.FilterExpr

	if len(f.Must) > 0 {
		for _, c := range f.Must {
			if expr := convertRESTCondition(c); expr != nil {
				operands = append(operands, expr)
			}
		}
	}

	if len(f.Should) > 0 {
		var shouldOps []ast.FilterExpr
		for _, c := range f.Should {
			if expr := convertRESTCondition(c); expr != nil {
				shouldOps = append(shouldOps, expr)
			}
		}
		if len(shouldOps) == 1 {
			operands = append(operands, shouldOps[0])
		} else if len(shouldOps) > 1 {
			operands = append(operands, ast.OrExpr{Operands: shouldOps})
		}
	}

	if len(f.MustNot) > 0 {
		for _, c := range f.MustNot {
			if expr := convertRESTCondition(c); expr != nil {
				operands = append(operands, ast.NotExpr{Operand: expr})
			}
		}
	}

	if len(operands) == 1 {
		return operands[0]
	} else if len(operands) > 1 {
		return ast.AndExpr{Operands: operands}
	}
	return nil
}

func convertRESTCondition(c RESTCondition) ast.FilterExpr {
	if len(c.Must) > 0 || len(c.Should) > 0 || len(c.MustNot) > 0 {
		return convertRESTFilter(&RESTFilter{Must: c.Must, Should: c.Should, MustNot: c.MustNot})
	}
	if c.IsEmpty != nil {
		if c.IsEmpty.Key != "" {
			return ast.IsEmptyExpr{Field: c.IsEmpty.Key}
		}
	}
	if c.IsNull != nil {
		if c.IsNull.Key != "" {
			return ast.IsNullExpr{Field: c.IsNull.Key}
		}
	}
	if c.Nested != nil {
		var nestedFilter RESTFilter
		if err := json.Unmarshal(c.Nested.Filter, &nestedFilter); err == nil {
			var key string
			json.Unmarshal(c.Nested.Key, &key)
			return convertRESTFilter(&nestedFilter)
		}
		return nil
	}
	if c.GeoBoundingBox != nil {
		return ast.CompareExpr{Field: c.Key, Op: "GEO_BBOX", Value: map[string]any{
			"top_left": map[string]any{
				"lat": c.GeoBoundingBox.TopLeft.Lat,
				"lon": c.GeoBoundingBox.TopLeft.Lon,
			},
			"bottom_right": map[string]any{
				"lat": c.GeoBoundingBox.BottomRight.Lat,
				"lon": c.GeoBoundingBox.BottomRight.Lon,
			},
		}}
	}
	if c.GeoRadius != nil {
		return ast.CompareExpr{Field: c.Key, Op: "GEO_RADIUS", Value: map[string]any{
			"center": map[string]any{
				"lat": c.GeoRadius.Center.Lat,
				"lon": c.GeoRadius.Center.Lon,
			},
			"radius": c.GeoRadius.Radius,
		}}
	}
	if c.ValuesCount != nil {
		return ast.CompareExpr{Field: c.Key, Op: "VALUES_COUNT", Value: c.ValuesCount}
	}
	if c.HasVector != nil && c.HasVector.Vector != "" {
		return ast.CompareExpr{Field: c.HasVector.Vector, Op: "HAS_VECTOR", Value: true}
	}
	if c.Key != "" {
		if c.Match != nil {
			if val, ok := c.Match["value"]; ok {
				return ast.CompareExpr{Field: c.Key, Op: "=", Value: val}
			}
			if val, ok := c.Match["keyword"]; ok {
				return ast.CompareExpr{Field: c.Key, Op: "=", Value: val}
			}
			if val, ok := c.Match["integer"]; ok {
				return ast.CompareExpr{Field: c.Key, Op: "=", Value: val}
			}
			if val, ok := c.Match["boolean"]; ok {
				return ast.CompareExpr{Field: c.Key, Op: "=", Value: val}
			}
			if val, ok := c.Match["text"]; ok {
				return ast.MatchTextExpr{Field: c.Key, Text: fmt.Sprintf("%v", val)}
			}
			if val, ok := c.Match["text_any"]; ok {
				return ast.MatchAnyExpr{Field: c.Key, Text: fmt.Sprintf("%v", val)}
			}
			if val, ok := c.Match["phrase"]; ok {
				return ast.MatchPhraseExpr{Field: c.Key, Text: fmt.Sprintf("%v", val)}
			}
			if anyList, ok := c.Match["any"].([]any); ok {
				return ast.InExpr{Field: c.Key, Values: anyList}
			}
			if exceptList, ok := c.Match["except"].([]any); ok {
				return ast.NotInExpr{Field: c.Key, Values: exceptList}
			}
		}
		if c.Range != nil {
			gte, hasGte := c.Range["gte"]
			gt, hasGt := c.Range["gt"]
			lte, hasLte := c.Range["lte"]
			lt, hasLt := c.Range["lt"]

			if (hasGte || hasGt) && (hasLte || hasLt) {
				low := gte
				if hasGt {
					low = gt
				}
				high := lte
				if hasLt {
					high = lt
				}
				return ast.BetweenExpr{Field: c.Key, Low: low, High: high}
			}
			if hasGte {
				return ast.CompareExpr{Field: c.Key, Op: ">=", Value: gte}
			}
			if hasGt {
				return ast.CompareExpr{Field: c.Key, Op: ">", Value: gt}
			}
			if hasLte {
				return ast.CompareExpr{Field: c.Key, Op: "<=", Value: lte}
			}
			if hasLt {
				return ast.CompareExpr{Field: c.Key, Op: "<", Value: lt}
			}
		}
	}
	if len(c.HasID) > 0 {
		return ast.InExpr{Field: "id", Values: c.HasID}
	}
	return nil
}
