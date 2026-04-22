package commands

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/spf13/cobra"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/internal/dump"
	"github.com/srimon12/qql-go/internal/embedding"
	"github.com/srimon12/qql-go/internal/filters"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/srimon12/qql-go/internal/output"
	"github.com/srimon12/qql-go/internal/parser"
	"github.com/srimon12/qql-go/internal/repl"
	"github.com/srimon12/qql-go/internal/script"
	"github.com/srimon12/qql-go/internal/sparse"
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
	defaultInferenceMode    = "cloud"
)

var Version = "0.1.2"

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

func (e *Executor) ExecuteFile(path string, stopOnError bool) (string, error) {
	okCount, failCount, err := script.RunFile(path, e, stopOnError)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Executed script %s (%d succeeded, %d failed)", path, okCount, failCount), nil
}

func (e *Executor) DumpCollection(collection, outputPath string) (string, error) {
	written, skipped, err := dump.Collection(context.Background(), e.client, collection, outputPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Dumped collection '%s' to %s (%d written, %d skipped)", collection, outputPath, written, skipped), nil
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
	case *ast.InsertBulkStmt:
		return e.doInsertBulk(n)
	case *ast.SearchStmt:
		return e.doSearch(n)
	case *ast.RecommendStmt:
		return e.doRecommend(n)
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
		if n.Model != nil && *n.Model != "" {
			plan.WriteString(fmt.Sprintf("Model: %s\n", *n.Model))
		}
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
		if n.PointID != nil {
			plan.WriteString(fmt.Sprintf("Point ID: %v\n", n.PointID))
		}
		plan.WriteString("Action: Insert point with auto-vectorization\n")
	case *ast.InsertBulkStmt:
		plan.WriteString(fmt.Sprintf("Statement: INSERT BULK INTO %s\n", n.Collection))
		if n.Model != nil && *n.Model != "" {
			plan.WriteString(fmt.Sprintf("Model: %s\n", *n.Model))
		}
		if n.Hybrid {
			plan.WriteString("Search: HYBRID (dense + sparse)\n")
		} else {
			plan.WriteString("Search: DENSE\n")
		}
		plan.WriteString(fmt.Sprintf("Batch size: %d\n", len(n.ValuesList)))
		plan.WriteString("Action: Insert multiple points with auto-vectorization\n")
	case *ast.SearchStmt:
		plan.WriteString(fmt.Sprintf("Statement: SEARCH %s SIMILAR TO '%s' LIMIT %d\n",
			n.Collection, n.QueryText, n.Limit))
		if n.Model != nil && *n.Model != "" {
			plan.WriteString(fmt.Sprintf("Model: %s\n", *n.Model))
		}
		if n.SparseOnly {
			plan.WriteString("Search: SPARSE\n")
		} else if n.Hybrid {
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
	case *ast.RecommendStmt:
		plan.WriteString(fmt.Sprintf("Statement: RECOMMEND FROM %s LIMIT %d\n", n.Collection, n.Limit))
		plan.WriteString(fmt.Sprintf("Positive IDs: %d\n", len(n.PositiveIDs)))
		if len(n.NegativeIDs) > 0 {
			plan.WriteString(fmt.Sprintf("Negative IDs: %d\n", len(n.NegativeIDs)))
		}
		if n.Strategy != nil && *n.Strategy != "" {
			plan.WriteString(fmt.Sprintf("Strategy: %s\n", *n.Strategy))
		}
		if n.QueryFilter != nil {
			plan.WriteString(fmt.Sprintf("Filter: %s\n", e.filterToString(n.QueryFilter)))
		}
		plan.WriteString("Action: Recommend points by example IDs\n")
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

	denseSize, err := e.resolveDenseVectorSize(ctx, n.Model)
	if err != nil {
		return nil, err
	}

	collection := &qdrant.CreateCollection{
		CollectionName: n.Collection,
		VectorsConfig:  qdrant.NewVectorsConfigMap(collectionVectorParams(denseSize, n.Rerank)),
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
			"dense_size": denseSize,
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
	e.deleteCorpusStats(n.Collection)
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

	model := e.resolveDenseModel(n.Model)
	sparseModel := e.resolveSparseModel(n.SparseModel)
	useHybrid, err := e.shouldUseHybrid(ctx, n.Collection, n.Hybrid)
	if err != nil {
		return nil, err
	}
	includeRerank, err := e.collectionHasRerankVector(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}

	pointID, payload, err := insertPointIDAndPayload(n.PointID, n.Values)
	if err != nil {
		return nil, err
	}

	vectors, err := e.buildInsertVectors(ctx, text, model, sparseModel, useHybrid, includeRerank, n.Collection)
	if err != nil {
		return nil, err
	}

	_, err = e.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: n.Collection,
		Points: []*qdrant.PointStruct{
			{
				Id:      newPointID(pointID),
				Vectors: qdrant.NewVectorsMap(vectors),
				Payload: e.buildPayload(payload),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to insert: %w", err)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "insert",
		Message:   fmt.Sprintf("Inserted 1 point [%v] into '%s'", pointID, n.Collection),
		Data: map[string]any{
			"id":           pointID,
			"collection":   n.Collection,
			"hybrid":       useHybrid,
			"dense_model":  model,
			"sparse_model": sparseModel,
			"rerank":       includeRerank,
		},
	}, nil
}

func (e *Executor) doInsertBulk(n *ast.InsertBulkStmt) (*ExecResponse, error) {
	ctx := context.Background()
	if len(n.ValuesList) == 0 {
		return nil, fmt.Errorf("INSERT BULK VALUES list is empty")
	}

	model := e.resolveDenseModel(n.Model)
	sparseModel := e.resolveSparseModel(n.SparseModel)
	useHybrid, err := e.shouldUseHybrid(ctx, n.Collection, n.Hybrid)
	if err != nil {
		return nil, err
	}
	includeRerank, err := e.collectionHasRerankVector(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}

	texts := make([]string, 0, len(n.ValuesList))
	pointIDs := make([]interface{}, 0, len(n.ValuesList))
	payloads := make([]map[string]interface{}, 0, len(n.ValuesList))
	for idx, values := range n.ValuesList {
		textVal, ok := values["text"]
		if !ok {
			return nil, fmt.Errorf("INSERT BULK item %d requires a 'text' field in VALUES", idx)
		}
		text, ok := textVal.(string)
		if !ok {
			return nil, fmt.Errorf("INSERT BULK item %d 'text' field must be a string", idx)
		}
		pointID, payload, err := insertPointIDAndPayload(nil, values)
		if err != nil {
			return nil, fmt.Errorf("INSERT BULK item %d: %w", idx, err)
		}
		texts = append(texts, text)
		pointIDs = append(pointIDs, pointID)
		payloads = append(payloads, payload)
	}

	vectorsBatch, err := e.buildInsertVectorsBatch(ctx, texts, model, sparseModel, useHybrid, includeRerank, n.Collection)
	if err != nil {
		return nil, err
	}

	points := make([]*qdrant.PointStruct, 0, len(texts))
	for idx, vectors := range vectorsBatch {
		points = append(points, &qdrant.PointStruct{
			Id:      newPointID(pointIDs[idx]),
			Vectors: qdrant.NewVectorsMap(vectors),
			Payload: e.buildPayload(payloads[idx]),
		})
	}

	_, err = e.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: n.Collection,
		Points:         points,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to insert bulk points: %w", err)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "insert_bulk",
		Message:   fmt.Sprintf("Inserted %d point(s) into '%s'", len(points), n.Collection),
		Data: map[string]any{
			"count":        len(points),
			"collection":   n.Collection,
			"hybrid":       useHybrid,
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

	model := e.resolveDenseModel(n.Model)
	sparseModel := e.resolveSparseModel(n.SparseModel)

	hasRerankVector, err := e.collectionHasRerankVector(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}

	limit := uint64(n.Limit)
	if limit == 0 {
		limit = 10
	}

	fetchLimit := effectiveSearchLimit(limit, n.Rerank)

	searchReq, err := e.buildSearchRequest(ctx, n, model, sparseModel, hasRerankVector, fetchLimit)
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

func (e *Executor) doRecommend(n *ast.RecommendStmt) (*ExecResponse, error) {
	ctx := context.Background()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	query := qdrant.NewQueryRecommend(&qdrant.RecommendInput{
		Positive: buildRecommendVectorInputs(n.PositiveIDs),
		Negative: buildRecommendVectorInputs(n.NegativeIDs),
	})
	if n.Strategy != nil && *n.Strategy != "" {
		strategy, ok := recommendStrategy(*n.Strategy)
		if !ok {
			return nil, fmt.Errorf("unknown recommend strategy '%s'", *n.Strategy)
		}
		query = qdrant.NewQueryRecommend(&qdrant.RecommendInput{
			Positive: buildRecommendVectorInputs(n.PositiveIDs),
			Negative: buildRecommendVectorInputs(n.NegativeIDs),
			Strategy: strategy.Enum(),
		})
	}

	req := &qdrant.QueryPoints{
		CollectionName: n.Collection,
		Query:          query,
		Limit:          qdrant.PtrOf(uint64(n.Limit)),
		Using:          qdrant.PtrOf(denseVectorName),
	}
	if n.QueryFilter != nil {
		filter, err := filters.NewFilterConverter().BuildFilter(n.QueryFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to build filter: %w", err)
		}
		req.Filter = addExcludedIDsToFilter(filter, append(append([]interface{}{}, n.PositiveIDs...), n.NegativeIDs...))
	} else {
		req.Filter = addExcludedIDsToFilter(nil, append(append([]interface{}{}, n.PositiveIDs...), n.NegativeIDs...))
	}

	results, err := e.client.Query(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("recommend failed: %w", err)
	}

	message, hits := e.formatSearchResults(results)
	return &ExecResponse{
		OK:        true,
		Operation: "recommend",
		Message:   message,
		Data: map[string]any{
			"count":      len(hits),
			"results":    hits,
			"collection": n.Collection,
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

func collectionVectorParams(denseSize int, includeRerank bool) map[string]*qdrant.VectorParams {
	vectors := map[string]*qdrant.VectorParams{
		denseVectorName: {
			Size:     uint64(denseSize),
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

func (e *Executor) buildInsertVectors(ctx context.Context, text, denseModel, sparseModel string, includeSparse, includeRerank bool, collection string) (map[string]*qdrant.Vector, error) {
	if e.usesLocalEmbeddings() {
		embedClient, err := e.embeddingClient(denseModel)
		if err != nil {
			return nil, err
		}
		denseVector, err := embedClient.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed insert text: %w", err)
		}
		vectors := map[string]*qdrant.Vector{
			denseVectorName: qdrant.NewVectorDense(denseVector),
		}
		if includeSparse {
			stats := e.loadCorpusStats(collection)
			sv := sparse.BuildDocument(text, stats)
			vectors[sparseVectorName] = qdrant.NewVectorSparse(sv.Indices, sv.Values)
			stats.Update(sparse.Tokenize(text))
			e.saveCorpusStats(collection, stats)
		}
		if includeRerank {
			return nil, fmt.Errorf("local/external rerank vectors are not implemented yet")
		}
		return vectors, nil
	}

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
	return vectors, nil
}

func (e *Executor) buildInsertVectorsBatch(ctx context.Context, texts []string, denseModel, sparseModel string, includeSparse, includeRerank bool, collection string) ([]map[string]*qdrant.Vector, error) {
	if e.usesLocalEmbeddings() {
		embedClient, err := e.embeddingClient(denseModel)
		if err != nil {
			return nil, err
		}
		denseVectors, err := embedClient.EmbedBatch(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("failed to embed insert texts: %w", err)
		}
		if includeRerank {
			return nil, fmt.Errorf("local/external rerank vectors are not implemented yet")
		}

		// Build corpus stats from the batch for BM25 weighting
		var stats *sparse.CorpusStats
		if includeSparse {
			stats = e.loadCorpusStats(collection)
			docTokens := make([][]string, len(texts))
			for i, text := range texts {
				docTokens[i] = sparse.Tokenize(text)
			}
			stats.UpdateBatch(docTokens)
		}

		batch := make([]map[string]*qdrant.Vector, 0, len(texts))
		for idx, text := range texts {
			vectors := map[string]*qdrant.Vector{
				denseVectorName: qdrant.NewVectorDense(denseVectors[idx]),
			}
			if includeSparse {
				sv := sparse.BuildDocument(text, stats)
				vectors[sparseVectorName] = qdrant.NewVectorSparse(sv.Indices, sv.Values)
			}
			batch = append(batch, vectors)
		}

		if includeSparse {
			e.saveCorpusStats(collection, stats)
		}
		return batch, nil
	}

	batch := make([]map[string]*qdrant.Vector, 0, len(texts))
	for _, text := range texts {
		vectors, err := e.buildInsertVectors(ctx, text, denseModel, sparseModel, includeSparse, includeRerank, collection)
		if err != nil {
			return nil, err
		}
		batch = append(batch, vectors)
	}
	return batch, nil
}

func (e *Executor) buildSearchRequest(ctx context.Context, n *ast.SearchStmt, denseModel, sparseModel string, hasRerankVector bool, limit uint64) (*qdrant.QueryPoints, error) {
	params := searchParamsFromWithClause(n.WithClause)

	if n.Rerank {
		if e.usesLocalEmbeddings() {
			return nil, fmt.Errorf("RERANK is currently only available in cloud inference mode")
		}
		if !hasRerankVector {
			return nil, fmt.Errorf("collection '%s' does not support rerank; create it with HYBRID RERANK", n.Collection)
		}
		rerankModel := rerankModelDefault
		if n.RerankModel != nil && *n.RerankModel != "" {
			rerankModel = *n.RerankModel
		}
		prefetch, err := e.buildSearchPrefetches(ctx, n.QueryText, denseModel, sparseModel, limit, params)
		if err != nil {
			return nil, err
		}
		return buildRerankSearchRequest(n.Collection, n.QueryText, rerankModel, limit, prefetch, params), nil
	}

	if n.Hybrid {
		prefetch, err := e.buildSearchPrefetches(ctx, n.QueryText, denseModel, sparseModel, limit, params)
		if err != nil {
			return nil, err
		}
		return &qdrant.QueryPoints{
			CollectionName: n.Collection,
			Prefetch:       prefetch,
			Query:          qdrant.NewQueryFusion(qdrant.Fusion_RRF),
			Limit:          qdrant.PtrOf(limit),
			Params:         params,
		}, nil
	}

	if n.SparseOnly {
		query := qdrant.NewQueryDocument(&qdrant.Document{
			Text:  n.QueryText,
			Model: sparseModel,
		})
		if e.usesLocalEmbeddings() {
			sv := sparse.BuildQuery(n.QueryText)
			query = qdrant.NewQuerySparse(sv.Indices, sv.Values)
		}
		return &qdrant.QueryPoints{
			CollectionName: n.Collection,
			Query:          query,
			Using:          qdrant.PtrOf(sparseVectorName),
			Limit:          qdrant.PtrOf(limit),
			Params:         params,
		}, nil
	}

	query := qdrant.NewQueryDocument(&qdrant.Document{
		Text:  n.QueryText,
		Model: denseModel,
	})
	if e.usesLocalEmbeddings() {
		embedClient, err := e.embeddingClient(denseModel)
		if err != nil {
			return nil, err
		}
		denseVector, err := embedClient.Embed(ctx, n.QueryText)
		if err != nil {
			return nil, fmt.Errorf("failed to embed search query: %w", err)
		}
		query = qdrant.NewQueryDense(denseVector)
	}

	return &qdrant.QueryPoints{
		CollectionName: n.Collection,
		Query:          query,
		Using:          qdrant.PtrOf(denseVectorName),
		Limit:          qdrant.PtrOf(limit),
		Params:         params,
	}, nil
}

func (e *Executor) buildSearchPrefetches(ctx context.Context, queryText, denseModel, sparseModel string, limit uint64, params *qdrant.SearchParams) ([]*qdrant.PrefetchQuery, error) {
	denseQuery := qdrant.NewQueryDocument(&qdrant.Document{
		Text:  queryText,
		Model: denseModel,
	})
	sparseQuery := qdrant.NewQueryDocument(&qdrant.Document{
		Text:  queryText,
		Model: sparseModel,
	})
	if e.usesLocalEmbeddings() {
		embedClient, err := e.embeddingClient(denseModel)
		if err != nil {
			return nil, fmt.Errorf("failed to create embedding client for search: %w", err)
		}
		denseVector, err := embedClient.Embed(ctx, queryText)
		if err != nil {
			return nil, fmt.Errorf("failed to embed search query: %w", err)
		}
		denseQuery = qdrant.NewQueryDense(denseVector)
		sv := sparse.BuildQuery(queryText)
		sparseQuery = qdrant.NewQuerySparse(sv.Indices, sv.Values)
	}

	return []*qdrant.PrefetchQuery{
		{
			Query:  sparseQuery,
			Using:  qdrant.PtrOf(sparseVectorName),
			Limit:  qdrant.PtrOf(limit),
			Params: params,
		},
		{
			Query:  denseQuery,
			Using:  qdrant.PtrOf(denseVectorName),
			Limit:  qdrant.PtrOf(limit),
			Params: params,
		},
	}, nil
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

func (e *Executor) resolveDenseModel(override *string) string {
	if override != nil && *override != "" {
		return *override
	}
	if e != nil && e.config != nil {
		if e.config.EmbeddingModel != "" {
			return e.config.EmbeddingModel
		}
		if e.config.InferenceModel != "" {
			return e.config.InferenceModel
		}
	}
	return denseModelDefault
}

func (e *Executor) resolveSparseModel(override *string) string {
	if override != nil && *override != "" {
		return *override
	}
	return sparseModelDefault
}

func (e *Executor) inferenceMode() string {
	if e != nil && e.config != nil && strings.TrimSpace(e.config.InferenceMode) != "" {
		return strings.ToLower(strings.TrimSpace(e.config.InferenceMode))
	}
	return defaultInferenceMode
}

func (e *Executor) usesLocalEmbeddings() bool {
	mode := e.inferenceMode()
	return mode == "local" || mode == "external"
}

func (e *Executor) embeddingClient(model string) (*embedding.Client, error) {
	if e == nil || e.config == nil {
		return nil, fmt.Errorf("embedding configuration is missing")
	}
	if e.config.EmbeddingDimension <= 0 {
		return nil, fmt.Errorf("embedding_dimension must be configured for %s inference mode", e.inferenceMode())
	}
	endpoint := strings.TrimSpace(e.config.EmbeddingEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("embedding_endpoint must be configured for %s inference mode", e.inferenceMode())
	}
	return embedding.NewClient(embedding.Config{
		Endpoint:  endpoint,
		Model:     model,
		APIKey:    e.config.EmbeddingAPIKey,
		Dimension: e.config.EmbeddingDimension,
	})
}

func (e *Executor) corpusStatsPath(collection string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	// Sanitize collection name to prevent directory traversal
	safe := strings.ReplaceAll(collection, "/", "_")
	safe = strings.ReplaceAll(safe, "\\", "_")
	safe = strings.ReplaceAll(safe, "..", "_")
	return filepath.Join(home, ".qql", "corpus", safe+".json")
}

func (e *Executor) loadCorpusStats(collection string) *sparse.CorpusStats {
	path := e.corpusStatsPath(collection)
	if path == "" {
		return sparse.NewCorpusStats()
	}
	stats, err := sparse.LoadCorpusStats(path)
	if err != nil {
		return sparse.NewCorpusStats()
	}
	return stats
}

func (e *Executor) saveCorpusStats(collection string, stats *sparse.CorpusStats) {
	path := e.corpusStatsPath(collection)
	if path == "" {
		return
	}
	_ = stats.Save(path)
}

func (e *Executor) deleteCorpusStats(collection string) {
	path := e.corpusStatsPath(collection)
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

func (e *Executor) resolveDenseVectorSize(ctx context.Context, model *string) (int, error) {
	if e.usesLocalEmbeddings() {
		if e.config != nil && e.config.EmbeddingDimension > 0 {
			return e.config.EmbeddingDimension, nil
		}
		// Auto-probe dimension from embedding endpoint when not configured
		if e.config != nil && e.config.EmbeddingEndpoint != "" && e.config.EmbeddingModel != "" {
			probeClient, err := embedding.NewClient(embedding.Config{
				Endpoint:   e.config.EmbeddingEndpoint,
				Model:      e.config.EmbeddingModel,
				APIKey:     e.config.EmbeddingAPIKey,
				Dimension:  1, // ignored by ProbeDimension
				HTTPClient: nil,
			})
			if err != nil {
				return 0, fmt.Errorf("failed to create probe client: %w", err)
			}
			dim, err := probeClient.ProbeDimension(ctx, "probe")
			if err != nil {
				return 0, fmt.Errorf("failed to probe embedding dimension (set --embedding-dimension or ensure endpoint is reachable): %w", err)
			}
			return dim, nil
		}
		return 0, fmt.Errorf("embedding_dimension must be configured for %s inference mode", e.inferenceMode())
	}
	if e != nil && e.config != nil && e.config.EmbeddingDimension > 0 {
		return e.config.EmbeddingDimension, nil
	}
	_ = ctx
	if model != nil && *model != "" && e != nil && e.config != nil && e.config.EmbeddingDimension == 0 {
		return 0, fmt.Errorf("embedding_dimension must be configured when creating collections with USING MODEL")
	}
	return denseVectorSize, nil
}

func (e *Executor) shouldUseHybrid(ctx context.Context, collection string, requested bool) (bool, error) {
	if requested {
		return true, nil
	}
	return e.collectionHasSparseVector(ctx, collection)
}

func insertPointIDAndPayload(pointID interface{}, values map[string]interface{}) (interface{}, map[string]interface{}, error) {
	payload := make(map[string]interface{}, len(values))
	for key, value := range values {
		payload[key] = value
	}
	rawID := pointID
	if rawID == nil {
		var ok bool
		rawID, ok = payload["id"]
		if ok {
			delete(payload, "id")
		}
	}
	if rawID == nil {
		return uuid.New().String(), payload, nil
	}
	switch value := rawID.(type) {
	case int:
		if value < 0 {
			return nil, nil, fmt.Errorf("INSERT id must be an unsigned integer or UUID string when provided")
		}
		return value, payload, nil
	case string:
		if _, err := uuid.Parse(value); err != nil {
			return nil, nil, fmt.Errorf("INSERT id must be an unsigned integer or UUID string when provided")
		}
		return value, payload, nil
	default:
		return nil, nil, fmt.Errorf("INSERT id must be an unsigned integer or UUID string when provided")
	}
}

func newPointID(value interface{}) *qdrant.PointId {
	switch id := value.(type) {
	case int:
		return qdrant.NewIDNum(uint64(id))
	case uint64:
		return qdrant.NewIDNum(id)
	case string:
		return qdrant.NewIDUUID(id)
	default:
		return qdrant.NewIDUUID(fmt.Sprintf("%v", value))
	}
}

func buildRecommendVectorInputs(ids []interface{}) []*qdrant.VectorInput {
	if len(ids) == 0 {
		return nil
	}
	inputs := make([]*qdrant.VectorInput, 0, len(ids))
	for _, id := range ids {
		inputs = append(inputs, qdrant.NewVectorInputID(newPointID(id)))
	}
	return inputs
}

func recommendStrategy(value string) (qdrant.RecommendStrategy, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "average_vector":
		return qdrant.RecommendStrategy_AverageVector, true
	case "best_score":
		return qdrant.RecommendStrategy_BestScore, true
	case "sum_scores":
		return qdrant.RecommendStrategy_SumScores, true
	default:
		return 0, false
	}
}

func addExcludedIDsToFilter(filter *qdrant.Filter, ids []interface{}) *qdrant.Filter {
	if len(ids) == 0 {
		return filter
	}
	pointIDs := make([]*qdrant.PointId, 0, len(ids))
	for _, id := range ids {
		pointIDs = append(pointIDs, newPointID(id))
	}
	exclude := &qdrant.Condition{
		ConditionOneOf: &qdrant.Condition_HasId{
			HasId: &qdrant.HasIdCondition{
				HasId: pointIDs,
			},
		},
	}
	if filter == nil {
		return &qdrant.Filter{
			MustNot: []*qdrant.Condition{exclude},
		}
	}
	filter.MustNot = append(filter.MustNot, exclude)
	return filter
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
	vectors := info.GetConfig().GetParams().GetVectorsConfig().GetParamsMap()
	if vectors == nil {
		return false, nil
	}
	_, ok := vectors.GetMap()[rerankVectorName]
	return ok, nil
}

func (e *Executor) collectionHasSparseVector(ctx context.Context, collection string) (bool, error) {
	exists, err := e.client.CollectionExists(ctx, collection)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	info, err := e.client.GetCollectionInfo(ctx, collection)
	if err != nil {
		return false, err
	}
	sparseVectors := info.GetConfig().GetParams().GetSparseVectorsConfig()
	if sparseVectors == nil {
		return false, nil
	}
	_, ok := sparseVectors.GetMap()[sparseVectorName]
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
		return nil, fmt.Errorf("not connected. Run: qql-go connect --url <url>")
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
	return fmt.Sprintf("qql-go %s", displayVersion())
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
			inferenceMode, _ := cmd.Flags().GetString("inference-mode")
			embeddingEndpoint, _ := cmd.Flags().GetString("embedding-endpoint")
			embeddingKey, _ := cmd.Flags().GetString("embedding-key")
			embeddingModel, _ := cmd.Flags().GetString("embedding-model")
			embeddingDimension, _ := cmd.Flags().GetInt("embedding-dimension")

			if url == "" {
				return commandError(out, mode, "connect", "", fmt.Errorf("--url is required"))
			}
			if inferenceMode == "" {
				inferenceMode = defaultInferenceMode
			}
		if (inferenceMode == "local" || inferenceMode == "external") && (embeddingEndpoint == "" || embeddingModel == "") {
			return commandError(out, mode, "connect", "", fmt.Errorf("--embedding-endpoint and --embedding-model are required for %s mode", inferenceMode))
		}

		// Auto-probe embedding dimension if not provided
		if (inferenceMode == "local" || inferenceMode == "external") && embeddingDimension <= 0 && embeddingEndpoint != "" && embeddingModel != "" {
			if !mode.json && !mode.quiet {
				out.Print("Probing embedding dimension from endpoint...")
			}
			probeClient, probeErr := embedding.NewClient(embedding.Config{
				Endpoint:  embeddingEndpoint,
				Model:     embeddingModel,
				APIKey:    embeddingKey,
				Dimension: 1,
			})
			if probeErr == nil {
				dim, probeErr := probeClient.ProbeDimension(context.Background(), "probe")
				if probeErr == nil {
					embeddingDimension = dim
					if !mode.json && !mode.quiet {
						out.Print(fmt.Sprintf("Auto-detected embedding dimension: %d", dim))
					}
				}
			}
			if probeErr != nil && !mode.json && !mode.quiet {
				out.Print(fmt.Sprintf("Warning: could not probe embedding dimension: %v", probeErr))
			}
		}
		if (inferenceMode == "local" || inferenceMode == "external") && embeddingDimension <= 0 {
			return commandError(out, mode, "connect", "", fmt.Errorf("--embedding-dimension is required (or endpoint must be reachable for auto-probe) for %s mode", inferenceMode))
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
			URL:                url,
			Secret:             secret,
			InferenceMode:      inferenceMode,
			EmbeddingEndpoint:  embeddingEndpoint,
			EmbeddingAPIKey:    embeddingKey,
			EmbeddingModel:     embeddingModel,
			EmbeddingDimension: embeddingDimension,
		}

		// Validate embedding endpoint is reachable in local/external mode
		if (inferenceMode == "local" || inferenceMode == "external") && embeddingEndpoint != "" {
			testClient, testErr := embedding.NewClient(embedding.Config{
				Endpoint:  embeddingEndpoint,
				Model:     embeddingModel,
				APIKey:    embeddingKey,
				Dimension: embeddingDimension,
			})
			if testErr == nil {
				_, testErr = testClient.Embed(context.Background(), "test")
			}
			if testErr != nil && !mode.json && !mode.quiet {
				out.Print(fmt.Sprintf("Warning: embedding endpoint test failed: %v", testErr))
			}
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
	cmd.Flags().String("inference-mode", defaultInferenceMode, "Inference mode: cloud, external, or local")
	cmd.Flags().String("embedding-endpoint", "", "OpenAI-compatible embeddings endpoint for local/external modes")
	cmd.Flags().String("embedding-key", "", "API key for the embeddings endpoint")
	cmd.Flags().String("embedding-model", "", "Embedding model name for local/external modes")
	cmd.Flags().Int("embedding-dimension", 0, "Embedding dimension for local/external modes")
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

func NewExecuteCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute a .qql script file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			stopOnError, _ := cmd.Flags().GetBool("stop-on-error")
			cfg, client, err := loadSavedConfigAndClient()
			if err != nil {
				return commandError(out, mode, "execute", args[0], err)
			}
			executor := NewExecutor(client, cfg)
			okCount, failCount, err := script.RunFile(args[0], executor, stopOnError)
			if err != nil {
				return commandError(out, mode, "execute", args[0], err)
			}
			message := fmt.Sprintf("Executed script %s (%d succeeded, %d failed)", args[0], okCount, failCount)
			if mode.json {
				return writeJSON(out, &ScriptResponse{
					OK:        true,
					Command:   "execute",
					Path:      args[0],
					Succeeded: okCount,
					Failed:    failCount,
					Message:   message,
				}, mode.quiet)
			}
			out.Print(message)
			return nil
		},
	}
	cmd.Flags().Bool("stop-on-error", false, "Stop after the first failing statement")
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

func NewDumpCmd(out *output.Outputter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump a collection to a .qql script file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := readOutputMode(cmd)
			cfg, client, err := loadSavedConfigAndClient()
			if err != nil {
				return commandError(out, mode, "dump", strings.Join(args, " "), err)
			}
			_ = cfg
			written, skipped, err := dump.Collection(context.Background(), client, args[0], args[1])
			if err != nil {
				return commandError(out, mode, "dump", strings.Join(args, " "), err)
			}
			message := fmt.Sprintf("Dumped collection '%s' to %s (%d written, %d skipped)", args[0], args[1], written, skipped)
			if mode.json {
				return writeJSON(out, &DumpResponse{
					OK:         true,
					Command:    "dump",
					Collection: args[0],
					Path:       args[1],
					Written:    written,
					Skipped:    skipped,
					Message:    message,
				}, mode.quiet)
			}
			out.Print(message)
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
			out.PrintSection("qql-go Version", version)
			return nil
		},
	}
	addOutputFlags(cmd)
	return cmd
}
