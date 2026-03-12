package errs

import "errors"

// New creates a new Application Error with a stack trace.
func New(kind Kind, message string) error {
	return &appError{
		kind:       kind,
		message:    message,
		stackTrace: captureStackTrace(3),
	}
}

// Wrap embeds an existing error as the cause of a new Application Error.
// It retains the stack trace.
func Wrap(err error, kind Kind, message string) error {
	if err == nil {
		return nil
	}

	// If the error is already an appError, we might just want to 
	// wrap it but technically preserve its stack trace or accumulate it.
	// For simplicity in this architecture, we capture the new stack trace here.
	return &appError{
		kind:       kind,
		message:    message,
		cause:      err,
		stackTrace: captureStackTrace(3),
	}
}

// AddMetadata attaches structured data to an existing AppError.
func AddMetadata(err error, meta Metadata) error {
	var appErr *appError
	if errors.As(err, &appErr) {
		if appErr.metadata == nil {
			appErr.metadata = make(Metadata)
		}
		for k, v := range meta {
			appErr.metadata[k] = v
		}
		return appErr
	}
	// If it's not an AppError, we wrap it internally and add metadata
	return &appError{
		kind:       KindInternal,
		message:    err.Error(),
		cause:      err,
		metadata:   meta,
		stackTrace: captureStackTrace(3),
	}
}

// Domain-specific shorthand constructors

// NotFound is a syntactic shortcut for a Resource Not Found error.
func NotFound(message string) error {
	return New(KindNotFound, message)
}

// InvalidInput is a syntactic shortcut for Validation failures.
func InvalidInput(message string) error {
	return New(KindInvalidInput, message)
}

// Database wraps a sql/db error with the Database Kind.
func Database(err error, message string) error {
	return Wrap(err, KindDatabase, message)
}

// Internal creates a generic 500 server error.
func Internal(message string) error {
	return New(KindInternal, message)
}
