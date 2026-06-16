package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewQQLSyntaxError(t *testing.T) {
	err := NewQQLSyntaxError("unexpected token", 42)
	assert.Equal(t, "unexpected token (at position 42)", err.Error())
	assert.Equal(t, 42, err.Pos)
}

func TestNewQQLSyntaxErrorNegativePos(t *testing.T) {
	err := NewQQLSyntaxError("unexpected token", -1)
	assert.Equal(t, "unexpected token", err.Error())
}

func TestNewQQLRuntimeError(t *testing.T) {
	err := NewQQLRuntimeError("connection refused")
	assert.Equal(t, "connection refused", err.Error())
}
