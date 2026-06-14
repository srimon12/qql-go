package commands

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"maps"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
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

var Version = "0.2.0"

type commandOutputMode struct {
	json  bool
	quiet bool
}

type Executor struct {
	client qdrantClient
	config *config.Config
}

type qdrantClient interface {
	ListCollections(context.Context) ([]string, error)
	CollectionExists(context.Context, string) (bool, error)
	GetCollectionInfo(context.Context, string) (*qdrant.CollectionInfo, error)
	CreateCollection(context.Context, *qdrant.CreateCollection) error
	UpdateCollection(context.Context, *qdrant.UpdateCollection) error
	DeleteCollection(context.Context, string) error
	Upsert(context.Context, *qdrant.UpsertPoints) (*qdrant.UpdateResult, error)
	Query(context.Context, *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error)
	QueryGroups(context.Context, *qdrant.QueryPointGroups) ([]*qdrant.PointGroup, error)
	Delete(context.Context, *qdrant.DeletePoints) (*qdrant.UpdateResult, error)
	UpdateVectors(context.Context, *qdrant.UpdatePointVectors) (*qdrant.UpdateResult, error)
	SetPayload(context.Context, *qdrant.SetPayloadPoints) (*qdrant.UpdateResult, error)
	CreateFieldIndex(context.Context, *qdrant.CreateFieldIndexCollection) (*qdrant.UpdateResult, error)
	Count(context.Context, *qdrant.CountPoints) (uint64, error)
	ScrollAndOffset(context.Context, *qdrant.ScrollPoints) ([]*qdrant.RetrievedPoint, *qdrant.PointId, error)
	Get(context.Context, *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error)
}

func NewExecutor(client qdrantClient, cfg *config.Config) *Executor {
	return &Executor{
		client: client,
		config: cfg,
	}
}

func NewClient(cfg *config.Config) (*qdrant.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	return newClientFromURL(cfg.URL, cfg.Secret, cfg.NoVerify, cfg.CACert)
}

func newClientFromURL(rawURL, apiKey string, noVerify bool, caCert string) (*qdrant.Client, error) {
	cfg, err := buildClientConfig(rawURL, apiKey, noVerify, caCert)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	return qdrant.NewClient(cfg)
}

func buildClientConfig(rawURL, apiKey string, noVerify bool, caCert string) (*qdrant.Config, error) {
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

	var tlsConf *tls.Config
	if strings.EqualFold(parsed.Scheme, "https") {
		tlsConf = &tls.Config{MinVersion: tls.VersionTLS13}
		if noVerify {
			tlsConf.InsecureSkipVerify = true
		}
		if caCert != "" {
			certPEM, err := os.ReadFile(caCert)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA cert: %w", err)
			}
			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(certPEM) {
				return nil, fmt.Errorf("failed to parse CA cert from %s", caCert)
			}
			tlsConf.RootCAs = certPool
		}
	}

	return &qdrant.Config{
		Host:                   host,
		Port:                   port,
		APIKey:                 apiKey,
		UseTLS:                 tlsConf != nil,
		TLSConfig:              tlsConf,
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

func (e *Executor) DumpCollection(collection, outputPath string, batchSize int) (string, error) {
	written, skipped, err := dump.Collection(context.Background(), e.client, collection, outputPath, batchSize)
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
	case *ast.ShowCollectionStmt:
		return e.doShowCollection(n)
	case *ast.CreateCollectionStmt:
		return e.doCreateCollection(n)
	case *ast.AlterCollectionStmt:
		return e.doAlterCollection(n)
	case *ast.DropCollectionStmt:
		return e.doDropCollection(n)
	case *ast.InsertStmt:
		return e.doInsert(n)
	case *ast.InsertBulkStmt:
		return e.doInsertBulk(n)
	case *ast.SelectStmt:
		return e.doSelect(n)
	case *ast.ScrollStmt:
		return e.doScroll(n)
	case *ast.SearchStmt:
		return e.doSearch(n)
	case *ast.RecommendStmt:
		return e.doRecommend(n)
	case *ast.DeleteStmt:
		return e.doDelete(n)
	case *ast.UpdateVectorStmt:
		return e.doUpdateVector(n)
	case *ast.UpdatePayloadStmt:
		return e.doUpdatePayload(n)
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
	case *ast.ShowCollectionStmt:
		plan.WriteString(fmt.Sprintf("Statement: SHOW COLLECTION %s\n", n.Collection))
		plan.WriteString("Action: Inspect collection diagnostics\n")
	case *ast.CreateCollectionStmt:
		plan.WriteString(fmt.Sprintf("Statement: CREATE COLLECTION %s\n", n.Collection))
		if n.Model != nil && *n.Model != "" {
			plan.WriteString(fmt.Sprintf("Model: %s\n", *n.Model))
		}
		if n.Config != nil {
			if n.Config.Hnsw != nil && n.Config.Hnsw.PayloadM != nil {
				plan.WriteString(fmt.Sprintf("HNSW payload_m: %d\n", *n.Config.Hnsw.PayloadM))
			}
		}
		if n.Quantization != nil {
			plan.WriteString(fmt.Sprintf("Quantization: %s\n", n.Quantization.Type))
			if n.Quantization.Quantile != nil {
				plan.WriteString(fmt.Sprintf("Quantile: %.4f\n", *n.Quantization.Quantile))
			}
			if n.Quantization.TurboBits != nil {
				plan.WriteString(fmt.Sprintf("Turbo bits: %g\n", *n.Quantization.TurboBits))
			}
			if n.Quantization.AlwaysRAM {
				plan.WriteString("Quantization storage: ALWAYS RAM\n")
			}
		}
		if n.Rerank {
			plan.WriteString("Type: HYBRID + RERANK (dense + sparse + ColBERT multivector)\n")
		} else if n.Hybrid {
			plan.WriteString("Type: HYBRID (dense + sparse)\n")
		} else {
			plan.WriteString("Type: DENSE\n")
		}
		plan.WriteString("Action: Create new collection\n")
	case *ast.AlterCollectionStmt:
		plan.WriteString(fmt.Sprintf("Statement: ALTER COLLECTION %s\n", n.Collection))
		if n.Config != nil {
			if n.Config.Hnsw != nil {
				plan.WriteString("Alteration: HNSW config\n")
			}
			if n.Config.Vectors != nil {
				plan.WriteString("Alteration: Vectors config\n")
			}
			if n.Config.Optimizers != nil {
				plan.WriteString("Alteration: Optimizers config\n")
			}
			if n.Config.Params != nil {
				plan.WriteString("Alteration: Params config\n")
			}
		}
		if n.Quantization != nil {
			if n.Quantization.Disabled {
				plan.WriteString("Alteration: Disable quantization\n")
			} else {
				plan.WriteString(fmt.Sprintf("Alteration: %s quantization\n", n.Quantization.Config.Type))
			}
		}
		plan.WriteString("Action: Alter existing collection\n")
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
	case *ast.SelectStmt:
		plan.WriteString(fmt.Sprintf("Statement: SELECT * FROM %s WHERE id = '%v'\n", n.Collection, n.PointID))
		plan.WriteString("Action: Retrieve a single point by ID\n")
	case *ast.ScrollStmt:
		plan.WriteString(fmt.Sprintf("Statement: SCROLL FROM %s LIMIT %d\n", n.Collection, n.Limit))
		if n.QueryFilter != nil {
			plan.WriteString(fmt.Sprintf("Filter: %s\n", e.filterToString(n.QueryFilter)))
		}
		if n.After != nil {
			plan.WriteString(fmt.Sprintf("After: %v\n", n.After))
		}
		plan.WriteString("Action: Scroll (paginate) through points\n")
	case *ast.SearchStmt:
		plan.WriteString(fmt.Sprintf("Statement: SEARCH %s SIMILAR TO '%s' LIMIT %d\n",
			n.Collection, n.QueryText, n.Limit))
		if n.Offset > 0 {
			plan.WriteString(fmt.Sprintf("Offset: %d\n", n.Offset))
		}
		if n.ScoreThreshold != nil {
			plan.WriteString(fmt.Sprintf("Score threshold: %.4f\n", *n.ScoreThreshold))
		}
		if n.LookupFrom != "" {
			plan.WriteString(fmt.Sprintf("Lookup from: %s", n.LookupFrom))
			if n.LookupVector != nil && *n.LookupVector != "" {
				plan.WriteString(fmt.Sprintf(" (vector: %s)", *n.LookupVector))
			}
			plan.WriteString("\n")
		}
		if n.Model != nil && *n.Model != "" {
			plan.WriteString(fmt.Sprintf("Model: %s\n", *n.Model))
		}
		if n.SparseOnly {
			plan.WriteString("Search: SPARSE\n")
		} else if n.Hybrid {
			mode := "HYBRID (dense + sparse)"
			if n.Fusion != nil && *n.Fusion != "" {
				mode = fmt.Sprintf("HYBRID (dense + sparse, fusion=%s)", *n.Fusion)
			}
			plan.WriteString("Search: " + mode + "\n")
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
		if n.WithClause != nil && n.WithClause.Acorn {
			plan.WriteString("Search params: acorn=true\n")
		}
		if n.WithClause != nil && n.WithClause.IndexedOnly {
			plan.WriteString("Search params: indexed_only=true\n")
		}
		if n.WithClause != nil && n.WithClause.Quantization != nil {
			plan.WriteString("Search params: quantization enabled\n")
		}
		if n.WithClause != nil && n.WithClause.MmrDiversity != nil {
			plan.WriteString(fmt.Sprintf("Search params: mmr_diversity=%.4f\n", *n.WithClause.MmrDiversity))
		}
		if n.WithClause != nil && n.WithClause.MmrCandidates != nil {
			plan.WriteString(fmt.Sprintf("Search params: mmr_candidates=%d\n", *n.WithClause.MmrCandidates))
		}
		if n.GroupBy != "" {
			plan.WriteString(fmt.Sprintf("Group by: %s\n", n.GroupBy))
			plan.WriteString(fmt.Sprintf("Group size: %d\n", n.GroupSize))
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
		if n.LookupFrom != "" {
			plan.WriteString(fmt.Sprintf("Lookup from: %s", n.LookupFrom))
			if n.LookupVector != nil && *n.LookupVector != "" {
				plan.WriteString(fmt.Sprintf(" (vector: %s)", *n.LookupVector))
			}
			plan.WriteString("\n")
		}
		if n.Using != nil && *n.Using != "" {
			plan.WriteString(fmt.Sprintf("Using vector: %s\n", *n.Using))
		}
		if n.Offset > 0 {
			plan.WriteString(fmt.Sprintf("Offset: %d\n", n.Offset))
		}
		if n.ScoreThreshold != nil {
			plan.WriteString(fmt.Sprintf("Score threshold: %.4f\n", *n.ScoreThreshold))
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
		if n.WithClause != nil && n.WithClause.IndexedOnly {
			plan.WriteString("Search params: indexed_only=true\n")
		}
		if n.WithClause != nil && n.WithClause.Quantization != nil {
			plan.WriteString("Search params: quantization enabled\n")
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
	case *ast.UpdateVectorStmt:
		plan.WriteString(fmt.Sprintf("Statement: UPDATE %s SET VECTOR WHERE id = '%v'\n", n.Collection, n.PointID))
		plan.WriteString(fmt.Sprintf("Vector length: %d\n", len(n.Vector)))
		plan.WriteString("Action: Update point vector\n")
	case *ast.UpdatePayloadStmt:
		if n.QueryFilter != nil {
			plan.WriteString(fmt.Sprintf("Statement: UPDATE %s SET PAYLOAD WHERE %s\n", n.Collection, e.filterToString(n.QueryFilter)))
			plan.WriteString("Action: Update payload for points matching filter\n")
		} else {
			plan.WriteString(fmt.Sprintf("Statement: UPDATE %s SET PAYLOAD WHERE id = '%v'\n", n.Collection, n.PointID))
			plan.WriteString("Action: Update payload for point by ID\n")
		}
	case *ast.CreateIndexStmt:
		plan.WriteString(fmt.Sprintf("Statement: CREATE INDEX ON COLLECTION %s FOR %s", n.Collection, n.Field))
		if n.FieldType != "" && n.FieldType != "keyword" {
			plan.WriteString(fmt.Sprintf(" TYPE %s", n.FieldType))
		}
		plan.WriteString("\n")
		if len(n.Options) > 0 {
			plan.WriteString(fmt.Sprintf("Options: %v\n", n.Options))
		}
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

func (e *Executor) doShowCollection(n *ast.ShowCollectionStmt) (*ExecResponse, error) {
	ctx := context.Background()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	info, err := e.client.GetCollectionInfo(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}

	config := info.GetConfig()
	if config == nil {
		return nil, fmt.Errorf("collection '%s' has no configuration", n.Collection)
	}
	params := config.GetParams()
	if params == nil || params.GetVectorsConfig() == nil {
		return nil, fmt.Errorf("collection '%s' has no vector configuration", n.Collection)
	}

	data := e.extractCollectionDiagnostics(n.Collection, info, config, params)

	return &ExecResponse{
		OK:        true,
		Operation: "show_collection",
		Message:   formatCollectionDiagnostics(data),
		Data:      data,
	}, nil
}

func formatCollectionDiagnostics(data map[string]any) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Collection: %v\n", data["name"])
	fmt.Fprintf(&b, "  Status               : %v\n", data["status"])
	fmt.Fprintf(&b, "  Points               : %v\n", data["points_count"])
	fmt.Fprintf(&b, "  Indexed vectors      : %v\n", data["indexed_vectors_count"])
	fmt.Fprintf(&b, "  Segments             : %v\n", data["segments_count"])
	fmt.Fprintf(&b, "  Topology             : %v\n", data["topology"])

	if vectors, ok := data["vectors"].(map[string]map[string]any); ok {
		for vname, vconf := range vectors {
			label := fmt.Sprintf("  Vector '%s'", vname)
			if vname == "" {
				label = "  Vector"
			}
			fmt.Fprintf(&b, "%s        : %v dims, %v distance", label, vconf["size"], vconf["distance"])
			if onDisk, ok := vconf["on_disk"]; ok && onDisk != nil {
				fmt.Fprintf(&b, ", on_disk=%v", onDisk)
			}
			if inlineStorage, ok := vconf["hnsw_inline_storage"]; ok && inlineStorage != nil {
				fmt.Fprintf(&b, ", hnsw_inline_storage=%v", inlineStorage)
			}
			b.WriteString("\n")
		}
	}

	if sparseVectors, ok := data["sparse_vectors"].(map[string]map[string]any); ok && len(sparseVectors) > 0 {
		for sname, sconf := range sparseVectors {
			fmt.Fprintf(&b, "  Sparse '%s'          : modifier=%v\n", sname, sconf["modifier"])
		}
	}

	quantization := data["quantization"]
	if quantization == nil {
		quantization = "none"
	}
	fmt.Fprintf(&b, "  Quantization         : %v\n", quantization)

	if hnsw, ok := data["hnsw_config"].(map[string]any); ok && len(hnsw) > 0 {
		fmt.Fprintf(&b, "  HNSW M               : %v\n", hnsw["m"])
		fmt.Fprintf(&b, "  HNSW ef_construct    : %v\n", hnsw["ef_construct"])
		if v, ok := hnsw["full_scan_threshold"]; ok && v != nil {
			fmt.Fprintf(&b, "  HNSW full_scan_thres : %v\n", v)
		}
		if v, ok := hnsw["max_indexing_threads"]; ok && v != nil {
			fmt.Fprintf(&b, "  HNSW max_idx_threads : %v\n", v)
		}
		if v, ok := hnsw["on_disk"]; ok && v != nil {
			fmt.Fprintf(&b, "  HNSW on_disk         : %v\n", v)
		}
		if v, ok := hnsw["payload_m"]; ok && v != nil {
			fmt.Fprintf(&b, "  HNSW payload_m       : %v\n", v)
		}
		if v, ok := hnsw["inline_storage"]; ok && v != nil {
			fmt.Fprintf(&b, "  HNSW inline_storage  : %v\n", v)
		}
	}

	if schema, ok := data["payload_schema"].(map[string]any); ok && len(schema) > 0 {
		b.WriteString("  Payload indexes:\n")
		for field, raw := range schema {
			if entry, ok := raw.(map[string]any); ok {
				line := fmt.Sprintf("    %s: %v", field, entry["type"])
				if params, ok := entry["params"].(map[string]any); ok && len(params) > 0 {
					rendered := make([]string, 0, len(params))
					for key, value := range params {
						rendered = append(rendered, fmt.Sprintf("%s=%v", key, value))
					}
					line += " (" + strings.Join(rendered, ", ") + ")"
				}
				b.WriteString(line + "\n")
				continue
			}
			fmt.Fprintf(&b, "    %s: %v\n", field, raw)
		}
	} else {
		b.WriteString("  Payload indexes      : none\n")
	}

	if sh, ok := data["sharding"].(map[string]any); ok {
		fmt.Fprintf(&b, "  Shards               : %v\n", sh["shard_number"])
		fmt.Fprintf(&b, "  Replication factor   : %v\n", sh["replication_factor"])
		fmt.Fprintf(&b, "  Write consistency    : %v\n", sh["write_consistency_factor"])
		if v, ok := sh["read_fan_out_factor"]; ok && v != nil {
			fmt.Fprintf(&b, "  Read fan-out factor  : %v\n", v)
		}
		if v, ok := sh["read_fan_out_delay_ms"]; ok && v != nil {
			fmt.Fprintf(&b, "  Read fan-out delay ms: %v\n", v)
		}
		if v, ok := sh["on_disk_payload"]; ok && v != nil {
			fmt.Fprintf(&b, "  On-disk payload      : %v\n", v)
		}
	}

	return b.String()
}

func (e *Executor) extractCollectionDiagnostics(
	collectionName string,
	info *qdrant.CollectionInfo,
	config *qdrant.CollectionConfig,
	params *qdrant.CollectionParams,
) map[string]any {
	// Vector topology
	vectorsConfig := params.GetVectorsConfig()
	sparseVectorsConfig := params.GetSparseVectorsConfig()

	vectorDetails := make(map[string]map[string]any)
	if vectorsConfig != nil {
		if paramsMap := vectorsConfig.GetParamsMap(); paramsMap != nil {
			for vname, vconfig := range paramsMap.GetMap() {
				entry := map[string]any{
					"size":     vconfig.Size,
					"distance": vconfig.Distance.String(),
				}
				if vconfig.OnDisk != nil {
					entry["on_disk"] = vconfig.GetOnDisk()
				}
				if vconfig.HnswConfig != nil && vconfig.HnswConfig.InlineStorage != nil {
					entry["hnsw_inline_storage"] = vconfig.HnswConfig.GetInlineStorage()
				}
				vectorDetails[vname] = entry
			}
		} else if singleParams := vectorsConfig.GetParams(); singleParams != nil {
			entry := map[string]any{
				"size":     singleParams.Size,
				"distance": singleParams.Distance.String(),
			}
			if singleParams.OnDisk != nil {
				entry["on_disk"] = singleParams.GetOnDisk()
			}
			if singleParams.HnswConfig != nil && singleParams.HnswConfig.InlineStorage != nil {
				entry["hnsw_inline_storage"] = singleParams.HnswConfig.GetInlineStorage()
			}
			vectorDetails[""] = entry
		}
	}

	topology := "dense"
	if sparseVectorsConfig != nil && len(sparseVectorsConfig.GetMap()) > 0 {
		topology = "hybrid"
	}

	// Sparse vector config
	sparseVectors := make(map[string]map[string]any)
	if sparseVectorsConfig != nil {
		for sname, sconfig := range sparseVectorsConfig.GetMap() {
			modifier := "none"
			if sconfig.Modifier != nil {
				modifier = sconfig.Modifier.String()
			}
			sparseVectors[sname] = map[string]any{
				"modifier": modifier,
			}
		}
	}
	if len(sparseVectors) == 0 {
		sparseVectors = nil
	}

	// Quantization
	var quantization any
	if qtype := detectQuantizationType(config.GetQuantizationConfig()); qtype != "" {
		quantization = qtype
	}

	// HNSW config
	hnswConfig := config.GetHnswConfig()
	var hnsw map[string]any
	if hnswConfig != nil {
		hnsw = map[string]any{
			"m":            hnswConfig.GetM(),
			"ef_construct": hnswConfig.GetEfConstruct(),
		}
		if hnswConfig.FullScanThreshold != nil {
			hnsw["full_scan_threshold"] = hnswConfig.GetFullScanThreshold()
		}
		if hnswConfig.MaxIndexingThreads != nil {
			hnsw["max_indexing_threads"] = hnswConfig.GetMaxIndexingThreads()
		}
		if hnswConfig.OnDisk != nil {
			hnsw["on_disk"] = hnswConfig.GetOnDisk()
		}
		if hnswConfig.PayloadM != nil {
			hnsw["payload_m"] = hnswConfig.GetPayloadM()
		}
		if hnswConfig.InlineStorage != nil {
			hnsw["inline_storage"] = hnswConfig.GetInlineStorage()
		}
	}

	// Payload schema / indexes
	payloadIndexes := make(map[string]any)
	for fieldName, idxInfo := range info.PayloadSchema {
		payloadIndexes[fieldName] = serializePayloadSchemaInfo(idxInfo)
	}
	if len(payloadIndexes) == 0 {
		payloadIndexes = nil
	}

	// Sharding / replication
	replicationFactor := params.GetReplicationFactor()
	writeConsistencyFactor := params.GetWriteConsistencyFactor()

	sharding := map[string]any{
		"shard_number":             params.ShardNumber,
		"replication_factor":       replicationFactor,
		"write_consistency_factor": writeConsistencyFactor,
		"read_fan_out_factor":      params.GetReadFanOutFactor(),
		"read_fan_out_delay_ms":    params.GetReadFanOutDelayMs(),
		"on_disk_payload":          params.GetOnDiskPayload(),
	}

	var pointsCountVal any
	var indexedVectorsCountVal any
	if info.PointsCount != nil {
		pointsCountVal = *info.PointsCount
	}
	if info.IndexedVectorsCount != nil {
		indexedVectorsCountVal = *info.IndexedVectorsCount
	}

	return map[string]any{
		"name":                  collectionName,
		"status":                info.Status.String(),
		"points_count":          pointsCountVal,
		"indexed_vectors_count": indexedVectorsCountVal,
		"segments_count":        info.SegmentsCount,
		"topology":              topology,
		"vectors":               vectorDetails,
		"sparse_vectors":        sparseVectors,
		"quantization":          quantization,
		"hnsw_config":           hnsw,
		"payload_schema":        payloadIndexes,
		"sharding":              sharding,
	}
}

func detectQuantizationType(qc *qdrant.QuantizationConfig) string {
	if qc == nil {
		return ""
	}
	switch qc.Quantization.(type) {
	case *qdrant.QuantizationConfig_Scalar:
		return "scalar"
	case *qdrant.QuantizationConfig_Binary:
		return "binary"
	case *qdrant.QuantizationConfig_Product:
		return "product"
	case *qdrant.QuantizationConfig_Turboquant:
		return "turbo"
	}
	return ""
}

func serializePayloadSchemaInfo(idxInfo *qdrant.PayloadSchemaInfo) map[string]any {
	data := map[string]any{
		"type": strings.ToLower(strings.TrimPrefix(idxInfo.GetDataType().String(), "PayloadSchemaType_")),
	}
	if params := idxInfo.GetParams(); params != nil {
		if serialized := serializePayloadIndexParams(params); len(serialized) > 0 {
			data["params"] = serialized
		}
	}
	return data
}

func serializePayloadIndexParams(params *qdrant.PayloadIndexParams) map[string]any {
	switch typed := params.GetIndexParams().(type) {
	case *qdrant.PayloadIndexParams_KeywordIndexParams:
		return serializeKeywordIndexParams(typed.KeywordIndexParams)
	case *qdrant.PayloadIndexParams_TextIndexParams:
		return serializeTextIndexParams(typed.TextIndexParams)
	case *qdrant.PayloadIndexParams_UuidIndexParams:
		return serializeUUIDIndexParams(typed.UuidIndexParams)
	default:
		return nil
	}
}

func serializeKeywordIndexParams(params *qdrant.KeywordIndexParams) map[string]any {
	if params == nil {
		return nil
	}
	data := map[string]any{}
	if params.IsTenant != nil {
		data["is_tenant"] = params.GetIsTenant()
	}
	if params.OnDisk != nil {
		data["on_disk"] = params.GetOnDisk()
	}
	if params.EnableHnsw != nil {
		data["enable_hnsw"] = params.GetEnableHnsw()
	}
	return data
}

func serializeUUIDIndexParams(params *qdrant.UuidIndexParams) map[string]any {
	if params == nil {
		return nil
	}
	data := map[string]any{}
	if params.IsTenant != nil {
		data["is_tenant"] = params.GetIsTenant()
	}
	if params.OnDisk != nil {
		data["on_disk"] = params.GetOnDisk()
	}
	if params.EnableHnsw != nil {
		data["enable_hnsw"] = params.GetEnableHnsw()
	}
	return data
}

func serializeTextIndexParams(params *qdrant.TextIndexParams) map[string]any {
	if params == nil {
		return nil
	}
	data := map[string]any{}
	if params.Tokenizer != qdrant.TokenizerType_Unknown {
		data["tokenizer"] = strings.ToLower(strings.TrimPrefix(params.Tokenizer.String(), "TokenizerType_"))
	}
	if params.MinTokenLen != nil {
		data["min_token_len"] = params.GetMinTokenLen()
	}
	if params.MaxTokenLen != nil {
		data["max_token_len"] = params.GetMaxTokenLen()
	}
	if params.Lowercase != nil {
		data["lowercase"] = params.GetLowercase()
	}
	if params.AsciiFolding != nil {
		data["ascii_folding"] = params.GetAsciiFolding()
	}
	if params.PhraseMatching != nil {
		data["phrase_matching"] = params.GetPhraseMatching()
	}
	if params.OnDisk != nil {
		data["on_disk"] = params.GetOnDisk()
	}
	if params.EnableHnsw != nil {
		data["enable_hnsw"] = params.GetEnableHnsw()
	}
	if params.Stopwords != nil {
		if len(params.Stopwords.Languages) > 0 {
			data["stopwords"] = params.Stopwords.Languages
		} else if len(params.Stopwords.Custom) > 0 {
			data["stopwords"] = params.Stopwords.Custom
		}
	}
	return data
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
	if n.Config != nil {
		if n.Config.Vectors != nil && n.Config.Vectors.OnDisk != nil {
			for _, v := range collection.GetVectorsConfig().GetParamsMap().GetMap() {
				v.OnDisk = n.Config.Vectors.OnDisk
			}
		}
		if n.Config.Hnsw != nil {
			collection.HnswConfig = buildHnswConfigDiff(n.Config.Hnsw)
		}
		if n.Config.Optimizers != nil {
			collection.OptimizersConfig = buildOptimizersConfigDiff(n.Config.Optimizers)
		}
		if n.Config.Params != nil {
			applyCollectionParamsCreate(n.Config.Params, collection)
		}
	}
	if n.Quantization != nil {
		collection.QuantizationConfig, err = buildQuantizationConfig(n.Quantization)
		if err != nil {
			return nil, err
		}
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
	if n.Quantization != nil {
		message = strings.TrimSuffix(message, ")") + fmt.Sprintf(", %s quantization)", n.Quantization.Type)
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

func (e *Executor) doAlterCollection(n *ast.AlterCollectionStmt) (*ExecResponse, error) {
	ctx := context.Background()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	req := &qdrant.UpdateCollection{
		CollectionName: n.Collection,
	}
	if n.Config != nil {
		if n.Config.Vectors != nil && n.Config.Vectors.OnDisk != nil {
			info, err := e.client.GetCollectionInfo(ctx, n.Collection)
			if err != nil {
				return nil, fmt.Errorf("failed to get collection info: %w", err)
			}
			vectors := info.GetConfig().GetParams().GetVectorsConfig()
			if vectors == nil {
				return nil, fmt.Errorf("collection '%s' has no dense vectors to alter", n.Collection)
			}
			if vectors.GetParams() != nil {
				req.VectorsConfig = &qdrant.VectorsConfigDiff{
					Config: &qdrant.VectorsConfigDiff_Params{
						Params: &qdrant.VectorParamsDiff{OnDisk: n.Config.Vectors.OnDisk},
					},
				}
			} else {
				diffMap := map[string]*qdrant.VectorParamsDiff{}
				if paramsMap := vectors.GetParamsMap(); paramsMap != nil {
					for vname := range paramsMap.GetMap() {
						if vname != sparseVectorName && vname != rerankVectorName {
							diffMap[vname] = &qdrant.VectorParamsDiff{OnDisk: n.Config.Vectors.OnDisk}
						}
					}
				}
				if len(diffMap) == 0 {
					return nil, fmt.Errorf("collection '%s' has no dense vectors to alter", n.Collection)
				}
				req.VectorsConfig = &qdrant.VectorsConfigDiff{
					Config: &qdrant.VectorsConfigDiff_ParamsMap{
						ParamsMap: &qdrant.VectorParamsDiffMap{
							Map: diffMap,
						},
					},
				}
			}
		}
		if n.Config.Hnsw != nil {
			req.HnswConfig = buildHnswConfigDiff(n.Config.Hnsw)
		}
		if n.Config.Optimizers != nil {
			req.OptimizersConfig = buildOptimizersConfigDiff(n.Config.Optimizers)
		}
		if n.Config.Params != nil {
			req.Params = buildCollectionParamsDiff(n.Config.Params)
		}
	}
	if n.Quantization != nil {
		var err error
		req.QuantizationConfig, err = buildAlterQuantizationConfig(n.Quantization)
		if err != nil {
			return nil, err
		}
	}

	if err := e.client.UpdateCollection(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to alter collection: %w", err)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "alter_collection",
		Message:   fmt.Sprintf("Collection '%s' altered", n.Collection),
		Data: map[string]any{
			"collection": n.Collection,
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

	created, err := e.ensureCollectionForInsert(ctx, n.Collection, n.Model, n.Hybrid, n.DenseVector, n.SparseVector)
	if err != nil {
		return nil, err
	}

	model := e.resolveDenseModel(n.Model)
	sparseModel := e.resolveSparseModel(n.SparseModel)
	useHybrid, err := e.shouldUseHybrid(ctx, n.Collection, n.Hybrid)
	if err != nil {
		return nil, err
	}
	topo, err := e.resolveVectorTopology(ctx, n.Collection)
	includeRerank := topo != nil && topo.RerankVector != nil
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}

	pointID, payload, err := insertPointIDAndPayload(n.PointID, n.Values)
	if err != nil {
		return nil, err
	}

	denseName, sparseName := denseVectorName, sparseVectorName
	if topo != nil && topo.DenseVector != nil && *topo.DenseVector != "" {
		denseName = *topo.DenseVector
	}
	if topo != nil && topo.SparseVector != nil && *topo.SparseVector != "" {
		sparseName = *topo.SparseVector
	}
	if n.DenseVector != nil {
		denseName = *n.DenseVector
	}
	if n.SparseVector != nil {
		sparseName = *n.SparseVector
	}
	vectors, err := e.buildInsertVectors(ctx, text, model, sparseModel, useHybrid, includeRerank, n.Collection, denseName, sparseName)
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
			"created":      created,
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

	texts := make([]string, 0, len(n.ValuesList))
	pointIDs := make([]any, 0, len(n.ValuesList))
	payloads := make([]map[string]any, 0, len(n.ValuesList))
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

	created, err := e.ensureCollectionForInsert(ctx, n.Collection, n.Model, n.Hybrid, n.DenseVector, n.SparseVector)
	if err != nil {
		return nil, err
	}
	useHybrid, err := e.shouldUseHybrid(ctx, n.Collection, n.Hybrid)
	if err != nil {
		return nil, err
	}
	topo, err := e.resolveVectorTopology(ctx, n.Collection)
	includeRerank := topo != nil && topo.RerankVector != nil
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}

	denseName, sparseName := denseVectorName, sparseVectorName
	if topo != nil && topo.DenseVector != nil && *topo.DenseVector != "" {
		denseName = *topo.DenseVector
	}
	if topo != nil && topo.SparseVector != nil && *topo.SparseVector != "" {
		sparseName = *topo.SparseVector
	}
	if n.DenseVector != nil {
		denseName = *n.DenseVector
	}
	if n.SparseVector != nil {
		sparseName = *n.SparseVector
	}
	vectorsBatch, err := e.buildInsertVectorsBatch(ctx, texts, model, sparseModel, useHybrid, includeRerank, n.Collection, denseName, sparseName)
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
			"created":      created,
			"hybrid":       useHybrid,
			"dense_model":  model,
			"sparse_model": sparseModel,
			"rerank":       includeRerank,
		},
	}, nil
}

func (e *Executor) doSelect(n *ast.SelectStmt) (*ExecResponse, error) {
	ctx := context.Background()
	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}
	pointID := newPointID(n.PointID)
	records, err := e.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: n.Collection,
		Ids:            []*qdrant.PointId{pointID},
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(false),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve point: %w", err)
	}
	if len(records) == 0 {
		return &ExecResponse{
			OK:        true,
			Operation: "select",
			Message:   fmt.Sprintf("Point '%v' not found in '%s'", n.PointID, n.Collection),
			Data:      nil,
		}, nil
	}
	record := records[0]
	return &ExecResponse{
		OK:        true,
		Operation: "select",
		Message:   fmt.Sprintf("Retrieved point '%v' from '%s'", n.PointID, n.Collection),
		Data: map[string]any{
			"id":      pointIDString(record.GetId()),
			"payload": convertRetrievedPayload(record.GetPayload()),
		},
	}, nil
}

func (e *Executor) doScroll(n *ast.ScrollStmt) (*ExecResponse, error) {
	ctx := context.Background()
	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	req := &qdrant.ScrollPoints{
		CollectionName: n.Collection,
		Limit:          qdrant.PtrOf(uint32(n.Limit)),
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(false),
	}
	if n.After != nil {
		req.Offset = newPointID(n.After)
	}
	if n.QueryFilter != nil {
		filter, err := filters.NewFilterConverter().BuildFilter(n.QueryFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to build filter: %w", err)
		}
		req.Filter = filter
	}

	records, nextOffset, err := e.client.ScrollAndOffset(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("scroll failed: %w", err)
	}

	points := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		points = append(points, map[string]any{
			"id":      pointIDString(rec.GetId()),
			"payload": convertRetrievedPayload(rec.GetPayload()),
		})
	}

	var next any
	if nextOffset != nil {
		next = pointIDValue(nextOffset)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "scroll",
		Message:   fmt.Sprintf("Scrolled %d point(s) from '%s'", len(points), n.Collection),
		Data: map[string]any{
			"points":      points,
			"next_offset": next,
		},
	}, nil
}

func convertRetrievedPayload(payload map[string]*qdrant.Value) map[string]any {
	result := make(map[string]any, len(payload))
	for key, val := range payload {
		result[key] = convertValue(val)
	}
	return result
}

func convertValue(val *qdrant.Value) any {
	switch v := val.GetKind().(type) {
	case *qdrant.Value_StringValue:
		return v.StringValue
	case *qdrant.Value_IntegerValue:
		return int(v.IntegerValue)
	case *qdrant.Value_DoubleValue:
		return v.DoubleValue
	case *qdrant.Value_BoolValue:
		return v.BoolValue
	case *qdrant.Value_NullValue:
		return nil
	case *qdrant.Value_ListValue:
		items := make([]any, 0, len(v.ListValue.GetValues()))
		for _, item := range v.ListValue.GetValues() {
			items = append(items, convertValue(item))
		}
		return items
	case *qdrant.Value_StructValue:
		return convertRetrievedPayload(v.StructValue.GetFields())
	default:
		return nil
	}
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
	if err := validateSearchMMRUsage(n); err != nil {
		return nil, err
	}

	topo, err := e.resolveVectorTopology(ctx, n.Collection)
	hasRerankVector := topo != nil && topo.RerankVector != nil
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}

	limit := uint64(n.Limit)
	if limit == 0 {
		limit = 10
	}

	fetchLimit := effectiveSearchLimit(limit, n.Rerank)

	denseName, sparseName := denseVectorName, sparseVectorName
	if topo != nil && topo.DenseVector != nil && *topo.DenseVector != "" {
		denseName = *topo.DenseVector
	}
	if topo != nil && topo.SparseVector != nil && *topo.SparseVector != "" {
		sparseName = *topo.SparseVector
	}
	if n.DenseVector != nil {
		denseName = *n.DenseVector
	}
	if n.SparseVector != nil {
		sparseName = *n.SparseVector
	}
	searchReq, err := e.buildSearchRequest(ctx, n, model, sparseModel, hasRerankVector, fetchLimit, denseName, sparseName)
	if err != nil {
		return nil, err
	}

	var filter *qdrant.Filter
	if n.QueryFilter != nil {
		filter, err = filters.NewFilterConverter().BuildFilter(n.QueryFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to build filter: %w", err)
		}
	}

	if n.Offset > 0 {
		searchReq.Offset = qdrant.PtrOf(uint64(n.Offset))
	}
	if n.ScoreThreshold != nil {
		searchReq.ScoreThreshold = qdrant.PtrOf(float32(*n.ScoreThreshold))
	}
	if n.LookupFrom != "" {
		searchReq.LookupFrom = &qdrant.LookupLocation{
			CollectionName: n.LookupFrom,
		}
		if n.LookupVector != nil && *n.LookupVector != "" {
			searchReq.LookupFrom.VectorName = n.LookupVector
		}
	}

	if n.GroupBy != "" {
		groupReq := buildGroupSearchRequest(n, searchReq, filter)
		results, err := e.client.QueryGroups(ctx, groupReq)
		if err != nil {
			return nil, fmt.Errorf("grouped search failed: %w", err)
		}
		message, groups := formatGroupSearchResults(n.GroupBy, n.Hybrid, n.SparseOnly, results)
		return &ExecResponse{
			OK:        true,
			Operation: "search",
			Message:   message,
			Data: map[string]any{
				"count":      len(groups),
				"groups":     groups,
				"collection": n.Collection,
				"group_by":   n.GroupBy,
				"hybrid":     n.Hybrid,
			},
		}, nil
	}

	searchReq.Filter = filter

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

func hasMMR(withClause *ast.SearchWith) bool {
	return withClause != nil && (withClause.MmrDiversity != nil || withClause.MmrCandidates != nil)
}

func validateSearchMMRUsage(n *ast.SearchStmt) error {
	if !hasMMR(n.WithClause) {
		return nil
	}
	if n.SparseOnly {
		return fmt.Errorf("MMR is not supported with USING SPARSE yet")
	}
	return nil
}

func buildRecommendRequest(n *ast.RecommendStmt, usingName string) (*qdrant.QueryPoints, error) {
	if hasMMR(n.WithClause) {
		return nil, fmt.Errorf("MMR is supported only for SEARCH statements")
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

	using := usingName
	if n.Using != nil {
		using = *n.Using
	}

	req := &qdrant.QueryPoints{
		CollectionName: n.Collection,
		Query:          query,
		Limit:          qdrant.PtrOf(uint64(n.Limit)),
		Using:          qdrant.PtrOf(using),
		Params:         searchParamsFromWithClause(n.WithClause),
	}
	if n.Offset > 0 {
		req.Offset = qdrant.PtrOf(uint64(n.Offset))
	}
	if n.ScoreThreshold != nil {
		req.ScoreThreshold = qdrant.PtrOf(float32(*n.ScoreThreshold))
	}
	if n.LookupFrom != "" {
		req.LookupFrom = &qdrant.LookupLocation{
			CollectionName: n.LookupFrom,
		}
		if n.LookupVector != nil && *n.LookupVector != "" {
			req.LookupFrom.VectorName = n.LookupVector
		}
	}
	if n.QueryFilter != nil {
		filter, err := filters.NewFilterConverter().BuildFilter(n.QueryFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to build filter: %w", err)
		}
		req.Filter = addExcludedIDsToFilter(filter, append(append([]any{}, n.PositiveIDs...), n.NegativeIDs...))
	} else {
		req.Filter = addExcludedIDsToFilter(nil, append(append([]any{}, n.PositiveIDs...), n.NegativeIDs...))
	}

	return req, nil
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

	topo, err := e.resolveVectorTopology(ctx, n.Collection)
	denseName := denseVectorName
	if topo != nil && topo.DenseVector != nil && *topo.DenseVector != "" {
		denseName = *topo.DenseVector
	}
	if n.Using != nil {
		denseName = *n.Using
	}
	req, err := buildRecommendRequest(n, denseName)
	if err != nil {
		return nil, err
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

func (e *Executor) doUpdateVector(n *ast.UpdateVectorStmt) (*ExecResponse, error) {
	ctx := context.Background()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	topo, err := e.resolveVectorTopology(ctx, n.Collection)
	denseName := denseVectorName
	if topo != nil && topo.DenseVector != nil && *topo.DenseVector != "" {
		denseName = *topo.DenseVector
	}
	if n.VectorName != nil {
		denseName = *n.VectorName
	}
	request, err := e.buildUpdateVectorRequest(ctx, n, denseName)
	if err != nil {
		return nil, err
	}
	if _, err := e.client.UpdateVectors(ctx, request); err != nil {
		return nil, fmt.Errorf("failed to update vector: %w", err)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "update_vector",
		Message:   fmt.Sprintf("Updated vector for point [%v] in '%s'", n.PointID, n.Collection),
		Data: map[string]any{
			"collection": n.Collection,
			"point_id":   n.PointID,
		},
	}, nil
}

func (e *Executor) doUpdatePayload(n *ast.UpdatePayloadStmt) (*ExecResponse, error) {
	ctx := context.Background()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	request, err := buildUpdatePayloadRequest(n)
	if err != nil {
		return nil, err
	}
	if _, err := e.client.SetPayload(ctx, request); err != nil {
		return nil, fmt.Errorf("failed to update payload: %w", err)
	}

	message := fmt.Sprintf("Payload updated for point [%v] in '%s'", n.PointID, n.Collection)
	if n.QueryFilter != nil {
		message = fmt.Sprintf("Payload updated in '%s' (filter-based)", n.Collection)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "update_payload",
		Message:   message,
		Data: map[string]any{
			"collection": n.Collection,
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
	} else if n.FieldType == "text" {
		fieldType = qdrant.FieldType_FieldTypeText
	} else if n.FieldType == "geo" {
		fieldType = qdrant.FieldType_FieldTypeGeo
	} else if n.FieldType == "datetime" {
		fieldType = qdrant.FieldType_FieldTypeDatetime
	} else if n.FieldType == "uuid" {
		fieldType = qdrant.FieldType_FieldTypeUuid
	}

	fieldIndexParams, err := buildPayloadIndexParams(n.FieldType, n.Options)
	if err != nil {
		return nil, err
	}
	wait := true
	_, err = e.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName:   n.Collection,
		FieldName:        n.Field,
		FieldType:        &fieldType,
		FieldIndexParams: fieldIndexParams,
		Wait:             &wait,
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

func (e *Executor) buildPayload(values map[string]any) map[string]*qdrant.Value {
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

func buildQuantizationConfig(cfg *ast.QuantizationConfig) (*qdrant.QuantizationConfig, error) {
	if cfg == nil {
		return nil, nil
	}

	switch cfg.Type {
	case ast.QuantizationTypeScalar:
		scalar := &qdrant.ScalarQuantization{
			Type:      qdrant.QuantizationType_Int8,
			AlwaysRam: qdrant.PtrOf(cfg.AlwaysRAM),
		}
		if cfg.Quantile != nil {
			scalar.Quantile = qdrant.PtrOf(float32(*cfg.Quantile))
		}
		return qdrant.NewQuantizationScalar(scalar), nil
	case ast.QuantizationTypeBinary:
		return qdrant.NewQuantizationBinary(&qdrant.BinaryQuantization{
			AlwaysRam: qdrant.PtrOf(cfg.AlwaysRAM),
		}), nil
	case ast.QuantizationTypeProduct:
		return qdrant.NewQuantizationProduct(&qdrant.ProductQuantization{
			Compression: qdrant.CompressionRatio_x4,
			AlwaysRam:   qdrant.PtrOf(cfg.AlwaysRAM),
		}), nil
	case ast.QuantizationTypeTurbo:
		turbo := &qdrant.TurboQuantization{
			AlwaysRam: qdrant.PtrOf(cfg.AlwaysRAM),
		}
		if cfg.TurboBits != nil {
			bits := turboBitsEnum(*cfg.TurboBits)
			if bits == nil {
				return nil, fmt.Errorf("unsupported TURBO bit depth %.4g; expected one of 1, 1.5, 2, or 4", *cfg.TurboBits)
			}
			turbo.Bits = bits
		}
		return qdrant.NewQuantizationTurbo(turbo), nil
	default:
		return nil, nil
	}
}

func buildHnswConfigDiff(cfg *ast.HnswRuntimeConfig) *qdrant.HnswConfigDiff {
	if cfg == nil {
		return nil
	}
	diff := &qdrant.HnswConfigDiff{}
	if cfg.M != nil {
		diff.M = cfg.M
	}
	if cfg.EfConstruct != nil {
		diff.EfConstruct = cfg.EfConstruct
	}
	if cfg.FullScanThreshold != nil {
		diff.FullScanThreshold = cfg.FullScanThreshold
	}
	if cfg.MaxIndexingThreads != nil {
		diff.MaxIndexingThreads = cfg.MaxIndexingThreads
	}
	if cfg.OnDisk != nil {
		diff.OnDisk = cfg.OnDisk
	}
	if cfg.PayloadM != nil {
		diff.PayloadM = cfg.PayloadM
	}
	if cfg.InlineStorage != nil {
		diff.InlineStorage = cfg.InlineStorage
	}
	return diff
}

func buildOptimizersConfigDiff(cfg *ast.OptimizersRuntimeConfig) *qdrant.OptimizersConfigDiff {
	if cfg == nil {
		return nil
	}
	diff := &qdrant.OptimizersConfigDiff{}
	if cfg.DeletedThreshold != nil {
		diff.DeletedThreshold = cfg.DeletedThreshold
	}
	if cfg.VacuumMinVectorNumber != nil {
		diff.VacuumMinVectorNumber = cfg.VacuumMinVectorNumber
	}
	if cfg.DefaultSegmentNumber != nil {
		diff.DefaultSegmentNumber = cfg.DefaultSegmentNumber
	}
	if cfg.MaxSegmentSize != nil {
		diff.MaxSegmentSize = cfg.MaxSegmentSize
	}
	if cfg.MemmapThreshold != nil {
		diff.MemmapThreshold = cfg.MemmapThreshold
	}
	if cfg.IndexingThreshold != nil {
		diff.IndexingThreshold = cfg.IndexingThreshold
	}
	if cfg.FlushIntervalSec != nil {
		diff.FlushIntervalSec = cfg.FlushIntervalSec
	}
	if cfg.PreventUnoptimized != nil {
		diff.PreventUnoptimized = cfg.PreventUnoptimized
	}
	if cfg.MaxOptimizationThreads != nil {
		switch v := cfg.MaxOptimizationThreads.(type) {
		case int:
			diff.MaxOptimizationThreads = &qdrant.MaxOptimizationThreads{
				Variant: &qdrant.MaxOptimizationThreads_Value{
					Value: uint64(v),
				},
			}
		case string:
			if toLowerStr(v) == "auto" {
				diff.MaxOptimizationThreads = &qdrant.MaxOptimizationThreads{
					Variant: &qdrant.MaxOptimizationThreads_Setting_{
						Setting: qdrant.MaxOptimizationThreads_Auto,
					},
				}
			}
		}
	}
	return diff
}

func applyCollectionParamsCreate(cfg *ast.CollectionParamsConfig, req *qdrant.CreateCollection) {
	if cfg.ReplicationFactor != nil {
		req.ReplicationFactor = qdrant.PtrOf(uint32(*cfg.ReplicationFactor))
	}
	if cfg.WriteConsistencyFactor != nil {
		req.WriteConsistencyFactor = qdrant.PtrOf(uint32(*cfg.WriteConsistencyFactor))
	}
	if cfg.OnDiskPayload != nil {
		req.OnDiskPayload = cfg.OnDiskPayload
	}
}

func buildCollectionParamsDiff(cfg *ast.CollectionParamsConfig) *qdrant.CollectionParamsDiff {
	if cfg == nil {
		return nil
	}
	diff := &qdrant.CollectionParamsDiff{}
	if cfg.ReplicationFactor != nil {
		diff.ReplicationFactor = qdrant.PtrOf(uint32(*cfg.ReplicationFactor))
	}
	if cfg.WriteConsistencyFactor != nil {
		diff.WriteConsistencyFactor = qdrant.PtrOf(uint32(*cfg.WriteConsistencyFactor))
	}
	if cfg.ReadFanOutFactor != nil {
		diff.ReadFanOutFactor = qdrant.PtrOf(uint32(*cfg.ReadFanOutFactor))
	}
	if cfg.ReadFanOutDelayMs != nil {
		diff.ReadFanOutDelayMs = cfg.ReadFanOutDelayMs
	}
	if cfg.OnDiskPayload != nil {
		diff.OnDiskPayload = cfg.OnDiskPayload
	}
	return diff
}

func buildAlterQuantizationConfig(update *ast.QuantizationUpdate) (*qdrant.QuantizationConfigDiff, error) {
	if update == nil {
		return nil, nil
	}
	if update.Disabled {
		return &qdrant.QuantizationConfigDiff{
			Quantization: &qdrant.QuantizationConfigDiff_Disabled{
				Disabled: &qdrant.Disabled{},
			},
		}, nil
	}
	if update.Config != nil {
		cfg, err := buildQuantizationConfig(update.Config)
		if err != nil {
			return nil, err
		}
		if cfg != nil {
			switch cfg.Quantization.(type) {
			case *qdrant.QuantizationConfig_Scalar:
				return &qdrant.QuantizationConfigDiff{
					Quantization: &qdrant.QuantizationConfigDiff_Scalar{
						Scalar: cfg.GetScalar(),
					},
				}, nil
			case *qdrant.QuantizationConfig_Binary:
				return &qdrant.QuantizationConfigDiff{
					Quantization: &qdrant.QuantizationConfigDiff_Binary{
						Binary: cfg.GetBinary(),
					},
				}, nil
			case *qdrant.QuantizationConfig_Product:
				return &qdrant.QuantizationConfigDiff{
					Quantization: &qdrant.QuantizationConfigDiff_Product{
						Product: cfg.GetProduct(),
					},
				}, nil
			case *qdrant.QuantizationConfig_Turboquant:
				return &qdrant.QuantizationConfigDiff{
					Quantization: &qdrant.QuantizationConfigDiff_Turboquant{
						Turboquant: cfg.GetTurboquant(),
					},
				}, nil
			}
		}
	}
	return nil, nil
}

func toLowerStr(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func (e *Executor) ensureCollectionForInsert(ctx context.Context, collection string, model *string, requestedHybrid bool, explicitDense, explicitSparse *string) (bool, error) {
	exists, err := e.client.CollectionExists(ctx, collection)
	if err != nil {
		return false, fmt.Errorf("failed to check collection: %w", err)
	}
	if exists {
		return false, nil
	}

	denseSize, err := e.resolveDenseVectorSize(ctx, model)
	if err != nil {
		return false, err
	}
	denseName := denseVectorName
	if explicitDense != nil {
		denseName = *explicitDense
	}
	vectorsMap := map[string]*qdrant.VectorParams{
		denseName: collectionVectorParams(denseSize, false)[denseVectorName],
	}
	createReq := &qdrant.CreateCollection{
		CollectionName: collection,
		VectorsConfig:  qdrant.NewVectorsConfigMap(vectorsMap),
	}
	if requestedHybrid {
		sparseName := sparseVectorName
		if explicitSparse != nil {
			sparseName = *explicitSparse
		}
		createReq.SparseVectorsConfig = qdrant.NewSparseVectorsConfig(map[string]*qdrant.SparseVectorParams{
			sparseName: {Modifier: qdrant.Modifier_Idf.Enum()},
		})
	}
	if err := e.client.CreateCollection(ctx, createReq); err != nil {
		return false, fmt.Errorf("failed to create collection: %w", err)
	}
	if err := e.waitForCollectionReady(ctx, collection); err != nil {
		return false, err
	}
	return true, nil
}

func (e *Executor) buildInsertVectors(ctx context.Context, text, denseModel, sparseModel string, includeSparse, includeRerank bool, collection string, denseName, sparseName string) (map[string]*qdrant.Vector, error) {
	if e.usesLocalEmbeddings() {
		embedClient, err := e.embeddingClient(denseModel)
		if err != nil {
			return nil, err
		}

		denseVector, sparseVec, err := embedConcurrent(ctx, embedClient, text, includeSparse)
		if err != nil {
			return nil, err
		}

		vectors := map[string]*qdrant.Vector{
			denseName: qdrant.NewVectorDense(denseVector),
		}
		if includeSparse {
			vectors[sparseName] = qdrant.NewVectorSparse(sparseVec.Indices, sparseVec.Values)
		}
		if includeRerank {
			return nil, fmt.Errorf("local/external rerank vectors are not implemented yet")
		}
		return vectors, nil
	}

	vectors := map[string]*qdrant.Vector{
		denseName: qdrant.NewVectorDocument(&qdrant.Document{
			Text:  text,
			Model: denseModel,
		}),
	}
	if includeSparse {
		vectors[sparseName] = qdrant.NewVectorDocument(&qdrant.Document{
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

func (e *Executor) buildInsertVectorsBatch(ctx context.Context, texts []string, denseModel, sparseModel string, includeSparse, includeRerank bool, collection string, denseName, sparseName string) ([]map[string]*qdrant.Vector, error) {
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

		batch := make([]map[string]*qdrant.Vector, len(texts))
		sparseVectors := make([]sparse.Vector, len(texts))

		if includeSparse {
			var wg sync.WaitGroup
			wg.Add(len(texts))
			for i, text := range texts {
				go func(idx int, t string) {
					defer wg.Done()
					sparseVectors[idx] = sparse.BuildDocument(t)
				}(i, text)
			}
			wg.Wait()
		}

		for idx := range texts {
			vectors := map[string]*qdrant.Vector{
				denseName: qdrant.NewVectorDense(denseVectors[idx]),
			}
			if includeSparse {
				sv := sparseVectors[idx]
				vectors[sparseVectorName] = qdrant.NewVectorSparse(sv.Indices, sv.Values)
			}
			batch[idx] = vectors
		}
		return batch, nil
	}

	batch := make([]map[string]*qdrant.Vector, 0, len(texts))
	for _, text := range texts {
		vectors, err := e.buildInsertVectors(ctx, text, denseModel, sparseModel, includeSparse, includeRerank, collection, denseName, sparseName)
		if err != nil {
			return nil, err
		}
		batch = append(batch, vectors)
	}
	return batch, nil
}

func (e *Executor) buildSearchRequest(ctx context.Context, n *ast.SearchStmt, denseModel, sparseModel string, hasRerankVector bool, limit uint64, denseName, sparseName string) (*qdrant.QueryPoints, error) {
	params := searchParamsFromWithClause(n.WithClause)

	if n.SparseOnly {
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
			sparsePrefetch := &qdrant.PrefetchQuery{
				Query:  qdrant.NewQueryDocument(&qdrant.Document{Text: n.QueryText, Model: sparseModel}),
				Using:  qdrant.PtrOf(sparseName),
				Limit:  qdrant.PtrOf(limit * rerankPrefetchFactor),
				Params: params,
			}
			if e.usesLocalEmbeddings() {
				sv := sparse.BuildQuery(n.QueryText)
				sparsePrefetch.Query = qdrant.NewQuerySparse(sv.Indices, sv.Values)
			}
			return buildRerankSearchRequest(n.Collection, n.QueryText, rerankModel, limit, []*qdrant.PrefetchQuery{sparsePrefetch}, params), nil
		}
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
			Using:          qdrant.PtrOf(sparseName),
			Limit:          qdrant.PtrOf(limit),
			Params:         params,
		}, nil
	}

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
		prefetch, err := e.buildSearchPrefetches(ctx, n.QueryText, denseModel, sparseModel, limit, params, nil, denseName, sparseName)
		if err != nil {
			return nil, err
		}
		return buildRerankSearchRequest(n.Collection, n.QueryText, rerankModel, limit, prefetch, params), nil
	}

	if n.Hybrid {
		prefetch, err := e.buildSearchPrefetches(ctx, n.QueryText, denseModel, sparseModel, limit, params, n.WithClause, denseName, sparseName)
		if err != nil {
			return nil, err
		}
		fusionMode := qdrant.Fusion_RRF
		if n.Fusion != nil && *n.Fusion == "dbsf" {
			fusionMode = qdrant.Fusion_DBSF
		}
		return &qdrant.QueryPoints{
			CollectionName: n.Collection,
			Prefetch:       prefetch,
			Query:          qdrant.NewQueryFusion(fusionMode),
			Limit:          qdrant.PtrOf(limit),
			Params:         params,
		}, nil
	}

	query := qdrant.NewQueryDocument(&qdrant.Document{
		Text:  n.QueryText,
		Model: denseModel,
	})
	var mmrNearest *qdrant.VectorInput
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
		if hasMMR(n.WithClause) {
			mmrNearest = qdrant.NewVectorInputDense(denseVector)
		}
	} else if hasMMR(n.WithClause) {
		mmrNearest = qdrant.NewVectorInputDocument(&qdrant.Document{
			Text:  n.QueryText,
			Model: denseModel,
		})
	}

	if hasMMR(n.WithClause) {
		query = qdrant.NewQueryMMR(mmrNearest, &qdrant.Mmr{
			Diversity:       float32PtrFromFloat64(n.WithClause.MmrDiversity),
			CandidatesLimit: uint32PtrFromInt(n.WithClause.MmrCandidates),
		})
	}

	return &qdrant.QueryPoints{
		CollectionName: n.Collection,
		Query:          query,
		Using:          qdrant.PtrOf(denseName),
		Limit:          qdrant.PtrOf(limit),
		Params:         params,
	}, nil
}

func (e *Executor) buildSearchPrefetches(ctx context.Context, queryText, denseModel, sparseModel string, limit uint64, params *qdrant.SearchParams, withClause *ast.SearchWith, denseName, sparseName string) ([]*qdrant.PrefetchQuery, error) {
	denseQuery := qdrant.NewQueryDocument(&qdrant.Document{
		Text:  queryText,
		Model: denseModel,
	})
	var mmrNearest *qdrant.VectorInput
	sparseQuery := qdrant.NewQueryDocument(&qdrant.Document{
		Text:  queryText,
		Model: sparseModel,
	})
	if e.usesLocalEmbeddings() {
		embedClient, err := e.embeddingClient(denseModel)
		if err != nil {
			return nil, fmt.Errorf("failed to create embedding client for search: %w", err)
		}
		denseVector, sv, err := embedConcurrentQuery(ctx, embedClient, queryText)
		if err != nil {
			return nil, fmt.Errorf("failed to embed search query: %w", err)
		}
		denseQuery = qdrant.NewQueryDense(denseVector)
		if hasMMR(withClause) {
			mmrNearest = qdrant.NewVectorInputDense(denseVector)
		}
		sparseQuery = qdrant.NewQuerySparse(sv.Indices, sv.Values)
	} else if hasMMR(withClause) {
		mmrNearest = qdrant.NewVectorInputDocument(&qdrant.Document{
			Text:  queryText,
			Model: denseModel,
		})
	}

	if hasMMR(withClause) {
		denseQuery = qdrant.NewQueryMMR(mmrNearest, &qdrant.Mmr{
			Diversity:       float32PtrFromFloat64(withClause.MmrDiversity),
			CandidatesLimit: uint32PtrFromInt(withClause.MmrCandidates),
		})
	}

	return []*qdrant.PrefetchQuery{
		{
			Query:  sparseQuery,
			Using:  qdrant.PtrOf(sparseName),
			Limit:  qdrant.PtrOf(limit),
			Params: params,
		},
		{
			Query:  denseQuery,
			Using:  qdrant.PtrOf(denseName),
			Limit:  qdrant.PtrOf(limit),
			Params: params,
		},
	}, nil
}

func buildGroupSearchRequest(n *ast.SearchStmt, req *qdrant.QueryPoints, filter *qdrant.Filter) *qdrant.QueryPointGroups {
	groupSize := uint64(n.GroupSize)
	if groupSize == 0 {
		groupSize = 3
	}

	return &qdrant.QueryPointGroups{
		CollectionName: req.GetCollectionName(),
		Prefetch:       req.GetPrefetch(),
		Query:          req.GetQuery(),
		Using:          req.Using,
		Filter:         filter,
		Params:         req.GetParams(),
		Limit:          req.Limit,
		GroupSize:      qdrant.PtrOf(groupSize),
		GroupBy:        n.GroupBy,
		WithPayload:    qdrant.NewWithPayload(true),
		ScoreThreshold: req.ScoreThreshold,
		LookupFrom:     req.LookupFrom,
	}
}

func (e *Executor) buildUpdateVectorRequest(ctx context.Context, n *ast.UpdateVectorStmt, vectorName string) (*qdrant.UpdatePointVectors, error) {
	info, err := e.client.GetCollectionInfo(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}

	wait := true
	vectors := qdrant.NewVectors(n.Vector...)
	if info.GetConfig().GetParams().GetVectorsConfig().GetParamsMap() != nil {
		name := vectorName
		if n.VectorName != nil {
			name = *n.VectorName
		}
		vectors = qdrant.NewVectorsMap(map[string]*qdrant.Vector{
			name: qdrant.NewVectorDense(n.Vector),
		})
	}

	return &qdrant.UpdatePointVectors{
		CollectionName: n.Collection,
		Wait:           &wait,
		Points: []*qdrant.PointVectors{
			{
				Id:      newPointID(n.PointID),
				Vectors: vectors,
			},
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
	if withClause.IndexedOnly {
		params.IndexedOnly = qdrant.PtrOf(true)
	}
	if withClause.Quantization != nil {
		params.Quantization = &qdrant.QuantizationSearchParams{}
		if withClause.Quantization.Ignore != nil {
			params.Quantization.Ignore = qdrant.PtrOf(*withClause.Quantization.Ignore)
		}
		if withClause.Quantization.Rescore != nil {
			params.Quantization.Rescore = qdrant.PtrOf(*withClause.Quantization.Rescore)
		}
		if withClause.Quantization.Oversampling != nil {
			params.Quantization.Oversampling = qdrant.PtrOf(*withClause.Quantization.Oversampling)
		}
	}

	if params.HnswEf == nil && params.Exact == nil && params.Acorn == nil && params.IndexedOnly == nil && params.Quantization == nil {
		return nil
	}

	return params
}

func buildPayloadIndexParams(fieldType string, options map[string]any) (*qdrant.PayloadIndexParams, error) {
	if len(options) == 0 {
		return nil, nil
	}

	switch fieldType {
	case "keyword":
		if err := validateIndexOptionKeys(fieldType, options, []string{"is_tenant", "on_disk", "enable_hnsw"}); err != nil {
			return nil, err
		}
		isTenant, err := boolOption(options, "is_tenant")
		if err != nil {
			return nil, err
		}
		onDisk, err := boolOption(options, "on_disk")
		if err != nil {
			return nil, err
		}
		enableHnsw, err := boolOption(options, "enable_hnsw")
		if err != nil {
			return nil, err
		}
		return qdrant.NewPayloadIndexParamsKeyword(&qdrant.KeywordIndexParams{
			IsTenant:   isTenant,
			OnDisk:     onDisk,
			EnableHnsw: enableHnsw,
		}), nil
	case "uuid":
		if err := validateIndexOptionKeys(fieldType, options, []string{"is_tenant", "on_disk", "enable_hnsw"}); err != nil {
			return nil, err
		}
		isTenant, err := boolOption(options, "is_tenant")
		if err != nil {
			return nil, err
		}
		onDisk, err := boolOption(options, "on_disk")
		if err != nil {
			return nil, err
		}
		enableHnsw, err := boolOption(options, "enable_hnsw")
		if err != nil {
			return nil, err
		}
		return qdrant.NewPayloadIndexParamsUUID(&qdrant.UuidIndexParams{
			IsTenant:   isTenant,
			OnDisk:     onDisk,
			EnableHnsw: enableHnsw,
		}), nil
	case "text":
		if err := validateIndexOptionKeys(fieldType, options, []string{"tokenizer", "min_token_len", "max_token_len", "lowercase", "ascii_folding", "phrase_matching", "stopwords", "on_disk", "enable_hnsw"}); err != nil {
			return nil, err
		}
		minTokenLen, err := uint64Option(options, "min_token_len")
		if err != nil {
			return nil, err
		}
		maxTokenLen, err := uint64Option(options, "max_token_len")
		if err != nil {
			return nil, err
		}
		if minTokenLen != nil && maxTokenLen != nil && *minTokenLen > *maxTokenLen {
			return nil, fmt.Errorf("CREATE INDEX text option min_token_len cannot be greater than max_token_len")
		}
		tokenizer, err := tokenizerOption(options, "tokenizer")
		if err != nil {
			return nil, err
		}
		stopwords, err := stopwordsOption(options, "stopwords")
		if err != nil {
			return nil, err
		}
		lowercase, err := boolOption(options, "lowercase")
		if err != nil {
			return nil, err
		}
		asciiFolding, err := boolOption(options, "ascii_folding")
		if err != nil {
			return nil, err
		}
		phraseMatching, err := boolOption(options, "phrase_matching")
		if err != nil {
			return nil, err
		}
		onDisk, err := boolOption(options, "on_disk")
		if err != nil {
			return nil, err
		}
		enableHnsw, err := boolOption(options, "enable_hnsw")
		if err != nil {
			return nil, err
		}
		return qdrant.NewPayloadIndexParamsText(&qdrant.TextIndexParams{
			Tokenizer:      tokenizer,
			MinTokenLen:    minTokenLen,
			MaxTokenLen:    maxTokenLen,
			Lowercase:      lowercase,
			AsciiFolding:   asciiFolding,
			PhraseMatching: phraseMatching,
			Stopwords:      stopwords,
			OnDisk:         onDisk,
			EnableHnsw:     enableHnsw,
		}), nil
	default:
		return nil, fmt.Errorf("CREATE INDEX type '%s' does not support advanced options yet", fieldType)
	}
}

func validateIndexOptionKeys(fieldType string, options map[string]any, allowed []string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range options {
		if _, ok := allowedSet[key]; !ok {
			return fmt.Errorf("Unknown CREATE INDEX option '%s' for type '%s'. Expected one of: %s", key, fieldType, strings.Join(allowed, ", "))
		}
	}
	return nil
}

func boolOption(options map[string]any, key string) (*bool, error) {
	value, ok := options[key]
	if !ok {
		return nil, nil
	}
	typed, ok := value.(bool)
	if !ok {
		return nil, fmt.Errorf("CREATE INDEX option '%s' must be a boolean", key)
	}
	return qdrant.PtrOf(typed), nil
}

func uint64Option(options map[string]any, key string) (*uint64, error) {
	value, ok := options[key]
	if !ok {
		return nil, nil
	}
	typed, ok := value.(int)
	if !ok || typed <= 0 {
		return nil, fmt.Errorf("CREATE INDEX option '%s' must be a positive integer", key)
	}
	result := uint64(typed)
	return &result, nil
}

func tokenizerOption(options map[string]any, key string) (qdrant.TokenizerType, error) {
	value, ok := options[key]
	if !ok {
		return qdrant.TokenizerType_Unknown, nil
	}
	typed, ok := value.(string)
	if !ok {
		return qdrant.TokenizerType_Unknown, fmt.Errorf("CREATE INDEX option '%s' must be a string", key)
	}
	switch strings.ToLower(typed) {
	case "prefix":
		return qdrant.TokenizerType_Prefix, nil
	case "whitespace":
		return qdrant.TokenizerType_Whitespace, nil
	case "word":
		return qdrant.TokenizerType_Word, nil
	case "multilingual":
		return qdrant.TokenizerType_Multilingual, nil
	default:
		return qdrant.TokenizerType_Unknown, fmt.Errorf("CREATE INDEX option '%s' must be one of: prefix, whitespace, word, multilingual", key)
	}
}

func stopwordsOption(options map[string]any, key string) (*qdrant.StopwordsSet, error) {
	value, ok := options[key]
	if !ok {
		return nil, nil
	}
	switch typed := value.(type) {
	case string:
		return &qdrant.StopwordsSet{Languages: []string{strings.ToLower(typed)}}, nil
	case []any:
		words := make([]string, 0, len(typed))
		for _, item := range typed {
			word, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("CREATE INDEX option '%s' list values must be strings", key)
			}
			words = append(words, word)
		}
		return &qdrant.StopwordsSet{Custom: words}, nil
	default:
		return nil, fmt.Errorf("CREATE INDEX option '%s' must be a string language name or a list of strings", key)
	}
}

func float32PtrFromFloat64(value *float64) *float32 {
	if value == nil {
		return nil
	}
	converted := float32(*value)
	return &converted
}

func uint32PtrFromInt(value *int) *uint32 {
	if value == nil {
		return nil
	}
	converted := uint32(*value)
	return &converted
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

// embedConcurrent runs dense embedding (HTTP I/O) and sparse document build (CPU)
// concurrently. When skipSparse is true, the dense vector is returned with a zero
// sparse vector.
func embedConcurrent(ctx context.Context, client *embedding.Client, text string, includeSparse bool) (denseVec []float32, sparseVec sparse.Vector, err error) {
	if !includeSparse {
		denseVec, err = client.Embed(ctx, text)
		if err != nil {
			return nil, sparse.Vector{}, fmt.Errorf("failed to embed insert text: %w", err)
		}
		return denseVec, sparse.Vector{}, nil
	}

	type denseOut struct {
		vec []float32
		err error
	}
	dc := make(chan denseOut, 1)
	go func() {
		v, e := client.Embed(ctx, text)
		dc <- denseOut{v, e}
	}()

	sparseVec = sparse.BuildDocument(text)
	dr := <-dc
	if dr.err != nil {
		return nil, sparse.Vector{}, fmt.Errorf("failed to embed insert text: %w", dr.err)
	}
	return dr.vec, sparseVec, nil
}

// embedConcurrentQuery is like embedConcurrent but uses BuildQuery for search
// queries (log-TF weighting instead of normalized-TF).
func embedConcurrentQuery(ctx context.Context, client *embedding.Client, text string) ([]float32, sparse.Vector, error) {
	type denseOut struct {
		vec []float32
		err error
	}
	dc := make(chan denseOut, 1)
	go func() {
		v, e := client.Embed(ctx, text)
		dc <- denseOut{v, e}
	}()

	sv := sparse.BuildQuery(text)
	dr := <-dc
	if dr.err != nil {
		return nil, sparse.Vector{}, dr.err
	}
	return dr.vec, sv, nil
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

func insertPointIDAndPayload(pointID any, values map[string]any) (any, map[string]any, error) {
	payload := make(map[string]any, len(values))
	maps.Copy(payload, values)
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

func newPointID(value any) *qdrant.PointId {
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

func buildRecommendVectorInputs(ids []any) []*qdrant.VectorInput {
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

func turboBitsEnum(value float64) *qdrant.TurboQuantBitSize {
	switch value {
	case 1.0:
		return qdrant.TurboQuantBitSize_Bits1.Enum()
	case 1.5:
		return qdrant.TurboQuantBitSize_Bits1_5.Enum()
	case 2.0:
		return qdrant.TurboQuantBitSize_Bits2.Enum()
	case 4.0:
		return qdrant.TurboQuantBitSize_Bits4.Enum()
	default:
		return nil
	}
}

func addExcludedIDsToFilter(filter *qdrant.Filter, ids []any) *qdrant.Filter {
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

func buildUpdatePayloadRequest(n *ast.UpdatePayloadStmt) (*qdrant.SetPayloadPoints, error) {
	wait := true
	request := &qdrant.SetPayloadPoints{
		CollectionName: n.Collection,
		Payload:        qdrant.NewValueMap(n.Payload),
		Wait:           &wait,
	}

	if n.QueryFilter != nil {
		filter, err := filters.NewFilterConverter().BuildFilter(n.QueryFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to build update payload filter: %w", err)
		}
		request.PointsSelector = qdrant.NewPointsSelectorFilter(filter)
		return request, nil
	}

	request.PointsSelector = qdrant.NewPointsSelector(newPointID(n.PointID))
	return request, nil
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

	client, err := newClientFromURL(cfg.URL, cfg.Secret, cfg.NoVerify, cfg.CACert)
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

func formatGroupSearchResults(groupBy string, hybrid, sparseOnly bool, groups []*qdrant.PointGroup) (string, []GroupedSearchResult) {
	if len(groups) == 0 {
		return fmt.Sprintf("Found 0 group(s) by '%s' (grouped)", groupBy), []GroupedSearchResult{}
	}

	label := "grouped"
	if hybrid {
		label = "hybrid, grouped"
	} else if sparseOnly {
		label = "sparse, grouped"
	}

	lines := make([]string, 0, len(groups))
	formatted := make([]GroupedSearchResult, 0, len(groups))
	for _, group := range groups {
		groupID := groupIDString(group.GetId())
		if groupID == "" {
			groupID = fmt.Sprintf("%v", group.GetId())
		}
		lines = append(lines, fmt.Sprintf("group:%s hits:%d", groupID, len(group.GetHits())))

		hits := make([]SearchHit, 0, len(group.GetHits()))
		for _, hit := range group.GetHits() {
			jsonID := pointIDString(hit.GetId())
			text := ""
			if payload := hit.GetPayload(); payload != nil {
				if value, ok := payload["text"]; ok {
					if sv, ok := value.GetKind().(*qdrant.Value_StringValue); ok {
						text = sv.StringValue
					}
				}
			}
			hits = append(hits, SearchHit{
				ID:    jsonID,
				Score: hit.GetScore(),
				Text:  text,
			})
		}
		formatted = append(formatted, GroupedSearchResult{GroupID: groupID, Hits: hits})
	}

	return fmt.Sprintf("Found %d group(s) by '%s' (%s)\n%s", len(groups), groupBy, label, strings.Join(lines, "\n")), formatted
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
	switch value := pointIDValue(id).(type) {
	case string:
		return value
	case uint64:
		return strconv.FormatUint(value, 10)
	}
	return fmt.Sprintf("%v", id)
}

func pointIDValue(id *qdrant.PointId) any {
	if id == nil {
		return ""
	}
	if uuid := id.GetUuid(); uuid != "" {
		return uuid
	}
	if num, ok := id.GetPointIdOptions().(*qdrant.PointId_Num); ok {
		return num.Num
	}
	return fmt.Sprintf("%v", id)
}

func groupIDString(id *qdrant.GroupId) string {
	if id == nil {
		return ""
	}
	if value := id.GetStringValue(); value != "" {
		return value
	}
	if value := id.GetUnsignedValue(); value != 0 {
		return strconv.FormatUint(value, 10)
	}
	if value := id.GetIntegerValue(); value != 0 {
		return strconv.FormatInt(value, 10)
	}
	return ""
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
			noVerify, _ := cmd.Flags().GetBool("no-verify")
			caCert, _ := cmd.Flags().GetString("ca-cert")

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

			client, err := newClientFromURL(url, secret, noVerify, caCert)
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
				NoVerify:           noVerify,
				CACert:             caCert,
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
			batchSize, _ := cmd.Flags().GetInt("batch-size")
			if batchSize <= 0 {
				return commandError(out, mode, "dump", strings.Join(args, " "), fmt.Errorf("--batch-size must be greater than 0"))
			}
			cfg, client, err := loadSavedConfigAndClient()
			if err != nil {
				return commandError(out, mode, "dump", strings.Join(args, " "), err)
			}
			_ = cfg
			written, skipped, err := dump.Collection(context.Background(), client, args[0], args[1], batchSize)
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
	cmd.Flags().Int("batch-size", 50, "Number of points per INSERT BULK batch in dump output")
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

type VectorTopology struct {
	DenseVector  *string
	SparseVector *string
	RerankVector *string
}

func (e *Executor) resolveVectorTopology(ctx context.Context, collection string) (*VectorTopology, error) {
	info, err := e.client.GetCollectionInfo(ctx, collection)
	if err != nil {
		return nil, err
	}
	topo := &VectorTopology{}
	config := info.GetConfig()
	if config == nil {
		return topo, nil
	}
	params := config.GetParams()
	if params == nil {
		return topo, nil
	}

	vectorsConfig := params.GetVectorsConfig()
	if vectorsConfig != nil {
		if paramsMap := vectorsConfig.GetParamsMap(); paramsMap != nil {
			for vname := range paramsMap.GetMap() {
				if vname == denseVectorName {
					topo.DenseVector = qdrant.PtrOf(denseVectorName)
				} else if vname == rerankVectorName {
					topo.RerankVector = qdrant.PtrOf(rerankVectorName)
				} else if topo.DenseVector == nil || *topo.DenseVector == "" {
					v := vname
					topo.DenseVector = &v
				}
			}
		} else if vectorsConfig.GetParams() != nil {
			topo.DenseVector = qdrant.PtrOf("")
		}
	}

	sparseVectorsConfig := params.GetSparseVectorsConfig()
	if sparseVectorsConfig != nil {
		for vname := range sparseVectorsConfig.GetMap() {
			if vname == sparseVectorName {
				topo.SparseVector = qdrant.PtrOf(sparseVectorName)
			} else if topo.SparseVector == nil || *topo.SparseVector == "" {
				v := vname
				topo.SparseVector = &v
			}
		}
	}

	return topo, nil
}
