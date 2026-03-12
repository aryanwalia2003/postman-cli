package errs

import (
	"fmt"
	"log"
	"net/http"
)

// RecoverHTTP captures any panics inside HTTP handlers, converts them into 
// internal AppErrors, and writes a safe response.
func RecoverHTTP(w http.ResponseWriter) {
	if r := recover(); r != nil {
		err, ok := r.(error)
		if !ok {
			err = fmt.Errorf("%v", r)
		}

		// Convert the panic into our robust error system specifically as an Internal Error
		// We use captureStackTrace(4) because runtime.Callers needs a higher skip 
		// count when recovering from a panic down the middleware chain.
		appErr := &appError{
			kind:       KindInternal,
			message:    "A critical system error occurred.",
			cause:      err,
			stackTrace: captureStackTrace(4),
		}

		log.Printf("[PANIC RECOVERED] Intercepted panic in HTTP handler.\n")
		// Safely translate what normally crashes the server into a controlled response.
		WriteHTTPError(w, appErr)
	}
}
