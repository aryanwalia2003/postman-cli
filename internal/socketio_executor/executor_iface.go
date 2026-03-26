package socketio_executor

import "reqx/internal/collection"

// SocketIOExecutor defines the interface for executing Socket.IO request flows.
type SocketIOExecutor interface {
	Execute(url string, headers map[string]string, events []collection.SocketIOEvent, readyChan chan error, stopChan chan struct{}) error
	SetQuiet(bool)
	SetPauseCh(ch <-chan struct{})
}