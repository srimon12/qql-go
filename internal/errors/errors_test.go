package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewQQLSyntaxError(t *testing.T) {
	err := NewQQLSyntaxError("unexpected token", 42)
	assert.Equal(t, "unexpected token (at position 42)", err.Error())
	assert.Equal(t, 42, err.Pos)
	assert.Nil(t, err.Unwrap())
}

func TestNewQQLSyntaxErrorNegativePos(t *testing.T) {
	err := NewQQLSyntaxError("unexpected token", -1)
	assert.Equal(t, "unexpected token", err.Error())
}

func TestWrapQQLSyntaxError(t *testing.T) {
	cause := fmt.Errorf("underlying")
	err := WrapQQLSyntaxError("bad input", 10, cause)
	assert.Equal(t, "bad input (at position 10)", err.Error())
	assert.Equal(t, cause, err.Unwrap())
	assert.True(t, errors.Is(err, cause))
}

func TestNewQQLRuntimeError(t *testing.T) {
	err := NewQQLRuntimeError("connection refused")
	assert.Equal(t, "connection refused", err.Error())
	assert.Nil(t, err.Unwrap())
}

func TestWrapQQLRuntimeError(t *testing.T) {
	cause := fmt.Errorf("dial tcp: connection refused")
	err := WrapQQLRuntimeError("qdrant unavailable", cause)
	assert.Equal(t, "qdrant unavailable", err.Error())
	assert.Equal(t, cause, err.Unwrap())
	assert.True(t, errors.Is(err, cause))
}

func TestErrorChainWithFmt(t *testing.T) {
	inner := NewQQLSyntaxError("bad token", 5)
	wrapped := fmt.Errorf("parse failed: %w", inner)

	var synErr *QQLSyntaxError
	assert.True(t, errors.As(wrapped, &synErr))
	assert.Equal(t, 5, synErr.Pos)
}
