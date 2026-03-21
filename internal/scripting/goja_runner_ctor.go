package scripting

import "github.com/dop251/goja"

// NewGojaRunner constructs a ScriptRunner that uses the dop251/goja VM.
func NewGojaRunner() ScriptRunner {
	vm := goja.New()

	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json",true))
	return &GojaRunner{
		vm: vm,
	}
}
