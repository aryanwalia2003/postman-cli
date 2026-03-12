package errs

// Kind represents the high-level category of an error.
type Kind string

const (
	KindInternal     Kind = "internal_error"
	KindInvalidInput Kind = "invalid_input"
	KindNotFound     Kind = "not_found"
	KindForbidden    Kind = "forbidden"
	KindUnauthorized Kind = "unauthorized"
	KindConflict     Kind = "conflict"
	KindDatabase     Kind = "database_error"
	KindExternal     Kind = "external_service_error"
)
