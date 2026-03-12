package errs

import (
	"fmt"
)

// Error satisfies the standard error interface.
// It formats the error message and appends the underlying cause if one exists.
func (e *appError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

// Kind returns the category of the error.
func (e *appError) Kind() Kind {
	return e.kind
}

// Message returns a client-safe response message.
func (e *appError) Message() string {
	return e.message
}

// Metadata returns structured context attached to the error.
func (e *appError) Metadata() Metadata {
	return e.metadata
}

// StackTrace returns the runtime execution stack when the error was captured.
func (e *appError) StackTrace() string {
	return e.stackTrace
}

// Unwrap supports Go 1.13+ error unwrapping chains (e.g. errors.Is, errors.As).
func (e *appError) Unwrap() error {
	return e.cause
}
