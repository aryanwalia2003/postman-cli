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
	"sync"
	"time"

	"reqx/internal/collection"
	"reqx/internal/http_executor"
	"reqx/internal/planner"
	"reqx/internal/scripting"

	"github.com/dop251/goja"
	"github.com/fatih/color"
)

func (cr *CollectionRunner) SetClearCookiesPerRequest(v bool) {
	cr.clearCookiesPerRequest = v
}

// Run executes all requests in the plan and returns per-request metrics.
// When plan.DAG is non-nil, execution is delegated to RunDAG which runs
// independent nodes in parallel. Otherwise the existing sequential path runs.
func (cr *CollectionRunner) Run(plan *planner.ExecutionPlan, ctx *RuntimeContext) ([]RequestMetric, error) {
	if plan.DAG != nil {
		return cr.RunDAG(plan, ctx)
	}
	return cr.runLinear(plan, ctx)
}

// runLinear is the original sequential execution path, extracted verbatim.
func (cr *CollectionRunner) runLinear(plan *planner.ExecutionPlan, ctx *RuntimeContext) ([]RequestMetric, error) {
	verboseColor := color.New(color.FgYellow)
	timingColor := color.New(color.FgHiCyan)

	metrics := make([]RequestMetric, 0, len(plan.Requests))
	
	// If it's a sub-node in a DAG, we DON'T wait here. 
	// The parent RunDAG will handle the final cleanup once ALL levels finish.
	if !plan.IsDAGNode {
		defer func() {
			if cr.verbosity >= VerbosityNormal {
				color.Cyan("\nCollection run finished. Waiting for background connections...\n")
			}
			ctx.AsyncStopOnce.Do(func() { close(ctx.AsyncStop) })
			ctx.AsyncWG.Wait()
			if cr.verbosity >= VerbosityNormal {
				color.Green("All connections closed cleanly.\n")
			}
		}()
	}

	for reqIdx, req := range plan.Requests {
		if cr.verbosity >= VerbosityNormal {
			fmt.Printf("\n[RUN] Running request: %s\n", req.Name)
		}

		cr.runScripts("prerequest", req.Scripts, ctx, nil, plan, reqIdx)
		urlStr := cr.replaceVars(req.URL, ctx)

		if strings.ToUpper(req.Protocol) == "WS" {
			metrics = cr.runWebSocket(req, urlStr, ctx, ctx.AsyncWG, metrics, ctx.AsyncStop, plan, reqIdx)
			continue
		}

		if strings.ToUpper(req.Protocol) == "SOCKETIO" {
			metrics = cr.runSocketIO(req, urlStr, ctx, ctx.AsyncWG, metrics, ctx.AsyncStop, plan, reqIdx)
			continue
		}

		// ── HTTP ─────────────────────────────────────────────────────────────
		reqBody := cr.replaceVars(req.Body, ctx)
		var bodyReader io.Reader
		if reqBody != "" {
			bodyReader = bytes.NewBufferString(reqBody)
		}

		httpReq, err := http.NewRequest(strings.ToUpper(req.Method), urlStr, bodyReader)
		if err != nil {
			fmt.Printf("Error building request: %v\n", err)
			metrics = append(metrics, RequestMetric{Name: req.Name, Protocol: "HTTP", Error: err})
			continue
		}

		for k, v := range req.Headers {
			httpReq.Header.Set(k, cr.replaceVars(v, ctx))
		}
		http_executor.ApplyAuth(httpReq, cr.resolveAuth(req.Auth, plan.CollectionAuth, ctx))

		if cr.verbosity >= VerbosityFull {
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
		var reused bool
		trace := &httptrace.ClientTrace{
			GotConn:              func(info httptrace.GotConnInfo) { reused = info.Reused },
			DNSStart:             func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
			DNSDone:              func(_ httptrace.DNSDoneInfo) { dnsDone = time.Now() },
			ConnectStart:         func(_, _ string) { connStart = time.Now() },
			ConnectDone:          func(_, _ string, _ error) { connDone = time.Now() },
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
			if cr.verbosity >= VerbosityNormal {
				fmt.Printf("Request Failed: %v\n", err)
			}
			metrics = append(metrics, RequestMetric{
				Name: req.Name, Protocol: "HTTP",
				Duration: totalTime, Error: err, ErrorMsg: err.Error(),
				BytesSent: int64(len(reqBody)),
			})
			continue
		}

		buf := acquireBodyBuf()
		bytesReceived, _ := io.Copy(buf, resp.Body)
		resp.Body.Close()
		bodyBytes := buf.Bytes()

		var ttfb time.Duration
		if !resStart.IsZero() {
			if !reqDone.IsZero() {
				ttfb = resStart.Sub(reqDone)
			} else {
				ttfb = resStart.Sub(t0)
			}
		}

		m := RequestMetric{
			Name:          req.Name,
			Protocol:      "HTTP",
			StatusCode:    resp.StatusCode,
			StatusString:  resp.Status,
			Duration:      totalTime,
			BytesSent:     int64(len(reqBody)),
			BytesReceived: bytesReceived,
			TTFB:          ttfb,
		}
		if resp.StatusCode >= 400 {
			m.ErrorMsg = resp.Status
		}
		metrics = append(metrics, m)

		if cr.verbosity >= VerbosityFull {
			var dnsTime, tcpTime, tlsTime, transferTime time.Duration
			if !dnsDone.IsZero() {
				dnsTime = dnsDone.Sub(dnsStart)
			}
			if !connDone.IsZero() {
				tcpTime = connDone.Sub(connStart)
			}
			if !tlsDone.IsZero() {
				tlsTime = tlsDone.Sub(tlsStart)
			}
			if !resStart.IsZero() {
				transferTime = time.Since(resStart)
			}

			verboseColor.Println("-------------------- RESPONSE -------------------")
			verboseColor.Printf("< %s %s\n", resp.Proto, resp.Status)
			for key, values := range resp.Header {
				if isNoisyHeader(key) {
					continue
				}
				for _, value := range values {
					verboseColor.Printf("< %s: %s\n", key, value)
				}
			}
			verboseColor.Println("<")
			if len(bodyBytes) > 0 {
				if strings.Contains(resp.Header.Get("Content-Type"), "json") {
					prettyBuf := acquireBodyBuf()
					if err := json.Indent(prettyBuf, bodyBytes, "", "  "); err == nil {
						fmt.Println(prettyBuf.String())
					} else {
						fmt.Println(string(bodyBytes))
					}
					releaseBodyBuf(prettyBuf)
				} else {
					fmt.Println(string(bodyBytes))
				}
			}
			timingColor.Println("-------------------- TIMINGS --------------------")
			if reused {
				timingColor.Printf("  Connection     : Reused (Pooled)\n")
			} else {
				timingColor.Printf("  DNS Lookup     : %v\n", roundMs(dnsTime))
				timingColor.Printf("  TCP Connection : %v\n", roundMs(tcpTime))
				timingColor.Printf("  TLS Handshake  : %v\n", roundMs(tlsTime))
			}
			timingColor.Printf("  Server (TTFB)  : %v\n", roundMs(ttfb))
			timingColor.Printf("  Data Transfer  : %v\n", roundMs(transferTime))
			timingColor.Printf("  Bytes Received : %d B\n", bytesReceived)
			timingColor.Printf("  --------------------------\n")
			timingColor.Printf("  Total Time     : %v\n", roundMs(totalTime))
			timingColor.Println("-------------------------------------------------")
		}
		if cr.verbosity >= VerbosityNormal {
			statusColor := color.New(color.FgHiGreen).SprintfFunc()
			if resp.StatusCode >= 400 {
				statusColor = color.New(color.FgHiRed).SprintfFunc()
			}
			fmt.Printf("Status: %s  |  Time: %v  |  %d B received\n",
				statusColor(resp.Status), roundMs(totalTime), bytesReceived)
		}

		bodyString := string(bodyBytes)
		releaseBodyBuf(buf)
		scriptResp := &scripting.ResponseAPI{
			BodyString: bodyString,
			Headers:    &scripting.ResponseHeaders{Headers: make(map[string]string)},
		}
		for k, v := range resp.Header {
			if len(v) > 0 {
				scriptResp.Headers.Headers[k] = v[0]
			}
		}
		cr.runScripts("test", req.Scripts, ctx, scriptResp, plan, reqIdx)
	}

	return metrics, nil
}

// ── WebSocket helper ──────────────────────────────────────────────────────────

func (cr *CollectionRunner) runWebSocket(
	req collection.Request, urlStr string,
	ctx *RuntimeContext, wg *sync.WaitGroup,
	metrics []RequestMetric, stop chan struct{},
	plan *planner.ExecutionPlan, reqIdx int,
) []RequestMetric {
	headers := cr.resolvedHeaders(req.Headers, ctx)
	var events []collection.WebSocketEvent
	for _, ev := range req.WSEvents {
		events = append(events, collection.WebSocketEvent{Type: ev.Type, Payload: cr.replaceVars(ev.Payload, ctx)})
	}
	start := time.Now()
	if req.Async {
		readyChan := make(chan error, 1)
		wg.Add(1)
		go func(name, url string, hdrs map[string]string, evs []collection.WebSocketEvent) {
			defer wg.Done()
			if err := cr.weExecutor.Execute(url, hdrs, evs, readyChan, stop); err != nil {
				color.Red("\n[BACKGROUND ERROR] WS '%s': %v\n", name, err)
			}
		}(req.Name, urlStr, headers, events)
		color.Cyan("Waiting for WebSocket to connect...\n")
		err := <-readyChan
		metrics = append(metrics, RequestMetric{Name: req.Name, Protocol: "WS", Duration: time.Since(start), StatusString: "ASYNC", Error: err})
		if err != nil {
			color.Yellow("⚠ Background WS failed: %v. Continuing...\n", err)
		} else {
			color.Green("Background WS ready!\n")
		}
		cr.runScripts("test", req.Scripts, ctx, nil, plan, reqIdx)
		return metrics
	}
	err := cr.weExecutor.Execute(urlStr, headers, events, nil, nil)
	metrics = append(metrics, RequestMetric{Name: req.Name, Protocol: "WS", Duration: time.Since(start), StatusString: "SYNC", Error: err})
	if err != nil {
		fmt.Printf("WS request %s failed: %v\n", req.Name, err)
	}
	cr.runScripts("test", req.Scripts, ctx, nil, plan, reqIdx)
	return metrics
}

// ── Socket.IO helper ──────────────────────────────────────────────────────────

func (cr *CollectionRunner) runSocketIO(
	req collection.Request, urlStr string,
	ctx *RuntimeContext, wg *sync.WaitGroup,
	metrics []RequestMetric, stop chan struct{},
	plan *planner.ExecutionPlan, reqIdx int,
) []RequestMetric {
	headers := cr.resolvedHeaders(req.Headers, ctx)
	var events []collection.SocketIOEvent
	for _, ev := range req.Events {
		events = append(events, collection.SocketIOEvent{Type: ev.Type, Name: cr.replaceVars(ev.Name, ctx), Payload: cr.replaceVars(ev.Payload, ctx)})
	}
	start := time.Now()
	if req.Async {
		readyChan := make(chan error, 1)
		wg.Add(1)
		go func(name, url string, hdrs map[string]string, evs []collection.SocketIOEvent) {
			defer wg.Done()
			if err := cr.sioExecutor.Execute(url, hdrs, evs, readyChan, stop); err != nil {
				color.Red("\n[BACKGROUND ERROR] SIO '%s': %v\n", name, err)
			}
		}(req.Name, urlStr, headers, events)
		color.Cyan("Waiting for Socket.IO to connect...\n")
		err := <-readyChan
		metrics = append(metrics, RequestMetric{Name: req.Name, Protocol: "SOCKET", Duration: time.Since(start), StatusString: "ASYNC", Error: err})
		if err != nil {
			color.Yellow("⚠ Background Socket failed: %v. Continuing...\n", err)
		} else {
			color.Green("Background Socket ready!\n")
		}
		cr.runScripts("test", req.Scripts, ctx, nil, plan, reqIdx)
		return metrics
	}
	err := cr.sioExecutor.Execute(urlStr, headers, events, nil, nil)
	metrics = append(metrics, RequestMetric{Name: req.Name, Protocol: "SOCKET", Duration: time.Since(start), StatusString: "SYNC", Error: err})
	if err != nil {
		fmt.Printf("SIO request %s failed: %v\n", req.Name, err)
	} else {
		fmt.Printf("SIO request %s OK\n", req.Name)
	}
	cr.runScripts("test", req.Scripts, ctx, nil, plan, reqIdx)
	return metrics
}

// ── Shared helpers ────────────────────────────────────────────────────────────

func (cr *CollectionRunner) resolveAuth(reqAuth, collAuth *collection.Auth, ctx *RuntimeContext) *collection.Auth {
	src := reqAuth
	if src == nil {
		src = collAuth
	}
	if src == nil {
		return nil
	}
	resolved := &collection.Auth{
		Type: src.Type, Token: cr.replaceVars(src.Token, ctx),
		Username: cr.replaceVars(src.Username, ctx), Password: cr.replaceVars(src.Password, ctx),
		Key: cr.replaceVars(src.Key, ctx), Value: cr.replaceVars(src.Value, ctx), In: src.In,
	}
	if src.Cookies != nil {
		resolved.Cookies = make(map[string]string, len(src.Cookies))
		for k, v := range src.Cookies {
			resolved.Cookies[k] = cr.replaceVars(v, ctx)
		}
	}
	return resolved
}

func (cr *CollectionRunner) runScripts(
	scriptType string,
	scripts []collection.Script,
	ctx *RuntimeContext,
	resp *scripting.ResponseAPI,
	plan *planner.ExecutionPlan,
	reqIdx int,
) {
	for _, s := range scripts {
		if s.Type == scriptType {
			var compiled *goja.Program
			if plan != nil && plan.CompiledScripts != nil {
				key := planner.ScriptKey{RequestIndex: reqIdx, ScriptType: scriptType}
				compiled = plan.CompiledScripts[key]
			}
			if err := cr.scriptRunner.Execute(&s, ctx.Environment, resp, compiled); err != nil {
				fmt.Printf("Warning: script execution failed: %v\n", err)
			}
		}
	}
}

func (cr *CollectionRunner) replaceVars(input string, ctx *RuntimeContext) string {
	if ctx == nil || ctx.Environment == nil {
		return input
	}
	return replaceVarsFast(input, ctx.Environment.Variables)
}

func (cr *CollectionRunner) resolvedHeaders(raw map[string]string, ctx *RuntimeContext) map[string]string {
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = cr.replaceVars(v, ctx)
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
	if d == 0 {
		return "0ms"
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}