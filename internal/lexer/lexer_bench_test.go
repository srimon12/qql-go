package lexer

import "testing"

func BenchmarkTokenize(b *testing.B) {
	query := "SELECT * FROM collection WHERE id = 123 AND score > 0.5"
	l := &Lexer{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Tokenize(query)
	}
}

func BenchmarkLookupKeywordFast(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lookupKeywordFast("collection")
		lookupKeywordFast("SELECT")
		lookupKeywordFast("nonexistent")
	}
}
