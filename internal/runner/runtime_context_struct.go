package runner

import (
	"reqx/internal/environment"
	"sync"
)

// RuntimeContext holds state and variables during execution.
type RuntimeContext struct {
	GlobalVariables map[string]interface{}//this is a map that will hold the global variables
	Environment     *environment.Environment //this is a pointer to the environment struct that will hold the environment variables
	AsyncWG         *sync.WaitGroup          // Shared across DAG parallel nodes to track background tasks
	AsyncStop       chan struct{}            // Shared across DAG parallel nodes to signal background stop
	AsyncStopOnce   *sync.Once
	ownsAsyncStop 	bool						//this is a boolean that will tell us if the current context owns the async stop

	// PersistConnections: when true (scheduler VU mode), async socket
	// connections are NOT torn down between iterations. RunDAG will skip
	// closing AsyncStop at iteration end; the worker goroutine owns that.
	PersistConnections bool

	// connectedURLs tracks which async socket URLs are already connected for
	// this worker, so each URL is dialled only once across all iterations.
	connectedURLs map[string]struct{}
	connMu        sync.Mutex
	
	// pauseCh is replaced with a new open channel when the worker goes idle
	// and closed when the worker becomes active again. Background socket
	// operations block on pauseCh until it is reopened.
	pauseCh chan struct{}
	pauseMu sync.RWMutex
}
