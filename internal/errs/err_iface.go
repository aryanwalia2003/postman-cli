package errs

// AppError defines the core capabilities that every internal error must support.
// This allows external packages to inspect errors without coupling to concrete structs.
type AppError interface {
	error
	Kind() Kind              // Categorized error type
	Message() string         // Safe message for external clients
	Metadata() Metadata      // Rich contextual key-value pairs
	StackTrace() string      // The captured stack trace for debugging
	Unwrap() error           // The underlying root cause, if any
}
