package lexer

import (
	"testing"
)

func BenchmarkLookupKeyword(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lookupKeyword("select")
		lookupKeyword("where")
		lookupKeyword("limit")
	}
}

func BenchmarkLookupKeywordAlloc(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lookupKeyword("select")
	}
}

func BenchmarkLookupKeywordFast(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lookupKeywordFast("select")
		lookupKeywordFast("where")
		lookupKeywordFast("limit")
	}
}
