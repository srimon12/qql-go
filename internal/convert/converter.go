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
	var raw struct {
		Query    json.RawMessage `json:"query"`
		Prefetch json.RawMessage `json:"prefetch"`
		Payload  json.RawMessage `json:"payload"`
		Points   json.RawMessage `json:"points"`
		Filter   json.RawMessage `json:"filter"`
		Vector   json.RawMessage `json:"vector"`
		Positive      json.RawMessage `json:"positive"`
		Target        json.RawMessage `json:"target"`
		Ids           json.RawMessage `json:"ids"`
		Vectors       json.RawMessage `json:"vectors"`
		VectorsConfig json.RawMessage `json:"vectors_config"`
		FieldName     json.RawMessage `json:"field_name"`
		Limit         json.RawMessage `json:"limit"`
	}
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Check for query with formula/MMR/relevance_feedback (top-level "query" object)
	if len(raw.Query) > 0 {
		var queryObj struct {
			Formula           json.RawMessage `json:"formula"`
			Nearest           json.RawMessage `json:"nearest"`
			RelevanceFeedback json.RawMessage `json:"relevance_feedback"`
		}
		if json.Unmarshal(raw.Query, &queryObj) == nil {
			if len(queryObj.Formula) > 0 {
				return convertFormulaQuery(input, "collection")
			}
			if len(queryObj.Nearest) > 0 {
				return convertMMRQuery(input, "collection")
			}
			if len(queryObj.RelevanceFeedback) > 0 {
				return convertRelevanceFeedback(input, "collection")
			}
		}
	}

	// Check for top-level prefetch (formula query with prefetch)
	if len(raw.Prefetch) > 0 {
		return convertFormulaQuery(input, "collection")
	}

	// Check for set payload first (has "payload" field with "points" or "filter")
	if len(raw.Payload) > 0 {
		if len(raw.Points) > 0 || len(raw.Filter) > 0 {
			return convertSetPayload(input, "collection")
		}
	}

	// Detect by field presence
	if len(raw.Points) > 0 {
		// Could be upsert or delete
		var probe struct {
			Points []json.RawMessage `json:"points"`
		}
		json.Unmarshal([]byte(input), &probe)
		if len(probe.Points) > 0 {
			var pointProbe struct {
				Vector  json.RawMessage `json:"vector"`
				Payload json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(probe.Points[0], &pointProbe) == nil {
				if len(pointProbe.Vector) > 0 || len(pointProbe.Payload) > 0 {
					return convertUpsert(input, "collection")
				}
			}
			// Points without vectors = delete by IDs
			return convertDeletePoints(input, "collection")
		}
	}

	if len(raw.Vector) > 0 {
		return convertSearch(input, "collection")
	}

	if len(raw.Positive) > 0 {
		return convertRecommend(input, "collection")
	}

	if len(raw.Target) > 0 {
		return convertDiscover(input, "collection")
	}

	if len(raw.Ids) > 0 {
		return convertGetPoints(input, "collection")
	}

	if len(raw.Vectors) > 0 || len(raw.VectorsConfig) > 0 {
		return convertCreateCollection(input, "collection")
	}

	if len(raw.FieldName) > 0 {
		return convertCreateIndex(input, "collection")
	}

	if len(raw.Filter) > 0 {
		// Could be scroll or delete by filter
		if len(raw.Limit) > 0 {
			return convertScroll(input, "collection")
		}
		return convertDeleteByFilter(input, "collection")
	}

	return nil, fmt.Errorf("cannot detect operation from JSON structure")
}

// --- Converters ---
