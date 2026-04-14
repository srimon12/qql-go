package commands

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/qdrant/qql-go/internal/ast"
	"github.com/qdrant/qql-go/internal/config"
	"github.com/qdrant/qql-go/internal/filters"
	"github.com/qdrant/qql-go/internal/lexer"
	"github.com/qdrant/qql-go/internal/output"
	"github.com/qdrant/qql-go/internal/parser"
	"github.com/qdrant/qql-go/internal/repl"
	"github.com/spf13/cobra"
)

const (
	denseVectorName    = "dense"
	sparseVectorName   = "sparse"
	rerankVectorName   = "colbert"
	denseModelDefault  = "sentence-transformers/all-minilm-l6-v2"
	sparseModelDefault = "qdrant/bm25"
	rerankModelDefault = "answerdotai/answerai-colbert-small-v1"
	denseVectorSize    = 384
	rerankVectorSize   = 96

	collectionReadyTimeout  = 10 * time.Second
	collectionReadyInterval = 250 * time.Millisecond
	rerankPrefetchFactor    = 4
	rerankPrefetchCap       = 200
)

var Version = "0.1.0"

type commandOutputMode struct {
	json  bool
	quiet bool
}

type Executor struct {
	client *qdrant.Client
	config *config.Config
}

func NewExecutor(client *qdrant.Client, cfg *config.Config) *Executor {
	return &Executor{
		client: client,
		config: cfg,
	}
}

func NewClient(cfg *config.Config) (*qdrant.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	return newClientFromURL(cfg.URL, cfg.Secret)
}

func newClientFromURL(rawURL, apiKey string) (*qdrant.Client, error) {
	cfg, err := buildClientConfig(rawURL, apiKey)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	return qdrant.NewClient(cfg)
}

func buildClientConfig(rawURL, apiKey string) (*qdrant.Config, error) {
	normalized := strings.TrimSpace(rawURL)
	if normalized == "" {
		return nil, fmt.Errorf("empty URL")
	}

	if !strings.Contains(normalized, "://") {
		normalized = "http://" + normalized
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return nil, err
	}

	host := parsed.Hostname()
	if host == "" {
		host = parsed.Host
	}
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	port := 6333
	if parsedPort := parsed.Port(); parsedPort != "" {
		port, err = strconv.Atoi(parsedPort)
		if err != nil {
			return nil, fmt.Errorf("invalid port %q", parsedPort)
		}
	}
	if port == 6333 {
		port = 6334
	}

	return &qdrant.Config{
		Host:                   host,
		Port:                   port,
		APIKey:                 apiKey,
		UseTLS:                 strings.EqualFold(parsed.Scheme, "https"),
		SkipCompatibilityCheck: true,
	}, nil
}

func (e *Executor) Execute(query string) (string, error) {
	result, err := e.ExecuteResult(query)
	if err != nil {
		return "", err
	}
	return result.Message, nil
}

func (e *Executor) ExecuteResult(query string) (*ExecResponse, error) {
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(query)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	p := parser.NewParser()
	node, err := p.Parse(tokens)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	switch n := node.(type) {
	case *ast.ShowCollectionsStmt:
		return e.doShowCollections()
	case *ast.CreateCollectionStmt:
		return e.doCreateCollection(n)
	case *ast.DropCollectionStmt:
		return e.doDropCollection(n)
	case *ast.InsertStmt:
		return e.doInsert(n)
	case *ast.SearchStmt:
		return e.doSearch(n)
	case *ast.DeleteStmt:
		return e.doDelete(n)
	case *ast.CreateIndexStmt:
		return e.doCreateIndex(n)
	default:
		return nil, fmt.Errorf("unsupported statement type")
	}
}

func (e *Executor) Explain(query string) (string, error) {
	result, err := e.ExplainResult(query)
	if err != nil {
		return "", err
	}
	return result.Plan, nil
}

func (e *Executor) ExplainResult(query string) (*ExplainResponse, error) {
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(query)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	p := parser.NewParser()
	node, err := p.Parse(tokens)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	var plan strings.Builder

	switch n := node.(type) {
	case *ast.ShowCollectionsStmt:
		plan.WriteString("Statement: SHOW COLLECTIONS\n")
		plan.WriteString("Action: List all collections\n")
	case *ast.CreateCollectionStmt:
		plan.WriteString(fmt.Sprintf("Statement: CREATE COLLECTION %s\n", n.Collection))
		if n.Rerank {
			plan.WriteString("Type: HYBRID + RERANK (dense + sparse + ColBERT multivector)\n")
		} else if n.Hybrid {
			plan.WriteString("Type: HYBRID (dense + sparse)\n")
		} else {
			plan.WriteString("Type: DENSE\n")
		}
		plan.WriteString("Action: Create new collection\n")
	case *ast.DropCollectionStmt:
		plan.WriteString(fmt.Sprintf("Statement: DROP COLLECTION %s\n", n.Collection))
		plan.WriteString("Action: Delete collection and all points\n")
	case *ast.InsertStmt:
		plan.WriteString(fmt.Sprintf("Statement: INSERT INTO %s\n", n.Collection))
		if n.Model != nil && *n.Model != "" {
			plan.WriteString(fmt.Sprintf("Model: %s\n", *n.Model))
		}
		if n.Hybrid {
			plan.WriteString("Search: HYBRID (dense + sparse)\n")
		} else {
			plan.WriteString("Search: DENSE\n")
		}
		plan.WriteString("Action: Insert point with auto-vectorization\n")
	case *ast.SearchStmt:
		plan.WriteString(fmt.Sprintf("Statement: SEARCH %s SIMILAR TO '%s' LIMIT %d\n",
			n.Collection, n.QueryText, n.Limit))
		if n.Model != nil && *n.Model != "" {
			plan.WriteString(fmt.Sprintf("Model: %s\n", *n.Model))
		}
		if n.Hybrid {
			plan.WriteString("Search: HYBRID (dense + sparse)\n")
		} else {
			plan.WriteString("Search: DENSE\n")
		}
		if n.QueryFilter != nil {
			plan.WriteString(fmt.Sprintf("Filter: %s\n", e.filterToString(n.QueryFilter)))
		}
		if n.WithClause != nil && n.WithClause.Exact {
			plan.WriteString("Search params: EXACT (bypass HNSW)\n")
		}
		if n.WithClause != nil && n.WithClause.HnswEf > 0 {
			plan.WriteString(fmt.Sprintf("Search params: hnsw_ef=%d\n", n.WithClause.HnswEf))
		}
		if n.Rerank {
			plan.WriteString("Rerank: enabled\n")
			if n.RerankModel != nil && *n.RerankModel != "" {
				plan.WriteString(fmt.Sprintf("Rerank model: %s\n", *n.RerankModel))
			}
			plan.WriteString(fmt.Sprintf("Rerank vector: %s\n", rerankVectorName))
		}
	case *ast.DeleteStmt:
		if n.Field != "" {
			plan.WriteString(fmt.Sprintf("Statement: DELETE FROM %s WHERE %s = '%v'\n",
				n.Collection, n.Field, n.Value))
			plan.WriteString("Action: Delete points by filter\n")
		} else {
			plan.WriteString(fmt.Sprintf("Statement: DELETE FROM %s WHERE id = '%v'\n",
				n.Collection, n.PointID))
			plan.WriteString("Action: Delete point by ID\n")
		}
	case *ast.CreateIndexStmt:
		plan.WriteString(fmt.Sprintf("Statement: CREATE INDEX ON COLLECTION %s FOR %s", n.Collection, n.Field))
		if n.FieldType != "" && n.FieldType != "keyword" {
			plan.WriteString(fmt.Sprintf(" TYPE %s", n.FieldType))
		}
		plan.WriteString("\n")
		plan.WriteString("Action: Create payload index on field\n")
	default:
		plan.WriteString("Statement: Unknown\n")
	}

	return &ExplainResponse{
		OK:    true,
		Query: query,
		Plan:  plan.String(),
	}, nil
}

func (e *Executor) filterToString(filter ast.FilterExpr) string {
	if filter == nil {
		return ""
	}
	return fmt.Sprintf("%v", filter)
}

func (e *Executor) configuredModel() string {
	if e != nil && e.config != nil && e.config.InferenceModel != "" {
		return e.config.InferenceModel
	}
	return denseModelDefault
}

func (e *Executor) doShowCollections() (*ExecResponse, error) {
	ctx := context.Background()
	names, err := e.client.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get collections: %w", err)
	}

	if len(names) == 0 {
		return &ExecResponse{
			OK:        true,
			Operation: "show_collections",
			Message:   "No collections found",
			Data: map[string]any{
				"count":       0,
				"collections": []string{},
			},
		}, nil
	}

	return &ExecResponse{
		OK:        true,
		Operation: "show_collections",
		Message:   fmt.Sprintf("Found %d collection(s): %s", len(names), strings.Join(names, ", ")),
		Data: map[string]any{
			"count":       len(names),
			"collections": names,
		},
	}, nil
}

func (e *Executor) doCreateCollection(n *ast.CreateCollectionStmt) (*ExecResponse, error) {
	ctx := context.Background()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if exists {
		return &ExecResponse{
			OK:        true,
			Operation: "create_collection",
			Message:   fmt.Sprintf("Collection '%s' already exists", n.Collection),
			Data: map[string]any{
				"collection": n.Collection,
				"exists":     true,
				"hybrid":     n.Hybrid,
				"rerank":     n.Rerank,
			},
		}, nil
	}

	collection := &qdrant.CreateCollection{
		CollectionName: n.Collection,
		VectorsConfig:  qdrant.NewVectorsConfigMap(collectionVectorParams(n.Rerank)),
	}
	if n.Hybrid || n.Rerank {
		collection.SparseVectorsConfig = qdrant.NewSparseVectorsConfig(map[string]*qdrant.SparseVectorParams{
			sparseVectorName: {
				Modifier: qdrant.Modifier_Idf.Enum(),
			},
		})
	}
	err = e.client.CreateCollection(ctx, collection)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}
	if err := e.waitForCollectionReady(ctx, n.Collection); err != nil {
		return nil, err
	}
	message := fmt.Sprintf("Collection '%s' created (dense)", n.Collection)
	if n.Hybrid || n.Rerank {
		if n.Rerank {
			message = fmt.Sprintf("Collection '%s' created (hybrid: dense + sparse + ColBERT)", n.Collection)
		} else {
			message = fmt.Sprintf("Collection '%s' created (hybrid: dense + sparse)", n.Collection)
		}
	}
	return &ExecResponse{
		OK:        true,
		Operation: "create_collection",
		Message:   message,
		Data: map[string]any{
			"collection": n.Collection,
			"exists":     false,
			"hybrid":     n.Hybrid,
			"rerank":     n.Rerank,
		},
	}, nil
}

func (e *Executor) doDropCollection(n *ast.DropCollectionStmt) (*ExecResponse, error) {
	ctx := context.Background()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	if err := e.client.DeleteCollection(ctx, n.Collection); err != nil {
		return nil, fmt.Errorf("failed to drop collection: %w", err)
	}
	return &ExecResponse{
		OK:        true,
		Operation: "drop_collection",
		Message:   fmt.Sprintf("Collection '%s' dropped", n.Collection),
		Data: map[string]any{
			"collection": n.Collection,
		},
	}, nil
}

func (e *Executor) doInsert(n *ast.InsertStmt) (*ExecResponse, error) {
	ctx := context.Background()

	textVal, ok := n.Values["text"]
	if !ok {
		return nil, fmt.Errorf("INSERT requires a 'text' field in VALUES")
	}
	text, ok := textVal.(string)
	if !ok {
		return nil, fmt.Errorf("'text' field must be a string")
	}

	model := e.configuredModel()
	if n.Model != nil && *n.Model != "" {
		model = *n.Model
	}

	sparseModel := sparseModelDefault
	if n.SparseModel != nil && *n.SparseModel != "" {
		sparseModel = *n.SparseModel
	}

	includeRerank, err := e.collectionHasRerankVector(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}

	pointID := uuid.New().String()

	if n.Hybrid {
		point := &qdrant.PointStruct{
			Id:      qdrant.NewID(pointID),
			Vectors: qdrant.NewVectorsMap(buildPointVectors(text, model, sparseModel, true, includeRerank)),
			Payload: e.buildPayload(n.Values),
		}
		_, err := e.client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: n.Collection,
			Points:         []*qdrant.PointStruct{point},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to insert: %w", err)
		}
	} else {
		point := &qdrant.PointStruct{
			Id:      qdrant.NewID(pointID),
			Vectors: qdrant.NewVectorsMap(buildPointVectors(text, model, sparseModel, false, includeRerank)),
			Payload: e.buildPayload(n.Values),
		}
		_, err := e.client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: n.Collection,
			Points:         []*qdrant.PointStruct{point},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to insert: %w", err)
		}
	}

	return &ExecResponse{
		OK:        true,
		Operation: "insert",
		Message:   fmt.Sprintf("Inserted 1 point [%s] into '%s'", pointID, n.Collection),
		Data: map[string]any{
			"id":           pointID,
			"collection":   n.Collection,
			"hybrid":       n.Hybrid,
			"dense_model":  model,
			"sparse_model": sparseModel,
			"rerank":       includeRerank,
		},
	}, nil
}

func (e *Executor) doSearch(n *ast.SearchStmt) (*ExecResponse, error) {
	ctx := context.Background()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	model := e.configuredModel()
	if n.Model != nil && *n.Model != "" {
		model = *n.Model
	}

	sparseModel := sparseModelDefault
	if n.SparseModel != nil && *n.SparseModel != "" {
		sparseModel = *n.SparseModel
	}

	hasRerankVector, err := e.collectionHasRerankVector(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}

	limit := uint64(n.Limit)
	if limit == 0 {
		limit = 10
	}

	fetchLimit := effectiveSearchLimit(limit, n.Rerank)

	searchReq, err := buildSearchRequest(n, model, sparseModel, hasRerankVector, fetchLimit)
	if err != nil {
		return nil, err
	}

	if n.QueryFilter != nil {
		filter, err := filters.NewFilterConverter().BuildFilter(n.QueryFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to build filter: %w", err)
		}
		searchReq.Filter = filter
	}

	results, err := e.client.Query(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	message, hits := e.formatSearchResults(results)
	return &ExecResponse{
		OK:        true,
		Operation: "search",
		Message:   message,
		Data: map[string]any{
			"count":      len(hits),
			"results":    hits,
			"collection": n.Collection,
			"hybrid":     n.Hybrid,
			"rerank":     n.Rerank,
		},
	}, nil
}

func (e *Executor) doDelete(n *ast.DeleteStmt) (*ExecResponse, error) {
	ctx := context.Background()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	request, err := buildDeleteRequest(n)
	if err != nil {
		return nil, err
	}

	_, err = e.client.Delete(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to delete point: %w", err)
	}

	if n.Field != "" {
		return &ExecResponse{
			OK:        true,
			Operation: "delete",
			Message:   fmt.Sprintf("Deleted points matching %s = '%v' from '%s'", n.Field, n.Value, n.Collection),
			Data: map[string]any{
				"collection": n.Collection,
				"field":      n.Field,
				"value":      n.Value,
			},
		}, nil
	}

	return &ExecResponse{
		OK:        true,
		Operation: "delete",
		Message:   fmt.Sprintf("Deleted point '%v' from '%s'", n.PointID, n.Collection),
		Data: map[string]any{
			"collection": n.Collection,
			"point_id":   n.PointID,
		},
	}, nil
}

func (e *Executor) doCreateIndex(n *ast.CreateIndexStmt) (*ExecResponse, error) {
	ctx := context.Background()

	fieldType := qdrant.FieldType_FieldTypeKeyword
	if n.FieldType == "integer" {
		fieldType = qdrant.FieldType_FieldTypeInteger
	} else if n.FieldType == "float" {
		fieldType = qdrant.FieldType_FieldTypeFloat
	} else if n.FieldType == "bool" {
		fieldType = qdrant.FieldType_FieldTypeBool
	}

	wait := true
	_, err := e.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName: n.Collection,
		FieldName:      n.Field,
		FieldType:      &fieldType,
		Wait:           &wait,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}
	return &ExecResponse{
		OK:        true,
		Operation: "create_index",
		Message:   fmt.Sprintf("Index created on '%s.%s'", n.Collection, n.Field),
		Data: map[string]any{
			"collection": n.Collection,
			"field":      n.Field,
			"field_type": n.FieldType,
		},
	}, nil
}

func (e *Executor) buildPayload(values map[string]interface{}) map[string]*qdrant.Value {
	return qdrant.NewValueMap(values)
}

func collectionVectorParams(includeRerank bool) map[string]*qdrant.VectorParams {
	vectors := map[string]*qdrant.VectorParams{
		denseVectorName: {
			Size:     denseVectorSize,
			Distance: qdrant.Distance_Cosine,
		},
	}
	if includeRerank {
		vectors[rerankVectorName] = &qdrant.VectorParams{
			Size:     rerankVectorSize,
			Distance: qdrant.Distance_Cosine,
			MultivectorConfig: &qdrant.MultiVectorConfig{
				Comparator: qdrant.MultiVectorComparator_MaxSim,
			},
			HnswConfig: &qdrant.HnswConfigDiff{
				M: qdrant.PtrOf(uint64(0)),
			},
		}
	}
	return vectors
}

func buildPointVectors(text, denseModel, sparseModel string, includeSparse, includeRerank bool) map[string]*qdrant.Vector {
	vectors := map[string]*qdrant.Vector{
		denseVectorName: qdrant.NewVectorDocument(&qdrant.Document{
			Text:  text,
			Model: denseModel,
		}),
	}
	if includeSparse {
		vectors[sparseVectorName] = qdrant.NewVectorDocument(&qdrant.Document{
			Text:  text,
			Model: sparseModel,
		})
	}
	if includeRerank {
		vectors[rerankVectorName] = qdrant.NewVectorDocument(&qdrant.Document{
			Text:  text,
			Model: rerankModelDefault,
		})
	}
	return vectors
}

func buildSearchRequest(n *ast.SearchStmt, denseModel, sparseModel string, hasRerankVector bool, limit uint64) (*qdrant.QueryPoints, error) {
	params := searchParamsFromWithClause(n.WithClause)

	if n.Rerank {
		if !hasRerankVector {
			return nil, fmt.Errorf("collection '%s' does not support rerank; create it with HYBRID RERANK", n.Collection)
		}
		rerankModel := rerankModelDefault
		if n.RerankModel != nil && *n.RerankModel != "" {
			rerankModel = *n.RerankModel
		}
		prefetch := buildSearchPrefetches(n.QueryText, denseModel, sparseModel, limit, params)
		return buildRerankSearchRequest(n.Collection, n.QueryText, rerankModel, limit, prefetch, params), nil
	}

	if n.Hybrid {
		return &qdrant.QueryPoints{
			CollectionName: n.Collection,
			Prefetch:       buildSearchPrefetches(n.QueryText, denseModel, sparseModel, limit, params),
			Query:          qdrant.NewQueryFusion(qdrant.Fusion_RRF),
			Limit:          qdrant.PtrOf(limit),
			Params:         params,
		}, nil
	}

	return &qdrant.QueryPoints{
		CollectionName: n.Collection,
		Query: qdrant.NewQueryDocument(&qdrant.Document{
			Text:  n.QueryText,
			Model: denseModel,
		}),
		Using:  qdrant.PtrOf(denseVectorName),
		Limit:  qdrant.PtrOf(limit),
		Params: params,
	}, nil
}

func buildSearchPrefetches(queryText, denseModel, sparseModel string, limit uint64, params *qdrant.SearchParams) []*qdrant.PrefetchQuery {
	return []*qdrant.PrefetchQuery{
		{
			Query: qdrant.NewQueryDocument(&qdrant.Document{
				Text:  queryText,
				Model: sparseModel,
			}),
			Using:  qdrant.PtrOf(sparseVectorName),
			Limit:  qdrant.PtrOf(limit),
			Params: params,
		},
		{
			Query: qdrant.NewQueryDocument(&qdrant.Document{
				Text:  queryText,
				Model: denseModel,
			}),
			Using:  qdrant.PtrOf(denseVectorName),
			Limit:  qdrant.PtrOf(limit),
			Params: params,
		},
	}
}

func searchParamsFromWithClause(withClause *ast.SearchWith) *qdrant.SearchParams {
	if withClause == nil {
		return nil
	}

	params := &qdrant.SearchParams{}
	if withClause.HnswEf > 0 {
		params.HnswEf = qdrant.PtrOf(uint64(withClause.HnswEf))
	}
	if withClause.Exact {
		params.Exact = qdrant.PtrOf(true)
	}
	if withClause.Acorn {
		params.Acorn = &qdrant.AcornSearchParams{Enable: qdrant.PtrOf(true)}
	}

	if params.HnswEf == nil && params.Exact == nil && params.Acorn == nil {
		return nil
	}

	return params
}

func buildDeleteRequest(n *ast.DeleteStmt) (*qdrant.DeletePoints, error) {
	wait := true

	if n.Field != "" {
		filter, err := filters.NewFilterConverter().BuildFilter(&ast.CompareExpr{
			Field: n.Field,
			Op:    "=",
			Value: n.Value,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build delete filter: %w", err)
		}
		return &qdrant.DeletePoints{
			CollectionName: n.Collection,
			Points:         qdrant.NewPointsSelectorFilter(filter),
			Wait:           &wait,
		}, nil
	}

	pointID := fmt.Sprintf("%v", n.PointID)
	var pid *qdrant.PointId
	if num, err := parseUint64(pointID); err == nil {
		pid = qdrant.NewIDNum(num)
	} else {
		pid = qdrant.NewIDUUID(pointID)
	}

	return &qdrant.DeletePoints{
		CollectionName: n.Collection,
		Points:         qdrant.NewPointsSelector(pid),
		Wait:           &wait,
	}, nil
}

func (e *Executor) collectionHasRerankVector(ctx context.Context, collection string) (bool, error) {
	info, err := e.client.GetCollectionInfo(ctx, collection)
	if err != nil {
		return false, err
	}
	params := info.GetConfig().GetParams()
	if params == nil {
		return false, nil
	}
	vectors := params.GetVectorsConfig().GetParamsMap()
	if vectors == nil {
		return false, nil
	}
	_, ok := vectors.GetMap()[rerankVectorName]
	return ok, nil
}

func buildRerankSearchRequest(collection, queryText, rerankModel string, limit uint64, prefetch []*qdrant.PrefetchQuery, params *qdrant.SearchParams) *qdrant.QueryPoints {
	return &qdrant.QueryPoints{
		CollectionName: collection,
		Prefetch:       prefetch,
		Query: qdrant.NewQueryDocument(&qdrant.Document{
			Text:  queryText,
			Model: rerankModel,
		}),
		Using:  qdrant.PtrOf(rerankVectorName),
		Limit:  qdrant.PtrOf(limit),
		Params: params,
	}
}

func (e *Executor) waitForCollectionReady(ctx context.Context, collection string) error {
	return waitForCollectionReady(ctx, collection, collectionReadyTimeout, collectionReadyInterval, e.collectionReady)
}

func waitForCollectionReady(
	ctx context.Context,
	collection string,
	timeout time.Duration,
	interval time.Duration,
	ready func(context.Context, string) (bool, bool, error),
) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		exists, readyNow, err := ready(waitCtx, collection)
		if err == nil && readyNow {
			return nil
		}

		select {
		case <-waitCtx.Done():
			if err != nil {
				return fmt.Errorf("collection '%s' did not become ready within %s: %w", collection, timeout, err)
			}
			if exists {
				return fmt.Errorf("collection '%s' exists but is not ready yet after %s", collection, timeout)
			}
			return fmt.Errorf("collection '%s' did not become visible within %s", collection, timeout)
		case <-time.After(interval):
		}
	}
}

func effectiveSearchLimit(limit uint64, rerank bool) uint64 {
	if !rerank || limit == 0 {
		return limit
	}

	if limit > ^uint64(0)/rerankPrefetchFactor {
		return limit
	}

	boosted := limit * rerankPrefetchFactor
	if boosted <= rerankPrefetchCap {
		return boosted
	}
	if limit > rerankPrefetchCap {
		return limit
	}
	return rerankPrefetchCap
}

func loadSavedConfig() (*config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg == nil || cfg.URL == "" {
		return nil, fmt.Errorf("not connected. Run: qql connect --url <url>")
	}
	return cfg, nil
}

func loadSavedConfigAndClient() (*config.Config, *qdrant.Client, error) {
	cfg, err := loadSavedConfig()
	if err != nil {
		return nil, nil, err
	}

	client, err := newClientFromURL(cfg.URL, cfg.Secret)
	if err != nil {
		return nil, nil, fmt.Errorf("connection failed: %w", err)
	}
	return cfg, client, nil
}

func savedConfigMessage() string {
	path, err := config.ConfigPath()
	if err != nil {
		return "Connected. Config saved."
	}
	return fmt.Sprintf("Connected. Config saved to %s", path)
}

func runConfiguredREPL() error {
	cfg, client, err := loadSavedConfigAndClient()
	if err != nil {
		return err
	}
	return repl.NewREPL(cfg, NewExecutor(client, cfg)).Run()
}

func writeJSON(out *output.Outputter, value any, quiet bool) error {
	if err := out.PrintJSON(value, !quiet); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}
	return nil
}

func commandError(out *output.Outputter, mode commandOutputMode, command, query string, err error) error {
	if mode.json {
		if jsonErr := out.PrintJSON(&ErrorResponse{
			OK:        false,
			Command:   command,
			Query:     query,
			Error:     err.Error(),
			ErrorType: "runtime_error",
		}, false); jsonErr != nil {
			return fmt.Errorf("failed to write JSON: %w", jsonErr)
		}
		return NewExitError(err, 1, true)
	}

	out.PrintError(err.Error())
	return NewExitError(err, 1, true)
}

func currentVersion() string {
	return strings.TrimSpace(Version)
}

func displayVersion() string {
	version := currentVersion()
	if version == "" {
		return "dev"
	}
	return version
}

func versionMessage() string {
	return fmt.Sprintf("QQL %s", displayVersion())
}

func (e *Executor) collectionReady(ctx context.Context, collection string) (bool, bool, error) {
	exists, err := e.client.CollectionExists(ctx, collection)
	if err != nil {
		return false, false, err
	}
	if !exists {
		return false, false, nil
	}
	if _, err := e.client.GetCollectionInfo(ctx, collection); err != nil {
		return true, false, err
	}
	return true, true, nil
}

func (e *Executor) formatSearchResults(results []*qdrant.ScoredPoint) (string, []SearchHit) {
	if len(results) == 0 {
		return "No results found", []SearchHit{}
	}

	var resultLines []string
	hits := make([]SearchHit, 0, len(results))
	for _, r := range results {
		id := fmt.Sprintf("%v", r.GetId())
		jsonID := pointIDString(r.GetId())
		score := r.GetScore()
		payload := r.GetPayload()
		text := ""
		if p, ok := payload["text"]; ok {
			if sv, ok := p.GetKind().(*qdrant.Value_StringValue); ok {
				text = sv.StringValue
			}
		}
		resultLines = append(resultLines, fmt.Sprintf("id:%s score:%.4f payload:%s", id, score, text))
		hits = append(hits, SearchHit{
			ID:    jsonID,
			Score: score,
			Text:  text,
		})
	}

	return fmt.Sprintf("Found %d result(s):\n%s", len(results), strings.Join(resultLines, "\n")), hits
}

func parseUint64(s string) (uint64, error) {
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid number")
		}
		n = n*10 + uint64(c-'0')
	}
	return n, nil
}

func pointIDString(id *qdrant.PointId) string {
	if id == nil {
		return ""
	}
	if uuid := id.GetUuid(); uuid != "" {
		return uuid
	}
	if num := id.GetNum(); num != 0 {
		return strconv.FormatUint(num, 10)
	}
	return fmt.Sprintf("%v", id)
}

func readOutputMode(cmd *cobra.Command) commandOutputMode {
	jsonOut, _ := cmd.Flags().GetBool("json")
	quiet, _ := cmd.Flags().GetBool("quiet")
	return commandOutputMode{
		json:  jsonOut,
		quiet: quiet,
	}
}

func addOutputFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "Emit structured JSON output")
	cmd.Flags().Bool("quiet", false, "Reduce decoration; with --json, emit compact JSON")
}

func NewConnectCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect to a Qdrant instance",
		Long:  `Connect to a Qdrant instance and save the configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			url, _ := cmd.Flags().GetString("url")
			secret, _ := cmd.Flags().GetString("secret")

			if url == "" {
				return commandError(out, mode, "connect", "", fmt.Errorf("--url is required"))
			}

			if !mode.json && !mode.quiet {
				out.Print(fmt.Sprintf("Connecting to %s...", url))
			}

			client, err := newClientFromURL(url, secret)
			if err != nil {
				return commandError(out, mode, "connect", "", fmt.Errorf("connection failed: %w", err))
			}

			collections, err := client.ListCollections(context.Background())
			if err != nil {
				return commandError(out, mode, "connect", "", fmt.Errorf("connection failed: %w", err))
			}

			cfg := &config.Config{
				URL:    url,
				Secret: secret,
			}

			if err := config.SaveConfig(cfg); err != nil {
				return commandError(out, mode, "connect", "", fmt.Errorf("failed to save config: %w", err))
			}

			message := savedConfigMessage()
			if mode.json {
				return writeJSON(out, &ConnectResponse{
					OK:          true,
					Command:     "connect",
					URL:         url,
					Connected:   true,
					Collections: len(collections),
					Message:     message,
				}, mode.quiet)
			}

			if mode.quiet {
				out.Print(message)
				return nil
			}

			out.PrintSuccess(message)

			repl := repl.NewREPL(cfg, NewExecutor(client, cfg))
			return repl.Run()
		},
	}

	cmd.Flags().String("url", "", "Qdrant instance URL (for text INSERT/SEARCH use your Qdrant Cloud URL)")
	cmd.Flags().String("secret", "", "API key / secret (optional)")
	addOutputFlags(cmd)
	_ = cmd.MarkFlagRequired("url")

	return cmd
}

func NewDisconnectCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disconnect",
		Short: "Remove saved connection config",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			if err := config.DeleteConfig(); err != nil {
				return commandError(out, mode, "disconnect", "", fmt.Errorf("failed to delete config: %w", err))
			}
			message := "Disconnected. Config removed."
			if mode.json {
				return writeJSON(out, &CommandResponse{
					OK:      true,
					Command: "disconnect",
					Message: message,
				}, mode.quiet)
			}
			if mode.quiet {
				out.Print(message)
				return nil
			}
			out.PrintSuccess(message)
			return nil
		},
	}
	addOutputFlags(cmd)
	return cmd
}

func NewREPLCmd(out *output.Outputter) *cobra.Command {
	return &cobra.Command{
		Use:   "repl",
		Short: "Launch interactive shell",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfiguredREPL()
		},
	}
}

func NewExecCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute a single query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			cfg, client, err := loadSavedConfigAndClient()
			if err != nil {
				return commandError(out, mode, "exec", args[0], err)
			}

			executor := NewExecutor(client, cfg)
			if mode.json {
				result, err := executor.ExecuteResult(args[0])
				if err != nil {
					return commandError(out, mode, "exec", args[0], err)
				}
				return writeJSON(out, result, mode.quiet)
			}

			result, err := executor.Execute(args[0])
			if err != nil {
				return commandError(out, mode, "exec", args[0], err)
			}

			out.Print(result)
			return nil
		},
	}

	addOutputFlags(cmd)
	return cmd
}

func NewExplainCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Show query plan without executing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			executor := NewExecutor(nil, nil)
			if mode.json {
				result, err := executor.ExplainResult(args[0])
				if err != nil {
					return commandError(out, mode, "explain", args[0], err)
				}
				return writeJSON(out, result, mode.quiet)
			}

			plan, err := executor.Explain(args[0])
			if err != nil {
				return commandError(out, mode, "explain", args[0], err)
			}

			if mode.quiet {
				out.Print(plan)
				return nil
			}
			out.PrintExplain(plan)
			return nil
		},
	}

	addOutputFlags(cmd)
	return cmd
}

func NewDoctorCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check connection health",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			cfg, client, err := loadSavedConfigAndClient()
			if err != nil {
				return commandError(out, mode, "doctor", "", err)
			}

			if !mode.json && !mode.quiet {
				out.Print(fmt.Sprintf("Checking connection to %s...", cfg.URL))
			}

			collections, err := client.ListCollections(context.Background())
			if err != nil {
				return commandError(out, mode, "doctor", "", fmt.Errorf("failed to connect: %w", err))
			}

			message := "Connection is healthy."
			if mode.json {
				return writeJSON(out, &DoctorResponse{
					OK:          true,
					Command:     "doctor",
					URL:         cfg.URL,
					Healthy:     true,
					Collections: len(collections),
					Message:     message,
				}, mode.quiet)
			}

			if mode.quiet {
				out.Print(fmt.Sprintf("healthy url=%s collections=%d", cfg.URL, len(collections)))
				return nil
			}

			out.PrintConnectionStatus(cfg.URL, true)
			out.Print(fmt.Sprintf("\nCollections: %d", len(collections)))
			out.Print("\n" + message)

			return nil
		},
	}

	addOutputFlags(cmd)

	return cmd
}

func NewVersionCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			version := displayVersion()
			message := versionMessage()
			if mode.json {
				return writeJSON(out, &VersionResponse{
					OK:      true,
					Command: "version",
					Version: version,
					Message: message,
				}, mode.quiet)
			}
			if mode.quiet {
				out.Print(version)
				return nil
			}
			out.PrintSection("QQL Version", version)
			return nil
		},
	}
	addOutputFlags(cmd)
	return cmd
}
