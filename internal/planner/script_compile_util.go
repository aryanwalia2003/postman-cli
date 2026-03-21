package planner

import (
	"strings"

	"reqx/internal/collection"

	"github.com/dop251/goja"
)

func compileScripts(requests []collection.Request) (map[ScriptKey]*goja.Program, error) {
	compiled := make(map[ScriptKey]*goja.Program)

	for reqIdx, req := range requests {
		for _, script := range req.Scripts {
			if len(script.Exec) == 0 {
				continue
			}

			src := strings.Join(script.Exec, "\n")

			// goja.Compile: third arg "strict" = false preserves Postman-compatible
			// loose JS semantics (pm scripts commonly rely on implicit globals).
			prog, err := goja.Compile("", src, false)
			if err != nil {
				// Return a descriptive error so the user knows which request
				// and script type has the syntax problem before any VU starts.
				return nil, &scriptCompileError{
					RequestName: req.Name,
					ScriptType:  script.Type,
					Cause:       err,
				}
			}

			compiled[ScriptKey{RequestIndex: reqIdx, ScriptType: script.Type}] = prog
		}
	}

	return compiled, nil
}

// scriptCompileError wraps a goja compile error with request context.
type scriptCompileError struct {
	RequestName string
	ScriptType  string
	Cause       error
}

func (e *scriptCompileError) Error() string {
	return "script compile error in request '" + e.RequestName +
		"' (" + e.ScriptType + "): " + e.Cause.Error()
}

func (e *scriptCompileError) Unwrap() error { return e.Cause }