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

	"reqx/internal/collection"
	"reqx/internal/http_executor"
	"reqx/internal/scripting"

	"github.com/fatih/color"
)

func (cr *CollectionRunner) SetClearCookiesPerRequest(v bool) {
	cr.clearCookiesPerRequest = v
}

// Run executes all requests and returns the metrics for this single run.
func (cr *CollectionRunner) Run(coll *collection.Collection, ctx *RuntimeContext) ([]RequestMetric, error) {
	verboseColor := color.New(color.FgYellow)
	timingColor := color.New(color.FgHiCyan)

	// Metrics are now local to a single run
	metrics := make([]RequestMetric, 0, len(coll.Requests))
	
	stopAsyncSockets := make(chan struct{})
	
	// This defer is for the async socket kill switch for this single run
	defer func() {
		close(stopAsyncSockets)
		time.Sleep(500 * time.Millisecond)
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
					Type: ev.Type, Name: cr.replaceVars(ev.Name, ctx), Payload: cr.replaceVars(ev.Payload, ctx),
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
					}
				}(req.Name, urlStr, headers, resolvedEvents)
				color.Cyan("⏳ Waiting for socket to establish connection...\n")
				err := <-readyChan
				metrics = append(metrics, RequestMetric{
					Name: req.Name, Protocol: "SOCKET", Duration: time.Since(startSio), StatusString: "ASYNC", Error: err,
				})
				if err != nil {
					color.Yellow("⚠ Background Socket failed to connect: %v. Continuing...\n", err)
				} else {
					color.Green("✔ Background Socket is ready and listening continuously!\n")
				}
				cr.runScripts("test", req.Scripts, ctx, nil)
				continue
			} else {
				err := cr.sioExecutor.Execute(urlStr, headers, resolvedEvents, nil, nil)
				metrics = append(metrics, RequestMetric{
					Name: req.Name, Protocol: "SOCKET", Duration: time.Since(startSio), StatusString: "SYNC", Error: err,
				})
				if err != nil { fmt.Printf("Socket.IO Request %s failed: %v\n", req.Name, err) } else { fmt.Printf("Socket.IO Request %s successful\n", req.Name) }
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
			metrics = append(metrics, RequestMetric{Name: req.Name, Protocol: "HTTP", Error: err})
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
			metrics = append(metrics, RequestMetric{Name: req.Name, Protocol: "HTTP", Duration: totalTime, Error: err})
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		metrics = append(metrics, RequestMetric{
			Name: req.Name, Protocol: "HTTP", StatusCode: resp.StatusCode, StatusString: resp.Status, Duration: totalTime,
		})

		var dnsTime, tcpTime, tlsTime, ttfbTime, transferTime time.Duration
		if !dnsDone.IsZero() { dnsTime = dnsDone.Sub(dnsStart) }
		if !connDone.IsZero() { tcpTime = connDone.Sub(connStart) }
		if !tlsDone.IsZero() { tlsTime = tlsDone.Sub(tlsStart) }
		if !resStart.IsZero() {
			if !reqDone.IsZero() { ttfbTime = resStart.Sub(reqDone) } else { ttfbTime = resStart.Sub(t0) }
			transferTime = time.Since(resStart)
		}

		if cr.verboseMode {
			verboseColor.Println("-------------------- RESPONSE -------------------")
			verboseColor.Printf("< %s %s\n", resp.Proto, resp.Status)
			for key, values := range resp.Header {
				if isNoisyHeader(key) { continue }
				for _, value := range values { verboseColor.Printf("< %s: %s\n", key, value) }
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

	return metrics, nil
}

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