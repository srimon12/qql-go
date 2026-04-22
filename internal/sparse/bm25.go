package sparse

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// Vector is a sparse vector with sorted indices and parallel values.
type Vector struct {
	Indices []uint32
	Values  []float32
}

// CorpusStats tracks corpus-level statistics for true BM25 weighting.
type CorpusStats struct {
	N     int            `json:"n"`     // total number of documents
	AvgDL float64        `json:"avgdl"` // average document length in tokens
	DF    map[string]int `json:"df"`    // document frequency per token
}

// NewCorpusStats creates empty stats.
func NewCorpusStats() *CorpusStats {
	return &CorpusStats{
		DF: make(map[string]int),
	}
}

// Update incorporates a new document's tokens into corpus statistics.
func (cs *CorpusStats) Update(tokens []string) {
	seen := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		seen[t] = struct{}{}
	}

	totalTokens := float64(len(tokens))
	oldN := float64(cs.N)
	cs.AvgDL = (cs.AvgDL*oldN + totalTokens) / (oldN + 1)
	cs.N++

	for t := range seen {
		cs.DF[t]++
	}
}

// UpdateBatch incorporates multiple documents at once.
func (cs *CorpusStats) UpdateBatch(documents [][]string) {
	for _, tokens := range documents {
		cs.Update(tokens)
	}
}

// Save persists stats to a JSON file.
func (cs *CorpusStats) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create stats directory: %w", err)
	}
	data, err := json.MarshalIndent(cs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode corpus stats: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write corpus stats: %w", err)
	}
	return nil
}

// LoadCorpusStats reads stats from a JSON file.
func LoadCorpusStats(path string) (*CorpusStats, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewCorpusStats(), nil
		}
		return nil, fmt.Errorf("failed to read corpus stats: %w", err)
	}
	var cs CorpusStats
	if err := json.Unmarshal(data, &cs); err != nil {
		return nil, fmt.Errorf("failed to decode corpus stats: %w", err)
	}
	if cs.DF == nil {
		cs.DF = make(map[string]int)
	}
	return &cs, nil
}

func idf(df, n int) float64 {
	if df <= 0 || n <= 0 {
		return 0.0
	}
	ratio := (float64(n) - float64(df) + 0.5) / (float64(df) + 0.5)
	return math.Max(0.0, math.Log(1.0+ratio))
}

func bm25Weight(tf, df, n int, dl, avgdl float64) float64 {
	if tf <= 0 || df <= 0 || n <= 0 || avgdl <= 0.0 {
		return 0.0
	}
	idfVal := idf(df, n)
	numerator := float64(tf) * (bm25K1 + 1.0)
	denominator := float64(tf) + bm25K1*(1.0-bm25B+bm25B*(dl/avgdl))
	if denominator == 0.0 {
		return 0.0
	}
	return idfVal * (numerator / denominator)
}

// BuildQuery creates a sparse vector for query text using log-TF weighting.
// This matches the Rust query-side behavior.
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
		tf := float64(counts[idx])
		values[i] = float32(1.0 + math.Log(tf))
	}

	return Vector{Indices: indices, Values: values}
}

// BuildDocument creates a sparse vector for document text.
// If stats is provided and has enough documents (N > 0), it uses BM25 weighting.
// Otherwise it falls back to normalized TF (tf / doc_length).
func BuildDocument(text string, stats *CorpusStats) Vector {
	tokens := Tokenize(text)
	if len(tokens) == 0 {
		return Vector{}
	}

	// Count term frequencies
	tfMap := make(map[string]int, len(tokens))
	for _, token := range tokens {
		tfMap[token]++
	}

	dl := float64(len(tokens))
	useBM25 := stats != nil && stats.N > 0 && stats.AvgDL > 0.0

	type pair struct {
		idx   uint32
		value float32
	}
	pairs := make([]pair, 0, len(tfMap))

	for token, tf := range tfMap {
		idx := hashToken(token)
		var weight float64
		if useBM25 {
			df := stats.DF[token]
			if df == 0 {
				// Token not seen in corpus stats; treat as DF=1 for smoothing
				df = 1
			}
			weight = bm25Weight(tf, df, stats.N, dl, stats.AvgDL)
		} else {
			// Normalized TF fallback when no corpus stats available
			weight = float64(tf) / dl
		}
		pairs = append(pairs, pair{idx: idx, value: float32(weight)})
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].idx < pairs[j].idx })

	indices := make([]uint32, len(pairs))
	values := make([]float32, len(pairs))
	for i, p := range pairs {
		indices[i] = p.idx
		values[i] = p.value
	}

	return Vector{Indices: indices, Values: values}
}

// Build is kept for backward compatibility. It uses raw term frequencies
// (the old behavior) and is deprecated in favor of BuildQuery and BuildDocument.
func Build(text string) Vector {
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
		values[i] = counts[idx]
	}

	return Vector{Indices: indices, Values: values}
}
