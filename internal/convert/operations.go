package convert

import (
	"encoding/json"
	"fmt"
	"strings"
)

func convertUpsert(input, collection string) ([]string, error) {
	var req struct {
		Points []struct {
			ID      interface{}            `json:"id"`
			Vector  interface{}            `json:"vector"`
			Vectors map[string][]float32   `json:"vectors"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"points"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid upsert JSON: %w", err)
	}

	var stmts []string
	for _, point := range req.Points {
		payload := make(map[string]interface{})
		if point.Payload != nil {
			payload = point.Payload
		}
		if point.ID != nil {
			payload["id"] = point.ID
		}

		// Build VALUES dict
		values := buildValuesDict(payload)
		stmts = append(stmts, fmt.Sprintf("INSERT INTO %s VALUES %s", collection, values))
	}

	if len(stmts) == 0 {
		return nil, fmt.Errorf("no points found in upsert payload")
	}
	return stmts, nil
}

func convertSearch(input, collection string) ([]string, error) {
	var req struct {
		Vector         interface{} `json:"vector"`
		Limit          int         `json:"limit"`
		Offset         int         `json:"offset"`
		Filter         interface{} `json:"filter"`
		WithPayload    interface{} `json:"with_payload"`
		WithVector     interface{} `json:"with_vector"`
		ScoreThreshold *float64    `json:"score_threshold"`
		Using          string      `json:"using"`
		Params         interface{} `json:"params"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid search JSON: %w", err)
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("QUERY '<query_text>' FROM %s", collection))

	if req.Limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", req.Limit))
	}
	if req.Offset > 0 {
		parts = append(parts, fmt.Sprintf("OFFSET %d", req.Offset))
	}

	// Using
	switch strings.ToLower(req.Using) {
	case "hybrid":
		parts = append(parts, "USING HYBRID")
	case "sparse":
		parts = append(parts, "USING SPARSE")
	}

	// Filter
	if req.Filter != nil {
		filterStr, err := convertFilter(req.Filter)
		if err == nil && filterStr != "" {
			parts = append(parts, "WHERE "+filterStr)
		}
	}

	// Score threshold
	if req.ScoreThreshold != nil {
		parts = append(parts, fmt.Sprintf("SCORE THRESHOLD %g", *req.ScoreThreshold))
	}

	// Params
	if req.Params != nil {
		paramsStr, err := convertSearchParams(req.Params)
		if err == nil && paramsStr != "" {
			parts = append(parts, "WITH ("+paramsStr+")")
		}
	}

	return []string{strings.Join(parts, " ")}, nil
}

func convertRecommend(input, collection string) ([]string, error) {
	var req struct {
		Positive []interface{} `json:"positive"`
		Negative []interface{} `json:"negative"`
		Limit    int           `json:"limit"`
		Strategy string        `json:"strategy"`
		Filter   interface{}   `json:"filter"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid recommend JSON: %w", err)
	}

	posIDs := formatIDList(req.Positive)
	negIDs := formatIDList(req.Negative)

	withClause := fmt.Sprintf("positive = (%s)", posIDs)
	if len(req.Negative) > 0 {
		withClause += fmt.Sprintf(", negative = (%s)", negIDs)
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("QUERY RECOMMEND WITH (%s) FROM %s", withClause, collection))

	if req.Limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", req.Limit))
	}
	if req.Strategy != "" {
		parts = append(parts, fmt.Sprintf("STRATEGY '%s'", req.Strategy))
	}
	if req.Filter != nil {
		filterStr, err := convertFilter(req.Filter)
		if err == nil && filterStr != "" {
			parts = append(parts, "WHERE "+filterStr)
		}
	}

	return []string{strings.Join(parts, " ")}, nil
}

func convertDiscover(input, collection string) ([]string, error) {
	var req struct {
		Target  interface{} `json:"target"`
		Context []struct {
			Positive interface{} `json:"positive"`
			Negative interface{} `json:"negative"`
		} `json:"context"`
		Limit  int         `json:"limit"`
		Filter interface{} `json:"filter"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid discover JSON: %w", err)
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("QUERY DISCOVER TARGET %s", formatID(req.Target)))

	if len(req.Context) > 0 {
		var pairs []string
		for _, c := range req.Context {
			pairs = append(pairs, fmt.Sprintf("(%s, %s)", formatID(c.Positive), formatID(c.Negative)))
		}
		parts = append(parts, fmt.Sprintf("CONTEXT PAIRS %s", strings.Join(pairs, ", ")))
	}

	parts = append(parts, fmt.Sprintf("FROM %s", collection))
	if req.Limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", req.Limit))
	}
	if req.Filter != nil {
		filterStr, err := convertFilter(req.Filter)
		if err == nil && filterStr != "" {
			parts = append(parts, "WHERE "+filterStr)
		}
	}

	return []string{strings.Join(parts, " ")}, nil
}

func convertScroll(input, collection string) ([]string, error) {
	var req struct {
		Limit  int         `json:"limit"`
		Offset interface{} `json:"offset"`
		Filter interface{} `json:"filter"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid scroll JSON: %w", err)
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("SCROLL FROM %s", collection))

	if req.Filter != nil {
		filterStr, err := convertFilter(req.Filter)
		if err == nil && filterStr != "" {
			parts = append(parts, "WHERE "+filterStr)
		}
	}
	if req.Offset != nil {
		parts = append(parts, fmt.Sprintf("AFTER %s", formatID(req.Offset)))
	}
	if req.Limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", req.Limit))
	}

	return []string{strings.Join(parts, " ")}, nil
}

func convertGetPoints(input, collection string) ([]string, error) {
	var req struct {
		Ids []interface{} `json:"ids"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid get points JSON: %w", err)
	}

	var stmts []string
	for _, id := range req.Ids {
		stmts = append(stmts, fmt.Sprintf("SELECT * FROM %s WHERE id = %s", collection, formatID(id)))
	}
	return stmts, nil
}

func convertDeletePoints(input, collection string) ([]string, error) {
	var req struct {
		Points []interface{} `json:"points"`
		Filter interface{}   `json:"filter"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid delete JSON: %w", err)
	}

	if req.Filter != nil {
		return convertDeleteByFilter(input, collection)
	}

	var stmts []string
	for _, id := range req.Points {
		stmts = append(stmts, fmt.Sprintf("DELETE FROM %s WHERE id = %s", collection, formatID(id)))
	}
	return stmts, nil
}

func convertDeleteByFilter(input, collection string) ([]string, error) {
	var req struct {
		Filter interface{} `json:"filter"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid delete filter JSON: %w", err)
	}

	filterStr, err := convertFilter(req.Filter)
	if err != nil {
		return nil, err
	}
	return []string{fmt.Sprintf("DELETE FROM %s WHERE %s", collection, filterStr)}, nil
}

func convertSetPayload(input, collection string) ([]string, error) {
	var req struct {
		Payload map[string]interface{} `json:"payload"`
		Points  []interface{}          `json:"points"`
		Filter  interface{}            `json:"filter"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid set payload JSON: %w", err)
	}

	payload := buildValuesDict(req.Payload)

	if req.Filter != nil {
		filterStr, err := convertFilter(req.Filter)
		if err != nil {
			return nil, err
		}
		return []string{fmt.Sprintf("UPDATE %s SET PAYLOAD = %s WHERE %s", collection, payload, filterStr)}, nil
	}

	if len(req.Points) > 0 {
		var stmts []string
		for _, id := range req.Points {
			stmts = append(stmts, fmt.Sprintf("UPDATE %s SET PAYLOAD = %s WHERE id = %s", collection, payload, formatID(id)))
		}
		return stmts, nil
	}

	return nil, fmt.Errorf("set payload requires points or filter")
}

func convertCreateCollection(input, collection string) ([]string, error) {
	var req struct {
		Vectors       interface{} `json:"vectors"`
		VectorsConfig interface{} `json:"vectors_config"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid create collection JSON: %w", err)
	}

	stmt := "CREATE COLLECTION " + collection

	vectors := req.Vectors
	if vectors == nil {
		vectors = req.VectorsConfig
	}

	if vectors != nil {
		switch v := vectors.(type) {
		case map[string]interface{}:
			if size, ok := v["size"]; ok {
				distance := "Cosine"
				if d, ok := v["distance"]; ok {
					distance = fmt.Sprintf("%v", d)
				}
				stmt += fmt.Sprintf(" (dense VECTOR(%v, %s))", size, distance)
			}
		}
	}

	return []string{stmt}, nil
}

func convertCreateIndex(input, collection string) ([]string, error) {
	var req struct {
		FieldName   interface{} `json:"field_name"`
		FieldSchema interface{} `json:"field_schema"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid create index JSON: %w", err)
	}

	field := fmt.Sprintf("%v", req.FieldName)
	schema := "keyword"
	if req.FieldSchema != nil {
		switch s := req.FieldSchema.(type) {
		case string:
			schema = s
		case map[string]interface{}:
			if t, ok := s["type"]; ok {
				schema = fmt.Sprintf("%v", t)
			}
		}
	}

	return []string{fmt.Sprintf("CREATE INDEX ON %s FOR %s TYPE %s", collection, field, schema)}, nil
}

// --- Formula / MMR / Relevance Feedback ---
