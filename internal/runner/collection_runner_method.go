package runner

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"strings"
	"time"

	"postman-cli/internal/collection"
	"postman-cli/internal/http_executor"
	"postman-cli/internal/scripting"

	"github.com/fatih/color"
)

func (cr *CollectionRunner) SetClearCookiesPerRequest(v bool) {
	cr.clearCookiesPerRequest = v
}

// Run executes all requests within a collection sequentially.
func (cr *CollectionRunner) Run(coll *collection.Collection, ctx *RuntimeContext) error {
	verboseColor := color.New(color.FgYellow)
	timingColor := color.New(color.FgHiCyan)
	
	// Initialize metrics tracking for this run
	cr.metrics = make([]RequestMetric, 0, len(coll.Requests))
	collectionStartTime := time.Now()

	stopAsyncSockets := make(chan struct{})

	defer func() {
		fmt.Println("\n⏳ Waiting 3 seconds for any final trailing socket events...")
		time.Sleep(3 * time.Second)
		close(stopAsyncSockets)
		time.Sleep(500 * time.Millisecond)

		// ==========================================
		// NEW: Print the Final Summary Dashboard
		// ==========================================
		cr.printSummary(time.Since(collectionStartTime))
	}()

	for _, req := range coll.Requests {
		fmt.Printf("\n▶ Running request: %s\n", req.Name)

		cr.runScripts("prerequest", req.Scripts, ctx, nil)
		urlStr := cr.replaceVars(req.URL, ctx)

		if strings.ToUpper(req.Protocol) == "SOCKETIO" {
			headers := make(map[string]string)
			for k, v := range req.Headers {
				headers[k] = cr.replaceVars(v, ctx)
			}

			var resolvedEvents []collection.SocketIOEvent
			for _, ev := range req.Events {
				resolvedEvents = append(resolvedEvents, collection.SocketIOEvent{
					Type:    ev.Type,
					Name:    cr.replaceVars(ev.Name, ctx),
					Payload: cr.replaceVars(ev.Payload, ctx),
				})
			}

			startSio := time.Now()

			if req.Async {
				fmt.Printf("Starting Background Socket.IO connection for '%s'...\n", req.Name)
				readyChan := make(chan error, 1)

				go func(name, url string, hdrs map[string]string, events []collection.SocketIOEvent) {
					err := cr.sioExecutor.Execute(url, hdrs, events, readyChan, stopAsyncSockets)
					if err != nil {
						color.Red("\n[BACKGROUND ERROR] Socket.IO '%s' failed: %v\n", name, err)
					} else {
						color.Green("\n[BACKGROUND SUCCESS] Socket.IO '%s' completed.\n", name)
					}
				}(req.Name, urlStr, headers, resolvedEvents)

				color.Cyan("⏳ Waiting for socket to establish connection...\n")
				err := <-readyChan

				// Track Metric for Async Socket
				cr.metrics = append(cr.metrics, RequestMetric{
					Name:         req.Name,
					Protocol:     "SOCKET",
					Duration:     time.Since(startSio),
					StatusString: "ASYNC",
					Error:        err,
				})

				if err != nil {
					color.Yellow("⚠ Background Socket failed to connect: %v. Continuing collection anyway...\n", err)
				} else {
					color.Green("✔ Background Socket is ready and listening continuously!\n")
				}

				cr.runScripts("test", req.Scripts, ctx, nil)
				continue

			} else {
				err := cr.sioExecutor.Execute(urlStr, headers, resolvedEvents, nil, nil)
				
				// Track Metric for Sync Socket
				cr.metrics = append(cr.metrics, RequestMetric{
					Name:         req.Name,
					Protocol:     "SOCKET",
					Duration:     time.Since(startSio),
					StatusString: "SYNC",
					Error:        err,
				})

				if err != nil {
					fmt.Printf("Socket.IO Request %s failed: %v\n", req.Name, err)
				} else {
					fmt.Printf("Socket.IO Request %s successful\n", req.Name)
				}
				cr.runScripts("test", req.Scripts, ctx, nil)
				continue
			}
		}

		var bodyReader io.Reader
		if req.Body != "" {
			bodyBytes := []byte(cr.replaceVars(req.Body, ctx))
			bodyReader = bytes.NewBuffer(bodyBytes)
		}

		httpReq, err := http.NewRequest(strings.ToUpper(req.Method), urlStr, bodyReader)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			cr.metrics = append(cr.metrics, RequestMetric{Name: req.Name, Protocol: "HTTP", Error: err})
			continue
		}

		for k, v := range req.Headers {
			httpReq.Header.Set(k, cr.replaceVars(v, ctx))
		}
		http_executor.ApplyAuth(httpReq, cr.resolveAuth(req.Auth, coll.Auth, ctx))

		if cr.verboseMode {
			verboseColor.Println("-------------------- REQUEST --------------------")
			dump, _ := httputil.DumpRequestOut(httpReq, true)
			scanner := bufio.NewScanner(strings.NewReader(string(dump)))
			for scanner.Scan() {
				verboseColor.Printf("> %s\n", scanner.Text())
			}
			verboseColor.Println("-------------------------------------------------")
		}

		if cr.clearCookiesPerRequest {
			cr.executor.ClearCookies()
		}

		var t0, dnsStart, dnsDone, connStart, connDone, tlsStart, tlsDone, reqDone, resStart time.Time

		trace := &httptrace.ClientTrace{
			DNSStart:             func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
			DNSDone:              func(_ httptrace.DNSDoneInfo) { dnsDone = time.Now() },
			ConnectStart:         func(_, _ string) { connStart = time.Now() },
			ConnectDone:          func(net, addr string, err error) { connDone = time.Now() },
			TLSHandshakeStart:    func() { tlsStart = time.Now() },
			TLSHandshakeDone:     func(_ tls.ConnectionState, _ error) { tlsDone = time.Now() },
			WroteRequest:         func(_ httptrace.WroteRequestInfo) { reqDone = time.Now() },
			GotFirstResponseByte: func() { resStart = time.Now() },
		}

		httpReq = httpReq.WithContext(httptrace.WithClientTrace(httpReq.Context(), trace))

		t0 = time.Now()
		resp, err := cr.executor.Execute(httpReq)
		totalTime := time.Since(t0)

		if err != nil {
			fmt.Printf("Request Failed: %v\n", err)
			cr.metrics = append(cr.metrics, RequestMetric{Name: req.Name, Protocol: "HTTP", Duration: totalTime, Error: err})
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Track Metric for HTTP
		cr.metrics = append(cr.metrics, RequestMetric{
			Name:         req.Name,
			Protocol:     "HTTP",
			StatusCode:   resp.StatusCode,
			StatusString: resp.Status,
			Duration:     totalTime,
		})

		var dnsTime, tcpTime, tlsTime, ttfbTime, transferTime time.Duration
		if !dnsDone.IsZero() { dnsTime = dnsDone.Sub(dnsStart) }
		if !connDone.IsZero() { tcpTime = connDone.Sub(connStart) }
		if !tlsDone.IsZero() { tlsTime = tlsDone.Sub(tlsStart) }
		if !resStart.IsZero() {
			if !reqDone.IsZero() { ttfbTime = resStart.Sub(reqDone) } else { ttfbTime = resStart.Sub(t0) }
			transferTime = time.Now().Sub(resStart)
		}

		if cr.verboseMode {
			verboseColor.Println("-------------------- RESPONSE -------------------")
			verboseColor.Printf("< %s %s\n", resp.Proto, resp.Status)
			for key, values := range resp.Header {
				if isNoisyHeader(key) { continue }
				for _, value := range values {
					verboseColor.Printf("< %s: %s\n", key, value)
				}
			}
			verboseColor.Println("<")

			if len(bodyBytes) > 0 {
				contentType := resp.Header.Get("Content-Type")
				if strings.Contains(contentType, "json") {
					var prettyJSON bytes.Buffer
					if err := json.Indent(&prettyJSON, bodyBytes, "", "  "); err == nil {
						fmt.Println(prettyJSON.String())
					} else {
						fmt.Println(string(bodyBytes))
					}
				} else {
					fmt.Println(string(bodyBytes))
				}
			}

			timingColor.Println("-------------------- TIMINGS --------------------")
			timingColor.Printf("  DNS Lookup     : %v\n", roundMs(dnsTime))
			timingColor.Printf("  TCP Connection : %v\n", roundMs(tcpTime))
			timingColor.Printf("  TLS Handshake  : %v\n", roundMs(tlsTime))
			timingColor.Printf("  Server (TTFB)  : %v\n", roundMs(ttfbTime))
			timingColor.Printf("  Data Transfer  : %v\n", roundMs(transferTime))
			timingColor.Printf("  --------------------------\n")
			timingColor.Printf("  Total Time     : %v\n", roundMs(totalTime))
			timingColor.Println("-------------------------------------------------")
		}

		statusColor := color.New(color.FgHiGreen).SprintfFunc()
		if resp.StatusCode >= 400 {
			statusColor = color.New(color.FgHiRed).SprintfFunc()
		}
		fmt.Printf("Status: %s  |  Time: %v\n", statusColor(resp.Status), roundMs(totalTime))

		scriptResp := &scripting.ResponseAPI{
			BodyString: string(bodyBytes),
			Headers:    &scripting.ResponseHeaders{Headers: make(map[string]string)},
		}
		for k, v := range resp.Header {
			if len(v) > 0 { scriptResp.Headers.Headers[k] = v[0] }
		}
		cr.runScripts("test", req.Scripts, ctx, scriptResp)
	}

	return nil
}

// ... [Keep existing helper functions resolveAuth, runScripts, replaceVars, isNoisyHeader, roundMs here] ...

// Helper functions below (Make sure these are pasted at the bottom)

func (cr *CollectionRunner) resolveAuth(reqAuth, collAuth *collection.Auth, ctx *RuntimeContext) *collection.Auth {
	src := reqAuth
	if src == nil { src = collAuth }
	if src == nil { return nil }
	resolved := &collection.Auth{
		Type:     src.Type,
		Token:    cr.replaceVars(src.Token, ctx),
		Username: cr.replaceVars(src.Username, ctx),
		Password: cr.replaceVars(src.Password, ctx),
		Key:      cr.replaceVars(src.Key, ctx),
		Value:    cr.replaceVars(src.Value, ctx),
		In:       src.In,
	}
	if src.Cookies != nil {
		resolved.Cookies = make(map[string]string, len(src.Cookies))
		for k, v := range src.Cookies {
			resolved.Cookies[k] = cr.replaceVars(v, ctx)
		}
	}
	return resolved
}

func (cr *CollectionRunner) runScripts(scriptType string, scripts []collection.Script, ctx *RuntimeContext, resp *scripting.ResponseAPI) {
	for _, s := range scripts {
		if s.Type == scriptType {
			err := cr.scriptRunner.Execute(&s, ctx.Environment, resp)
			if err != nil {
				fmt.Printf("Warning: script execution failed: %v\n", err)
			}
		}
	}
}

func (cr *CollectionRunner) replaceVars(input string, ctx *RuntimeContext) string {
	if ctx == nil || ctx.Environment == nil { return input }
	out := input
	for k, v := range ctx.Environment.Variables {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

func isNoisyHeader(header string) bool {
	noisy := map[string]bool{
		"Content-Security-Policy": true, "Strict-Transport-Security": true,
		"X-Dns-Prefetch-Control": true, "X-Frame-Options": true,
		"X-Xss-Protection": true, "X-Permitted-Cross-Domain-Policies": true,
		"Cross-Origin-Opener-Policy": true, "Cross-Origin-Resource-Policy": true,
		"Origin-Agent-Cluster": true, "Referrer-Policy": true,
		"X-Content-Type-Options": true, "X-Download-Options": true,
		"Etag": true, "Vary": true, "Access-Control-Allow-Credentials": true, "Date": true,
	}
	return noisy[http.CanonicalHeaderKey(header)]
}

func roundMs(d time.Duration) string {
	if d == 0 { return "0ms" }
	return fmt.Sprintf("%dms", d.Milliseconds())
}

// ========================================================
// SUMMARY LOGIC
// ========================================================
func (cr *CollectionRunner) printSummary(totalExecutionTime time.Duration) {
    // Replaced the helper logic above to take duration correctly.
	fmt.Println("\n" + strings.Repeat("=", 70))
	color.New(color.FgHiCyan, color.Bold).Println("  EXECUTION SUMMARY")
	fmt.Println(strings.Repeat("=", 70))

	var totalHttp, successHttp, failedHttp int
	var maxTime, minTime, totalHttpTime time.Duration
	var slowestReq string
	minTime = time.Hour // Default to a huge number

	for i, m := range cr.metrics {
		statusCol := color.New(color.FgHiGreen).SprintFunc()
		if m.Error != nil || m.StatusCode >= 400 {
			statusCol = color.New(color.FgHiRed).SprintFunc()
		}

		if m.Protocol == "SOCKET" {
			if m.Error != nil {
				fmt.Printf("  [%2d] %-8s %-20s %s\n", i+1, color.BlueString("SOCKET"), statusCol("ERR"), m.Name)
			} else {
				fmt.Printf("  [%2d] %-8s %-20s %s\n", i+1, color.BlueString("SOCKET"), statusCol(m.StatusString), m.Name)
			}
		} else {
			totalHttp++
			totalHttpTime += m.Duration
			if m.Error != nil || m.StatusCode >= 400 { failedHttp++ } else { successHttp++ }

			if m.Duration > maxTime { maxTime = m.Duration; slowestReq = m.Name }
			if m.Duration < minTime && m.Duration > 0 { minTime = m.Duration }

			statusTxt := m.StatusString
			if m.Error != nil { statusTxt = "ERR" }
			fmt.Printf("  [%2d] %-8s %-20s %-8s %s\n", i+1, "HTTP", statusCol(statusTxt), roundMs(m.Duration), m.Name)
		}
	}

	fmt.Println(strings.Repeat("-", 70))

	if totalHttp > 0 {
		avgTime := totalHttpTime / time.Duration(totalHttp)
		fmt.Printf("  HTTP Requests : %d Total | %s | %s\n", totalHttp, color.GreenString("%d Success", successHttp), color.RedString("%d Failed", failedHttp))
		fmt.Printf("  Avg Latency   : %s\n", color.CyanString(roundMs(avgTime)))
		fmt.Printf("  Min Latency   : %s\n", roundMs(minTime))
		fmt.Printf("  Max Latency   : %s (%s)\n", color.YellowString(roundMs(maxTime)), slowestReq)
	}
	fmt.Printf("  Total Run Time: %v\n", totalExecutionTime.Round(time.Millisecond))
	fmt.Println(strings.Repeat("=", 70))
}