package errors

import "fmt"

type QQLSyntaxError struct {
	Message string
	Pos     int
	Err     error
}

func (e *QQLSyntaxError) Error() string {
	if e.Pos >= 0 {
		return fmt.Sprintf("%s (at position %d)", e.Message, e.Pos)
	}
	return e.Message
}

func (e *QQLSyntaxError) Unwrap() error {
	return e.Err
}

func NewQQLSyntaxError(message string, pos int) *QQLSyntaxError {
	return &QQLSyntaxError{Message: message, Pos: pos}
}

func WrapQQLSyntaxError(message string, pos int, err error) *QQLSyntaxError {
	return &QQLSyntaxError{Message: message, Pos: pos, Err: err}
}

type QQLRuntimeError struct {
	Message string
	Err     error
}

func (e *QQLRuntimeError) Error() string {
	return e.Message
}

func (e *QQLRuntimeError) Unwrap() error {
	return e.Err
}

func NewQQLRuntimeError(message string) *QQLRuntimeError {
	return &QQLRuntimeError{Message: message}
}

func WrapQQLRuntimeError(message string, err error) *QQLRuntimeError {
	return &QQLRuntimeError{Message: message, Err: err}
}
