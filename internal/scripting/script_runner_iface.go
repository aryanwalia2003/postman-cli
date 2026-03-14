package scripting

import (
	"reqx/internal/collection"
	"reqx/internal/environment"
)

// ScriptRunner defines the interface for executing pre-request and test scripts.
type ScriptRunner interface {
	Execute(script *collection.Script, env *environment.Environment, resp *ResponseAPI) error //takes a pointer to the script struct, a pointer to the environment struct, and a response pointer
}

