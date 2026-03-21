package scripting

import (
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/fatih/color"

	"reqx/internal/collection"
	"reqx/internal/environment"
	"reqx/internal/errs"
)

// Execute runs a script on this VU's owned VM.
//
// If compiled is non-nil (normal load-test path via ExecutionPlan), the
// pre-compiled bytecode is run directly — goja.RunProgram skips parse+compile.
//
// If compiled is nil (single-request `req` command, which has no plan),
// the source is compiled on the fly from script.Exec via RunString. This
// fallback keeps the `req` command working without requiring a full planner.
//
// The VM is reused across calls — it persists for the VU's lifetime.
// Each call injects fresh pm/console bindings and clears them on exit,
// ensuring no state bleeds between iterations within the same VU.
func (g *GojaRunner) Execute(
	script   *collection.Script,
	env      *environment.Environment,
	resp     *ResponseAPI,
	compiled *goja.Program,
) error {
	if script == nil || len(script.Exec) == 0 {
		return nil
	}

	vm := g.vm

	// Inject per-call bindings.
	vm.Set("console", &ConsoleAPI{})

	testResults := make(TestResults, 0)
	pmObj := &PmAPI{
		Environment: &EnvironmentAPI{env: env},
		Response:    resp,
		TestResults: &testResults,
	}
	vm.Set("pm", pmObj)

	// Run — use pre-compiled program when available, fall back to RunString.
	var runErr error
	if compiled != nil {
		_, runErr = vm.RunProgram(compiled)
	} else {
		src := strings.Join(script.Exec, "\n")
		_, runErr = vm.RunString(src)
	}

	// Always clear injected bindings before returning, even on error.
	// This prevents pm/console from one iteration leaking into the next.
	vm.Set("pm", goja.Undefined())
	vm.Set("console", goja.Undefined())

	if runErr != nil {
		return errs.Wrap(runErr, errs.KindInternal, "script execution failed")
	}

	// Print test results.
	for _, res := range testResults {
		if res.Passed {
			fmt.Println(color.GreenString("✅ PASS: " + res.Name))
		} else {
			fmt.Println(color.RedString("❌ FAIL: " + res.Name + " | " + res.Error))
		}
	}

	return nil
}