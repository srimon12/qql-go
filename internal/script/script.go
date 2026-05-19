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
	out := make([]byte, 0, len(text))
	inString := false
	var quoteChar byte
	i := 0
	n := len(text)

	for i < n {
		ch := text[i]

		if inString {
			out = append(out, ch)
			if ch == '\\' && i+1 < n {
				out = append(out, text[i+1])
				i += 2
				continue
			}
			if ch == quoteChar {
				inString = false
				quoteChar = 0
			}
			i++
			continue
		}

		if ch == '\'' || ch == '"' {
			inString = true
			quoteChar = ch
			out = append(out, ch)
			i++
			continue
		}

		if ch == '-' && i+1 < n && text[i+1] == '-' {
			i += 2
			for i < n && text[i] != '\r' && text[i] != '\n' {
				i++
			}
			continue
		}

		out = append(out, ch)
		i++
	}

	return string(out)
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
		lexer.TokenKindAlter,
		lexer.TokenKindDrop,
		lexer.TokenKindShow,
		lexer.TokenKindSearch,
		lexer.TokenKindSelect,
		lexer.TokenKindScroll,
		lexer.TokenKindRecommend,
		lexer.TokenKindDelete,
		lexer.TokenKindUpdate:
		return true
	default:
		return false
	}
}
