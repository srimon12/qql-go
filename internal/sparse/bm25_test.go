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
	require.Equal(t, []string{"c", "go", "rs"}, got)
}

func TestTokenizeHandlesHyphenatedMedicalTerms(t *testing.T) {
	t.Parallel()

	got := Tokenize("B-cell anti-NMDA CD19-negative")
	require.Equal(t, []string{"b-cell", "cell", "anti-nmda", "anti", "nmda", "cd19-negative", "cd19", "negative"}, got)
}

func TestTokenizeHandlesUnicode(t *testing.T) {
	t.Parallel()

	got := Tokenize("Привет мир hello-world")
	require.Equal(t, []string{"привет", "мир", "hello-world", "hello", "world"}, got)
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

	a := hashToken("ab")
	b := hashToken("abc")
	require.NotEqual(t, a, b)
}

func TestBuildQueryUsesLogTF(t *testing.T) {
	t.Parallel()

	v := BuildQuery("hello hello world")
	require.Len(t, v.Indices, 2)
	require.Len(t, v.Values, 2)

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

	require.InDelta(t, float32(1.0+math.Log(2.0)), helloValue, 0.0001)
	require.InDelta(t, float32(1.0), worldValue, 0.0001)
}

func TestBuildDocumentUsesBM25SaturatedTF(t *testing.T) {
	t.Parallel()

	v := BuildDocument("hello hello world")
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

	// BM25 saturation: tf * (k1+1) / (tf + k1 * (1 - b + b * docLen / avgdl))
	// docLen=3, k1=1.2, b=0.75, avgdl=256
	// hello (tf=2): 2*2.2 / (2 + 1.2*(0.25 + 0.75*3/256)) = 4.4 / (2 + 1.2*0.2588) = 4.4/2.3105 ≈ 1.9043
	// world (tf=1): 1*2.2 / (1 + 1.2*0.2588) = 2.2/1.3105 ≈ 1.6787
	expectedHello := float32(4.4 / (2.0 + 1.2*(0.25+0.75*3.0/256.0)))
	expectedWorld := float32(2.2 / (1.0 + 1.2*(0.25+0.75*3.0/256.0)))

	require.InDelta(t, expectedHello, helloValue, 0.0001)
	require.InDelta(t, expectedWorld, worldValue, 0.0001)
}

func TestBuildReturnsEmptyForEmptyText(t *testing.T) {
	t.Parallel()

	require.Empty(t, BuildDocument("").Indices)
	require.Empty(t, BuildQuery("").Indices)
}
