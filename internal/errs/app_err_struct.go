package errs

// appError is the unexported concrete implementation of the AppError interface.
type appError struct {
	kind       Kind
	message    string
	cause      error
	metadata   Metadata
	stackTrace string
}
