package convert

import (
	"fmt"
	"strings"

	"github.com/srimon12/qql-go/internal/ast"
)

func convertFilter(f *RESTFilter) (string, error) {
	if f == nil {
		return "", nil
	}

	astFilter := convertRESTFilter(f)
	if astFilter == nil {
		return "", nil
	}

	return ast.FormatFilterExpr(astFilter), nil
}

func convertSearchParams(input any) (string, error) {
	params, ok := input.(map[string]any)
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
