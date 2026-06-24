package convert

import "strings"

// sanitizeCollectionName strips unsafe characters from a collection name
// to prevent injection into generated QQL statements.
// Only alphanumeric characters, underscores, and hyphens are allowed.
// Returns "unknown" if the result is empty.
func sanitizeCollectionName(name string) string {
	if name == "" {
		return "unknown"
	}
	var out strings.Builder
	out.Grow(len(name))
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			out.WriteRune(c)
		}
	}
	if out.Len() == 0 {
		return "unknown"
	}
	return out.String()
}
