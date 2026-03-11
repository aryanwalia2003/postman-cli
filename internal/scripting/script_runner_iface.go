package scripting

import (
	"postman-cli/internal/collection"
	"postman-cli/internal/environment"
)

// ScriptRunner defines the interface for executing pre-request and test scripts.
type ScriptRunner interface {
	Execute(script *collection.Script, env *environment.Environment) error //takes a pointer to the script struct and a pointer to the environment struct and returns an error
}

