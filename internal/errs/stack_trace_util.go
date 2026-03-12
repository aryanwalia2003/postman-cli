package errs

import (
	"fmt"
	"runtime"
	"strings"
)

// captureStackTrace generates a formatted string of the current call stack.
func captureStackTrace(skip int) string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	
	if n == 0 {
		return "No stack trace available"
	}

	frames := runtime.CallersFrames(pcs[:n])
	var builder strings.Builder

	for {
		frame, more := frames.Next()
		// Avoid littering the trace with internal Go runtime panics
		if !strings.Contains(frame.File, "runtime/") {
			fmt.Fprintf(&builder, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		}
		if !more {
			break
		}
	}

	return builder.String()
}
