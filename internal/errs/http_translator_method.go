package errs

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

// HTTPErrorResponse is the standard JSON structure returned to API clients.
type HTTPErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// WriteHTTPError translates an internal error into a consistent HTTP response.
func WriteHTTPError(w http.ResponseWriter, err error) {
	var appErr AppError
	var statusCode int
	var code string
	var message string

	// If the error conforms to our interface, we extract structured data.
	if errors.As(err, &appErr) {
		statusCode = httpStatusFromKind(appErr.Kind())
		code = string(appErr.Kind())
		message = appErr.Message()

		// Log internal details securely. Never exposed to the client.
		log.Printf("[ERR] %s | Cause: %v | Meta: %v\nStack: %s\n", 
			code, appErr.Unwrap(), appErr.Metadata(), appErr.StackTrace())
	} else {
		// Fallback for standard Go errors that haven't been wrapped.
		statusCode = http.StatusInternalServerError
		code = string(KindInternal)
		message = "An unexpected error occurred."
		log.Printf("[ERR] Unhandled Standard Error: %v\n", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := HTTPErrorResponse{}
	resp.Error.Code = code
	resp.Error.Message = message

	_ = json.NewEncoder(w).Encode(resp)
}

// httpStatusFromKind maps our domain error types to HTTP responses natively.
func httpStatusFromKind(k Kind) int {
	switch k {
	case KindInvalidInput:
		return http.StatusBadRequest // 400
	case KindNotFound:
		return http.StatusNotFound // 404
	case KindForbidden:
		return http.StatusForbidden // 403
	case KindUnauthorized:
		return http.StatusUnauthorized // 401
	case KindConflict:
		return http.StatusConflict // 409
	case KindDatabase, KindExternal, KindInternal:
		return http.StatusInternalServerError // 500
	default:
		return http.StatusInternalServerError
	}
}
