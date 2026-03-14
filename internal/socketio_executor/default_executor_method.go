package socketio_executor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"postman-cli/internal/collection"
	"postman-cli/internal/errs"
)

// Execute runs the Socket.IO flow, emitting and listening to defined events using raw V4 WebSockets.
func (e *DefaultSocketIOExecutor) Execute(rawURL string, headers map[string]string, events []collection.SocketIOEvent,readyChan chan error) error {
	if rawURL == "" {
		return errs.InvalidInput("invalid socket.io url: empty")
	}

	// 1. Format URL for WebSocket and Engine.IO v4
	u, err := url.Parse(rawURL)
	if err != nil {
		return errs.Wrap(err, errs.KindInvalidInput, "Invalid URL format")
	}
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/socket.io/"
	}

	q := u.Query()
	q.Set("EIO", "4") // FORCE SOCKET.IO v4
	q.Set("transport", "websocket")
	u.RawQuery = q.Encode()

	// 2. Prepare Custom Headers (e.g., Cookies, Authorization)
	reqHeaders := http.Header{}
	for k, v := range headers {
		reqHeaders.Add(k, v)
	}

	fmt.Printf("Connecting to Socket.IO Server (v4): %s\n", u.String())

	// 3. Connect via raw WebSocket
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(u.String(), reqHeaders)
	if err != nil {

		if readyChan != nil {
			readyChan <- errs.Wrap(err,errs.KindInternal,"Failed to connect to websocket")
		}
		return errs.Wrap(err, errs.KindInternal, "Failed to connect to websocket")
	}
	defer conn.Close()

	// 4. State Management for Listeners
	var mu sync.Mutex
	expectedListeners := 0
	listenTargets := make(map[string]int)

	for _, ev := range events {
		if ev.Type == "listen" {
			expectedListeners++
			listenTargets[ev.Name]++
			fmt.Printf("Registered listener for event: '%s'\n", ev.Name)
		}
	}

	done := make(chan struct{})
	connected := make(chan struct{}) // To ensure we wait for '40' before emitting

	// 5. Background Reader (Handles Protocol & Incoming Events)
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			msgStr := string(message)

			if strings.HasPrefix(msgStr, "0") {
				// Engine.IO Open -> Send Socket.IO Connect (40)
				conn.WriteMessage(websocket.TextMessage, []byte("40"))
			} else if strings.HasPrefix(msgStr, "2") {
				// Engine.IO Ping -> Reply with Pong (3)
				conn.WriteMessage(websocket.TextMessage, []byte("3"))
			} else if strings.HasPrefix(msgStr, "40") {
				// Socket.IO Connected
				fmt.Println("Connected successfully.")
				select {
				case <-connected:
				default:
					close(connected) // Signal that it's safe to emit
					if readyChan != nil {
						readyChan <- nil
					}
				}
			} else if strings.HasPrefix(msgStr, "42") {
				// Incoming Event
				dataStr := msgStr[2:]
				var arr []interface{}
				if json.Unmarshal([]byte(dataStr), &arr) == nil && len(arr) > 0 {
					if eventName, ok := arr[0].(string); ok {
						mu.Lock()
						needed := listenTargets[eventName]
						if needed > 0 {
							listenTargets[eventName]--
							expectedListeners--

							payload := ""
							if len(arr) > 1 {
								payloadBytes, _ := json.Marshal(arr[1])
								payload = string(payloadBytes)
							}
							fmt.Printf("\n[RECEIVED] Event: '%s' | Data: %v\n", eventName, payload)

							// If all expected events are received, close the done channel
							if expectedListeners == 0 {
								select {
								case <-done:
								default:
									close(done)
								}
							}
						}
						mu.Unlock()
					}
				}
			}
		}
	}()

	// Wait up to 5 seconds for the Socket.IO connection to fully establish
	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		err := errs.Internal("Timeout waiting for Socket.IO connect (40) packet")
		if readyChan != nil {
			readyChan <- err
		}
		return err
	}

	// 6. Emit predefined events
	for _, ev := range events {
		if ev.Type == "emit" {
			fmt.Printf("[EMIT] Event: '%s' | Payload: %s\n", ev.Name, ev.Payload)

			var payload interface{} = ""
			if ev.Payload != "" {
				if err := json.Unmarshal([]byte(ev.Payload), &payload); err != nil {
					payload = ev.Payload
				}
			}

			packet := []interface{}{ev.Name, payload}
			packetBytes, _ := json.Marshal(packet)
			finalMessage := "42" + string(packetBytes)

			conn.WriteMessage(websocket.TextMessage, []byte(finalMessage))
			time.Sleep(200 * time.Millisecond) // Slight delay between emits to preserve ordering
		}
	}

	// 7. Wait for expected listeners to trigger
	if expectedListeners > 0 {
		fmt.Printf("Waiting up to %v for expected listener(s)...\n", e.timeout)
		select {
		case <-done:
			fmt.Println("All expected events received.")
		case <-time.After(e.timeout):
			fmt.Printf("Timeout reached! Missed %d event(s).\n", expectedListeners)
		}
	} else {
		// Just wait a tiny bit to ensure final emits go out before closing the conn
		time.Sleep(1 * time.Second)
	}

	fmt.Println("Closing Socket.IO connection.")
	return nil
}