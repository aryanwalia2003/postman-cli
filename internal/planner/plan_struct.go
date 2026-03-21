package planner

import (
	"reqx/internal/collection"

	"github.com/dop251/goja"
)

type ScriptKey struct{
	RequestIndex int
	ScriptType string
}

// ExecutionPlan is the immutable, pre-computed set of instructions for one test run.
// It is built once by BuildExecutionPlan from a Collection + CLI flags, then handed
// to every Scheduler worker and WorkerPool goroutine as a read-only value.
// Workers never modify an ExecutionPlan. Per-iteration mutable state
// (environment variables, cookie jars) lives in RuntimeContext, not here.
type ExecutionPlan struct {
	Requests []collection.Request
	CollectionAuth *collection.Auth
	CompiledScripts map[ScriptKey]*goja.Program
}