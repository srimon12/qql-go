package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/srimon12/qql-go/internal/output"
)

func NewConvertCmd(out *output.Outputter) *cobra.Command {
	var (
		filePath string
		validate bool
		quiet    bool
		jsonOut  bool
	)

	cmd := &cobra.Command{
		Use:   "convert [file]",
		Short: "Convert Qdrant REST API JSON to QQL",
		Long: `Convert Qdrant REST API JSON payloads to native QQL statements.

Accepts JSON from stdin, a file path, or as a direct argument.
Auto-detects the operation type from the JSON structure and outputs
equivalent QQL statements that can be piped to qql-go execute.

Supported operations:
  - Upsert points      → INSERT INTO
  - Search             → QUERY
  - Recommend          → QUERY RECOMMEND
  - Discover           → QUERY DISCOVER
  - Scroll             → SCROLL FROM
  - Get points         → SELECT * FROM ... WHERE id IN
  - Delete points      → DELETE FROM
  - Create collection  → CREATE COLLECTION
  - Create index       → CREATE INDEX

Examples:
  qql-go convert payload.json
  qql-go convert --validate payload.json
  cat payload.json | qql-go convert
  echo '{"points":[{"id":1,"payload":{"text":"hi"}}]}' | qql-go convert`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var input []byte
			var err error

			if len(args) > 0 {
				filePath = args[0]
			}

			if filePath != "" {
				input, err = os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("cannot read file: %w", err)
				}
			} else {
				input, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("cannot read stdin: %w", err)
				}
			}

			input = []byte(strings.TrimSpace(string(input)))
			if len(input) == 0 {
				return fmt.Errorf("no input provided")
			}

			statements, err := ConvertJSONToQQL(string(input))
			if err != nil {
				return fmt.Errorf("conversion error: %w", err)
			}

			if validate {
				exec := NewExecutor(nil, nil)
				for i, stmt := range statements {
					_, err := exec.Explain(stmt)
					if err != nil {
						return fmt.Errorf("statement %d failed validation: %w\n%s", i+1, err, stmt)
					}
				}
			}

			output := strings.Join(statements, "\n\n")
			if quiet {
				out.Print(output)
			} else if jsonOut {
				out.PrintJSON(map[string]any{
					"ok":         true,
					"statements": statements,
					"count":      len(statements),
				}, true)
			} else {
				for i, stmt := range statements {
					if i > 0 {
						out.Print("")
					}
					out.Print(stmt)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&validate, "validate", false, "Validate generated QQL with explain")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Output only the QQL statements")
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "Output as JSON")

	return cmd
}

// ConvertJSONToQQL converts a Qdrant REST API JSON payload to QQL statements.
// It auto-detects the operation from the JSON structure.
func ConvertJSONToQQL(input string) ([]string, error) {
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

func convertFormulaQuery(input, collection string) ([]string, error) {
	var req struct {
		Prefetch interface{} `json:"prefetch"`
		Query    struct {
			Formula  interface{} `json:"formula"`
			Nearest  interface{} `json:"nearest"`
			Document interface{} `json:"document"`
		} `json:"query"`
		Limit    int         `json:"limit"`
		Filter   interface{} `json:"filter"`
		Defaults interface{} `json:"defaults"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid formula query JSON: %w", err)
	}

	var stmts []string
	var cteNames []string

	// Handle prefetches (single object or array)
	if req.Prefetch != nil {
		prefetches := normalizePrefetchArray(req.Prefetch)
		for i, pf := range prefetches {
			cteName := fmt.Sprintf("_pf%d", i)
			cteNames = append(cteNames, cteName)
			cteQQL, err := convertSinglePrefetch(pf, cteName, collection)
			if err == nil {
				stmts = append(stmts, cteQQL)
			}
		}
	}

	// Build main query
	var parts []string
	parts = append(parts, fmt.Sprintf("QUERY '<query_text>' FROM %s", collection))

	if req.Limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", req.Limit))
	}

	// Convert formula expression
	if req.Query.Formula != nil {
		formulaStr, err := convertFormulaExpression(req.Query.Formula)
		if err == nil && formulaStr != "" {
			parts = append(parts, fmt.Sprintf("BOOST (%s)", formulaStr))
		}
	}

	// Convert defaults
	if req.Defaults != nil {
		if defs, ok := req.Defaults.(map[string]interface{}); ok && len(defs) > 0 {
			var entries []string
			for k, v := range defs {
				entries = append(entries, fmt.Sprintf("%s = %s", k, formatFormulaValue(v)))
			}
			parts = append(parts, fmt.Sprintf("DEFAULTS (%s)", strings.Join(entries, ", ")))
		}
	}

	mainQQL := strings.Join(parts, " ")

	// If we have CTEs, build WITH ... QUERY ... PREFETCH (...) syntax
	if len(cteNames) > 0 {
		var cteDefs []string
		for i, name := range cteNames {
			cteDefs = append(cteDefs, fmt.Sprintf("%s AS (%s)", name, stmts[i]))
		}
		withClause := "WITH " + strings.Join(cteDefs, ", ")
		prefetchClause := fmt.Sprintf("PREFETCH (%s)", strings.Join(cteNames, ", "))
		return []string{fmt.Sprintf("%s %s %s", withClause, mainQQL, prefetchClause)}, nil
	}

	return []string{mainQQL}, nil
}

func normalizePrefetchArray(input interface{}) []interface{} {
	switch pf := input.(type) {
	case []interface{}:
		return pf
	case map[string]interface{}:
		return []interface{}{pf}
	default:
		return nil
	}
}

func convertSinglePrefetch(pf interface{}, _, _ string) (string, error) {
	pfMap, ok := pf.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid prefetch object")
	}

	// CTE bodies don't support FROM — use simplified QUERY syntax
	var parts []string
	parts = append(parts, "QUERY '<query_text>'")

	if limit, ok := pfMap["limit"].(float64); ok && limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", int(limit)))
	}

	if pfMap["filter"] != nil {
		filterStr, err := convertFilter(pfMap["filter"])
		if err == nil && filterStr != "" {
			parts = append(parts, "WHERE "+filterStr)
		}
	}

	return strings.Join(parts, " "), nil
}

func convertFormulaExpression(input interface{}) (string, error) {
	switch expr := input.(type) {
	case string:
		// Variable reference: "$score", "field_name"
		return expr, nil
	case float64:
		return fmt.Sprintf("%v", expr), nil
	case map[string]interface{}:
		return convertFormulaObject(expr)
	default:
		return fmt.Sprintf("%v", input), nil
	}
}

func convertFormulaObject(expr map[string]interface{}) (string, error) {
	// Sum
	if sum, ok := expr["sum"]; ok {
		return convertNaryOp(" + ", sum)
	}
	// Mult
	if mult, ok := expr["mult"]; ok {
		return convertNaryOp(" * ", mult)
	}
	// Div
	if div, ok := expr["div"]; ok {
		if arr, ok := div.([]interface{}); ok && len(arr) == 2 {
			left, _ := convertFormulaExpression(arr[0])
			right, _ := convertFormulaExpression(arr[1])
			return fmt.Sprintf("(%s / %s)", left, right), nil
		}
	}
	// Abs
	if abs, ok := expr["abs"]; ok {
		inner, _ := convertFormulaExpression(abs)
		return fmt.Sprintf("ABS(%s)", inner), nil
	}
	// Sqrt
	if sqrt, ok := expr["sqrt"]; ok {
		inner, _ := convertFormulaExpression(sqrt)
		return fmt.Sprintf("SQRT(%s)", inner), nil
	}
	// Pow
	if pow, ok := expr["pow"]; ok {
		if arr, ok := pow.([]interface{}); ok && len(arr) == 2 {
			base, _ := convertFormulaExpression(arr[0])
			exp, _ := convertFormulaExpression(arr[1])
			return fmt.Sprintf("POW(%s, %s)", base, exp), nil
		}
	}
	// Log10
	if log, ok := expr["log10"]; ok {
		inner, _ := convertFormulaExpression(log)
		return fmt.Sprintf("LOG(%s)", inner), nil
	}
	// Ln
	if ln, ok := expr["ln"]; ok {
		inner, _ := convertFormulaExpression(ln)
		return fmt.Sprintf("LN(%s)", inner), nil
	}
	// Exp
	if exp, ok := expr["exp"]; ok {
		inner, _ := convertFormulaExpression(exp)
		return fmt.Sprintf("EXP(%s)", inner), nil
	}
	// Condition (match/key)
	if key, ok := expr["key"]; ok {
		if match, ok := expr["match"]; ok {
			matchMap, ok := match.(map[string]interface{})
			if ok {
				if any, ok := matchMap["any"]; ok {
					return convertMatchAnyCondition(fmt.Sprintf("%v", key), any), nil
				}
				if value, ok := matchMap["value"]; ok {
					return fmt.Sprintf("%s = %s", key, formatValue(value)), nil
				}
				if keyword, ok := matchMap["keyword"]; ok {
					return fmt.Sprintf("%s = %s", key, formatValue(keyword)), nil
				}
			}
		}
	}
	// Geo distance
	if geoDist, ok := expr["geo_distance"]; ok {
		return convertGeoDistanceExpr(geoDist)
	}
	// Gauss decay
	if decay, ok := expr["gauss_decay"]; ok {
		return convertDecayExpr("GAUSS_DECAY", decay)
	}
	// Exp decay
	if decay, ok := expr["exp_decay"]; ok {
		return convertDecayExpr("EXP_DECAY", decay)
	}
	// Lin decay
	if decay, ok := expr["lin_decay"]; ok {
		return convertDecayExpr("LIN_DECAY", decay)
	}
	// Datetime
	if dt, ok := expr["datetime"]; ok {
		return fmt.Sprintf("datetime('%v')", dt), nil
	}
	// Datetime key
	if dk, ok := expr["datetime_key"]; ok {
		return fmt.Sprintf("datetime_key('%v')", dk), nil
	}

	return fmt.Sprintf("%v", expr), nil
}

func convertNaryOp(op string, input interface{}) (string, error) {
	arr, ok := input.([]interface{})
	if !ok {
		return "", fmt.Errorf("expected array")
	}
	var parts []string
	for _, item := range arr {
		s, _ := convertFormulaExpression(item)
		parts = append(parts, s)
	}
	return "(" + strings.Join(parts, op) + ")", nil
}

func convertMatchAnyCondition(key string, any interface{}) string {
	values, ok := any.([]interface{})
	if !ok {
		return fmt.Sprintf("%s = %v", key, any)
	}
	var vals []string
	for _, v := range values {
		vals = append(vals, formatValue(v))
	}
	return fmt.Sprintf("%s IN (%s)", key, strings.Join(vals, ", "))
}

func convertGeoDistanceExpr(input interface{}) (string, error) {
	geoMap, ok := input.(map[string]interface{})
	if !ok {
		return "GEO_DISTANCE(...)", nil
	}
	origin, _ := geoMap["origin"].(map[string]interface{})
	to, _ := geoMap["to"].(string)
	if origin != nil {
		lat, _ := origin["lat"].(float64)
		lon, _ := origin["lon"].(float64)
		return fmt.Sprintf("GEO_DISTANCE({lat: %v, lon: %v}, %s)", lat, lon, to), nil
	}
	return fmt.Sprintf("GEO_DISTANCE(origin, %s)", to), nil
}

func convertDecayExpr(name string, input interface{}) (string, error) {
	decayMap, ok := input.(map[string]interface{})
	if !ok {
		return fmt.Sprintf("%s(...)", name), nil
	}

	var args []string

	// x parameter
	if x, ok := decayMap["x"]; ok {
		xStr, _ := convertFormulaExpression(x)
		args = append(args, xStr)
	}

	// target
	if target, ok := decayMap["target"]; ok {
		targetStr, _ := convertFormulaExpression(target)
		args = append(args, fmt.Sprintf("target: %s", targetStr))
	}

	// scale
	if scale, ok := decayMap["scale"]; ok {
		args = append(args, fmt.Sprintf("scale: %v", scale))
	}

	// midpoint
	if midpoint, ok := decayMap["midpoint"]; ok {
		args = append(args, fmt.Sprintf("midpoint: %v", midpoint))
	}

	return fmt.Sprintf("%s(%s)", name, strings.Join(args, ", ")), nil
}

func convertMMRQuery(input, collection string) ([]string, error) {
	var req struct {
		Query struct {
			Nearest interface{} `json:"nearest"`
			MMR     struct {
				Diversity       float64 `json:"diversity"`
				CandidatesLimit int     `json:"candidates_limit"`
			} `json:"mmr"`
		} `json:"query"`
		Limit  int         `json:"limit"`
		Filter interface{} `json:"filter"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid MMR query JSON: %w", err)
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("QUERY '<query_text>' FROM %s", collection))

	if req.Limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", req.Limit))
	}

	mmrParams := []string{
		fmt.Sprintf("mmr_diversity = %g", req.Query.MMR.Diversity),
	}
	if req.Query.MMR.CandidatesLimit > 0 {
		mmrParams = append(mmrParams, fmt.Sprintf("mmr_candidates = %d", req.Query.MMR.CandidatesLimit))
	}
	parts = append(parts, fmt.Sprintf("WITH (%s)", strings.Join(mmrParams, ", ")))

	if req.Filter != nil {
		filterStr, err := convertFilter(req.Filter)
		if err == nil && filterStr != "" {
			parts = append(parts, "WHERE "+filterStr)
		}
	}

	return []string{strings.Join(parts, " ")}, nil
}

func convertRelevanceFeedback(input, collection string) ([]string, error) {
	var req struct {
		Query struct {
			RelevanceFeedback struct {
				Target   interface{} `json:"target"`
				Feedback []struct {
					Example interface{} `json:"example"`
					Score   float64     `json:"score"`
				} `json:"feedback"`
				Strategy interface{} `json:"strategy"`
			} `json:"relevance_feedback"`
		} `json:"query"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return nil, fmt.Errorf("invalid relevance feedback JSON: %w", err)
	}

	rf := req.Query.RelevanceFeedback

	var feedbackItems []string
	for _, f := range rf.Feedback {
		feedbackItems = append(feedbackItems, fmt.Sprintf("(%s, %g)", formatID(f.Example), f.Score))
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("QUERY RELEVANCE FEEDBACK TARGET %s", formatFormulaValue(rf.Target)))
	if len(feedbackItems) > 0 {
		parts = append(parts, fmt.Sprintf("FEEDBACK (%s)", strings.Join(feedbackItems, ", ")))
	}
	parts = append(parts, fmt.Sprintf("FROM %s", collection))

	// Strategy
	if rf.Strategy != nil {
		if stratMap, ok := rf.Strategy.(map[string]interface{}); ok {
			if naive, ok := stratMap["naive"].(map[string]interface{}); ok {
				a, _ := naive["a"].(float64)
				b, _ := naive["b"].(float64)
				c, _ := naive["c"].(float64)
				parts = append(parts, fmt.Sprintf("STRATEGY NAIVE (a = %g, b = %g, c = %g)", a, b, c))
			}
		}
	}

	return []string{strings.Join(parts, " ")}, nil
}

func formatFormulaValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", val)
	case []interface{}:
		var items []string
		for _, item := range val {
			items = append(items, formatFormulaValue(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case map[string]interface{}:
		// Geo point
		if lat, ok := val["lat"]; ok {
			if lon, ok := val["lon"]; ok {
				return fmt.Sprintf("{lat: %v, lon: %v}", lat, lon)
			}
		}
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// --- Filter conversion ---

func convertFilter(input interface{}) (string, error) {
	if input == nil {
		return "", nil
	}

	filterMap, ok := input.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("filter must be an object")
	}

	return convertFilterObject(filterMap)
}

func convertFilterObject(filter map[string]interface{}) (string, error) {
	var parts []string

	// Handle "must" (AND)
	if must, ok := filter["must"]; ok {
		conds, err := convertConditions(must)
		if err != nil {
			return "", err
		}
		parts = append(parts, conds...)
	}

	// Handle "should" (OR)
	if should, ok := filter["should"]; ok {
		conds, err := convertConditions(should)
		if err != nil {
			return "", err
		}
		if len(conds) > 1 {
			parts = append(parts, "("+strings.Join(conds, " OR ")+")")
		} else if len(conds) == 1 {
			parts = append(parts, conds[0])
		}
	}

	// Handle "must_not" (NOT)
	if mustNot, ok := filter["must_not"]; ok {
		conds, err := convertConditions(mustNot)
		if err != nil {
			return "", err
		}
		for _, cond := range conds {
			parts = append(parts, "NOT "+cond)
		}
	}

	return strings.Join(parts, " AND "), nil
}

func convertConditions(input interface{}) ([]string, error) {
	conds, ok := input.([]interface{})
	if !ok {
		return nil, fmt.Errorf("conditions must be an array")
	}

	var result []string
	for _, cond := range conds {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		s, err := convertCondition(condMap)
		if err != nil {
			continue
		}
		if s != "" {
			result = append(result, s)
		}
	}
	return result, nil
}

func convertCondition(cond map[string]interface{}) (string, error) {
	// Nested filter
	if _, ok := cond["must"]; ok {
		return convertFilterObject(cond)
	}
	if _, ok := cond["should"]; ok {
		return convertFilterObject(cond)
	}
	if _, ok := cond["must_not"]; ok {
		return convertFilterObject(cond)
	}

	// Field condition: {"key": "...", "match": {...}} or {"key": "...", "range": {...}}
	key, _ := cond["key"].(string)
	if key == "" {
		return "", fmt.Errorf("condition missing 'key'")
	}

	if match, ok := cond["match"].(map[string]interface{}); ok {
		return convertMatchCondition(key, match)
	}

	if rangeCond, ok := cond["range"].(map[string]interface{}); ok {
		return convertRangeCondition(key, rangeCond)
	}

	if isNull, ok := cond["is_null"].(map[string]interface{}); ok {
		if v, ok := isNull["value"].(bool); ok && v {
			return fmt.Sprintf("%s IS NULL", key), nil
		}
	}

	if isEmpty, ok := cond["is_empty"].(map[string]interface{}); ok {
		if v, ok := isEmpty["value"].(bool); ok && v {
			return fmt.Sprintf("%s IS EMPTY", key), nil
		}
	}

	if hasId, ok := cond["has_id"].(map[string]interface{}); ok {
		if ids, ok := hasId["has_id"].([]interface{}); ok {
			return fmt.Sprintf("%s IN (%s)", key, formatIDList(ids)), nil
		}
	}

	return "", nil
}

func convertMatchCondition(key string, match map[string]interface{}) (string, error) {
	if value, ok := match["value"]; ok {
		return fmt.Sprintf("%s = %s", key, formatValue(value)), nil
	}
	if keyword, ok := match["keyword"]; ok {
		return fmt.Sprintf("%s = %s", key, formatValue(keyword)), nil
	}
	if integer, ok := match["integer"]; ok {
		return fmt.Sprintf("%s = %v", key, integer), nil
	}
	if boolean, ok := match["boolean"]; ok {
		return fmt.Sprintf("%s = %v", key, boolean), nil
	}
	if text, ok := match["text"]; ok {
		return fmt.Sprintf("%s MATCH %s", key, formatValue(text)), nil
	}
	if any, ok := match["any"]; ok {
		if values, ok := any.([]interface{}); ok {
			var vals []string
			for _, v := range values {
				vals = append(vals, formatValue(v))
			}
			return fmt.Sprintf("%s IN (%s)", key, strings.Join(vals, ", ")), nil
		}
	}
	except, hasExcept := match["except"]
	if hasExcept {
		if values, ok := except.([]interface{}); ok {
			var vals []string
			for _, v := range values {
				vals = append(vals, formatValue(v))
			}
			return fmt.Sprintf("%s NOT IN (%s)", key, strings.Join(vals, ", ")), nil
		}
	}

	return fmt.Sprintf("%s = <value>", key), nil
}

func convertRangeCondition(key string, r map[string]interface{}) (string, error) {
	gte, hasGte := r["gte"]
	gt, hasGt := r["gt"]
	lte, hasLte := r["lte"]
	lt, hasLt := r["lt"]

	// Between
	if (hasGte || hasGt) && (hasLte || hasLt) {
		low := gte
		if hasGt {
			low = gt
		}
		high := lte
		if hasLt {
			high = lt
		}
		return fmt.Sprintf("%s BETWEEN %v AND %v", key, low, high), nil
	}

	// Single comparisons
	if hasGte {
		return fmt.Sprintf("%s >= %v", key, gte), nil
	}
	if hasGt {
		return fmt.Sprintf("%s > %v", key, gt), nil
	}
	if hasLte {
		return fmt.Sprintf("%s <= %v", key, lte), nil
	}
	if hasLt {
		return fmt.Sprintf("%s < %v", key, lt), nil
	}

	return "", nil
}

func convertSearchParams(input interface{}) (string, error) {
	params, ok := input.(map[string]interface{})
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

// --- Helpers ---

func buildValuesDict(payload map[string]interface{}) string {
	if len(payload) == 0 {
		return "{}"
	}

	var entries []string
	for k, v := range payload {
		entries = append(entries, fmt.Sprintf("'%s': %s", k, formatValue(v)))
	}

	return "{" + strings.Join(entries, ", ") + "}"
}

func formatID(id interface{}) string {
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

func formatIDList(ids []interface{}) string {
	var parts []string
	for _, id := range ids {
		parts = append(parts, formatID(id))
	}
	return strings.Join(parts, ", ")
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "\\'"))
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	case []interface{}:
		var items []string
		for _, item := range val {
			items = append(items, formatValue(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	default:
		return fmt.Sprintf("'%v'", val)
	}
}
