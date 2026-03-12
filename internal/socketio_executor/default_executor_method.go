package socketio_executor

import (
	"fmt"
	"time"

	"github.com/zhouhui8915/go-socket.io-client"
	
	"postman-cli/internal/collection"
	"postman-cli/internal/errs"
)

// Execute runs the Socket.IO flow, emitting and listening to defined events.
func (e *DefaultSocketIOExecutor) Execute(rawURL string, headers map[string]string, events []collection.SocketIOEvent) error {
	// We use rawURL directly now
	if rawURL == "" {
		return errs.InvalidInput("invalid socket.io url: empty")
	}

	opts := &socketio_client.Options{
		Query: make(map[string]string),
	}
	
	if len(headers) > 0 {
		opts.Header = make(map[string][]string)
		for k, v := range headers {
			opts.Header[k] = []string{v}
		}
	}

	fmt.Printf("Connecting to Socket.IO Server: %s\n", rawURL)

	client, err := socketio_client.NewClient(rawURL, opts)
	if err != nil {
		return errs.Wrap(err, errs.KindInternal, "failed to create socket.io client")
	}

	expectedListeners := 0
	done := make(chan struct{})

	// Register listeners
	for _, ev := range events {
		if ev.Type == "listen" {
			expectedListeners++
			fmt.Printf("Registered listener for event: '%s'\n", ev.Name)
			
			// Capture the ev variable for the closure
			captureEv := ev
			client.On(captureEv.Name, func(msg string) {
				fmt.Printf("\n[RECEIVED] Event: '%s' | Data: %v\n", captureEv.Name, msg)
				expectedListeners--
				if expectedListeners <= 0 {
					// We use a non-blocking send just in case
					select {
					case done <- struct{}{}:
					default:
					}
				}
			})
		}
	}

	client.On("error", func() {
		fmt.Printf("Socket.IO Error!\n")
	})

	client.On("connection", func() {
		fmt.Println("Connected successfully.")
	})

	client.On("disconnection", func() {
		fmt.Println("Disconnected.")
	})

	// Client is already connected upon NewClient

	// Wait 1 sec max for the connection to fully establish
	time.Sleep(1 * time.Second)

	// Emit events
	for _, ev := range events {
		if ev.Type == "emit" {
			fmt.Printf("[EMIT] Event: '%s' | Payload: %s\n", ev.Name, ev.Payload)
			// client.Emit takes variadic interface{} but string works best
			client.Emit(ev.Name, ev.Payload)
			time.Sleep(200 * time.Millisecond) // Slight delay between emits
		}
	}

	// If we have listeners, we need to wait for them.
	if expectedListeners > 0 {
		fmt.Printf("Waiting up to %v for %d expected listener(s)...\n", e.timeout, expectedListeners)
		select {
		case <-done:
			fmt.Println("All expected events received.")
		case <-time.After(e.timeout):
			fmt.Println("Timeout reached while waiting for events.")
		}
	} else {
		// Just wait a tiny bit to ensure emits go out before closing
		time.Sleep(1 * time.Second)
	}

	fmt.Println("Closing Socket.IO connection.")
	return nil
}
