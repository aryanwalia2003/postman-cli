package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"reqx/internal/errs"
)

func NewSioCmd() *cobra.Command {
	var headers []string

	c := &cobra.Command{
		Use:   "sio [url]",
		Short: "Start an interactive Socket.IO (v4) debugging session",
		Long: `🔌 Open a real-time, interactive REPL for Socket.IO v4 servers.
This command turns your terminal into a powerful Socket.IO debugger. 
You can manually emit events, track incoming data, and debug 
complex real-time flows without writing any code.

🛠 REPL Capabilities:
- listen <event> : Start tracking a specific event and print its data.
- emit <event> <json> : Send data to the server for a specific event.
- Real-time output: Incoming messages are printed with timestamps and metadata.`,
		Example: `  # 📡 Connect to a local dev server
  reqx sio http://localhost:3000
  
  # 🔐 Connect to a secure production API with an Auth Cookie
  reqx sio wss://api.prod.com -H "Cookie: session-token={{token}}"
  
  # 💡 Commands inside the interactive REPL:
  > listen user_updated
  > emit delete_user {"id": 123}
  > exit`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawURL := args[0]

			// Output Formatters
			infoMsg := color.New(color.FgCyan).PrintfFunc()
			serverMsg := color.New(color.FgGreen).PrintfFunc()
			clientMsg := color.New(color.FgYellow).PrintfFunc()
			errorMsg := color.New(color.FgRed).PrintfFunc()

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
			q.Set("EIO", "4") // FORCE SOCKET.IO v4 Protocol
			q.Set("transport", "websocket")
			u.RawQuery = q.Encode()

			// 2. Parse custom headers (Auth/Cookies)
			reqHeaders := http.Header{}
			for _, h := range headers {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) == 2 {
					reqHeaders.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}

			infoMsg("⏳ Connecting to Socket.IO Server: %s\n", u.String())

			// 3. Connect via raw WebSocket
			dialer := websocket.DefaultDialer
			conn, _, err := dialer.Dial(u.String(), reqHeaders)
			if err != nil {
				return errs.Wrap(err, errs.KindInternal, "Failed to connect to websocket")
			}
			defer conn.Close()

			isConnected := false
			listeners := make(map[string]bool)
			var mu sync.Mutex

			// 4. Background Goroutine to handle Socket.IO Protocol
			go func() {
				for {
					_, message, err := conn.ReadMessage()
					if err != nil {
						errorMsg("\n❌ Disconnected from server.\n> ")
						isConnected = false
						return
					}

					msgStr := string(message)

					// Engine.IO / Socket.IO Protocol Handling
					if strings.HasPrefix(msgStr, "0") {
						// Engine.IO Open -> Send Socket.IO Connect (40)
						conn.WriteMessage(websocket.TextMessage, []byte("40"))
					} else if strings.HasPrefix(msgStr, "2") {
						// Engine.IO Ping -> Reply with Pong (3)
						conn.WriteMessage(websocket.TextMessage, []byte("3"))
					} else if strings.HasPrefix(msgStr, "40") {
						// Socket.IO Connected
						isConnected = true
						infoMsg("\n✅ Connected successfully!\n> ")
					} else if strings.HasPrefix(msgStr, "42") {
						// Socket.IO Event (Format: 42["event_name", payload])
						dataStr := msgStr[2:]
						var arr []interface{}
						if json.Unmarshal([]byte(dataStr), &arr) == nil && len(arr) > 0 {
							if eventName, ok := arr[0].(string); ok {
								mu.Lock()
								isListening := listeners[eventName]
								mu.Unlock()

								if isListening {
									payload := ""
									if len(arr) > 1 {
										payloadBytes, _ := json.Marshal(arr[1])
										payload = string(payloadBytes)
									}
									fmt.Print("\r") // Clear line
									serverMsg("⬇️  [SERVER | event: '%s']: %s\n> ", eventName, payload)
								}
							}
						}
					}
				}
			}()

			time.Sleep(1 * time.Second)

			// 5. Print Instructions
			fmt.Println(strings.Repeat("-", 60))
			fmt.Println("Interactive Socket.IO Mode Started (v4 Protocol).")
			fmt.Println("Commands:")
			fmt.Println("  listen <event_name>          - Start listening to an event")
			fmt.Println("  emit <event_name> [payload]  - Send an event with JSON/text data")
			fmt.Println("  exit                         - Close connection and quit")
			fmt.Println(strings.Repeat("-", 60))

			// 6. Interactive REPL
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Print("\n> ")

			for {
				if !scanner.Scan() {
					break
				}

				text := strings.TrimSpace(scanner.Text())
				if text == "" {
					fmt.Print("> ")
					continue
				}

				parts := strings.SplitN(text, " ", 3)
				command := strings.ToLower(parts[0])

				switch command {
				case "exit", "quit":
					infoMsg("Closing connection...\n")
					return nil

				case "listen":
					if len(parts) < 2 {
						errorMsg("Usage: listen <event_name>\n> ")
						continue
					}
					eventName := parts[1]
					mu.Lock()
					listeners[eventName] = true
					mu.Unlock()
					infoMsg("👂 Now listening for event: '%s'\n> ", eventName)

				case "emit":
					if !isConnected {
						errorMsg("⚠️ Cannot emit. You are disconnected.\n> ")
						continue
					}
					if len(parts) < 2 {
						errorMsg("Usage: emit <event_name> [payload]\n> ")
						continue
					}
					eventName := parts[1]
					
					// Construct Socket.IO format: 42["event", "payload"]
					var payload interface{} = ""
					if len(parts) > 2 {
						// Try to parse as JSON, else send as string
						if err := json.Unmarshal([]byte(parts[2]), &payload); err != nil {
							payload = parts[2] 
						}
					}
					
					packet := []interface{}{eventName, payload}
					packetBytes, _ := json.Marshal(packet)
					finalMessage := "42" + string(packetBytes)

					conn.WriteMessage(websocket.TextMessage, []byte(finalMessage))
					
					payloadStr, _ := json.Marshal(payload)
					clientMsg("⬆️  [SENT | event: '%s']: %s\n> ", eventName, string(payloadStr))

				default:
					errorMsg("Unknown command. Use 'listen', 'emit', or 'exit'.\n> ")
				}
			}

			return nil
		},
	}

	c.Flags().StringSliceVarP(&headers, "header", "H", []string{}, "Custom headers (e.g., 'Authorization: Bearer token')")

	return c
}