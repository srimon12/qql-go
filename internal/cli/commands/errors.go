package commands

import "errors"

type ExitError struct {
	err     error
	code    int
	printed bool
}

func (e *ExitError) Error() string {
	return e.err.Error()
}

func (e *ExitError) Unwrap() error {
	return e.err
}

func (e *ExitError) ExitCode() int {
	return e.code
}

func (e *ExitError) Printed() bool {
	return e.printed
}

func NewExitError(err error, code int, printed bool) error {
	if err == nil {
		return nil
	}
	return &ExitError{
		err:     err,
		code:    code,
		printed: printed,
	}
}

func ExitCode(err error) int {
	var exitErr *ExitError
	if errors.As(err, &exitErr) && exitErr.code > 0 {
		return exitErr.code
	}
	if err != nil {
		return 1
	}
	return 0
}

func ErrorPrinted(err error) bool {
	var exitErr *ExitError
	return errors.As(err, &exitErr) && exitErr.printed
}
