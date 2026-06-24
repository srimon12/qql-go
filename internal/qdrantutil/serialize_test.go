package qdrantutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSerializeKeywordBoolFieldsAllNil(t *testing.T) {
	m := SerializeKeywordBoolFields(nil, nil, nil)
	assert.Empty(t, m)
}

func TestSerializeKeywordBoolFieldsIsTenantOnly(t *testing.T) {
	m := SerializeKeywordBoolFields(ptr(true), nil, nil)
	assert.Equal(t, map[string]any{"is_tenant": true}, m)
}

func TestSerializeKeywordBoolFieldsOnDiskOnly(t *testing.T) {
	m := SerializeKeywordBoolFields(nil, ptr(true), nil)
	assert.Equal(t, map[string]any{"on_disk": true}, m)
}

func TestSerializeKeywordBoolFieldsEnableHnswOnly(t *testing.T) {
	m := SerializeKeywordBoolFields(nil, nil, ptr(false))
	assert.Equal(t, map[string]any{"enable_hnsw": false}, m)
}

func TestSerializeKeywordBoolFieldsAllSet(t *testing.T) {
	m := SerializeKeywordBoolFields(ptr(true), ptr(false), ptr(true))
	assert.Equal(t, map[string]any{
		"is_tenant":   true,
		"on_disk":     false,
		"enable_hnsw": true,
	}, m)
}

func ptr[T any](v T) *T {
	return &v
}
