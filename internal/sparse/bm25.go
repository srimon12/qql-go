package sparse

import (
	"math"
	"slices"
	"sync/atomic"

	"github.com/srimon12/qql-go/internal/config"
)

// Vector is a sparse vector with sorted indices and parallel values.
type Vector struct {
	Indices []uint32
	Values  []float32
}

type bm25Params struct {
	k1    float64
	b     float64
	avgdl float64
}

var cachedBM25 atomic.Pointer[bm25Params]

func loadBM25Params() bm25Params {
	if p := cachedBM25.Load(); p != nil {
		return *p
	}
	cfg := config.GetConfig()
	p := bm25Params{k1: 1.2, b: 0.75, avgdl: 256.0}
	if cfg != nil {
		if cfg.BM25K1 != nil {
			p.k1 = *cfg.BM25K1
		}
		if cfg.BM25B != nil {
			p.b = *cfg.BM25B
		}
		if cfg.BM25AvgDL != nil {
			p.avgdl = *cfg.BM25AvgDL
		}
	}
	cachedBM25.Store(&p)
	return p
}

// InvalidateBM25Cache forces re-reading BM25 params from config on next call.
// Call this after saving config with new BM25 values.
func InvalidateBM25Cache() {
	cachedBM25.Store(nil)
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
	params := loadBM25Params()
	denomScale := params.k1 * (1.0 - params.b + params.b*docLen/params.avgdl)
	k1p1 := params.k1 + 1.0

	indices := make([]uint32, 0, len(counts))
	for idx := range counts {
		indices = append(indices, idx)
	}
	slices.Sort(indices)

	values := make([]float32, len(indices))
	for i, idx := range indices {
		tfCount := float64(counts[idx])
		denom := tfCount + denomScale
		values[i] = float32(tfCount * k1p1 / denom)
	}

	return Vector{Indices: indices, Values: values}
}
