package qql

import (
	"github.com/srimon12/qql-go/internal/convert"
)

// ConvertJSONToQQL converts a Qdrant REST API JSON payload into QQL statements.
// It auto-detects the operation type (search, recommend, upsert, etc.)
// from the JSON structure or endpoint path if wrapped.
func ConvertJSONToQQL(input string) ([]string, error) {
	return convert.JSONToQQL([]byte(input))
}

// ConvertJSONBytesToQQL converts a Qdrant REST API JSON payload into QQL statements.
// This is the highly optimized path that avoids string allocations.
func ConvertJSONBytesToQQL(input []byte) ([]string, error) {
	return convert.JSONToQQL(input)
}
