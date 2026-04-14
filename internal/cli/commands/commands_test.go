package commands

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/qdrant/qql-go/internal/ast"
	"github.com/qdrant/qql-go/internal/config"
	"github.com/qdrant/qql-go/internal/output"
	"github.com/stretchr/testify/require"
)

func TestConfiguredModelUsesConfigOverride(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{InferenceModel: "test-model"})

	require.Equal(t, "test-model", exec.configuredModel())
}

func TestConfiguredModelFallsBackToDenseDefault(t *testing.T) {
	exec := NewExecutor(nil, nil)

	require.Equal(t, denseModelDefault, exec.configuredModel())
}

func TestCollectionVectorParamsOmitsColbertByDefault(t *testing.T) {
	vectors := collectionVectorParams(false)

	dense := vectors[denseVectorName]
	require.NotNil(t, dense)
	require.EqualValues(t, denseVectorSize, dense.GetSize())
	require.Equal(t, qdrant.Distance_Cosine, dense.GetDistance())
	require.NotContains(t, vectors, rerankVectorName)
}

func TestCollectionVectorParamsIncludesColbertWhenEnabled(t *testing.T) {
	vectors := collectionVectorParams(true)

	rerank := vectors[rerankVectorName]
	require.NotNil(t, rerank)
	require.EqualValues(t, rerankVectorSize, rerank.GetSize())
	require.Equal(t, qdrant.Distance_Cosine, rerank.GetDistance())
	require.NotNil(t, rerank.GetMultivectorConfig())
	require.Equal(t, qdrant.MultiVectorComparator_MaxSim, rerank.GetMultivectorConfig().GetComparator())
	require.NotNil(t, rerank.GetHnswConfig())
	require.Zero(t, rerank.GetHnswConfig().GetM())
}

func TestBuildPointVectorsIncludesColbertOnlyWhenEnabled(t *testing.T) {
	vectors := buildPointVectors("hello world", "dense-model", "sparse-model", true, true)

	dense := vectors[denseVectorName]
	require.NotNil(t, dense)
	require.NotNil(t, dense.GetDocument())
	require.Equal(t, "hello world", dense.GetDocument().GetText())
	require.Equal(t, "dense-model", dense.GetDocument().GetModel())

	rerank := vectors[rerankVectorName]
	require.NotNil(t, rerank)
	require.NotNil(t, rerank.GetDocument())
	require.Equal(t, rerankModelDefault, rerank.GetDocument().GetModel())

	sparse := vectors[sparseVectorName]
	require.NotNil(t, sparse)
	require.NotNil(t, sparse.GetDocument())
	require.Equal(t, "sparse-model", sparse.GetDocument().GetModel())

	withoutRerank := buildPointVectors("hello world", "dense-model", "sparse-model", true, false)
	require.NotContains(t, withoutRerank, rerankVectorName)
}

func TestBuildRerankSearchRequestTargetsColbertVector(t *testing.T) {
	prefetch := []*qdrant.PrefetchQuery{{}, {}}
	req := buildRerankSearchRequest("demo", "late interaction query", "answerai-colbert-test", 7, prefetch, nil)

	require.Equal(t, "demo", req.GetCollectionName())
	require.Equal(t, rerankVectorName, req.GetUsing())
	require.Equal(t, uint64(7), req.GetLimit())
	require.Len(t, req.GetPrefetch(), 2)
	require.NotNil(t, req.GetQuery().GetNearest())
	require.NotNil(t, req.GetQuery().GetNearest().GetDocument())
	require.Equal(t, "answerai-colbert-test", req.GetQuery().GetNearest().GetDocument().GetModel())
}

func TestBuildSearchRequestAppliesWithClauseAndSparseOverride(t *testing.T) {
	sparseModel := "custom-sparse"
	req, err := buildSearchRequest(&ast.SearchStmt{
		Collection:  "demo",
		QueryText:   "vector database",
		Limit:       5,
		Hybrid:      true,
		SparseModel: &sparseModel,
		WithClause: &ast.SearchWith{
			HnswEf: 128,
			Exact:  true,
			Acorn:  true,
		},
	}, "dense-model", sparseModel, false, 5)
	require.NoError(t, err)

	require.Equal(t, "demo", req.GetCollectionName())
	require.Equal(t, uint64(5), req.GetLimit())
	require.NotNil(t, req.GetParams())
	require.Equal(t, uint64(128), req.GetParams().GetHnswEf())
	require.True(t, req.GetParams().GetExact())
	require.NotNil(t, req.GetParams().GetAcorn())
	require.True(t, req.GetParams().GetAcorn().GetEnable())

	prefetch := req.GetPrefetch()
	require.Len(t, prefetch, 2)
	require.Equal(t, "custom-sparse", prefetch[0].GetQuery().GetNearest().GetDocument().GetModel())
	require.Equal(t, "dense-model", prefetch[1].GetQuery().GetNearest().GetDocument().GetModel())
	require.NotNil(t, prefetch[0].GetParams())
	require.Equal(t, uint64(128), prefetch[0].GetParams().GetHnswEf())
}

func TestBuildSearchRequestRejectsRerankWithoutCollectionSupport(t *testing.T) {
	_, err := buildSearchRequest(&ast.SearchStmt{
		Collection: "demo",
		QueryText:  "vector database",
		Limit:      5,
		Rerank:     true,
	}, "dense-model", "custom-sparse", false, 5)
	require.Error(t, err)
}

func TestBuildDeleteRequestByFieldUsesFilterSelector(t *testing.T) {
	req, err := buildDeleteRequest(&ast.DeleteStmt{
		Collection: "demo",
		Field:      "status",
		Value:      "archived",
	})
	require.NoError(t, err)

	require.Equal(t, "demo", req.GetCollectionName())
	filter := req.GetPoints().GetFilter()
	require.NotNil(t, filter)
	require.Len(t, filter.GetMust(), 1)
	match := filter.GetMust()[0].GetField().GetMatch()
	require.NotNil(t, match)
	require.Equal(t, "archived", match.GetKeyword())
}

func TestBuildDeleteRequestByIDUsesPointSelector(t *testing.T) {
	req, err := buildDeleteRequest(&ast.DeleteStmt{
		Collection: "demo",
		PointID:    "point-123",
	})
	require.NoError(t, err)

	require.Equal(t, "demo", req.GetCollectionName())
	points, ok := req.GetPoints().GetPointsSelectorOneOf().(*qdrant.PointsSelector_Points)
	require.True(t, ok)
	require.NotNil(t, points.Points)
	require.Len(t, points.Points.GetIds(), 1)
	require.Equal(t, "point-123", points.Points.GetIds()[0].GetUuid())
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
			name:  "hybrid search rerank",
			query: "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 USING HYBRID RERANK",
			wants: []string{
				"Statement: SEARCH docs SIMILAR TO 'vector database' LIMIT 5",
				"Search: HYBRID (dense + sparse)",
				"Rerank: enabled",
				"Rerank vector: colbert",
			},
		},
		{
			name:  "search with with clause",
			query: "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 WITH { hnsw_ef: 128, exact: true }",
			wants: []string{
				"Search params: EXACT (bypass HNSW)",
				"Search params: hnsw_ef=128",
			},
		},
		{
			name:  "search with filter",
			query: "SEARCH notes SIMILAR TO 'vector search' LIMIT 5 USING HYBRID WHERE topic = 'search'",
			wants: []string{
				"Search: HYBRID (dense + sparse)",
				"Filter:",
				"topic = search",
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

func TestBuildClientConfigNormalizesSchemeAndPort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantPort int
		wantTLS  bool
	}{
		{
			name:     "host only",
			input:    "qdrant.local",
			wantHost: "qdrant.local",
			wantPort: 6334,
		},
		{
			name:     "http with default rest port",
			input:    "http://localhost:6333",
			wantHost: "localhost",
			wantPort: 6334,
		},
		{
			name:     "https with trailing slash",
			input:    "https://example.com/",
			wantHost: "example.com",
			wantPort: 6334,
			wantTLS:  true,
		},
		{
			name:     "explicit grpc port",
			input:    "http://localhost:6334",
			wantHost: "localhost",
			wantPort: 6334,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := buildClientConfig(tt.input, "api-key")
			require.NoError(t, err)
			require.Equal(t, tt.wantHost, cfg.Host)
			require.Equal(t, tt.wantPort, cfg.Port)
			require.Equal(t, tt.wantTLS, cfg.UseTLS)
			require.Equal(t, "api-key", cfg.APIKey)
			require.True(t, cfg.SkipCompatibilityCheck)
		})
	}
}

func TestExplainCommandDoesNotNeedSavedConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	stdout, stderr := captureCommandStreams(t, func() {
		cmd := NewExplainCmd(output.NewOutputter())
		err := cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
		require.NoError(t, err)
	})

	require.Contains(t, stdout, "Query Plan")
	require.Contains(t, stdout, "Statement: SHOW COLLECTIONS")
	require.Empty(t, stderr)
}

func TestExplainCommandJSON(t *testing.T) {
	stdout, stderr := captureCommandStreams(t, func() {
		cmd := NewExplainCmd(output.NewOutputter())
		require.NoError(t, cmd.Flags().Set("json", "true"))
		err := cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	var payload ExplainResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.True(t, payload.OK)
	require.Equal(t, "SHOW COLLECTIONS", payload.Query)
	require.Contains(t, payload.Plan, "Statement: SHOW COLLECTIONS")
}

func TestExplainCommandQuietJSONIsCompact(t *testing.T) {
	stdout, stderr := captureCommandStreams(t, func() {
		cmd := NewExplainCmd(output.NewOutputter())
		require.NoError(t, cmd.Flags().Set("json", "true"))
		require.NoError(t, cmd.Flags().Set("quiet", "true"))
		err := cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.NotContains(t, stdout, "\n  ")
	var payload ExplainResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.True(t, payload.OK)
}

func TestExplainCommandQuietTextOmitsSectionHeader(t *testing.T) {
	stdout, stderr := captureCommandStreams(t, func() {
		cmd := NewExplainCmd(output.NewOutputter())
		require.NoError(t, cmd.Flags().Set("quiet", "true"))
		err := cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.Contains(t, stdout, "Statement: SHOW COLLECTIONS")
	require.NotContains(t, stdout, "\033[1mQuery Plan\033[0m")
}

func TestDisconnectCommandJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	stdout, stderr := captureCommandStreams(t, func() {
		cmd := NewDisconnectCmd(output.NewOutputter())
		require.NoError(t, cmd.Flags().Set("json", "true"))
		err := cmd.RunE(cmd, nil)
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	var payload CommandResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.True(t, payload.OK)
	require.Equal(t, "disconnect", payload.Command)
	require.Equal(t, "Disconnected. Config removed.", payload.Message)
}

func TestVersionCommandSupportsQuietAndJSON(t *testing.T) {
	t.Run("quiet text", func(t *testing.T) {
		stdout, stderr := captureCommandStreams(t, func() {
			cmd := NewVersionCmd(output.NewOutputter())
			require.NoError(t, cmd.Flags().Set("quiet", "true"))
			err := cmd.RunE(cmd, nil)
			require.NoError(t, err)
		})

		require.Empty(t, stderr)
		require.Equal(t, versionString+"\n", stdout)
	})

	t.Run("json", func(t *testing.T) {
		stdout, stderr := captureCommandStreams(t, func() {
			cmd := NewVersionCmd(output.NewOutputter())
			require.NoError(t, cmd.Flags().Set("json", "true"))
			require.NoError(t, cmd.Flags().Set("quiet", "true"))
			err := cmd.RunE(cmd, nil)
			require.NoError(t, err)
		})

		require.Empty(t, stderr)
		var payload VersionResponse
		require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
		require.True(t, payload.OK)
		require.Equal(t, "version", payload.Command)
		require.Equal(t, versionString, payload.Version)
	})
}

func TestExplainResultReturnsErrorForInvalidQuery(t *testing.T) {
	_, err := NewExecutor(nil, nil).ExplainResult("EXPLAIN SEARCH docs SIMILAR TO 'x' LIMIT 1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse error")
}

func TestExecuteResultForShowCollectionsRequiresNoOutputParsing(t *testing.T) {
	result := &ExecResponse{
		OK:        true,
		Operation: "show_collections",
		Message:   "Found 2 collection(s): a, b",
		Data: map[string]any{
			"count":       2,
			"collections": []string{"a", "b"},
		},
	}

	encoded, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded ExecResponse
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.True(t, decoded.OK)
	require.Equal(t, "show_collections", decoded.Operation)
	require.Equal(t, "Found 2 collection(s): a, b", decoded.Message)
}

func captureCommandStreams(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = stdoutW
	os.Stderr = stderrW
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	fn()

	require.NoError(t, stdoutW.Close())
	require.NoError(t, stderrW.Close())

	stdoutData, err := io.ReadAll(stdoutR)
	require.NoError(t, err)
	stderrData, err := io.ReadAll(stderrR)
	require.NoError(t, err)
	require.NoError(t, stdoutR.Close())
	require.NoError(t, stderrR.Close())

	return string(stdoutData), string(stderrData)
}
