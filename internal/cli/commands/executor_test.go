package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectionVectorParamsOmitsColbertByDefault(t *testing.T) {
	vectors := collectionVectorParams(denseVectorSize, false)

	dense := vectors[denseVectorName]
	require.NotNil(t, dense)
	require.EqualValues(t, denseVectorSize, dense.GetSize())
	require.Equal(t, qdrant.Distance_Cosine, dense.GetDistance())
	require.NotContains(t, vectors, rerankVectorName)
}

func TestCollectionVectorParamsIncludesColbertWhenEnabled(t *testing.T) {
	vectors := collectionVectorParams(denseVectorSize, true)

	rerank := vectors[rerankVectorName]
	require.NotNil(t, rerank)
	require.EqualValues(t, rerankVectorSize, rerank.GetSize())
	require.Equal(t, qdrant.Distance_Cosine, rerank.GetDistance())
	require.NotNil(t, rerank.GetMultivectorConfig())
	require.Equal(t, qdrant.MultiVectorComparator_MaxSim, rerank.GetMultivectorConfig().GetComparator())
	require.NotNil(t, rerank.GetHnswConfig())
	require.Zero(t, rerank.GetHnswConfig().GetM())
}

func TestPointIDString(t *testing.T) {
	require.Equal(t, "abc", pointIDString(qdrant.NewIDUUID("abc")))
	require.Equal(t, "42", pointIDString(qdrant.NewIDNum(42)))
	require.Equal(t, "", pointIDString(nil))
}

func strPtr(s string) *string {
	return &s
}

func TestExecutorExplainDocumentedQueries(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{})

	tests := []struct {
		name  string
		query string
		wants []string
	}{
		{
			name:  "create hybrid rerank",
			query: "CREATE COLLECTION docs HYBRID RERANK",
			wants: []string{
				"Statement: CREATE COLLECTION docs",
				"Type: HYBRID + RERANK (dense + sparse + ColBERT multivector)",
			},
		},
		{
			name:  "create with quantization",
			query: "CREATE COLLECTION docs WITH QUANTIZATION (type = 'scalar', quantile = 0.95, always_ram = true)",
			wants: []string{
				"Statement: CREATE COLLECTION docs",
				"Quantization: scalar",
				"Quantile: 0.9500",
				"Quantization storage: ALWAYS RAM",
			},
		},
		{
			name:  "hybrid search rerank",
			query: "QUERY NEAREST 'vector database' FROM docs LIMIT 5 USING HYBRID RERANK",
			wants: []string{
				"Statement: QUERY NEAREST FROM docs LIMIT 5",
				"Query: 'vector database'",
				"Action: Universal Query",
			},
		},
		{
			name:  "search with with clause",
			query: "QUERY NEAREST 'vector database' FROM docs LIMIT 5 WITH (hnsw_ef = 128, exact = true)",
			wants: []string{
				"Statement: QUERY NEAREST FROM docs LIMIT 5",
				"Query: 'vector database'",
				"Action: Universal Query",
			},
		},
		{
			name:  "search with filter",
			query: "QUERY NEAREST 'vector search' FROM notes LIMIT 5 USING HYBRID WHERE topic = 'search'",
			wants: []string{
				"Statement: QUERY NEAREST FROM notes LIMIT 5",
				"Query: 'vector search'",
				"Action: Universal Query",
			},
		},
		{
			name:  "delete by id",
			query: "DELETE FROM notes WHERE id = 'uuid'",
			wants: []string{
				"Statement: DELETE FROM notes WHERE id = 'uuid'",
				"Action: Delete point by ID",
			},
		},
		{
			name:  "delete by field",
			query: "DELETE FROM notes WHERE status = 'archived'",
			wants: []string{
				"Statement: DELETE FROM notes WHERE status = 'archived'",
				"Action: Delete points by filter",
			},
		},
		{
			name:  "sample random",
			query: "QUERY SAMPLE FROM docs LIMIT 10",
			wants: []string{
				"Statement: QUERY SAMPLE FROM docs LIMIT 10",
				"Action: Universal Query",
			},
		},
		{
			name:  "sample with filter",
			query: "QUERY SAMPLE FROM docs LIMIT 5 WHERE category = 'tech'",
			wants: []string{
				"Statement: QUERY SAMPLE FROM docs LIMIT 5",
				"Filter:",
				"Action: Universal Query",
			},
		},
		{
			name:  "query with all fields",
			query: "QUERY NEAREST 'search' FROM docs LIMIT 10 OFFSET 5 USING HYBRID SCORE THRESHOLD 0.5 WHERE topic = 'ai' RERANK MODEL 'reranker' STRATEGY 'best_score' WITH (hnsw_ef = 128, exact = true) WITH PAYLOAD (include = ['title']) WITH VECTORS true GROUP BY 'category' GROUP_SIZE 3 WITH LOOKUP FROM metadata",
			wants: []string{
				"Statement: QUERY NEAREST FROM docs LIMIT 10",
				"Query: 'search'",
				"Using: HYBRID",
				"Exact: true",
				"HNSW ef: 128",
				"With payload include: [title]",
				"With vectors: true",
				"Filter:",
				"Offset: 5",
				"Score threshold: 0.5",
				"Group by: category",
				"Group size: 3",
				"With lookup: metadata",
				"Rerank: model 'reranker'",
				"Strategy: best_score",
				"Action: Universal Query",
			},
		},
		{
			name:  "update named vector",
			query: "UPDATE docs SET VECTOR 'colbert' = [1.0, 2.0] WHERE id = 12",
			wants: []string{
				"Statement: UPDATE docs SET VECTOR 'colbert' = [...] WHERE id = '12'",
				"Vector length: 2",
			},
		},
		{
			name:  "nested CTE query",
			query: "WITH p2 AS (WITH p1 AS (QUERY 'inner' USING dense LIMIT 50) QUERY 'outer' USING sparse LIMIT 100 PREFETCH (p1)) QUERY 'test' FROM docs PREFETCH (p2)",
			wants: []string{
				"Statement: QUERY NEAREST FROM docs LIMIT 10",
				"Prefetch: [p2]",
				"CTEs: 1 defined",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := exec.Explain(tt.query)
			require.NoError(t, err)
			for _, want := range tt.wants {
				require.Contains(t, got, want)
			}
		})
	}
}

func TestExplainCommandQuietTextOmitsSectionHeader(t *testing.T) {
	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewExplainCmd(out)
		require.NoError(t, cmd.Flags().Set("quiet", "true"))
		err := cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.Contains(t, stdout, "Statement: SHOW COLLECTIONS")
	require.NotContains(t, stdout, "\033[1mQuery Plan\033[0m")
}

func TestVersionCommandDefaultsToDevWhenVersionBlank(t *testing.T) {
	original := Version
	Version = "   "
	t.Cleanup(func() {
		Version = original
	})

	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewVersionCmd(out)
		require.NoError(t, cmd.Flags().Set("quiet", "true"))
		require.NoError(t, cmd.RunE(cmd, nil))
	})

	require.Empty(t, stderr)
	require.Equal(t, "dev\n", stdout)
}

func TestDumpCommandRejectsInvalidBatchSize(t *testing.T) {
	stdout, stderr, err := captureCommandResult(t, func(out *output.Outputter) error {
		cmd := NewDumpCmd(out)
		require.NoError(t, cmd.Flags().Set("batch-size", "0"))
		return cmd.RunE(cmd, []string{"docs", "backup.qql"})
	})

	require.Error(t, err)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "--batch-size must be greater than 0")
}

func TestConnectCommandMissingURLReturnsPrintedError(t *testing.T) {
	stdout, stderr, err := captureCommandResult(t, func(out *output.Outputter) error {
		cmd := NewConnectCmd(out)
		return cmd.RunE(cmd, nil)
	})

	require.Empty(t, stdout)
	require.Error(t, err)
	require.True(t, ErrorPrinted(err))
	require.Equal(t, 1, ExitCode(err))
	require.Contains(t, stderr, "--url is required")
}

func TestExplainResultReturnsErrorForInvalidQuery(t *testing.T) {
	_, err := NewExecutor(nil, nil).ExplainResult("EXPLAIN QUERY NEAREST 'x' FROM docs LIMIT 1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse error")
}

type failingWriter struct{}

func float64Ptr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(v int) *int {
	return &v
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}

func newEmbeddingServer(t *testing.T, embedding []float32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/embeddings", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": embedding},
			},
		}))
	}))
}

func TestBuildWithPayload(t *testing.T) {
	val := false
	assert.Equal(t, false, buildWithPayload(&ast.PayloadSelector{Enable: &val}).GetEnable())

	include := []string{"title"}
	assert.Equal(t, include, buildWithPayload(&ast.PayloadSelector{Include: include}).GetInclude().GetFields())

	exclude := []string{"embedding"}
	assert.Equal(t, exclude, buildWithPayload(&ast.PayloadSelector{Exclude: exclude}).GetExclude().GetFields())

	assert.Nil(t, buildWithPayload(nil))
	assert.Nil(t, buildWithPayload(&ast.PayloadSelector{})) // Empty selector
}

func TestBuildWithVectors(t *testing.T) {
	val := true
	assert.Equal(t, true, buildWithVectors(&ast.VectorsSelector{Enable: &val}).GetEnable())

	vectors := []string{"dense"}
	assert.Equal(t, vectors, buildWithVectors(&ast.VectorsSelector{Vectors: vectors}).GetInclude().GetNames())

	assert.Nil(t, buildWithVectors(nil))
	assert.Nil(t, buildWithVectors(&ast.VectorsSelector{})) // Empty selector
}
