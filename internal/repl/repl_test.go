package repl

import (
	"bufio"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/qdrant/qql-go/internal/config"
	"github.com/stretchr/testify/require"
)

type stubExecutor struct {
	executeQuery  string
	executeResult string
	executeErr    error

	explainQuery  string
	explainResult string
	explainErr    error
}

func (s *stubExecutor) Execute(query string) (string, error) {
	s.executeQuery = query
	return s.executeResult, s.executeErr
}

func (s *stubExecutor) Explain(query string) (string, error) {
	s.explainQuery = query
	return s.explainResult, s.explainErr
}

func captureREPL(t *testing.T, exec QueryExecutor, fn func(r *REPL)) (string, string) {
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

	repl := NewREPL(&config.Config{URL: "http://localhost:6333"}, exec)
	fn(repl)

	require.NoError(t, stdoutW.Close())
	require.NoError(t, stderrW.Close())

	stdout, err := io.ReadAll(stdoutR)
	require.NoError(t, err)
	stderr, err := io.ReadAll(stderrR)
	require.NoError(t, err)

	require.NoError(t, stdoutR.Close())
	require.NoError(t, stderrR.Close())

	return string(stdout), string(stderr)
}

func TestHandleCommandBuiltinCommands(t *testing.T) {
	exec := &stubExecutor{}

	stdout, stderr := captureREPL(t, exec, func(r *REPL) {
		require.NoError(t, r.handleCommand("help"))
		require.True(t, r.running)
		require.NoError(t, r.handleCommand("exit"))
		require.False(t, r.running)
	})

	require.Empty(t, stderr)
	require.Contains(t, stdout, "Available Statements")
	require.Contains(t, stdout, "Bye.")
	require.Empty(t, exec.executeQuery)
	require.Empty(t, exec.explainQuery)
}

func TestHandleCommandExplainDispatchesToExecutor(t *testing.T) {
	exec := &stubExecutor{explainResult: "plan body"}

	stdout, stderr := captureREPL(t, exec, func(r *REPL) {
		err := r.handleCommand("explain SEARCH docs SIMILAR TO 'vector database' LIMIT 5")
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.Equal(t, "SEARCH docs SIMILAR TO 'vector database' LIMIT 5", exec.explainQuery)
	require.Contains(t, stdout, "Query Plan")
	require.Contains(t, stdout, "plan body")
}

func TestHandleCommandExecutesQuery(t *testing.T) {
	exec := &stubExecutor{executeResult: "executed"}

	stdout, stderr := captureREPL(t, exec, func(r *REPL) {
		err := r.handleCommand("SHOW COLLECTIONS")
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.Equal(t, "SHOW COLLECTIONS", exec.executeQuery)
	require.Contains(t, stdout, "executed")
}

func TestReadLineSupportsMultilineStatements(t *testing.T) {
	input := "INSERT INTO COLLECTION docs VALUES {\n  \"text\": \"hello\"\n}\n"

	var line string
	stdout, stderr := captureREPL(t, &stubExecutor{}, func(r *REPL) {
		r.reader = bufio.NewReader(strings.NewReader(input))
		var err error
		line, err = r.readLine()
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.Contains(t, stdout, "  -> ")
	require.Equal(t, input, line)
}
