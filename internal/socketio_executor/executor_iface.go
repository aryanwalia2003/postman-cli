package socketio_executor

import "reqx/internal/collection"

// SocketIOExecutor defines the interface for executing Socket.IO request flows.
type SocketIOExecutor interface {
	//this now takes a ready chan , it will send nil over that channel if succesfully connect ho jaata hai else it will send error over that channel
	Execute(url string, headers map[string]string, events []collection.SocketIOEvent,readyChan chan error, stopChan chan struct{}) error
}
