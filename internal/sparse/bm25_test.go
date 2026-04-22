package sparse

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTokenizeLowercasesAndFilters(t *testing.T) {
	t.Parallel()

	got := Tokenize("Hello, World! 123 TEST_token")
	require.Equal(t, []string{"hello", "world", "123", "test_token"}, got)
}

func TestTokenizeFiltersSingleCharsExceptSpecial(t *testing.T) {
	t.Parallel()

	got := Tokenize("a b c d go rs")
	// "c" is the only single-char special token; "go" and "rs" are length >= 2
	require.Equal(t, []string{"c", "go", "rs"}, got)
}

func TestTokenizeHandlesUnicode(t *testing.T) {
	t.Parallel()

	got := Tokenize("Привет мир hello-world")
	require.Equal(t, []string{"привет", "мир", "hello", "world"}, got)
}

func TestTokenizeHandlesUnderscore(t *testing.T) {
	t.Parallel()

	got := Tokenize("test_fn main_loop")
	require.Equal(t, []string{"test_fn", "main_loop"}, got)
}

func TestHashTokenDeterministic(t *testing.T) {
	t.Parallel()

	a := hashToken("hello")
	b := hashToken("hello")
	require.Equal(t, a, b)
}

func TestHashTokenDifferentForDifferentInputs(t *testing.T) {
	t.Parallel()

	a := hashToken("hello")
	b := hashToken("world")
	require.NotEqual(t, a, b)
}

func TestHashTokenLengthPrefixAvoidsCollision(t *testing.T) {
	t.Parallel()

	// Without length prefix, "ab" and "abc" might collide in some hash schemes.
	// FNV-1a with length prefix should distinguish them.
	a := hashToken("ab")
	b := hashToken("abc")
	require.NotEqual(t, a, b)
}

func TestBuildQueryUsesLogTF(t *testing.T) {
	t.Parallel()

	v := BuildQuery("hello hello world")
	require.Len(t, v.Indices, 2)
	require.Len(t, v.Values, 2)

	// Find which index corresponds to "hello" (appears twice)
	helloIdx := hashToken("hello")
	worldIdx := hashToken("world")

	var helloValue, worldValue float32
	for i, idx := range v.Indices {
		if idx == helloIdx {
			helloValue = v.Values[i]
		}
		if idx == worldIdx {
			worldValue = v.Values[i]
		}
	}

	// hello appears twice -> log(2) + 1
	require.InDelta(t, float32(1.0+math.Log(2.0)), helloValue, 0.0001)
	// world appears once -> log(1) + 1 = 1
	require.InDelta(t, float32(1.0), worldValue, 0.0001)
}

func TestBuildDocumentWithNoStatsUsesNormalizedTF(t *testing.T) {
	t.Parallel()

	v := BuildDocument("hello hello world", nil)
	require.Len(t, v.Indices, 2)

	helloIdx := hashToken("hello")
	worldIdx := hashToken("world")

	var helloValue, worldValue float32
	for i, idx := range v.Indices {
		if idx == helloIdx {
			helloValue = v.Values[i]
		}
		if idx == worldIdx {
			worldValue = v.Values[i]
		}
	}

	// 3 tokens total. hello=2/3, world=1/3
	require.InDelta(t, float32(2.0/3.0), helloValue, 0.0001)
	require.InDelta(t, float32(1.0/3.0), worldValue, 0.0001)
}

func TestBuildDocumentWithStatsUsesBM25(t *testing.T) {
	t.Parallel()

	stats := NewCorpusStats()
	stats.Update([]string{"hello", "world", "foo"})
	stats.Update([]string{"hello", "bar"})
	stats.Update([]string{"baz"})

	v := BuildDocument("hello hello world", stats)
	require.Len(t, v.Indices, 2)

	// Values should be positive and hello should be weighted higher than world
	require.True(t, v.Values[0] > 0)
	require.True(t, v.Values[1] > 0)
}

func TestBuildDocumentReturnsEmptyForEmptyText(t *testing.T) {
	t.Parallel()

	require.Empty(t, BuildDocument("", nil).Indices)
	require.Empty(t, BuildQuery("").Indices)
}

func TestCorpusStatsSaveAndLoad(t *testing.T) {
	t.Parallel()

	stats := NewCorpusStats()
	stats.Update([]string{"hello", "world"})
	stats.Update([]string{"hello", "foo"})

	path := t.TempDir() + "/stats.json"
	require.NoError(t, stats.Save(path))

	loaded, err := LoadCorpusStats(path)
	require.NoError(t, err)
	require.Equal(t, stats.N, loaded.N)
	require.Equal(t, stats.AvgDL, loaded.AvgDL)
	require.Equal(t, stats.DF, loaded.DF)
}

func TestLoadCorpusStatsMissingFileReturnsEmpty(t *testing.T) {
	t.Parallel()

	loaded, err := LoadCorpusStats(t.TempDir() + "/nonexistent.json")
	require.NoError(t, err)
	require.Equal(t, 0, loaded.N)
	require.NotNil(t, loaded.DF)
}

func TestBM25WeightHandlesEdgeCases(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0.0, bm25Weight(0, 1, 10, 5.0, 5.0))
	require.Equal(t, 0.0, bm25Weight(1, 0, 10, 5.0, 5.0))
	require.Equal(t, 0.0, bm25Weight(1, 1, 0, 5.0, 5.0))
	require.Equal(t, 0.0, bm25Weight(1, 1, 10, 5.0, 0.0))
}

func TestCorpusStatsUpdateBatch(t *testing.T) {
	t.Parallel()

	stats := NewCorpusStats()
	stats.UpdateBatch([][]string{
		{"hello", "world"},
		{"hello", "foo"},
	})
	require.Equal(t, 2, stats.N)
	require.Equal(t, 2, stats.DF["hello"])
	require.Equal(t, 1, stats.DF["world"])
	require.Equal(t, 1, stats.DF["foo"])
}

func TestBuildBackwardCompatibility(t *testing.T) {
	t.Parallel()

	v := Build("hello hello world")
	require.Len(t, v.Indices, 2)
	require.Len(t, v.Values, 2)

	// Should be sorted
	require.Less(t, v.Indices[0], v.Indices[1])
}
