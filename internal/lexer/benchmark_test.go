package lexer

import "testing"

func BenchmarkLookupKeywordFast(b *testing.B) {
	s := "select"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = lookupKeywordFast(s)
	}
}

func BenchmarkLookupKeywordFastLong(b *testing.B) {
	s := "RECOMMEND_SOMETHING" // length > 16
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = lookupKeywordFast(s)
	}
}
