package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/internal/output"
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

func TestEffectiveSearchLimit(t *testing.T) {
	tests := []struct {
		name   string
		limit  uint64
		rerank bool
		want   uint64
	}{
		{name: "plain search", limit: 12, want: 12},
		{name: "rerank small", limit: 10, rerank: true, want: 40},
		{name: "rerank capped", limit: 60, rerank: true, want: rerankPrefetchCap},
		{name: "rerank large keeps limit", limit: 500, rerank: true, want: 500},
		{name: "rerank overflow falls back to limit", limit: ^uint64(0), rerank: true, want: ^uint64(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, effectiveSearchLimit(tt.limit, tt.rerank))
		})
	}
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

	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewExplainCmd(out)
		err := cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
		require.NoError(t, err)
	})

	require.Contains(t, stdout, "Query Plan")
	require.Contains(t, stdout, "Statement: SHOW COLLECTIONS")
	require.Empty(t, stderr)
}

func TestExplainCommandJSON(t *testing.T) {
	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewExplainCmd(out)
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
	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewExplainCmd(out)
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

func TestDisconnectCommandJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
		cmd := NewDisconnectCmd(out)
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
		stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
			cmd := NewVersionCmd(out)
			require.NoError(t, cmd.Flags().Set("quiet", "true"))
			err := cmd.RunE(cmd, nil)
			require.NoError(t, err)
		})

		require.Empty(t, stderr)
		require.Equal(t, displayVersion()+"\n", stdout)
	})

	t.Run("json", func(t *testing.T) {
		stdout, stderr := captureCommandStreams(t, func(out *output.Outputter) {
			cmd := NewVersionCmd(out)
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
		require.Equal(t, displayVersion(), payload.Version)
	})
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

func TestExplainCommandInvalidJSONReturnsPrintedExitError(t *testing.T) {
	stdout, stderr, err := captureCommandResult(t, func(out *output.Outputter) error {
		cmd := NewExplainCmd(out)
		require.NoError(t, cmd.Flags().Set("json", "true"))
		return cmd.RunE(cmd, []string{"EXPLAIN SHOW COLLECTIONS"})
	})

	require.Empty(t, stderr)
	require.Error(t, err)
	require.True(t, ErrorPrinted(err))
	require.Equal(t, 1, ExitCode(err))

	var payload ErrorResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.False(t, payload.OK)
	require.Equal(t, "explain", payload.Command)
	require.Contains(t, payload.Error, "parse error")
}

func TestExecCommandWithoutConfigReturnsPrintedError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	stdout, stderr, err := captureCommandResult(t, func(out *output.Outputter) error {
		cmd := NewExecCmd(out)
		return cmd.RunE(cmd, []string{"SHOW COLLECTIONS"})
	})

	require.Empty(t, stdout)
	require.Error(t, err)
	require.True(t, ErrorPrinted(err))
	require.Equal(t, 1, ExitCode(err))
	require.Contains(t, stderr, "not connected. Run: qql-go connect --url <url>")
}

func TestDoctorCommandWithoutConfigReturnsPrintedError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	stdout, stderr, err := captureCommandResult(t, func(out *output.Outputter) error {
		cmd := NewDoctorCmd(out)
		return cmd.RunE(cmd, nil)
	})

	require.Empty(t, stdout)
	require.Error(t, err)
	require.True(t, ErrorPrinted(err))
	require.Equal(t, 1, ExitCode(err))
	require.Contains(t, stderr, "not connected. Run: qql-go connect --url <url>")
}

func TestREPLCommandWithoutConfigReturnsError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cmd := NewREPLCmd(output.NewOutputterWithWriters(&bytes.Buffer{}, &bytes.Buffer{}))
	err := cmd.RunE(cmd, nil)
	require.EqualError(t, err, "not connected. Run: qql-go connect --url <url>")
	require.False(t, ErrorPrinted(err))
}

func TestLoadSavedConfigAndClientWrapsInvalidURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	require.NoError(t, config.SaveConfig(&config.Config{URL: "http://localhost:bad-port"}))

	loaded, client, err := loadSavedConfigAndClient()
	require.Nil(t, loaded)
	require.Nil(t, client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "connection failed")
}

func TestSavedConfigMessageUsesResolvedPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	path, err := config.ConfigPath()
	require.NoError(t, err)
	require.Equal(t, "Connected. Config saved to "+path, savedConfigMessage())
}

func TestWriteJSONWrapsEncoderFailures(t *testing.T) {
	err := writeJSON(output.NewOutputterWithWriters(failingWriter{}, &bytes.Buffer{}), map[string]any{"ok": true}, false)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to write JSON")
}

func TestWaitForCollectionReadyMessages(t *testing.T) {
	err := waitForCollectionReady(context.Background(), "docs", 5*time.Millisecond, time.Millisecond, func(context.Context, string) (bool, bool, error) {
		return false, false, nil
	})
	require.EqualError(t, err, "collection 'docs' did not become visible within 5ms")

	err = waitForCollectionReady(context.Background(), "docs", 5*time.Millisecond, time.Millisecond, func(context.Context, string) (bool, bool, error) {
		return true, false, nil
	})
	require.EqualError(t, err, "collection 'docs' exists but is not ready yet after 5ms")
}

func TestWaitForCollectionReadyWrapsProbeErrors(t *testing.T) {
	err := waitForCollectionReady(context.Background(), "docs", 5*time.Millisecond, time.Millisecond, func(context.Context, string) (bool, bool, error) {
		return true, false, context.DeadlineExceeded
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "collection 'docs' did not become ready within 5ms")
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

func captureCommandStreams(t *testing.T, fn func(*output.Outputter)) (string, string) {
	t.Helper()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	fn(output.NewOutputterWithWriters(stdout, stderr))
	return stdout.String(), stderr.String()
}

func captureCommandResult(t *testing.T, fn func(*output.Outputter) error) (string, string, error) {
	t.Helper()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err := fn(output.NewOutputterWithWriters(stdout, stderr))
	return stdout.String(), stderr.String(), err
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, context.DeadlineExceeded
}
