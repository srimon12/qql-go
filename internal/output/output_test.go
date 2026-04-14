package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestOutputter() (*Outputter, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	return NewOutputterWithWriters(stdout, stderr), stdout, stderr
}

func TestPrintSection(t *testing.T) {
	out, stdout, _ := newTestOutputter()

	out.PrintSection("Title", "content")

	require.Contains(t, stdout.String(), "Title")
	require.Contains(t, stdout.String(), "content")
}

func TestPrintExplain(t *testing.T) {
	out, stdout, _ := newTestOutputter()

	out.PrintExplain("plan body")

	require.Contains(t, stdout.String(), "Query Plan")
	require.Contains(t, stdout.String(), "plan body")
}

func TestPrintBanner(t *testing.T) {
	out, stdout, _ := newTestOutputter()

	out.PrintBanner()

	require.Contains(t, stdout.String(), "QQL")
	require.Contains(t, stdout.String(), "Qdrant Query Language")
}

func TestPrintConnectionStatus(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		out, stdout, stderr := newTestOutputter()

		out.PrintConnectionStatus("http://localhost:6333", true)

		require.Contains(t, stdout.String(), "Connected to http://localhost:6333")
		require.Empty(t, stderr.String())
	})

	t.Run("unhealthy", func(t *testing.T) {
		out, stdout, stderr := newTestOutputter()

		out.PrintConnectionStatus("http://localhost:6333", false)

		require.Empty(t, stdout.String())
		require.Contains(t, stderr.String(), "Cannot connect to http://localhost:6333")
		require.True(t, strings.Contains(stderr.String(), "✗"))
	})
}

func TestPrintJSON(t *testing.T) {
	out, stdout, _ := newTestOutputter()

	err := out.PrintJSON(map[string]any{
		"ok":      true,
		"message": "hello",
	}, true)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	require.Equal(t, true, payload["ok"])
	require.Equal(t, "hello", payload["message"])
}

func TestPrintJSONQuiet(t *testing.T) {
	out, stdout, _ := newTestOutputter()

	err := out.PrintJSON(map[string]any{
		"ok":      true,
		"message": "hello",
	}, false)
	require.NoError(t, err)

	require.NotContains(t, stdout.String(), "\n  ")
	var payload map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	require.Equal(t, "hello", payload["message"])
}

func TestPrintErrorWritesToConfiguredStderr(t *testing.T) {
	out, stdout, stderr := newTestOutputter()

	out.PrintError("bad things happened")

	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "bad things happened")
}
