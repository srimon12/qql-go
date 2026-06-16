package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/output"
	"github.com/stretchr/testify/require"
)

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

func TestWriteJSONWrapsEncoderFailures(t *testing.T) {
	err := writeJSON(output.NewOutputterWithWriters(failingWriter{}, &bytes.Buffer{}), map[string]any{"ok": true}, false)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to write JSON")
}

func TestTurboBitsEnum(t *testing.T) {
	require.Equal(t, qdrant.TurboQuantBitSize_Bits1, *turboBitsEnum(1.0))
	require.Equal(t, qdrant.TurboQuantBitSize_Bits1_5, *turboBitsEnum(1.5))
	require.Equal(t, qdrant.TurboQuantBitSize_Bits2, *turboBitsEnum(2.0))
	require.Equal(t, qdrant.TurboQuantBitSize_Bits4, *turboBitsEnum(4.0))
	require.Nil(t, turboBitsEnum(3.0))
}
