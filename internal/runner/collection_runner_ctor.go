package runner

import (
	"reqx/internal/http_executor"
	"reqx/internal/scripting"
	"reqx/internal/socketio_executor"
	"reqx/internal/websocket_executor"
)

// NewCollectionRunner constructs the orchestration engine.
// exec must be a *http_executor.DefaultExecutor so cookie control is available.
func NewCollectionRunner(exec *http_executor.DefaultExecutor, sio socketio_executor.SocketIOExecutor, we websocket_executor.WebSocketExecutor, script scripting.ScriptRunner) *CollectionRunner {
	if exec == nil {
		exec = http_executor.NewDefaultExecutor()
	}
	if sio == nil {
		sio = socketio_executor.NewDefaultSocketIOExecutor()
	}
	if we == nil {
		we = websocket_executor.NewDefaultWebSocketExecutor()
	}
	if script == nil {
		script = scripting.NewGojaRunner()
	}

	return &CollectionRunner{
		executor:     exec,
		sioExecutor:  sio,
		weExecutor:   we,
		scriptRunner: script,
		verbosity:    VerbosityNormal,
	}
}

// SetVerbosity controls how much per-request output the runner emits.
// When v == VerbosityQuiet, it also silences all per-event console output
// from the Socket.IO and WebSocket executors, eliminating the OS-level
// syscall overhead that causes CPU spikes under high concurrency.
func (cr *CollectionRunner) SetVerbosity(v int) {
	cr.verbosity = v
	quiet := v <= VerbosityQuiet
	cr.sioExecutor.SetQuiet(quiet)
	cr.weExecutor.SetQuiet(quiet)
}

// SetVerbose is kept for backward compatibility; it maps to VerbosityFull.
func (cr *CollectionRunner) SetVerbose(v bool) {
	if v {
		cr.verbosity = VerbosityFull
	} else if cr.verbosity < VerbosityNormal {
		cr.verbosity = VerbosityNormal
	}
}

// ApplyRuntimeContext wires the RuntimeContext's pause channel into the
// Socket.IO executor. This must be called after NewCollectionRunner and
// before Run() in any context where idle-worker socket quiescing is desired
// (i.e. Scheduler VU mode).
//
// In WorkerPool mode or single-run mode, this is never called and the
// executor retains its default always-active behaviour.
func (cr *CollectionRunner) ApplyRuntimeContext(ctx *RuntimeContext) {
	cr.sioExecutor.SetPauseCh(ctx.ActiveCh())
}