package script

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubExecutor struct {
	queries []string
	failFor map[string]error
}

func (s *stubExecutor) Execute(query string) (string, error) {
	s.queries = append(s.queries, query)
	if err := s.failFor[query]; err != nil {
		return "", err
	}
	return "ok", nil
}

func TestStripComments(t *testing.T) {
	input := "SHOW COLLECTIONS -- comment\nINSERT INTO COLLECTION docs VALUES {'text': 'hello'}"
	got := StripComments(input)
	require.NotContains(t, got, "comment")
	require.Contains(t, got, "SHOW COLLECTIONS")
}

func TestSplitStatements(t *testing.T) {
	input := "SHOW COLLECTIONS\nINSERT INTO COLLECTION docs VALUES {'text': 'hello'}\nRECOMMEND FROM docs POSITIVE IDS ('1') LIMIT 5"
	statements, err := SplitStatements(input)
	require.NoError(t, err)
	require.Len(t, statements, 3)
	require.Equal(t, "SHOW COLLECTIONS", statements[0])
	require.Contains(t, statements[1], "INSERT INTO COLLECTION docs")
	require.Contains(t, statements[2], "RECOMMEND FROM docs")
}

func TestSplitStatementsSelectAndScroll(t *testing.T) {
	input := "SHOW COLLECTION docs\nSELECT * FROM docs WHERE id = 'pt-1'\nSCROLL FROM docs WHERE status = 'active' AFTER 'pt-1' LIMIT 10"
	statements, err := SplitStatements(input)
	require.NoError(t, err)
	require.Len(t, statements, 3)
	require.Equal(t, "SHOW COLLECTION docs", statements[0])
	require.Equal(t, "SELECT * FROM docs WHERE id = 'pt-1'", statements[1])
	require.Equal(t, "SCROLL FROM docs WHERE status = 'active' AFTER 'pt-1' LIMIT 10", statements[2])
}

func TestRunFileWithSelectAndScroll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.qql")
	content := "SHOW COLLECTION docs\nSELECT * FROM docs WHERE id = 'pt-1'\nSCROLL FROM docs WHERE status = 'active' AFTER 'pt-1' LIMIT 10\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	exec := &stubExecutor{}
	okCount, failCount, err := RunFile(path, exec, false)
	require.NoError(t, err)
	require.Equal(t, 3, okCount)
	require.Zero(t, failCount)
	require.Equal(t, []string{
		"SHOW COLLECTION docs",
		"SELECT * FROM docs WHERE id = 'pt-1'",
		"SCROLL FROM docs WHERE status = 'active' AFTER 'pt-1' LIMIT 10",
	}, exec.queries)
}

func TestRunFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.qql")
	require.NoError(t, os.WriteFile(path, []byte("SHOW COLLECTIONS\nDROP COLLECTION docs\n"), 0o644))

	exec := &stubExecutor{}
	okCount, failCount, err := RunFile(path, exec, false)
	require.NoError(t, err)
	require.Equal(t, 2, okCount)
	require.Zero(t, failCount)
	require.Equal(t, []string{"SHOW COLLECTIONS", "DROP COLLECTION docs"}, exec.queries)
}

func TestRunFileStopOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.qql")
	require.NoError(t, os.WriteFile(path, []byte("SHOW COLLECTIONS\nDROP COLLECTION docs\nSHOW COLLECTIONS\n"), 0o644))

	exec := &stubExecutor{
		failFor: map[string]error{
			"DROP COLLECTION docs": fmt.Errorf("boom"),
		},
	}
	okCount, failCount, err := RunFile(path, exec, true)
	require.NoError(t, err)
	require.Equal(t, 1, okCount)
	require.Equal(t, 1, failCount)
	require.Equal(t, []string{"SHOW COLLECTIONS", "DROP COLLECTION docs"}, exec.queries)
}
