package script

import (
	"fmt"
	"os"
	"strings"

	"github.com/srimon12/qql-go/internal/lexer"
)

type Executor interface {
	Execute(query string) (string, error)
}

func StripComments(text string) string {
	lines := make([]string, 0, len(strings.Split(text, "\n")))
	for _, line := range strings.Split(text, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func SplitStatements(text string) ([]string, error) {
	cleaned := StripComments(text)
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(cleaned)
	if err != nil {
		return nil, err
	}

	var starts []int
	depth := 0
	for _, tok := range tokens {
		if tok.Kind == lexer.TokenKindEof {
			break
		}
		if depth == 0 && isStatementStarter(tok.Kind) {
			starts = append(starts, tok.Pos)
		}
		switch tok.Kind {
		case lexer.TokenKindLbrace, lexer.TokenKindLbracket, lexer.TokenKindLparen:
			depth++
		case lexer.TokenKindRbrace, lexer.TokenKindRbracket, lexer.TokenKindRparen:
			if depth > 0 {
				depth--
			}
		}
	}

	if len(starts) == 0 {
		return nil, nil
	}

	statements := make([]string, 0, len(starts))
	for idx, start := range starts {
		end := len(cleaned)
		if idx+1 < len(starts) {
			end = starts[idx+1]
		}
		stmt := strings.TrimSpace(cleaned[start:end])
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}
	return statements, nil
}

func RunFile(path string, executor Executor, stopOnError bool) (int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read file: %w", err)
	}

	statements, err := SplitStatements(string(data))
	if err != nil {
		return 0, 0, fmt.Errorf("cannot parse script: %w", err)
	}

	okCount := 0
	failCount := 0
	for _, stmt := range statements {
		if _, err := executor.Execute(stmt); err != nil {
			failCount++
			if stopOnError {
				return okCount, failCount, nil
			}
			continue
		}
		okCount++
	}

	return okCount, failCount, nil
}

func isStatementStarter(kind lexer.TokenKind) bool {
	switch kind {
	case lexer.TokenKindInsert,
		lexer.TokenKindCreate,
		lexer.TokenKindDrop,
		lexer.TokenKindShow,
		lexer.TokenKindSearch,
		lexer.TokenKindRecommend,
		lexer.TokenKindDelete:
		return true
	default:
		return false
	}
}
