package sparse

import (
	"strings"
	"unicode"
)

var specialSingleCharTokens = map[rune]struct{}{
	'c': {},
}

func Tokenize(text string) []string {
	// First convert to lowercase efficiently
	text = strings.ToLower(text)

	var tokens []string

	// Split by non-token characters
	start := -1
	for i, r := range text {
		if isTokenChar(r) {
			if start == -1 {
				start = i
			}
		} else {
			if start != -1 {
				tokens = appendTokens(tokens, text[start:i])
				start = -1
			}
		}
	}
	if start != -1 {
		tokens = appendTokens(tokens, text[start:])
	}

	return tokens
}

func isTokenChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

func maybeToken(s string) string {
	if len(s) >= 2 {
		return s
	}
	if len(s) == 1 {
		if _, ok := specialSingleCharTokens[rune(s[0])]; ok {
			return s
		}
	}
	return ""
}

func appendTokens(tokens []string, raw string) []string {
	if tok := maybeToken(raw); tok != "" {
		tokens = append(tokens, tok)
	}

	// Check for hyphen fast path
	hasHyphen := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '-' {
			hasHyphen = true
			break
		}
	}

	if !hasHyphen {
		return tokens
	}

	// Split by hyphen
	start := -1
	for i, r := range raw {
		if r == '-' {
			if start != -1 {
				if tok := maybeToken(raw[start:i]); tok != "" {
					tokens = append(tokens, tok)
				}
				start = -1
			}
		} else {
			if start == -1 {
				start = i
			}
		}
	}
	if start != -1 {
		if tok := maybeToken(raw[start:]); tok != "" {
			tokens = append(tokens, tok)
		}
	}

	return tokens
}
