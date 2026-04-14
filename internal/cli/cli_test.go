package cli

import (
	"bytes"
	"testing"

	"github.com/qdrant/qql-go/internal/cli/commands"
	"github.com/qdrant/qql-go/internal/output"
	"github.com/stretchr/testify/require"
)

func TestRootCommandReturnsErrorWithoutSavedConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cmd := NewRootCmd(output.NewOutputterWithWriters(&bytes.Buffer{}, &bytes.Buffer{}))

	err := cmd.RunE(cmd, nil)
	require.Error(t, err)
	require.False(t, ErrorPrinted(err))
	require.Equal(t, 1, ExitCode(err))
	require.Equal(t, "not connected. Run: qql connect --url <url>", err.Error())
}

func TestCliExitHelpersUseCommandExitErrors(t *testing.T) {
	err := commands.NewExitError(assertionError("boom"), 7, true)

	require.True(t, ErrorPrinted(err))
	require.Equal(t, 7, ExitCode(err))
}

type assertionError string

func (e assertionError) Error() string {
	return string(e)
}
