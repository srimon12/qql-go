package sparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenizeEmptyString(t *testing.T) {
	tokens := Tokenize("")
	assert.Empty(t, tokens)
}

func TestTokenizeWhitespace(t *testing.T) {
	tokens := Tokenize("   ")
	assert.Empty(t, tokens)
}
