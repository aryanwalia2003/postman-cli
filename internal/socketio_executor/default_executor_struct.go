package socketio_executor

import (
	"sync"
	"time"
)

// DefaultSocketIOExecutor implements the Socket.IO flow using njones/socketio.
type DefaultSocketIOExecutor struct {
	timeout time.Duration
	quiet   bool
	pauseMu sync.RWMutex
	pauseCh <-chan struct{}
}
