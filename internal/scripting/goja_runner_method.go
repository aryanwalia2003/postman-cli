package scripting

import (
	"strings"

	"github.com/dop251/goja"
	"postman-cli/internal/collection"
	"postman-cli/internal/environment"
	"postman-cli/internal/errs"
)

// Execute runs a JavaScript snippet within a fresh VM, injecting the Environment.
func (g *GojaRunner) Execute(script *collection.Script, env *environment.Environment) error {
	if script == nil || len(script.Exec) == 0 {
		return nil
	}

	vm := goja.New()

	// Inject environment variables as an object called "env"
	envObj := vm.NewObject()
	if env != nil && env.Variables != nil {
		for k, v := range env.Variables {
			envObj.Set(k, v)
		}
	}
	vm.Set("env", envObj)

	// Combine script lines into one block
	scriptSource := strings.Join(script.Exec, "\n")

	// Run the script
	val, err := vm.RunString(scriptSource)
	if err != nil {
		return errs.Wrap(err, errs.KindInternal, "script execution failed")
	}

	// Just as a placeholder to show capturing script output
	if val != nil && val.Export() != nil {
		// Scripts could theoretically return something, but typically they just mutate the `env` object.
	}

	// Extract updated environment back to Environment struct
	for _, key := range envObj.Keys() {
		newVal := envObj.Get(key)
		if newVal != nil {
			if env != nil {
				env.Set(key, newVal.String())
			}
		}
	}

	return nil
}
