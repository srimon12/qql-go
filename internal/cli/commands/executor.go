package commands

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/internal/dump"
	"github.com/srimon12/qql-go/internal/embedding"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/srimon12/qql-go/internal/parser"
	"github.com/srimon12/qql-go/internal/qdrantutil"
	"github.com/srimon12/qql-go/internal/repl"
	"github.com/srimon12/qql-go/internal/script"
	"github.com/srimon12/qql-go/internal/sparse"
)

func parseQuery(query string) (ast.ASTNode, error) {
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
	return node, nil
}

func NewExecutor(client qdrantClient, cfg *config.Config) *Executor {
	return &Executor{
		client: client,
		config: cfg,
	}
}

func (e *Executor) Execute(query string) (string, error) {
	result, err := e.ExecuteResult(query)
	if err != nil {
		return "", err
	}
	return result.Message, nil
}

func (e *Executor) ExecuteResult(query string) (*ExecResponse, error) {
	node, err := parseQuery(query)
	if err != nil {
		return nil, err
	}
	return e.ExecuteNode(node)
}

// ExecuteNode executes a pre-parsed AST node. This allows the gateway to
// transform the AST (e.g. inject policy filters) before execution.
func (e *Executor) ExecuteNode(node ast.ASTNode) (*ExecResponse, error) {
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
	case *ast.SelectStmt:
		return e.doSelect(n)
	case *ast.ScrollStmt:
		return e.doScroll(n)
	case *ast.QueryStmt:
		return e.doQuery(n)
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

func (e *Executor) ExecuteFile(path string, stopOnError bool) (string, error) {
	okCount, failCount, err := script.RunFile(path, e, stopOnError)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Executed script %s (%d succeeded, %d failed)", path, okCount, failCount), nil
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
	vectorsMap := map[string]*qdrant.VectorParams{}
	params := collectionVectorParams(denseSize, false)
	if v, ok := params[denseVectorName]; ok {
		vectorsMap[denseName] = v
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
	if e != nil && e.config != nil {
		if e.config.SparseInferenceModel != "" {
			return e.config.SparseInferenceModel
		}
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

// cloudModelOptions returns the provider API key options for Qdrant cloud inference.
// Only populated when inference_mode = "cloud" and cloud_model_options is configured.
func (e *Executor) cloudModelOptions() map[string]string {
	if e == nil || e.config == nil || e.usesLocalEmbeddings() {
		return nil
	}
	return e.config.CloudModelOptions
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

func embedConcurrentQuery(ctx context.Context, client *embedding.Client, text string) ([]float32, sparse.Vector, error) {
	var denseVector []float32
	var sparseVector sparse.Vector
	var errDense, errSparse error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		denseVector, errDense = client.Embed(ctx, text)
	}()
	go func() {
		defer wg.Done()
		sparseVector = sparse.BuildQuery(text)
	}()
	wg.Wait()

	if errDense != nil {
		return nil, sparse.Vector{}, fmt.Errorf("dense embedding failed: %w", errDense)
	}
	if errSparse != nil {
		return nil, sparse.Vector{}, fmt.Errorf("sparse embedding failed: %w", errSparse)
	}

	return denseVector, sparseVector, nil
}

// EmbedDense satisfies the pipeline.Embedder interface
func (e *Executor) EmbedDense(ctx context.Context, text string, model string) ([]float32, error) {
	embedClient, err := e.embeddingClient(model)
	if err != nil {
		return nil, err
	}
	dense, _, err := embedConcurrentQuery(ctx, embedClient, text)
	return dense, err
}

// EmbedSparse satisfies the pipeline.Embedder interface
func (e *Executor) EmbedSparse(ctx context.Context, text string) ([]uint32, []float32, error) {
	sv := sparse.BuildQuery(text)
	return sv.Indices, sv.Values, nil
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
	if model != nil && *model != "" && e.config != nil && e.config.EmbeddingDimension == 0 {
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

func runConfiguredREPL() error {
	cfg, client, err := loadSavedConfigAndClient()
	if err != nil {
		return err
	}
	return repl.NewREPL(cfg, NewExecutor(client, cfg)).Run()
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
	defaultInferenceMode    = "local"
)

var Version = "0.5.0"

type commandOutputMode struct {
	json  bool
	quiet bool
}

type Executor struct {
	client qdrantClient
	config *config.Config
}

func (e *Executor) defaultContext() (context.Context, context.CancelFunc) {
	timeout := 30 * time.Second
	if e != nil && e.config != nil && e.config.RequestTimeout > 0 {
		timeout = time.Duration(e.config.RequestTimeout) * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}

// requestTimeout returns the Qdrant-native per-request timeout, or nil if unset.
func (e *Executor) requestTimeout() *uint64 {
	if e != nil && e.config != nil && e.config.RequestTimeout > 0 {
		return qdrant.PtrOf(uint64(e.config.RequestTimeout))
	}
	return nil
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
	QueryBatch(context.Context, *qdrant.QueryBatchPoints) ([]*qdrant.BatchResult, error)
	QueryGroups(context.Context, *qdrant.QueryPointGroups) ([]*qdrant.PointGroup, error)
	Delete(context.Context, *qdrant.DeletePoints) (*qdrant.UpdateResult, error)
	UpdateVectors(context.Context, *qdrant.UpdatePointVectors) (*qdrant.UpdateResult, error)
	SetPayload(context.Context, *qdrant.SetPayloadPoints) (*qdrant.UpdateResult, error)
	CreateFieldIndex(context.Context, *qdrant.CreateFieldIndexCollection) (*qdrant.UpdateResult, error)
	Count(context.Context, *qdrant.CountPoints) (uint64, error)
	ScrollAndOffset(context.Context, *qdrant.ScrollPoints) ([]*qdrant.RetrievedPoint, *qdrant.PointId, error)
	Get(context.Context, *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error)
}

func NewClient(cfg *config.Config) (*qdrant.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	return newClientFromURL(cfg.URL, cfg.Secret, cfg.NoVerify, cfg.CACert)
}

func (e *Executor) DumpCollection(collection, outputPath string, batchSize int) (string, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()
	denseModel := e.resolveDenseModel(nil)
	sparseModel := ""
	if e.config != nil && e.config.SparseInferenceModel != "" {
		sparseModel = e.config.SparseInferenceModel
	}
	written, skipped, err := dump.CollectionWithModel(ctx, e.client, collection, outputPath, batchSize, denseModel, sparseModel)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Dumped collection '%s' to %s (%d written, %d skipped)", collection, outputPath, written, skipped), nil
}

func (e *Executor) Explain(query string) (string, error) {
	result, err := e.ExplainResult(query)
	if err != nil {
		return "", err
	}
	return result.Plan, nil
}

func (e *Executor) ExplainResult(query string) (*ExplainResponse, error) {
	node, err := parseQuery(query)
	if err != nil {
		return nil, err
	}

	var plan strings.Builder

	switch n := node.(type) {
	case *ast.ShowCollectionsStmt:
		plan.WriteString("Statement: SHOW COLLECTIONS\n")
		plan.WriteString("Action: List all collections\n")
	case *ast.ShowCollectionStmt:
		fmt.Fprintf(&plan, "Statement: SHOW COLLECTION %s\n", n.Collection)
		plan.WriteString("Action: Inspect collection diagnostics\n")
	case *ast.CreateCollectionStmt:
		fmt.Fprintf(&plan, "Statement: CREATE COLLECTION %s\n", n.Collection)
		if n.Model != nil && *n.Model != "" {
			fmt.Fprintf(&plan, "Model: %s\n", *n.Model)
		}
		if n.Config != nil {
			if n.Config.Hnsw != nil && n.Config.Hnsw.PayloadM != nil {
				fmt.Fprintf(&plan, "HNSW payload_m: %d\n", *n.Config.Hnsw.PayloadM)
			}
		}
		if n.Config != nil && n.Config.Quantization != nil {
			fmt.Fprintf(&plan, "Quantization: %s\n", n.Config.Quantization.Type)
			if n.Config.Quantization.Quantile != nil {
				fmt.Fprintf(&plan, "Quantile: %.4f\n", *n.Config.Quantization.Quantile)
			}
			if n.Config.Quantization.TurboBits != nil {
				fmt.Fprintf(&plan, "Turbo bits: %g\n", *n.Config.Quantization.TurboBits)
			}
			if n.Config.Quantization.AlwaysRAM {
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
		for _, v := range n.Vectors {
			fmt.Fprintf(&plan, "Vector: %s, Size: %d\n", v.Name, v.Size)
			if v.Multivector != nil {
				fmt.Fprintf(&plan, "  Multivector: comparator=%s\n", v.Multivector.Comparator)
			}
			if v.Hnsw != nil {
				if v.Hnsw.M != nil {
					fmt.Fprintf(&plan, "  HNSW m: %d\n", *v.Hnsw.M)
				}
			}
		}
		plan.WriteString("Action: Create new collection\n")
	case *ast.AlterCollectionStmt:
		fmt.Fprintf(&plan, "Statement: ALTER COLLECTION %s\n", n.Collection)
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
		if n.Config != nil && n.Config.QuantizationUpdate != nil {
			if n.Config.QuantizationUpdate.Disabled {
				plan.WriteString("Alteration: Disable quantization\n")
			} else {
				fmt.Fprintf(&plan, "Alteration: %s quantization\n", n.Config.QuantizationUpdate.Config.Type)
			}
		}
		plan.WriteString("Action: Alter existing collection\n")
	case *ast.DropCollectionStmt:
		fmt.Fprintf(&plan, "Statement: DROP COLLECTION %s\n", n.Collection)
		plan.WriteString("Action: Delete collection and all points\n")
	case *ast.InsertStmt:
		fmt.Fprintf(&plan, "Statement: INSERT INTO %s\n", n.Collection)
		if n.Model != nil && *n.Model != "" {
			fmt.Fprintf(&plan, "Model: %s\n", *n.Model)
		}
		if len(n.EmbedDirectives) > 0 {
			plan.WriteString("Embed directives:\n")
			for _, dir := range n.EmbedDirectives {
				line := fmt.Sprintf("  %s -> %s", dir.SourceField, dir.TargetVector)
				if dir.SparseModel != nil {
					line += fmt.Sprintf(" (sparse: %s)", *dir.SparseModel)
				} else if dir.Model != nil {
					line += fmt.Sprintf(" (model: %s)", *dir.Model)
				}
				plan.WriteString(line + "\n")
			}
		} else if n.Hybrid {
			plan.WriteString("Search: HYBRID (dense + sparse)\n")
		} else {
			plan.WriteString("Search: DENSE\n")
		}
		fmt.Fprintf(&plan, "Rows: %d\n", len(n.ValuesList))
		plan.WriteString("Action: Insert point(s) with auto-vectorization\n")
	case *ast.SelectStmt:
		fmt.Fprintf(&plan, "Statement: SELECT * FROM %s WHERE id = '%v'\n", n.Collection, n.PointID)
		plan.WriteString("Action: Retrieve a single point by ID\n")
	case *ast.ScrollStmt:
		fmt.Fprintf(&plan, "Statement: SCROLL FROM %s LIMIT %d\n", n.Collection, n.Limit)
		if n.QueryFilter != nil {
			fmt.Fprintf(&plan, "Filter: %s\n", e.filterToString(n.QueryFilter))
		}
		if n.After != nil {
			fmt.Fprintf(&plan, "After: %v\n", n.After)
		}
		plan.WriteString("Action: Scroll (paginate) through points\n")

	case *ast.QueryStmt:
		// Statement line
		switch n.Mode {
		case ast.QueryModeOrderBy:
			dir := "ASC"
			if n.OrderByAsc != nil && !*n.OrderByAsc {
				dir = "DESC"
			}
			field := ""
			if n.OrderByField != nil {
				field = *n.OrderByField
			}
			fmt.Fprintf(&plan, "Statement: QUERY ORDER BY %s %s FROM %s LIMIT %v\n", field, dir, n.Collection, n.Limit)
		case ast.QueryModeSample:
			fmt.Fprintf(&plan, "Statement: QUERY SAMPLE FROM %s LIMIT %v\n", n.Collection, n.Limit)
		case ast.QueryModeRelevanceFeedback:
			fmt.Fprintf(&plan, "Statement: QUERY RELEVANCE FEEDBACK FROM %s LIMIT %v\n", n.Collection, n.Limit)
		default:
			fmt.Fprintf(&plan, "Statement: QUERY %s FROM %s LIMIT %v\n", string(n.Mode), n.Collection, n.Limit)
		}

		// Query text, ID, or RawVector
		if len(n.RawVector) > 0 {
			var strs []string
			for _, v := range n.RawVector {
				strs = append(strs, fmt.Sprintf("%g", v))
			}
			fmt.Fprintf(&plan, "Raw Vector: [%s]\n", strings.Join(strs, ", "))
		} else if n.QueryText != nil {
			fmt.Fprintf(&plan, "Query: '%s'\n", *n.QueryText)
		} else if n.QueryID != nil {
			fmt.Fprintf(&plan, "Query ID: %v\n", n.QueryID)
		}

		// USING
		if n.Type == ast.QueryTypeHybrid {
			plan.WriteString("Using: HYBRID\n")
		} else if n.Type == ast.QueryTypeSparse {
			if n.Using != nil {
				fmt.Fprintf(&plan, "Using: SPARSE '%s'\n", *n.Using)
			} else {
				plan.WriteString("Using: SPARSE\n")
			}
		} else if n.Using != nil {
			fmt.Fprintf(&plan, "Using: '%s'\n", *n.Using)
		}

		// WITH MODEL
		if n.Model != nil {
			fmt.Fprintf(&plan, "Model: %s\n", *n.Model)
		}

		// WITH { ... } params
		if n.WithClause != nil {
			if n.WithClause.HnswEf > 0 {
				fmt.Fprintf(&plan, "HNSW ef: %d\n", n.WithClause.HnswEf)
			}
			if n.WithClause.Exact {
				plan.WriteString("Exact: true\n")
			}
			if n.WithClause.Acorn {
				plan.WriteString("ACORN: true\n")
			}
			if n.WithClause.IndexedOnly {
				plan.WriteString("Indexed only: true\n")
			}
			if n.WithClause.Quantization != nil {
				plan.WriteString("Quantization: enabled\n")
			}
			if n.WithClause.MmrDiversity != nil {
				fmt.Fprintf(&plan, "MMR diversity: %v\n", *n.WithClause.MmrDiversity)
			}
			if n.WithClause.MmrCandidates != nil {
				fmt.Fprintf(&plan, "MMR candidates: %v\n", *n.WithClause.MmrCandidates)
			}
			if n.WithClause.RrfK != nil {
				fmt.Fprintf(&plan, "RRF K: %d\n", *n.WithClause.RrfK)
			}
			if len(n.WithClause.RrfWeights) > 0 {
				fmt.Fprintf(&plan, "RRF weights: %v\n", n.WithClause.RrfWeights)
			}
		}

		// WITH PAYLOAD / WITH VECTORS
		if n.WithPayload != nil {
			if n.WithPayload.Enable != nil {
				fmt.Fprintf(&plan, "With payload: %v\n", *n.WithPayload.Enable)
			} else if len(n.WithPayload.Include) > 0 {
				fmt.Fprintf(&plan, "With payload include: %v\n", n.WithPayload.Include)
			} else if len(n.WithPayload.Exclude) > 0 {
				fmt.Fprintf(&plan, "With payload exclude: %v\n", n.WithPayload.Exclude)
			}
		}
		if n.WithVectors != nil {
			if n.WithVectors.Enable != nil {
				fmt.Fprintf(&plan, "With vectors: %v\n", *n.WithVectors.Enable)
			} else if len(n.WithVectors.Vectors) > 0 {
				fmt.Fprintf(&plan, "With vectors: %v\n", n.WithVectors.Vectors)
			}
		}

		// WHERE
		if n.QueryFilter != nil {
			fmt.Fprintf(&plan, "Filter: %s\n", e.filterToString(n.QueryFilter))
		}

		// OFFSET
		if n.Offset > 0 {
			fmt.Fprintf(&plan, "Offset: %d\n", n.Offset)
		}

		// SCORE THRESHOLD
		if n.ScoreThreshold != nil {
			fmt.Fprintf(&plan, "Score threshold: %v\n", *n.ScoreThreshold)
		}

		// LOOKUP FROM
		if n.LookupFrom != "" {
			vec := ""
			if n.LookupVector != nil {
				vec = fmt.Sprintf(" VECTOR '%s'", *n.LookupVector)
			}
			fmt.Fprintf(&plan, "Lookup from: %s%s\n", n.LookupFrom, vec)
		}

		// GROUP BY / GROUP SIZE
		if n.GroupBy != nil {
			fmt.Fprintf(&plan, "Group by: %s\n", *n.GroupBy)
		}
		if n.GroupSize != nil {
			fmt.Fprintf(&plan, "Group size: %d\n", *n.GroupSize)
		}

		// WITH LOOKUP
		if n.WithLookupCollection != nil {
			fmt.Fprintf(&plan, "With lookup: %s\n", *n.WithLookupCollection)
		}

		// RERANK
		if n.Rerank {
			if n.RerankModel != nil {
				fmt.Fprintf(&plan, "Rerank: model '%s'\n", *n.RerankModel)
			} else {
				plan.WriteString("Rerank: enabled\n")
			}
		}

		// STRATEGY
		if n.Strategy != nil {
			fmt.Fprintf(&plan, "Strategy: %s\n", *n.Strategy)
		}

		// RECOMMEND mode details
		if n.Mode == ast.QueryModeRecommend {
			if len(n.PositiveIDs) > 0 {
				fmt.Fprintf(&plan, "Positive IDs: %v\n", n.PositiveIDs)
			}
			if len(n.NegativeIDs) > 0 {
				fmt.Fprintf(&plan, "Negative IDs: %v\n", n.NegativeIDs)
			}
		}

		// CONTEXT mode details
		if n.Mode == ast.QueryModeContext && len(n.ContextPairs) > 0 {
			fmt.Fprintf(&plan, "Context pairs: %d\n", len(n.ContextPairs))
		}

		// DISCOVER mode details
		if n.Mode == ast.QueryModeDiscover {
			if n.Target != nil {
				fmt.Fprintf(&plan, "Target: %v\n", n.Target)
			}
			if len(n.ContextPairs) > 0 {
				fmt.Fprintf(&plan, "Context pairs: %d\n", len(n.ContextPairs))
			}
		}

		// RELEVANCE FEEDBACK mode details
		if n.Mode == ast.QueryModeRelevanceFeedback {
			fmt.Fprintf(&plan, "Feedback target: %v\n", n.FeedbackTarget)
			if len(n.FeedbackItems) > 0 {
				fmt.Fprintf(&plan, "Feedback items: %d\n", len(n.FeedbackItems))
			}
			if n.FeedbackStrategy != nil {
				fmt.Fprintf(&plan, "Strategy: %s (a=%g, b=%g, c=%g)\n", n.FeedbackStrategy.Type, n.FeedbackStrategy.A, n.FeedbackStrategy.B, n.FeedbackStrategy.C)
			}
		}

		// CTEs
		if len(n.CTEs) > 0 {
			fmt.Fprintf(&plan, "CTEs: %d defined\n", len(n.CTEs))
		}

		// PREFETCH refs
		if len(n.PrefetchRefs) > 0 {
			refs := make([]string, len(n.PrefetchRefs))
			for i, ref := range n.PrefetchRefs {
				refs[i] = ref.CTEName
			}
			fmt.Fprintf(&plan, "Prefetch: %v\n", refs)
		}

		// FUSION
		if n.FusionType != nil {
			fmt.Fprintf(&plan, "Fusion: %s\n", *n.FusionType)
		}

		// FORMULA
		if n.Formula != nil {
			fmt.Fprintf(&plan, "Formula: %s\n", ast.FormulaExprString(n.Formula))
		}
		if len(n.FormulaDefaults) > 0 {
			var defs []string
			for k, v := range n.FormulaDefaults {
				defs = append(defs, fmt.Sprintf("%s = %g", k, v))
			}
			fmt.Fprintf(&plan, "Defaults: %s\n", strings.Join(defs, ", "))
		}

		plan.WriteString("Action: Universal Query\n")
	case *ast.DeleteStmt:
		if n.Field != "" {
			fmt.Fprintf(&plan, "Statement: DELETE FROM %s WHERE %s = '%v'\n",
				n.Collection, n.Field, n.Value)
			plan.WriteString("Action: Delete points by filter\n")
		} else {
			fmt.Fprintf(&plan, "Statement: DELETE FROM %s WHERE id = '%v'\n",
				n.Collection, n.PointID)
			plan.WriteString("Action: Delete point by ID\n")
		}
	case *ast.UpdateVectorStmt:
		name := ""
		if n.VectorName != nil {
			name = fmt.Sprintf(" '%s'", *n.VectorName)
		}
		fmt.Fprintf(&plan, "Statement: UPDATE %s SET VECTOR%s = [...] WHERE id = '%v'\n", n.Collection, name, n.PointID)
		fmt.Fprintf(&plan, "Vector length: %d\n", len(n.Vector))
		plan.WriteString("Action: Update point vector\n")
	case *ast.UpdatePayloadStmt:
		if n.QueryFilter != nil {
			fmt.Fprintf(&plan, "Statement: UPDATE %s SET PAYLOAD = {...} WHERE %s\n", n.Collection, e.filterToString(n.QueryFilter))
			plan.WriteString("Action: Update payload for points matching filter\n")
		} else {
			fmt.Fprintf(&plan, "Statement: UPDATE %s SET PAYLOAD = {...} WHERE id = '%v'\n", n.Collection, n.PointID)
			plan.WriteString("Action: Update payload for point by ID\n")
		}
	case *ast.CreateIndexStmt:
		fmt.Fprintf(&plan, "Statement: CREATE INDEX ON COLLECTION %s FOR %s", n.Collection, n.Field)
		if n.FieldType != "" && n.FieldType != "keyword" {
			fmt.Fprintf(&plan, " TYPE %s", n.FieldType)
		}
		plan.WriteString("\n")
		if len(n.Options) > 0 {
			fmt.Fprintf(&plan, "Options: %v\n", n.Options)
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
	return ast.FormatFilterExpr(filter)
}

func (e *Executor) configuredModel() string {
	if e != nil && e.config != nil && e.config.InferenceModel != "" {
		return e.config.InferenceModel
	}
	return denseModelDefault
}

func (e *Executor) doShowCollection(n *ast.ShowCollectionStmt) (*ExecResponse, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()

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
				b.WriteString(line)
				b.WriteString("\n")
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
	return qdrantutil.SerializeKeywordBoolFields(params.IsTenant, params.OnDisk, params.EnableHnsw)
}

func serializeUUIDIndexParams(params *qdrant.UuidIndexParams) map[string]any {
	if params == nil {
		return nil
	}
	return qdrantutil.SerializeKeywordBoolFields(params.IsTenant, params.OnDisk, params.EnableHnsw)
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
	ctx, cancel := e.defaultContext()
	defer cancel()

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

	var vectorsConfig *qdrant.VectorsConfig
	if len(n.Vectors) > 0 {
		paramsMap := make(map[string]*qdrant.VectorParams)
		for _, v := range n.Vectors {
			var qDist qdrant.Distance
			switch v.Distance {
			case ast.DistanceCosine:
				qDist = qdrant.Distance_Cosine
			case ast.DistanceDot:
				qDist = qdrant.Distance_Dot
			case ast.DistanceEuclid:
				qDist = qdrant.Distance_Euclid
			case ast.DistanceManhattan:
				qDist = qdrant.Distance_Manhattan
			}
			vp := &qdrant.VectorParams{
				Size:     v.Size,
				Distance: qDist,
			}
			if v.Hnsw != nil {
				vp.HnswConfig = buildHnswConfigDiff(v.Hnsw)
			}
			if v.Quantization != nil {
				cfg, err := buildQuantizationConfig(v.Quantization)
				if err != nil {
					return nil, err
				}
				vp.QuantizationConfig = cfg
			}
			if v.Multivector != nil {
				vp.MultivectorConfig = &qdrant.MultiVectorConfig{
					Comparator: qdrant.MultiVectorComparator_MaxSim,
				}
			}
			paramsMap[v.Name] = vp
		}
		vectorsConfig = qdrant.NewVectorsConfigMap(paramsMap)
	} else {
		denseSize, err := e.resolveDenseVectorSize(ctx, n.Model)
		if err != nil {
			return nil, err
		}
		params := collectionVectorParams(denseSize, n.Rerank)
		if n.DenseVector != nil {
			if vp, ok := params[denseVectorName]; ok {
				delete(params, denseVectorName)
				params[*n.DenseVector] = vp
			}
		}
		vectorsConfig = qdrant.NewVectorsConfigMap(params)
	}

	collection := &qdrant.CreateCollection{
		CollectionName: n.Collection,
		VectorsConfig:  vectorsConfig,
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
	if n.Config != nil && n.Config.Quantization != nil {
		collection.QuantizationConfig, err = buildQuantizationConfig(n.Config.Quantization)
		if err != nil {
			return nil, err
		}
	}
	if len(n.SparseVectors) > 0 {
		sparseMap := make(map[string]*qdrant.SparseVectorParams)
		for _, sv := range n.SparseVectors {
			sparseMap[sv.Name] = &qdrant.SparseVectorParams{
				Modifier: qdrant.Modifier_Idf.Enum(),
			}
		}
		collection.SparseVectorsConfig = qdrant.NewSparseVectorsConfig(sparseMap)
	} else if n.Hybrid || n.Rerank {
		sparseName := sparseVectorName
		if n.SparseVector != nil {
			sparseName = *n.SparseVector
		}
		collection.SparseVectorsConfig = qdrant.NewSparseVectorsConfig(map[string]*qdrant.SparseVectorParams{
			sparseName: {
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
	message := fmt.Sprintf("Collection '%s' created", n.Collection)
	if len(n.Vectors) == 0 {
		message += " (dense)"
		if n.Hybrid || n.Rerank {
			if n.Rerank {
				message = fmt.Sprintf("Collection '%s' created (hybrid: dense + sparse + ColBERT)", n.Collection)
			} else {
				message = fmt.Sprintf("Collection '%s' created (hybrid: dense + sparse)", n.Collection)
			}
		}
	} else {
		message += " (multi-vector schema)"
	}

	if n.Config != nil && n.Config.Quantization != nil {
		message = strings.TrimSuffix(message, ")") + fmt.Sprintf(", %s quantization)", n.Config.Quantization.Type)
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

func (e *Executor) doAlterCollection(n *ast.AlterCollectionStmt) (*ExecResponse, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()

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
	if n.Config != nil && n.Config.QuantizationUpdate != nil {
		var err error
		req.QuantizationConfig, err = buildAlterQuantizationConfig(n.Config.QuantizationUpdate)
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
	ctx, cancel := e.defaultContext()
	defer cancel()

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

type VectorTopology struct {
	DenseVector  *string
	SparseVector *string
	RerankVector *string
}
