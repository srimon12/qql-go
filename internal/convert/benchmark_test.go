package convert

import (
	"encoding/json"
	"testing"
)

func BenchmarkConvertComplexFilter(b *testing.B) {
	input := `{"vector":[0.1],"limit":5,"filter":{"must":[{"key":"status","match":{"value":"active"}}],"should":[{"key":"priority","match":{"value":"high"}},{"key":"priority","match":{"value":"medium"}}],"must_not":[{"key":"archived","match":{"boolean":true}}]}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = JSONToQQL([]byte(input))
	}
}

func BenchmarkConvertFormulaGeoDecay(b *testing.B) {
	input := `{"prefetch":{"query":[0.2],"limit":50},"query":{"formula":{"sum":["$score",{"gauss_decay":{"x":{"geo_distance":{"origin":{"lat":52.5,"lon":13.3},"to":"geo.location"}},"scale":5000}}]}},"defaults":{"geo.location":{"lat":48.1,"lon":11.5}}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = JSONToQQL([]byte(input))
	}
}

func BenchmarkConvertLargeUpsert(b *testing.B) {
	// Create a large vector of 1536 floats representing a dense embedding
	vector := make([]float64, 1536)
	for i := range vector {
		vector[i] = float64(i) * 0.01
	}

	// Create payload
	req := struct {
		Points []struct {
			ID     int       `json:"id"`
			Vector []float64 `json:"vector"`
		} `json:"points"`
	}{
		Points: []struct {
			ID     int       `json:"id"`
			Vector []float64 `json:"vector"`
		}{
			{ID: 1, Vector: vector},
		},
	}

	importJson, _ := json.Marshal(req)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = JSONToQQL(importJson)
	}
}
