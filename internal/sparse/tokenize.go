package sparse

import (
	"unicode"
	"unicode/utf8"
)

var specialSingleCharTokens = map[byte]struct{}{
	'c': {},
}

// Tokenize splits text into lowercase tokens for BM25 sparse vector construction.
// Uses fast byte-level iteration for ASCII text, falls back to rune-level for Unicode.
func Tokenize(text string) []string {
	if isASCII(text) {
		return tokenizeASCII(text)
	}
	return tokenizeUnicode(text)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func tokenizeASCII(text string) []string {
	var tokens []string
	start := -1
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if isTokenByte(ch) {
			if start == -1 {
				start = i
			}
		} else {
			if start != -1 {
				tokens = appendTokens(tokens, toLowerASCII(text[start:i]))
				start = -1
			}
		}
	}
	if start != -1 {
		tokens = appendTokens(tokens, toLowerASCII(text[start:]))
	}
	return tokens
}

func tokenizeUnicode(text string) []string {
	text = unicodeToLower(text)
	var tokens []string
	start := -1
	for i, r := range text {
		if isTokenRune(r) {
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

func unicodeToLower(s string) string {
	buf := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] < 0x80 {
			c := s[i]
			if c >= 'A' && c <= 'Z' {
				c += 32
			}
			buf = append(buf, c)
			i++
		} else {
			r, size := utf8.DecodeRuneInString(s[i:])
			buf = utf8.AppendRune(buf, unicode.ToLower(r))
			i += size
		}
	}
	return string(buf)
}

func isTokenByte(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-'
}

func isTokenRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

// toLowerASCII converts ASCII bytes to lowercase without allocation when possible.
func toLowerASCII(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			buf := make([]byte, len(s))
			copy(buf, s[:i])
			for j := i; j < len(s); j++ {
				c := s[j]
				if c >= 'A' && c <= 'Z' {
					c += 32
				}
				buf[j] = c
			}
			return string(buf)
		}
	}
	return s
}

func maybeToken(s string) string {
	if len(s) >= 2 {
		return s
	}
	if len(s) == 1 {
		if _, ok := specialSingleCharTokens[s[0]]; ok {
			return s
		}
	}
	return ""
}

func appendTokens(tokens []string, raw string) []string {
	hasHyphen := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '-' {
			hasHyphen = true
			break
		}
	}

	if !hasHyphen {
		if tok := maybeToken(raw); tok != "" {
			tokens = append(tokens, tok)
		}
		return tokens
	}

	start := -1
	for i := 0; i < len(raw); i++ {
		if raw[i] == '-' {
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
