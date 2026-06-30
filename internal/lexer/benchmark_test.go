package lexer

import (
	"strings"
	"testing"
)

func BenchmarkTokenize(b *testing.B) {
	lex := &Lexer{}
	query := `SELECT id, _score, VECTOR, VECTOR[my_vec], PAYLOAD, PAYLOAD.city,
		PAYLOAD[location],
		{ "price": 100 }
		FROM my_collection
		WHERE id = 12345 OR id IN (1, 2, 3)
		  AND NOT city = "London"
		  AND tags CONTAINS ANY ("a", "b")
		  AND location GEO_RADIUS 10.0 20.0 100
		SEARCH VECTOR [0.1, 0.2, 0.3] LIMIT 10 OFFSET 5`

	query = strings.Repeat(query, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lex.Tokenize(query)
	}
}
