package convert

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/srimon12/qql-go/internal/ast"
)

func convertUpsert(input []byte, collection string) ([]string, error) {
	var req struct {
		Points []struct {
			ID      any                  `json:"id"`
			Vector  any                  `json:"vector"`
			Vectors map[string][]float32 `json:"vectors"`
			Payload map[string]any       `json:"payload"`
		} `json:"points"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
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

		// Include vector data if present
		if point.Vector != nil {
			payload["vector"] = point.Vector
		} else if len(point.Vectors) > 0 {
			payload["vector"] = point.Vectors
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

func convertSearch(input []byte, collection string) ([]string, error) {
	var req struct {
		Vector         any         `json:"vector"`
		QueryRaw       any         `json:"query"`
		Limit          int         `json:"limit"`
		Offset         int         `json:"offset"`
		Filter         *RESTFilter `json:"filter"`
		WithPayload    any         `json:"with_payload"`
		WithVector     any         `json:"with_vector"`
		WithVectors    any         `json:"with_vectors"`
		ScoreThreshold *float64    `json:"score_threshold"`
		Using          string      `json:"using"`
		Params         any         `json:"params"`
		GroupBy        string      `json:"group_by"`
		GroupSize      int         `json:"group_size"`
		WithLookup     any         `json:"with_lookup"`
		LookupFrom     any         `json:"lookup_from"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("invalid search JSON: %w", err)
	}

	stmt := &ast.QueryStmt{
		Collection: collection,
		Mode:       ast.QueryModeNearest,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}

	// Query handling: check raw query (ID string, vector array, object with text/model/indices)
	if qm, ok := req.QueryRaw.(map[string]any); ok {
		if t, ok := qm["text"]; ok {
			if s, ok := t.(string); ok {
				stmt.QueryText = &s
			}
		}
		if m, ok := qm["model"]; ok {
			if s, ok := m.(string); ok {
				stmt.Model = &s
			}
		}
		if _, ok := qm["sample"]; ok {
			stmt.Mode = ast.QueryModeSample
		}
		if _, ok := qm["indices"]; ok {
			str := "<sparse_query>"
			stmt.QueryText = &str
			stmt.Type = ast.QueryTypeSparse
		}
	} else if s, ok := req.QueryRaw.(string); ok && s != "" {
		stmt.QueryID = s
	} else if vec, ok := req.QueryRaw.([]any); ok && len(vec) > 0 {
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
	} else if vec, ok := req.Vector.([]any); ok && len(vec) > 0 {
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

	// Group by / group size
	if req.GroupBy != "" {
		stmt.GroupBy = &req.GroupBy
		if req.GroupSize > 0 {
			stmt.GroupSize = &req.GroupSize
		}
	}

	// WithLookup
	if req.WithLookup != nil {
		if lookupMap, ok := req.WithLookup.(map[string]any); ok {
			if coll, ok := lookupMap["collection"]; ok {
				if s, ok := coll.(string); ok {
					stmt.WithLookupCollection = &s
				}
			}
		}
	}

	// Lookup from
	if req.LookupFrom != nil {
		if lookupMap, ok := req.LookupFrom.(map[string]any); ok {
			if coll, ok := lookupMap["collection"]; ok {
				if s, ok := coll.(string); ok {
					stmt.LookupFrom = s
				}
			}
			if vec, ok := lookupMap["vector"]; ok {
				if s, ok := vec.(string); ok {
					stmt.LookupVector = &s
				}
			}
		}
	}

	// WithPayload and WithVectors (prefer newer "with_vectors" over "with_vector")
	wp := req.WithPayload
	if wp == nil {
		wp = true
	} // default
	stmt.WithPayload = buildPayloadSelector(wp)
	wv := req.WithVector
	if wv == nil {
		wv = req.WithVectors
	}
	stmt.WithVectors = buildVectorsSelector(wv)

	return []string{ast.FormatQueryStmt(stmt)}, nil
}

func convertRecommend(input []byte, collection string) ([]string, error) {
	var req struct {
		Query struct {
			Recommend struct {
				Positive []any  `json:"positive"`
				Negative []any  `json:"negative"`
				Strategy string `json:"strategy"`
			} `json:"recommend"`
		} `json:"query"`
		Positive   []any       `json:"positive"`
		Negative   []any       `json:"negative"`
		Limit      int         `json:"limit"`
		Strategy   string      `json:"strategy"`
		Filter     *RESTFilter `json:"filter"`
		Using      string      `json:"using"`
		LookupFrom any         `json:"lookup_from"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("invalid recommend JSON: %w", err)
	}

	positive := req.Positive
	negative := req.Negative
	strategy := req.Strategy
	if len(req.Query.Recommend.Positive) > 0 || len(req.Query.Recommend.Negative) > 0 {
		positive = req.Query.Recommend.Positive
		negative = req.Query.Recommend.Negative
		if req.Query.Recommend.Strategy != "" {
			strategy = req.Query.Recommend.Strategy
		}
	}

	posIDs := formatIDList(positive)
	negIDs := formatIDList(negative)

	withClause := fmt.Sprintf("positive = (%s)", posIDs)
	if len(negative) > 0 {
		withClause += fmt.Sprintf(", negative = (%s)", negIDs)
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("QUERY RECOMMEND WITH (%s) FROM %s", withClause, collection))

	if req.Limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", req.Limit))
	}
	if strategy != "" {
		parts = append(parts, fmt.Sprintf("STRATEGY '%s'", strategy))
	}
	if req.Using != "" {
		parts = append(parts, fmt.Sprintf("USING '%s'", req.Using))
	}
	if req.LookupFrom != nil {
		if lookupMap, ok := req.LookupFrom.(map[string]any); ok {
			if coll, ok := lookupMap["collection"]; ok {
				if cn, ok := coll.(string); ok {
					if vec, ok := lookupMap["vector"]; ok {
						if vn, ok := vec.(string); ok {
							parts = append(parts, fmt.Sprintf("LOOKUP FROM %s VECTOR '%s'", cn, vn))
						}
					} else {
						parts = append(parts, fmt.Sprintf("LOOKUP FROM %s", cn))
					}
				}
			}
		}
	}
	if req.Filter != nil {
		filterStr, err := convertFilter(req.Filter)
		if err == nil && filterStr != "" {
			parts = append(parts, "WHERE "+filterStr)
		}
	}

	return []string{strings.Join(parts, " ")}, nil
}

func convertDiscover(input []byte, collection string) ([]string, error) {
	var req struct {
		Query struct {
			Discover struct {
				Target  any `json:"target"`
				Context []struct {
					Positive any `json:"positive"`
					Negative any `json:"negative"`
				} `json:"context"`
			} `json:"discover"`
			Context []struct {
				Positive any `json:"positive"`
				Negative any `json:"negative"`
			} `json:"context"`
		} `json:"query"`
		Target  any `json:"target"`
		Context []struct {
			Positive any `json:"positive"`
			Negative any `json:"negative"`
		} `json:"context"`
		Limit  int         `json:"limit"`
		Filter *RESTFilter `json:"filter"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("invalid discover JSON: %w", err)
	}

	target := req.Target
	ctxPairs := req.Context
	if req.Query.Discover.Target != nil || len(req.Query.Discover.Context) > 0 {
		target = req.Query.Discover.Target
		ctxPairs = req.Query.Discover.Context
	} else if len(req.Query.Context) > 0 {
		target = nil
		ctxPairs = req.Query.Context
	}

	var parts []string
	if target != nil {
		parts = append(parts, fmt.Sprintf("QUERY DISCOVER TARGET %s", formatID(target)))
	} else {
		parts = append(parts, "QUERY CONTEXT")
	}
	if len(ctxPairs) > 0 {
		var pairs []string
		for _, c := range ctxPairs {
			pairs = append(pairs, fmt.Sprintf("(%s, %s)", formatID(c.Positive), formatID(c.Negative)))
		}
		if target != nil {
			parts = append(parts, fmt.Sprintf("CONTEXT PAIRS %s", strings.Join(pairs, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("PAIRS %s", strings.Join(pairs, ", ")))
		}
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

func convertScroll(input []byte, collection string) ([]string, error) {
	var req struct {
		Limit  int         `json:"limit"`
		Offset any         `json:"offset"`
		Filter *RESTFilter `json:"filter"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
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

func convertGetPoints(input []byte, collection string) ([]string, error) {
	var req struct {
		Ids []any `json:"ids"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("invalid get points JSON: %w", err)
	}

	var stmts []string
	for _, id := range req.Ids {
		stmts = append(stmts, fmt.Sprintf("SELECT * FROM %s WHERE id = %s", collection, formatID(id)))
	}
	return stmts, nil
}

func convertDeletePoints(input []byte, collection string) ([]string, error) {
	var req struct {
		Points []any       `json:"points"`
		Filter *RESTFilter `json:"filter"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
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

func convertDeleteByFilter(input []byte, collection string) ([]string, error) {
	var req struct {
		Filter *RESTFilter `json:"filter"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("invalid delete filter JSON: %w", err)
	}

	filterStr, err := convertFilter(req.Filter)
	if err != nil {
		return nil, err
	}
	return []string{fmt.Sprintf("DELETE FROM %s WHERE %s", collection, filterStr)}, nil
}

func convertSetPayload(input []byte, collection string) ([]string, error) {
	var req struct {
		Payload map[string]any `json:"payload"`
		Points  []any          `json:"points"`
		Filter  *RESTFilter    `json:"filter"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
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

func convertCreateCollection(input []byte, collection string) ([]string, error) {
	var req struct {
		Vectors       any `json:"vectors"`
		VectorsConfig any `json:"vectors_config"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
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
			var vecDefs []string
			if _, hasSize := v["size"]; hasSize {
				// Single vector
				vecDefs = append(vecDefs, buildVectorDef("dense", v))
			} else {
				// Named vectors
				// Sort keys for deterministic output
				var names []string
				for name := range v {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					if vecObj, ok := v[name].(map[string]any); ok {
						vecDefs = append(vecDefs, buildVectorDef(name, vecObj))
					}
				}
			}
			if len(vecDefs) > 0 {
				stmt += " (\n    " + strings.Join(vecDefs, ",\n    ") + "\n)"
			}
		}
	}

	return []string{stmt}, nil
}

func buildVectorDef(name string, v map[string]any) string {
	size := v["size"]
	distance := "Cosine"
	if d, ok := v["distance"]; ok {
		distance = fmt.Sprintf("%v", d)
	}
	def := fmt.Sprintf("'%s' VECTOR(%v, %s)", name, size, distance)

	if mvc, ok := v["multivector_config"].(map[string]any); ok {
		if comp, ok := mvc["comparator"]; ok {
			def += fmt.Sprintf(" WITH MULTIVECTOR (comparator = '%v')", comp)
		}
	}

	if hnsw, ok := v["hnsw_config"].(map[string]any); ok {
		if m, ok := hnsw["m"]; ok {
			def += fmt.Sprintf(" WITH HNSW (m = %v)", m)
		}
	}

	return def
}

func convertCreateIndex(input []byte, collection string) ([]string, error) {
	var req struct {
		FieldName   any `json:"field_name"`
		FieldSchema any `json:"field_schema"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
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

	return []string{fmt.Sprintf("CREATE INDEX ON COLLECTION %s FOR %s TYPE %s", collection, field, schema)}, nil
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

func convertBatchSearch(input []byte) ([]string, error) {
	var req struct {
		Searches []json.RawMessage `json:"searches"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("invalid batch search JSON: %w", err)
	}
	if len(req.Searches) == 0 {
		return nil, fmt.Errorf("batch search requires at least one search")
	}
	var stmts []string
	for _, search := range req.Searches {
		subStmts, err := JSONToQQL(search)
		if err != nil {
			return nil, fmt.Errorf("batch search entry: %w", err)
		}
		stmts = append(stmts, subStmts...)
	}
	return stmts, nil
}
