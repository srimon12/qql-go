package output

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func captureStdout(t *testing.T, fn func(*Outputter)) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	out := NewOutputter()
	fn(out)

	require.NoError(t, w.Close())
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	return string(data)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
	}()

	fn()

	require.NoError(t, w.Close())
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	return string(data)
}

func TestPrintSection(t *testing.T) {
	got := captureStdout(t, func(out *Outputter) {
		out.PrintSection("Title", "content")
	})

	require.Contains(t, got, "Title")
	require.Contains(t, got, "content")
}

func TestPrintExplain(t *testing.T) {
	got := captureStdout(t, func(out *Outputter) {
		out.PrintExplain("plan body")
	})

	require.Contains(t, got, "Query Plan")
	require.Contains(t, got, "plan body")
}

func TestPrintBanner(t *testing.T) {
	got := captureStdout(t, func(out *Outputter) {
		out.PrintBanner()
	})

	require.Contains(t, got, "QQL")
	require.Contains(t, got, "Qdrant Query Language")
}

func TestPrintConnectionStatus(t *testing.T) {
	healthy := captureStdout(t, func(out *Outputter) {
		out.PrintConnectionStatus("http://localhost:6333", true)
	})
	require.Contains(t, healthy, "Connected to http://localhost:6333")

	unhealthy := captureStderr(t, func() {
		out := NewOutputter()
		out.PrintConnectionStatus("http://localhost:6333", false)
	})
	require.Contains(t, unhealthy, "Cannot connect to http://localhost:6333")
	require.True(t, strings.Contains(unhealthy, "✗"))
}

func TestPrintJSON(t *testing.T) {
	got := captureStdout(t, func(out *Outputter) {
		err := out.PrintJSON(map[string]any{
			"ok":      true,
			"message": "hello",
		}, true)
		require.NoError(t, err)
	})

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &payload))
	require.Equal(t, true, payload["ok"])
	require.Equal(t, "hello", payload["message"])
}

func TestPrintJSONQuiet(t *testing.T) {
	got := captureStdout(t, func(out *Outputter) {
		err := out.PrintJSON(map[string]any{
			"ok":      true,
			"message": "hello",
		}, false)
		require.NoError(t, err)
	})

	require.NotContains(t, got, "\n  ")
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &payload))
	require.Equal(t, "hello", payload["message"])
}
