package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatIDEscapesSingleQuotes(t *testing.T) {
	id := "it's"
	result := formatID(id)
	assert.Equal(t, "'it\\'s'", result)
}

func TestFormatIDNoEscapeForCleanString(t *testing.T) {
	id := "uuid-123"
	result := formatID(id)
	assert.Equal(t, "'uuid-123'", result)
}

func TestFormatIDHandlesFloatAsInt(t *testing.T) {
	result := formatID(float64(42))
	assert.Equal(t, "42", result)
}

func TestFormatIDHandlesFloat(t *testing.T) {
	result := formatID(float64(3.14))
	assert.Equal(t, "3.14", result)
}

func TestFormatValueEscapesSingleQuotes(t *testing.T) {
	result := formatValue("it's a test")
	assert.Equal(t, "'it\\'s a test'", result)
}

func TestFormatValueHandlesFloatAsInt(t *testing.T) {
	result := formatValue(float64(100))
	assert.Equal(t, "100", result)
}

func TestFormatValueHandlesFloat(t *testing.T) {
	result := formatValue(float64(2.5))
	assert.Equal(t, "2.5", result)
}

func TestFormatValueHandlesBool(t *testing.T) {
	assert.Equal(t, "true", formatValue(true))
	assert.Equal(t, "false", formatValue(false))
}

func TestFormatValueHandlesNil(t *testing.T) {
	assert.Equal(t, "null", formatValue(nil))
}

func TestFormatValueHandlesMap(t *testing.T) {
	result := formatValue(map[string]any{"key": "val"})
	require.Contains(t, result, "'key': 'val'")
}

func TestBuildValuesDictEmpty(t *testing.T) {
	assert.Equal(t, "{}", buildValuesDict(nil))
	assert.Equal(t, "{}", buildValuesDict(map[string]any{}))
}

func TestBuildValuesDictSingleEntry(t *testing.T) {
	result := buildValuesDict(map[string]any{"name": "hello"})
	assert.Contains(t, result, "'name': 'hello'")
}

func TestBuildValuesDictMultipleEntries(t *testing.T) {
	result := buildValuesDict(map[string]any{
		"name": "world",
		"id":   1,
	})
	assert.Contains(t, result, "'name': 'world'")
	assert.Contains(t, result, "'id': 1")
}
