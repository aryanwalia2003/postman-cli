package scripting

import (
	"reqx/internal/collection"
	"reqx/internal/environment"

	"github.com/dop251/goja"
)

// ScriptRunner defines the interface for executing pre-request and test scripts.
type ScriptRunner interface {
	Execute(script *collection.Script, env *environment.Environment, resp *ResponseAPI, compiled *goja.Program) error //takes a pointer to the script struct, a pointer to the environment struct, and a response pointer
}

