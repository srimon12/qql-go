package convert

import (
	"fmt"
	"strings"
)

func convertFilter(input interface{}) (string, error) {
	if input == nil {
		return "", nil
	}

	filterMap, ok := input.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("filter must be an object")
	}

	return convertFilterObject(filterMap)
}

func convertFilterObject(filter map[string]interface{}) (string, error) {
	var parts []string

	// Handle "must" (AND)
	if must, ok := filter["must"]; ok {
		conds, err := convertConditions(must)
		if err != nil {
			return "", err
		}
		parts = append(parts, conds...)
	}

	// Handle "should" (OR)
	if should, ok := filter["should"]; ok {
		conds, err := convertConditions(should)
		if err != nil {
			return "", err
		}
		if len(conds) > 1 {
			parts = append(parts, "("+strings.Join(conds, " OR ")+")")
		} else if len(conds) == 1 {
			parts = append(parts, conds[0])
		}
	}

	// Handle "must_not" (NOT)
	if mustNot, ok := filter["must_not"]; ok {
		conds, err := convertConditions(mustNot)
		if err != nil {
			return "", err
		}
		for _, cond := range conds {
			parts = append(parts, "NOT "+cond)
		}
	}

	return strings.Join(parts, " AND "), nil
}

func convertConditions(input interface{}) ([]string, error) {
	conds, ok := input.([]interface{})
	if !ok {
		return nil, fmt.Errorf("conditions must be an array")
	}

	var result []string
	for _, cond := range conds {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		s, err := convertCondition(condMap)
		if err != nil {
			continue
		}
		if s != "" {
			result = append(result, s)
		}
	}
	return result, nil
}

func convertCondition(cond map[string]interface{}) (string, error) {
	// Nested filter
	if _, ok := cond["must"]; ok {
		return convertFilterObject(cond)
	}
	if _, ok := cond["should"]; ok {
		return convertFilterObject(cond)
	}
	if _, ok := cond["must_not"]; ok {
		return convertFilterObject(cond)
	}

	// Field condition: {"key": "...", "match": {...}} or {"key": "...", "range": {...}}
	key, _ := cond["key"].(string)
	if key == "" {
		return "", fmt.Errorf("condition missing 'key'")
	}

	if match, ok := cond["match"].(map[string]interface{}); ok {
		return convertMatchCondition(key, match)
	}

	if rangeCond, ok := cond["range"].(map[string]interface{}); ok {
		return convertRangeCondition(key, rangeCond)
	}

	if isNull, ok := cond["is_null"].(map[string]interface{}); ok {
		if v, ok := isNull["value"].(bool); ok && v {
			return fmt.Sprintf("%s IS NULL", key), nil
		}
	}

	if isEmpty, ok := cond["is_empty"].(map[string]interface{}); ok {
		if v, ok := isEmpty["value"].(bool); ok && v {
			return fmt.Sprintf("%s IS EMPTY", key), nil
		}
	}

	if hasId, ok := cond["has_id"].(map[string]interface{}); ok {
		if ids, ok := hasId["has_id"].([]interface{}); ok {
			return fmt.Sprintf("%s IN (%s)", key, formatIDList(ids)), nil
		}
	}

	return "", nil
}

func convertMatchCondition(key string, match map[string]interface{}) (string, error) {
	if value, ok := match["value"]; ok {
		return fmt.Sprintf("%s = %s", key, formatValue(value)), nil
	}
	if keyword, ok := match["keyword"]; ok {
		return fmt.Sprintf("%s = %s", key, formatValue(keyword)), nil
	}
	if integer, ok := match["integer"]; ok {
		return fmt.Sprintf("%s = %v", key, integer), nil
	}
	if boolean, ok := match["boolean"]; ok {
		return fmt.Sprintf("%s = %v", key, boolean), nil
	}
	if text, ok := match["text"]; ok {
		return fmt.Sprintf("%s MATCH %s", key, formatValue(text)), nil
	}
	if any, ok := match["any"]; ok {
		if values, ok := any.([]interface{}); ok {
			var vals []string
			for _, v := range values {
				vals = append(vals, formatValue(v))
			}
			return fmt.Sprintf("%s IN (%s)", key, strings.Join(vals, ", ")), nil
		}
	}
	except, hasExcept := match["except"]
	if hasExcept {
		if values, ok := except.([]interface{}); ok {
			var vals []string
			for _, v := range values {
				vals = append(vals, formatValue(v))
			}
			return fmt.Sprintf("%s NOT IN (%s)", key, strings.Join(vals, ", ")), nil
		}
	}

	return fmt.Sprintf("%s = <value>", key), nil
}

func convertRangeCondition(key string, r map[string]interface{}) (string, error) {
	gte, hasGte := r["gte"]
	gt, hasGt := r["gt"]
	lte, hasLte := r["lte"]
	lt, hasLt := r["lt"]

	// Between
	if (hasGte || hasGt) && (hasLte || hasLt) {
		low := gte
		if hasGt {
			low = gt
		}
		high := lte
		if hasLt {
			high = lt
		}
		return fmt.Sprintf("%s BETWEEN %v AND %v", key, low, high), nil
	}

	// Single comparisons
	if hasGte {
		return fmt.Sprintf("%s >= %v", key, gte), nil
	}
	if hasGt {
		return fmt.Sprintf("%s > %v", key, gt), nil
	}
	if hasLte {
		return fmt.Sprintf("%s <= %v", key, lte), nil
	}
	if hasLt {
		return fmt.Sprintf("%s < %v", key, lt), nil
	}

	return "", nil
}

func convertSearchParams(input interface{}) (string, error) {
	params, ok := input.(map[string]interface{})
	if !ok {
		return "", nil
	}

	var parts []string
	if hnswEf, ok := params["hnsw_ef"].(float64); ok {
		parts = append(parts, fmt.Sprintf("hnsw_ef = %d", int(hnswEf)))
	}
	if exact, ok := params["exact"].(bool); ok {
		parts = append(parts, fmt.Sprintf("exact = %v", exact))
	}
	if indexedOnly, ok := params["indexed_only"].(bool); ok {
		parts = append(parts, fmt.Sprintf("indexed_only = %v", indexedOnly))
	}

	return strings.Join(parts, ", "), nil
}

// --- Helpers ---
