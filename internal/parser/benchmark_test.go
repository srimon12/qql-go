package parser

import (
	"testing"

	"github.com/srimon12/qql-go/internal/lexer"
)

var lexer_ = &lexer.Lexer{}

func parseBench(input string, b *testing.B) {
	b.Helper()
	for i := 0; i < b.N; i++ {
		tokens, _ := lexer_.Tokenize(input)
		p := NewParser()
		p.Parse(tokens)
	}
}

func BenchmarkParse_Simple(b *testing.B) {
	parseBench("QUERY 'search' FROM docs LIMIT 10", b)
}

func BenchmarkParse_Hybrid(b *testing.B) {
	parseBench("QUERY 'search' FROM docs LIMIT 10 USING HYBRID", b)
}

func BenchmarkParse_Full(b *testing.B) {
	parseBench("QUERY 'vector search' FROM docs LIMIT 10 OFFSET 5 USING HYBRID RERANK WHERE topic = 'search' WITH (hnsw_ef = 128, exact = true)", b)
}

func BenchmarkParse_CTE_Prefetch(b *testing.B) {
	input := `WITH a AS (QUERY 'search' USING dense LIMIT 100 WHERE category = 'tech'), b AS (QUERY 'search' USING sparse LIMIT 100)
QUERY 'search' FROM docs LIMIT 10 PREFETCH (a WHERE priority = 'high' SCORE THRESHOLD 0.8, b SCORE THRESHOLD 0.5) FUSION RRF`
	parseBench(input, b)
}

func BenchmarkParse_OrderBy(b *testing.B) {
	parseBench("QUERY ORDER BY 'created_at' DESC FROM docs LIMIT 20 WHERE status = 'active'", b)
}

func BenchmarkParse_WithPayloadVectors(b *testing.B) {
	parseBench("QUERY 'search' FROM docs LIMIT 10 WITH PAYLOAD (include = ['title', 'body']) WITH VECTORS ('dense')", b)
}

func BenchmarkLex_Simple(b *testing.B) {
	input := "QUERY 'search' FROM docs LIMIT 10"
	for i := 0; i < b.N; i++ {
		lexer_.Tokenize(input)
	}
}

func BenchmarkLex_Full(b *testing.B) {
	input := "QUERY 'vector search' FROM docs LIMIT 10 OFFSET 5 USING HYBRID RERANK WHERE topic = 'search' WITH (hnsw_ef = 128, exact = true)"
	for i := 0; i < b.N; i++ {
		lexer_.Tokenize(input)
	}
}
