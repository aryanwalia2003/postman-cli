package scripting

import "github.com/dop251/goja"

// GojaRunner is the concrete implementation of ScriptRunner using the goja JS engine.
type GojaRunner struct{
	vm *goja.Runtime
}
