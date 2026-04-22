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
			if tok := maybeToken(string(current)); tok != "" {
				tokens = append(tokens, tok)
			}
			current = current[:0]
		}
	}
	if len(current) > 0 {
		if tok := maybeToken(string(current)); tok != "" {
			tokens = append(tokens, tok)
		}
	}

	return tokens
}

func isTokenChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
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
