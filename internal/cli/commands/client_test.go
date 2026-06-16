package commands

import (
	"testing"

	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/internal/output"
	"github.com/stretchr/testify/require"
)

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
