package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeCollectionNameAllowsAlphanumeric(t *testing.T) {
	assert.Equal(t, "my-collection_42", sanitizeCollectionName("my-collection_42"))
}

func TestSanitizeCollectionNameStripsUnsafeChars(t *testing.T) {
	assert.Equal(t, "abcDROP", sanitizeCollectionName("abc; DROP;"))
}

func TestSanitizeCollectionNameAllowsMixedCase(t *testing.T) {
	assert.Equal(t, "MyCollection42", sanitizeCollectionName("MyCollection42"))
}

func TestSanitizeCollectionNameReturnsUnknownForEmpty(t *testing.T) {
	assert.Equal(t, "unknown", sanitizeCollectionName(""))
}

func TestSanitizeCollectionNameReturnsUnknownForAllUnsafe(t *testing.T) {
	assert.Equal(t, "unknown", sanitizeCollectionName(";;;!!!..."))
}

func TestSanitizeCollectionNameStripsSpaces(t *testing.T) {
	assert.Equal(t, "mycollection", sanitizeCollectionName("my collection"))
}

func TestSanitizeCollectionNamePreservesHyphens(t *testing.T) {
	assert.Equal(t, "my-collection", sanitizeCollectionName("my-collection"))
}

func TestSanitizeCollectionNamePreservesUnderscores(t *testing.T) {
	assert.Equal(t, "my_collection", sanitizeCollectionName("my_collection"))
}
