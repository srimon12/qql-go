package sparse

import (
	"math"
	"slices"
)

// BM25 parameters matching Qdrant FastEmbed defaults.
const (
	bm25K1    = 1.2
	bm25B     = 0.75
	bm25AvgDL = 256.0
)

// Vector is a sparse vector with sorted indices and parallel values.
type Vector struct {
	Indices []uint32
	Values  []float32
}

// BuildQuery creates a sparse query vector using log-TF weighting.
// Qdrant's sparse IDF modifier handles the collection-wide IDF term.
func BuildQuery(text string) Vector {
	tokens := Tokenize(text)
	if len(tokens) == 0 {
		return Vector{}
	}

	counts := make(map[uint32]float32, len(tokens))
	for _, token := range tokens {
		counts[hashToken(token)]++
	}

	indices := make([]uint32, 0, len(counts))
	for idx := range counts {
		indices = append(indices, idx)
	}
	slices.Sort(indices)

	values := make([]float32, len(indices))
	for i, idx := range indices {
		values[i] = float32(1.0 + math.Log(float64(counts[idx])))
	}

	return Vector{Indices: indices, Values: values}
}

// BuildDocument creates a sparse document vector using BM25-saturated TF weights.
// Qdrant's sparse IDF modifier supplies the collection-wide rarity signal.
// Uses k1=1.2, b=0.75, avgdl=256 matching Qdrant FastEmbed defaults.
func BuildDocument(text string) Vector {
	tokens := Tokenize(text)
	if len(tokens) == 0 {
		return Vector{}
	}

	counts := make(map[uint32]float32, len(tokens))
	for _, token := range tokens {
		counts[hashToken(token)]++
	}

	docLen := float64(len(tokens))
	indices := make([]uint32, 0, len(counts))
	for idx := range counts {
		indices = append(indices, idx)
	}
	slices.Sort(indices)

	values := make([]float32, len(indices))
	for i, idx := range indices {
		values[i] = bm25TF(float64(counts[idx]), docLen)
	}

	return Vector{Indices: indices, Values: values}
}

// bm25TF computes the BM25-saturated term frequency component for a document.
// Formula: tf * (k1 + 1) / (tf + k1 * (1 - b + b * docLen / avgdl))
// This is the per-term value stored in the sparse vector when Qdrant
// applies IDF on the server side.
func bm25TF(tfCount, docLen float64) float32 {
	denom := tfCount + bm25K1*(1.0-bm25B+bm25B*docLen/bm25AvgDL)
	return float32(tfCount * (bm25K1 + 1) / denom)
}
