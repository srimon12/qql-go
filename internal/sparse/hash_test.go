package sparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashTokenDifferentInputs(t *testing.T) {
	h1 := hashToken("hello")
	h2 := hashToken("world")
	assert.NotEqual(t, h1, h2)
}

func TestHashTokenEmpty(t *testing.T) {
	h := hashToken("")
	// FNV-32a with empty string — just verify it doesn't panic and is deterministic
	h2 := hashToken("")
	assert.Equal(t, h, h2)
}
