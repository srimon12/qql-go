package convert

import (
	"fmt"
	"strconv"
	"strings"
)

func buildValuesDict(payload map[string]any) string {
	if len(payload) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.Grow(256)
	formatMapBuilder(&b, payload)
	return b.String()
}

func formatMapBuilder(b *strings.Builder, val map[string]any) {
	b.WriteByte('{')
	first := true
	for k, item := range val {
		if !first {
			b.WriteString(", ")
		}
		b.WriteByte('\'')
		b.WriteString(k)
		b.WriteString("': ")
		formatValueBuilder(b, item)
		first = false
	}
	b.WriteByte('}')
}

func formatID(id any) string {
	switch v := id.(type) {
	case string:
		return "'" + strings.ReplaceAll(v, "'", "\\'") + "'"
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("'%v'", v)
	}
}

func formatIDList(ids []any) string {
	var b strings.Builder
	for i, id := range ids {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(formatID(id))
	}
	return b.String()
}

func formatValue(v any) string {
	var b strings.Builder
	formatValueBuilder(&b, v)
	return b.String()
}

func formatValueBuilder(b *strings.Builder, v any) {
	switch val := v.(type) {
	case string:
		b.WriteByte('\'')
		b.WriteString(strings.ReplaceAll(val, "'", "\\'"))
		b.WriteByte('\'')
	case float64:
		if val == float64(int64(val)) {
			b.WriteString(strconv.FormatInt(int64(val), 10))
		} else {
			b.WriteString(strconv.FormatFloat(val, 'f', -1, 64))
		}
	case float32:
		b.WriteString(strconv.FormatFloat(float64(val), 'f', -1, 32))
	case int:
		b.WriteString(strconv.Itoa(val))
	case int64:
		b.WriteString(strconv.FormatInt(val, 10))
	case uint64:
		b.WriteString(strconv.FormatUint(val, 10))
	case bool:
		if val {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case nil:
		b.WriteString("null")
	case []any:
		b.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				b.WriteString(", ")
			}
			formatValueBuilder(b, item)
		}
		b.WriteByte(']')
	case []float32:
		b.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(strconv.FormatFloat(float64(item), 'f', -1, 32))
		}
		b.WriteByte(']')
	case [][]float32:
		b.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				b.WriteString(", ")
			}
			formatValueBuilder(b, item)
		}
		b.WriteByte(']')
	case map[string]any:
		formatMapBuilder(b, val)
	default:
		b.WriteByte('\'')
		b.WriteString(fmt.Sprintf("%v", val))
		b.WriteByte('\'')
	}
}
