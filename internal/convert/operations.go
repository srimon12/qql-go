package convert

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/srimon12/qql-go/internal/ast"
)

func convertUpsert(input, collection string) ([]string, error) {
	var req struct {
		Points []struct {
			ID      any                  `json:"id"`
			Vector  any                  `json:"vector"`
			Vectors map[string][]float32 `json:"vectors"`
			Payload map[string]any       `json:"payload"`
		} `json:"points"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid upsert JSON: %w", err)
	}

	var stmts []string
	for _, point := range req.Points {
		payload := make(map[string]any)
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
		Vector         any         `json:"vector"`
		Limit          int         `json:"limit"`
		Offset         int         `json:"offset"`
		Filter         *RESTFilter `json:"filter"`
		WithPayload    any         `json:"with_payload"`
		WithVector     any         `json:"with_vector"`
		ScoreThreshold *float64    `json:"score_threshold"`
		Using          string      `json:"using"`
		Params         any         `json:"params"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid search JSON: %w", err)
	}

	stmt := &ast.QueryStmt{
		Collection: collection,
		Mode:       ast.QueryModeNearest,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}

	// Vector
	if vec, ok := req.Vector.([]any); ok && len(vec) > 0 {
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
	} else if req.Vector != nil {
		str := "<query_text>"
		stmt.QueryText = &str
	}

	// Using
	switch strings.ToLower(req.Using) {
	case "hybrid":
		stmt.Type = ast.QueryTypeHybrid
	case "sparse":
		stmt.Type = ast.QueryTypeSparse
	default:
		if req.Using != "" {
			using := req.Using
			stmt.Using = &using
		}
	}

	// Filter
	if req.Filter != nil {
		stmt.QueryFilter = convertRESTFilter(req.Filter)
	}

	// Score threshold
	if req.ScoreThreshold != nil {
		stmt.ScoreThreshold = req.ScoreThreshold
	}

	// Params
	if req.Params != nil {
		stmt.WithClause = buildWithClause(req.Params)
	}

	// WithPayload and WithVectors
	stmt.WithPayload = buildPayloadSelector(req.WithPayload)
	stmt.WithVectors = buildVectorsSelector(req.WithVector)

	return []string{ast.FormatQueryStmt(stmt)}, nil
}

func convertRecommend(input, collection string) ([]string, error) {
	var req struct {
		Positive []any       `json:"positive"`
		Negative []any       `json:"negative"`
		Limit    int         `json:"limit"`
		Strategy string      `json:"strategy"`
		Filter   *RESTFilter `json:"filter"`
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
		Target  any `json:"target"`
		Context []struct {
			Positive any `json:"positive"`
			Negative any `json:"negative"`
		} `json:"context"`
		Limit  int         `json:"limit"`
		Filter *RESTFilter `json:"filter"`
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
		Offset any         `json:"offset"`
		Filter *RESTFilter `json:"filter"`
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
		Ids []any `json:"ids"`
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
		Points []any       `json:"points"`
		Filter *RESTFilter `json:"filter"`
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
		Filter *RESTFilter `json:"filter"`
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
		Payload map[string]any `json:"payload"`
		Points  []any          `json:"points"`
		Filter  *RESTFilter    `json:"filter"`
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
		Vectors       any `json:"vectors"`
		VectorsConfig any `json:"vectors_config"`
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
		case map[string]any:
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
		FieldName   any `json:"field_name"`
		FieldSchema any `json:"field_schema"`
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
		case map[string]any:
			if t, ok := s["type"]; ok {
				schema = fmt.Sprintf("%v", t)
			}
		}
	}

	return []string{fmt.Sprintf("CREATE INDEX ON %s FOR %s TYPE %s", collection, field, schema)}, nil
}

// --- Formula / MMR / Relevance Feedback ---

func buildPayloadSelector(input any) *ast.PayloadSelector {
	if input == nil {
		return nil
	}
	if b, ok := input.(bool); ok {
		return &ast.PayloadSelector{Enable: &b}
	}
	if m, ok := input.(map[string]any); ok {
		sel := &ast.PayloadSelector{}
		if inc, ok := m["include"].([]any); ok {
			for _, v := range inc {
				if s, ok := v.(string); ok {
					sel.Include = append(sel.Include, s)
				}
			}
		}
		if exc, ok := m["exclude"].([]any); ok {
			for _, v := range exc {
				if s, ok := v.(string); ok {
					sel.Exclude = append(sel.Exclude, s)
				}
			}
		}
		return sel
	}
	return nil
}

func buildVectorsSelector(input any) *ast.VectorsSelector {
	if input == nil {
		return nil
	}
	if b, ok := input.(bool); ok {
		return &ast.VectorsSelector{Enable: &b}
	}
	if arr, ok := input.([]any); ok {
		sel := &ast.VectorsSelector{}
		for _, v := range arr {
			if s, ok := v.(string); ok {
				sel.Vectors = append(sel.Vectors, s)
			}
		}
		return sel
	}
	return nil
}

func buildWithClause(input any) *ast.SearchWith {
	if input == nil {
		return nil
	}
	if m, ok := input.(map[string]any); ok {
		w := &ast.SearchWith{}
		if exact, ok := m["exact"].(bool); ok {
			w.Exact = exact
		}
		if hnsw, ok := m["hnsw_ef"].(float64); ok {
			w.HnswEf = int(hnsw)
		}
		if idx, ok := m["indexed_only"].(bool); ok {
			w.IndexedOnly = idx
		}
		return w
	}
	return nil
}
