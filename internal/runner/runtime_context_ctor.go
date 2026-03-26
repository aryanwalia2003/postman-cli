package runner

import (
	"reqx/internal/environment"
	"sync"
)

// NewRuntimeContext constructs a new RuntimeContext.
// pauseCh starts closed — meaning "active, sockets may read freely".
func NewRuntimeContext() *RuntimeContext {
	activeCh := make(chan struct{})
	close(activeCh) // closed = active/running state

	return &RuntimeContext{
		GlobalVariables: make(map[string]interface{}),
		Environment:     environment.NewEnvironment("default"),
		AsyncWG:         new(sync.WaitGroup),
		AsyncStop:       make(chan struct{}),
		AsyncStopOnce:   new(sync.Once),
		ownsAsyncStop:   true,
		pauseCh:         activeCh,
	}
}

// CloneForNode creates a lightweight child context for a DAG node goroutine.
// The clone:
//   - gets its own Environment snapshot so concurrent pm.env.set is race-free
//   - shares AsyncWG / AsyncStop / AsyncStopOnce so background tasks are
//     tracked by the single top-level RunDAG defer
//   - has ownsAsyncStop = false so runLinear does NOT close the stop channel
//   - shares pauseCh so DAG node sockets respect the parent worker's idle state
func (rc *RuntimeContext) CloneForNode() *RuntimeContext {
	rc.pauseMu.RLock()
	ch := rc.pauseCh
	rc.pauseMu.RUnlock()

	return &RuntimeContext{
		GlobalVariables: rc.GlobalVariables,
		Environment:     newEnvSnapshot(rc.Environment),
		AsyncWG:         rc.AsyncWG,
		AsyncStop:       rc.AsyncStop,
		AsyncStopOnce:   rc.AsyncStopOnce,
		ownsAsyncStop:   false,
		pauseCh:         ch,
	}
}

func (rc *RuntimeContext) PauseConnections() {
	rc.pauseMu.Lock()
	defer rc.pauseMu.Unlock()
	// If the channel is already open (paused), nothing to do.
	select {
	case <-rc.pauseCh:
		// Channel is closed = currently active. Replace with a new open channel.
		rc.pauseCh = make(chan struct{})
	default:
		// Already paused (open channel). No-op.
	}
}


func (rc *RuntimeContext) ResumeConnections() {
	rc.pauseMu.Lock()
	defer rc.pauseMu.Unlock()
	// If the channel is already closed (active), nothing to do.
	select {
	case <-rc.pauseCh:
		// Already active (closed channel). No-op.
	default:
		// Currently paused (open channel). Close it to signal "resume".
		close(rc.pauseCh)
	}
}

func (rc *RuntimeContext) ActiveCh() <-chan struct{} {
	rc.pauseMu.Lock()
	ch := rc.pauseCh
	rc.pauseMu.Unlock()
	return ch
}