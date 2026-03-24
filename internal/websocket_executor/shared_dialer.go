package websocket_executor

import (
	"crypto/tls"

	"github.com/gorilla/websocket"
)

// sharedDialer is a package-level WebSocket dialer that enables TLS session
// resumption. Same rationale as socketio_executor's shared dialer — reuses
// TLS sessions across connections so the server certificate is NOT re-parsed
// on every reconnect, dramatically cutting memory allocations under load.
var sharedDialer = &websocket.Dialer{
	TLSClientConfig: &tls.Config{
		ClientSessionCache: tls.NewLRUClientSessionCache(1000),
	},
	HandshakeTimeout: websocket.DefaultDialer.HandshakeTimeout,
}

// SetInsecure allows bypassing TLS certificate verification on the shared dialer.
func SetInsecure(insecure bool) {
	sharedDialer.TLSClientConfig.InsecureSkipVerify = insecure
}
