package errors

import "fmt"

type QQLSyntaxError struct {
	Message string
	Pos     int
}

func (e *QQLSyntaxError) Error() string {
	if e.Pos >= 0 {
		return fmt.Sprintf("%s (at position %d)", e.Message, e.Pos)
	}
	return e.Message
}

func NewQQLSyntaxError(message string, pos int) *QQLSyntaxError {
	return &QQLSyntaxError{Message: message, Pos: pos}
}

type QQLRuntimeError struct {
	Message string
}

func (e *QQLRuntimeError) Error() string {
	return e.Message
}

func NewQQLRuntimeError(message string) *QQLRuntimeError {
	return &QQLRuntimeError{Message: message}
}
