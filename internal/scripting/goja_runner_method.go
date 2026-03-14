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

// Execute runs a JavaScript snippet within a fresh VM, injecting the Environment and optional Response.
func (g *GojaRunner) Execute(script *collection.Script, env *environment.Environment, resp *ResponseAPI) error {
	if script == nil || len(script.Exec) == 0 {
		return nil
	}

	vm := goja.New()
	
	// Ensure JS camelCase/lowercase maps to Go PascalCase methods
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))


	// 1. Inject Console Interceptor
	vm.Set("console", &ConsoleAPI{})

	// 2. Prepare the pm.environment API
	envAPI := &EnvironmentAPI{env: env}

	// 3. Prepare the tracking slice for pm.test results
	testResults := make(TestResults, 0)

	// 4. Construct the root `pm` object
	pmObj := &PmAPI{
		Environment: envAPI,
		Response:    resp,
		TestResults: &testResults,
	}
	
	vm.Set("pm", pmObj)

	// Combine script lines into one block
	scriptSource := strings.Join(script.Exec, "\n")

	// Run the script
	_, err := vm.RunString(scriptSource)
	if err != nil {
		return errs.Wrap(err, errs.KindInternal, "script execution failed")
	}

	// Dump test results to the terminal for visibility
	for _, res := range testResults {
		if res.Passed {
			// Eventually use colors, but plain text for now
			fmt.Println(color.GreenString("✅ PASS: " + res.Name))
		} else {
			fmt.Println(color.RedString("❌ FAIL: " + res.Name + " | " + res.Error))
		}
	}

	return nil
}
