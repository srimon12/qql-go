package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/srimon12/qql-go/internal/config"
	"github.com/stretchr/testify/require"
)

type stubExecutor struct {
	executeQuery  string
	executeResult string
	executeErr    error

	explainQuery  string
	explainResult string
	explainErr    error

	executeFilePath   string
	executeFileResult string
	executeFileErr    error

	dumpCollection string
	dumpPath       string
	dumpResult     string
	dumpErr        error
}

func (s *stubExecutor) Execute(query string) (string, error) {
	s.executeQuery = query
	return s.executeResult, s.executeErr
}

func (s *stubExecutor) Explain(query string) (string, error) {
	s.explainQuery = query
	return s.explainResult, s.explainErr
}

func (s *stubExecutor) ExecuteFile(path string, _ bool) (string, error) {
	s.executeFilePath = path
	return s.executeFileResult, s.executeFileErr
}

func (s *stubExecutor) DumpCollection(collection, outputPath string, batchSize int) (string, error) {
	s.dumpCollection = collection
	s.dumpPath = outputPath
	return s.dumpResult, s.dumpErr
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

	stdoutCh := make(chan string, 1)
	stderrCh := make(chan string, 1)

	go func() {
		data, _ := io.ReadAll(stdoutR)
		stdoutCh <- string(data)
	}()
	go func() {
		data, _ := io.ReadAll(stderrR)
		stderrCh <- string(data)
	}()

	repl := NewREPL(&config.Config{URL: "http://localhost:6333"}, exec)
	fn(repl)

	require.NoError(t, stdoutW.Close())
	require.NoError(t, stderrW.Close())

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return <-stdoutCh, <-stderrCh
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
	require.Contains(t, stdout, "SELECT")
	require.Contains(t, stdout, "SCROLL FROM")
	require.Contains(t, stdout, "USING SPARSE")
	require.Contains(t, stdout, "FUSION")
	require.Contains(t, stdout, "QUANTIZE TURBO")
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

func TestHandleCommandExplainDispatchesCaseInsensitively(t *testing.T) {
	exec := &stubExecutor{explainResult: "plan body"}

	_, stderr := captureREPL(t, exec, func(r *REPL) {
		err := r.handleCommand("ExPlAiN SEARCH docs SIMILAR TO 'vector database' LIMIT 5")
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.Equal(t, "SEARCH docs SIMILAR TO 'vector database' LIMIT 5", exec.explainQuery)
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

func TestHandleCommandExecuteFile(t *testing.T) {
	exec := &stubExecutor{executeFileResult: "script done"}

	stdout, stderr := captureREPL(t, exec, func(r *REPL) {
		err := r.handleCommand("execute demo.qql")
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.Equal(t, "demo.qql", exec.executeFilePath)
	require.Contains(t, stdout, "script done")
}

func TestHandleCommandDumpCollection(t *testing.T) {
	exec := &stubExecutor{dumpResult: "dump done"}

	stdout, stderr := captureREPL(t, exec, func(r *REPL) {
		err := r.handleCommand("dump docs dump.qql")
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.Equal(t, "docs", exec.dumpCollection)
	require.Equal(t, "dump.qql", exec.dumpPath)
	require.Contains(t, stdout, "dump done")
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

func TestReadLineIgnoresStrayClosingDelimiters(t *testing.T) {
	input := ")\n"

	var line string
	stdout, stderr := captureREPL(t, &stubExecutor{}, func(r *REPL) {
		r.reader = bufio.NewReader(strings.NewReader(input))
		var err error
		line, err = r.readLine()
		require.NoError(t, err)
	})

	require.Empty(t, stderr)
	require.Empty(t, stdout)
	require.Equal(t, input, line)
}

func TestReadLineIgnoresDelimitersInsideQuotedStrings(t *testing.T) {
	input := "INSERT INTO COLLECTION docs VALUES {\n  \"text\": \"value with } and ] and )\"\n}\n"

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

func TestAddToHistoryDeduplicatesAndCapsSize(t *testing.T) {
	repl := NewREPL(&config.Config{URL: "http://localhost:6333"}, &stubExecutor{})

	for i := range 101 {
		repl.addToHistory(fmt.Sprintf("cmd-%d", i))
	}
	repl.addToHistory("cmd-50")

	require.Len(t, repl.history, 100)
	require.Equal(t, "cmd-50", repl.history[len(repl.history)-1])
	require.NotContains(t, repl.history[:len(repl.history)-1], "cmd-50")
	require.NotContains(t, repl.history, "cmd-0")
}

func TestCutCommandPrefix(t *testing.T) {
	query, ok := cutCommandPrefix("ExPlAiN   SHOW COLLECTIONS", "explain")
	require.True(t, ok)
	require.Equal(t, "SHOW COLLECTIONS", query)

	_, ok = cutCommandPrefix("explanation", "explain")
	require.False(t, ok)
}
