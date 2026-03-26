package socketio_executor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"

	"reqx/internal/collection"
	"reqx/internal/errs"
)

// Execute runs the Socket.IO flow, emitting and listening to defined events using raw V4 WebSockets.
func (e *DefaultSocketIOExecutor) Execute(rawURL string, headers map[string]string, events []collection.SocketIOEvent, readyChan chan error, stopChan chan struct{}) error {
	if rawURL == "" {
		if readyChan != nil {
			readyChan <- errs.InvalidInput("invalid socket.io url: empty")
		}
		return errs.InvalidInput("invalid socket.io url: empty")
	}

	// 1. Format URL for WebSocket and Engine.IO v4
	u, err := url.Parse(rawURL)
	if err != nil {
		if readyChan != nil {
			readyChan <- errs.Wrap(err, errs.KindInvalidInput, "Invalid URL format")
		}
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
	q.Set("EIO", "4")
	q.Set("transport", "websocket")
	u.RawQuery = q.Encode()

	// 2. Prepare Custom Headers
	reqHeaders := http.Header{}
	for k, v := range headers {
		reqHeaders.Add(k, v)
	}

	if !e.quiet {
		fmt.Printf("Connecting to Socket.IO Server (v4): %s\n", u.String())
	}

	// 3. Connect via raw WebSocket
	conn, _, err := sharedDialer.Dial(u.String(), reqHeaders)
	if err != nil {
		if readyChan != nil {
			readyChan <- errs.Wrap(err, errs.KindInternal, "Failed to connect to websocket")
		}
		return errs.Wrap(err, errs.KindInternal, "Failed to connect to websocket")
	}
	defer conn.Close()

	// 4. State Management for Listeners
	var mu sync.Mutex      // Guards expectedListeners and listenTargets
	var writeMu sync.Mutex // Guards conn.WriteMessage (gorilla forbids concurrent writers)
	expectedListeners := 0
	listenTargets := make(map[string]int)

	for _, ev := range events {
		if ev.Type == "listen" {
			expectedListeners++
			listenTargets[ev.Name]++
			if !e.quiet {
				fmt.Printf("Registered listener for event: '%s'\n", ev.Name)
			}
		}
	}

	done := make(chan struct{})
	connected := make(chan struct{})

	// 5. Background Reader — Byte-First, Pause-Aware
	//
	// BYTE-FIRST: All protocol dispatch operates on the raw []byte frame from
	// ReadMessage, avoiding the string(message) allocation on every frame.
	// gjson.GetBytes operates directly on []byte without heap allocation.
	//
	// PAUSE-AWARE: When the scheduler worker goes idle (stage ramp-down),
	// rtCtx.PauseConnections() replaces the active channel with an open
	// channel. This goroutine detects the transition at the top of its loop
	// and parks in a select{} instead of calling ReadMessage. This frees the
	// OS thread that would otherwise be blocked on the network syscall,
	// which was causing runtime.allocm at 14.56% in the March 26 profiling.
	//
	// When the worker becomes active again (rtCtx.ResumeConnections()), the
	// channel is closed and the goroutine re-enters the read loop.
	go func() {
		for {
			// ── Idle gate ──────────────────────────────────────────────────
			// activeCh() is closed when the worker is active (fast path,
			// the select returns immediately). When it is open (idle state),
			// we block here without calling ReadMessage, releasing the OS thread.
			//
			// We also select on stopChan so that a worker shutdown during idle
			// does not leave this goroutine stuck forever.
			active := e.activeCh()
			select {
			case <-active:
				// Worker is active — fall through to ReadMessage.
			case <-stopChan:
				// Shutdown signal received while idle. Exit cleanly.
				return
			}

			// ── Read next frame ─────────────────────────────────────────────
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if len(msgBytes) == 0 {
				continue
			}

			// ── Protocol dispatch (byte comparisons, zero allocation) ───────
			switch {
			case msgBytes[0] == '0':
				// Engine.IO Open → send Socket.IO Connect (40)
				writeMu.Lock()
				conn.WriteMessage(websocket.TextMessage, []byte("40"))
				writeMu.Unlock()

			case msgBytes[0] == '2':
				// Engine.IO Ping → reply Pong (3)
				writeMu.Lock()
				conn.WriteMessage(websocket.TextMessage, []byte("3"))
				writeMu.Unlock()

			case len(msgBytes) >= 2 && msgBytes[0] == '4' && msgBytes[1] == '0':
				// Socket.IO Connected
				if !e.quiet {
					fmt.Println("Connected successfully.")
				}
				select {
				case <-connected:
				default:
					close(connected)
					if readyChan != nil {
						readyChan <- nil
					}
				}

			case len(msgBytes) >= 2 && msgBytes[0] == '4' && msgBytes[1] == '2':
				// Incoming Socket.IO event: "42[<name>,<payload>]"
				dataBytes := msgBytes[2:] // reslice — no copy

				res := gjson.GetBytes(dataBytes, "0") // zero-alloc byte scan
				if !res.Exists() {
					continue
				}

				eventName := res.String() // one alloc per matched event only
				mu.Lock()
				isListening := false
				for _, ev := range events {
					if ev.Type == "listen" && ev.Name == eventName {
						isListening = true
						break
					}
				}

				if isListening {
					if !e.quiet {
						payload := gjson.GetBytes(dataBytes, "1").Raw
						fmt.Printf("\n[RECEIVED] Event: '%s' | Data: %v\n", eventName, payload)
					}

					if stopChan == nil {
						if needed := listenTargets[eventName]; needed > 0 {
							listenTargets[eventName]--
							expectedListeners--
							if expectedListeners == 0 {
								select {
								case <-done:
								default:
									close(done)
								}
							}
						}
					}
				}
				mu.Unlock()
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
			if !e.quiet {
				fmt.Printf("[EMIT] Event: '%s' | Payload: %s\n", ev.Name, ev.Payload)
			}

			nameBytes, _ := json.Marshal(ev.Name)
			var finalMessage string

			if ev.Payload == "" {
				finalMessage = "42[" + string(nameBytes) + "]"
			} else if gjson.Valid(ev.Payload) {
				finalMessage = "42[" + string(nameBytes) + "," + ev.Payload + "]"
			} else {
				payloadBytes, _ := json.Marshal(ev.Payload)
				finalMessage = "42[" + string(nameBytes) + "," + string(payloadBytes) + "]"
			}

			writeMu.Lock()
			conn.WriteMessage(websocket.TextMessage, []byte(finalMessage))
			writeMu.Unlock()
			time.Sleep(10 * time.Millisecond)
		}
	}

	// 7. WAIT LOGIC (Async vs Sync)

	// ASYNC MODE: Wait indefinitely until Runner sends stop signal
	if stopChan != nil {
		<-stopChan
		if !e.quiet {
			fmt.Println("\nClosing Background Socket.IO connection...")
		}
		return nil
	}

	// SYNC MODE: Wait for specific events to arrive
	mu.Lock()
	remaining := expectedListeners
	mu.Unlock()

	if remaining > 0 {
		if !e.quiet {
			fmt.Printf("Waiting up to %v for expected listener(s)...\n", e.timeout)
		}
		select {
		case <-done:
			if !e.quiet {
				fmt.Println("All expected events received.")
			}
		case <-time.After(e.timeout):
			if !e.quiet {
				mu.Lock()
				missed := expectedListeners
				mu.Unlock()
				fmt.Printf("Timeout reached! Missed %d event(s).\n", missed)
			}
		}
	} else {
		time.Sleep(1 * time.Second)
	}

	if !e.quiet {
		fmt.Println("Closing Socket.IO connection.")
	}
	return nil
}