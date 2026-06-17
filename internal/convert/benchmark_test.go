package convert

import (
	"testing"
)

func BenchmarkConvertComplexFilter(b *testing.B) {
	input := `{"vector":[0.1],"limit":5,"filter":{"must":[{"key":"status","match":{"value":"active"}}],"should":[{"key":"priority","match":{"value":"high"}},{"key":"priority","match":{"value":"medium"}}],"must_not":[{"key":"archived","match":{"boolean":true}}]}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = JSONToQQL(input)
	}
}

func BenchmarkConvertFormulaGeoDecay(b *testing.B) {
	input := `{"prefetch":{"query":[0.2],"limit":50},"query":{"formula":{"sum":["$score",{"gauss_decay":{"x":{"geo_distance":{"origin":{"lat":52.5,"lon":13.3},"to":"geo.location"}},"scale":5000}}]}},"defaults":{"geo.location":{"lat":48.1,"lon":11.5}}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = JSONToQQL(input)
	}
}
