package convert

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSONToQQL converts a Qdrant REST API JSON payload to QQL statements.
// It auto-detects the operation from the JSON structure.
func JSONToQQL(input string) ([]string, error) {
	input = strings.TrimSpace(input)

	// Try to detect if it's a wrapped request with method+path
	var wrapped struct {
		Method  string          `json:"method"`
		Path    string          `json:"path"`
		Body    json.RawMessage `json:"body"`
		Request json.RawMessage `json:"request"`
	}
	if err := json.Unmarshal([]byte(input), &wrapped); err == nil {
		if wrapped.Method != "" && wrapped.Path != "" {
			body := wrapped.Body
			if len(body) == 0 {
				body = wrapped.Request
			}
			return convertByEndpoint(wrapped.Method, wrapped.Path, string(body))
		}
	}

	// Try to detect by JSON structure
	return convertByStructure(input)
}

func convertByEndpoint(method, path, body string) ([]string, error) {
	path = strings.TrimPrefix(path, "/")

	// Parse collection name from path
	collection := extractCollection(path)

	switch {
	// Create collection: PUT /collections/{name}
	case method == "PUT" && strings.HasPrefix(path, "collections/") && !strings.Contains(path, "/points"):
		return convertCreateCollection(body, collection)

	// Delete collection: DELETE /collections/{name}
	case method == "DELETE" && strings.HasPrefix(path, "collections/") && !strings.Contains(path, "/points"):
		return []string{fmt.Sprintf("DROP COLLECTION %s", collection)}, nil

	// Upsert points: PUT /collections/{name}/points
	case method == "PUT" && strings.HasSuffix(path, "/points"):
		return convertUpsert(body, collection)

	// Query (formula/MMR/nearest): POST /collections/{name}/points/query
	case method == "POST" && strings.HasSuffix(path, "/points/query"):
		return convertFormulaQuery(body, collection)

	// Search: POST /collections/{name}/points/search
	case method == "POST" && strings.HasSuffix(path, "/points/search"):
		return convertSearch(body, collection)

	// Recommend: POST /collections/{name}/points/recommend
	case method == "POST" && strings.HasSuffix(path, "/points/recommend"):
		return convertRecommend(body, collection)

	// Discover: POST /collections/{name}/points/discover
	case method == "POST" && strings.HasSuffix(path, "/points/discover"):
		return convertDiscover(body, collection)

	// Scroll: POST /collections/{name}/points/scroll
	case method == "POST" && strings.HasSuffix(path, "/points/scroll"):
		return convertScroll(body, collection)

	// Get points: POST /collections/{name}/points
	case method == "POST" && strings.HasSuffix(path, "/points") && !strings.Contains(path, "/search") && !strings.Contains(path, "/recommend"):
		return convertGetPoints(body, collection)

	// Delete points: POST /collections/{name}/points/delete
	case method == "POST" && strings.HasSuffix(path, "/points/delete"):
		return convertDeletePoints(body, collection)

	// Set payload: POST /collections/{name}/points/payload
	case method == "POST" && strings.HasSuffix(path, "/points/payload"):
		return convertSetPayload(body, collection)

	// Create index: PUT /collections/{name}/index
	case method == "PUT" && strings.HasSuffix(path, "/index"):
		return convertCreateIndex(body, collection)

	default:
		return nil, fmt.Errorf("unsupported endpoint: %s %s", method, path)
	}
}

func extractCollection(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 2 && parts[0] == "collections" {
		return parts[1]
	}
	return "unknown"
}

func convertByStructure(input string) ([]string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Check for query with formula/MMR/relevance_feedback (top-level "query" object)
	if queryRaw, ok := raw["query"]; ok {
		var queryObj map[string]json.RawMessage
		if json.Unmarshal(queryRaw, &queryObj) == nil {
			// Formula query
			if _, hasFormula := queryObj["formula"]; hasFormula {
				return convertFormulaQuery(input, "collection")
			}
			// MMR query
			if _, hasNearest := queryObj["nearest"]; hasNearest {
				return convertMMRQuery(input, "collection")
			}
			// Relevance feedback
			if _, hasRF := queryObj["relevance_feedback"]; hasRF {
				return convertRelevanceFeedback(input, "collection")
			}
		}
	}

	// Check for top-level prefetch (formula query with prefetch)
	if _, ok := raw["prefetch"]; ok {
		return convertFormulaQuery(input, "collection")
	}

	// Check for set payload first (has "payload" field with "points" or "filter")
	if _, ok := raw["payload"]; ok {
		if _, hasPoints := raw["points"]; hasPoints {
			return convertSetPayload(input, "collection")
		}
		if _, hasFilter := raw["filter"]; hasFilter {
			return convertSetPayload(input, "collection")
		}
	}

	// Detect by field presence
	if _, ok := raw["points"]; ok {
		// Could be upsert or delete
		var probe struct {
			Points []json.RawMessage `json:"points"`
		}
		json.Unmarshal([]byte(input), &probe)
		if len(probe.Points) > 0 {
			var pointProbe map[string]json.RawMessage
			if json.Unmarshal(probe.Points[0], &pointProbe) == nil {
				if _, hasVector := pointProbe["vector"]; hasVector {
					return convertUpsert(input, "collection")
				}
				if _, hasPayload := pointProbe["payload"]; hasPayload {
					return convertUpsert(input, "collection")
				}
			}
			// Points without vectors = delete by IDs
			return convertDeletePoints(input, "collection")
		}
	}

	if _, ok := raw["vector"]; ok {
		return convertSearch(input, "collection")
	}

	if _, ok := raw["positive"]; ok {
		return convertRecommend(input, "collection")
	}

	if _, ok := raw["target"]; ok {
		return convertDiscover(input, "collection")
	}

	if _, ok := raw["ids"]; ok {
		return convertGetPoints(input, "collection")
	}

	if _, ok := raw["vectors"]; ok {
		return convertCreateCollection(input, "collection")
	}

	if _, ok := raw["vectors_config"]; ok {
		return convertCreateCollection(input, "collection")
	}

	if _, ok := raw["field_name"]; ok {
		return convertCreateIndex(input, "collection")
	}

	if _, ok := raw["filter"]; ok {
		// Could be scroll or delete by filter
		if _, hasLimit := raw["limit"]; hasLimit {
			return convertScroll(input, "collection")
		}
		return convertDeleteByFilter(input, "collection")
	}

	return nil, fmt.Errorf("cannot detect operation from JSON structure")
}

// --- Converters ---
