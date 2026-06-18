package convert

import (
	"fmt"
	"strings"
)

func buildValuesDict(payload map[string]any) string {
	if len(payload) == 0 {
		return "{}"
	}

	var entries []string
	for k, v := range payload {
		entries = append(entries, fmt.Sprintf("'%s': %s", k, formatValue(v)))
	}

	return "{" + strings.Join(entries, ", ") + "}"
}

func formatID(id any) string {
	switch v := id.(type) {
	case string:
		return fmt.Sprintf("'%s'", v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("'%v'", v)
	}
}

func formatIDList(ids []any) string {
	var parts []string
	for _, id := range ids {
		parts = append(parts, formatID(id))
	}
	return strings.Join(parts, ", ")
}

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "\\'"))
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case uint64:
		return fmt.Sprintf("%d", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	case []any:
		var items []string
		for _, item := range val {
			items = append(items, formatValue(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case map[string]any:
		var entries []string
		for k, item := range val {
			entries = append(entries, fmt.Sprintf("'%s': %s", k, formatValue(item)))
		}
		return "{" + strings.Join(entries, ", ") + "}"
	default:
		return fmt.Sprintf("'%v'", val)
	}
}
