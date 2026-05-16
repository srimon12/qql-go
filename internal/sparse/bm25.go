package sparse

import (
	"math"
	"sort"
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
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })

	values := make([]float32, len(indices))
	for i, idx := range indices {
		values[i] = float32(1.0 + math.Log(float64(counts[idx])))
	}

	return Vector{Indices: indices, Values: values}
}

// BuildDocument creates a sparse document vector using normalized TF weights.
// Qdrant's sparse IDF modifier supplies the collection-wide rarity signal.
func BuildDocument(text string) Vector {
	tokens := Tokenize(text)
	if len(tokens) == 0 {
		return Vector{}
	}

	counts := make(map[uint32]float32, len(tokens))
	for _, token := range tokens {
		counts[hashToken(token)]++
	}

	docLength := float32(len(tokens))
	indices := make([]uint32, 0, len(counts))
	for idx := range counts {
		indices = append(indices, idx)
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })

	values := make([]float32, len(indices))
	for i, idx := range indices {
		values[i] = counts[idx] / docLength
	}

	return Vector{Indices: indices, Values: values}
}
