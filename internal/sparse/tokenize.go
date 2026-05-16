package sparse

import (
	"unicode"
)

var specialSingleCharTokens = map[rune]struct{}{
	'c': {},
}

func Tokenize(text string) []string {
	lower := []rune(text)
	for i, r := range lower {
		lower[i] = unicode.ToLower(r)
	}

	var tokens []string
	var current []rune

	for _, r := range lower {
		if isTokenChar(r) {
			current = append(current, r)
			continue
		}
		if len(current) > 0 {
			tokens = appendTokens(tokens, string(current))
			current = current[:0]
		}
	}
	if len(current) > 0 {
		tokens = appendTokens(tokens, string(current))
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
		_, ok := specialSingleCharTokens[[]rune(s)[0]]
		if ok {
			return s
		}
	}
	return ""
}

func appendTokens(tokens []string, raw string) []string {
	if tok := maybeToken(raw); tok != "" {
		tokens = append(tokens, tok)
	}
	if !containsHyphen(raw) {
		return tokens
	}
	for _, part := range splitHyphenatedToken(raw) {
		tokens = append(tokens, part)
	}
	return tokens
}

func containsHyphen(s string) bool {
	for _, r := range s {
		if r == '-' {
			return true
		}
	}
	return false
}

func splitHyphenatedToken(token string) []string {
	parts := []rune(token)
	var out []string
	var current []rune
	for _, r := range parts {
		if r == '-' {
			if len(current) > 0 {
				if tok := maybeToken(string(current)); tok != "" {
					out = append(out, tok)
				}
				current = current[:0]
			}
			continue
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		if tok := maybeToken(string(current)); tok != "" {
			out = append(out, tok)
		}
	}
	return out
}
