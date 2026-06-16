package qql

import "encoding/json"

// Result is the outcome of a single QQL query execution.
type Result struct {
	OK        bool   `json:"ok"`
	Operation string `json:"operation"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
}

// DataJSON returns the Data field as JSON bytes.
func (r *Result) DataJSON() []byte {
	if r.Data == nil {
		return nil
	}
	b, _ := json.Marshal(r.Data)
	return b
}

// ErrorResult creates a Result from an error.
func ErrorResult(err error) *Result {
	return &Result{
		OK:      false,
		Message: err.Error(),
	}
}
