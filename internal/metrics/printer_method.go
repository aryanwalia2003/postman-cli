package metrics

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
)

// PrintReport renders the full load test report to stdout.
func PrintReport(r Report) {
	sep  := strings.Repeat("═", 72)
	thin := strings.Repeat("─", 72)

	h := color.New(color.FgHiCyan, color.Bold)

	h.Printf("\n%s\n", sep)
	h.Println("  LOAD TEST REPORT")
	h.Printf("%s\n", sep)

	// ── Global overview ────────────────────────────────────────────────────────
	fmt.Printf("  Total Requests : %d\n", r.TotalRequests)
	successPct := color.GreenString("%.2f%%", r.SuccessRate)
	if r.SuccessRate < 95 {
		successPct = color.RedString("%.2f%%", r.SuccessRate)
	}
	fmt.Printf("  Success Rate   : %s  (%s passed / %s failed)\n",
		successPct,
		color.GreenString("%d", r.TotalSuccess),
		color.RedString("%d", r.TotalFailures),
	)
	fmt.Printf("  Throughput     : %s req/s\n", color.CyanString("%.2f", r.RPS))
	fmt.Printf("  Total Run Time : %v\n", r.TotalDuration.Round(1_000_000))

	// ── Bandwidth ─────────────────────────────────────────────────────────────
	h.Printf("\n  BANDWIDTH\n")
	h.Printf("  %s\n", thin[:40])
	fmt.Printf("  Sent           : %s\n", fmtBytes(r.TotalBytesSent))
	fmt.Printf("  Received       : %s\n", fmtBytes(r.TotalBytesReceived))
	fmt.Printf("  Network Speed  : %s\n", color.CyanString("%.3f MB/s", r.ThroughputMBps))

	// ── Global latency ────────────────────────────────────────────────────────
	h.Printf("\n  LATENCY  (all HTTP requests)\n")
	h.Printf("  %s\n", thin[:40])
	fmt.Printf("  %-8s %s\n", "Avg",  fmtDur(r.AvgLatency))
	fmt.Printf("  %-8s %s\n", "P50",  fmtDur(r.P50))
	fmt.Printf("  %-8s %s\n", "P90",  fmtDur(r.P90))
	fmt.Printf("  %-8s %s\n", color.YellowString("P95"),  color.YellowString(fmtDur(r.P95)))
	fmt.Printf("  %-8s %s\n", color.RedString("P99"), color.RedString(fmtDur(r.P99)))

	// ── Latency histogram sparkline ───────────────────────────────────────────
	h.Printf("\n  LATENCY DISTRIBUTION\n")
	h.Printf("  %s\n", thin[:40])
	printLatencyHistogram(r)

	// ── HTTP status code distribution ─────────────────────────────────────────
	if len(r.StatusCodes) > 0 {
		h.Printf("\n  HTTP STATUS DISTRIBUTION\n")
		h.Printf("  %s\n", thin[:40])
		printStatusDistribution(r)
	}

	// ── Per-request breakdown ─────────────────────────────────────────────────
	h.Printf("\n  PER-REQUEST BREAKDOWN\n")
	fmt.Printf("  %-30s %7s %7s %7s  %8s  %8s  %8s  %8s  %s\n",
		"Request", "Runs", "Pass", "Fail", "Avg", "P95", "TTFB P95", "Recv", "Top Error")
	h.Printf("  %s\n", thin)

	for _, s := range r.PerRequest {
		failCol := fmt.Sprintf("%7d", s.Failures)
		if s.Failures > 0 {
			failCol = color.RedString("%7d", s.Failures)
		}
		topErr := "-"
		if len(s.TopErrors) > 0 {
			e := s.TopErrors[0]
			msg := e.Message
			if len(msg) > 26 { msg = msg[:23] + "..." }
			topErr = fmt.Sprintf("%s ×%d", msg, e.Count)
		}
		fmt.Printf("  %-30s %7d %7d %s  %8s  %8s  %8s  %8s  %s\n",
			truncate(s.Name, 30),
			s.TotalRuns,
			s.Successes,
			failCol,
			fmtDur(s.AvgDuration),
			fmtDur(s.P95),
			fmtDur(s.P95TTFB),
			fmtBytes(s.BytesReceived),
			topErr,
		)
	}
	h.Printf("  %s\n\n", sep)
}

// ── Latency histogram sparkline ───────────────────────────────────────────────

// latencyBucket defines one bar in the histogram.
type latencyBucket struct {
	label string
	maxMs int64
}

var latencyBuckets = []latencyBucket{
	{"  0–50ms  ", 50},
	{" 50–100ms ", 100},
	{"100–250ms ", 250},
	{"250–500ms ", 500},
	{" 500ms–1s ", 1000},
	{"   1s–2s  ", 2000},
	{"   2s–5s  ", 5000},
	{"  5s–10s  ", 10000},
	{" 10s–30s  ", 30000},
	{"   30s+   ", 1<<62},
}

// printLatencyHistogram draws a horizontal bar chart of request latencies.
// Bucket counts are derived from the global HDR histogram percentiles.
func printLatencyHistogram(r Report) {
	// Compute bucket counts from raw per-request data via Report.PerRequest.
	// We accumulate by summing up distribution bars.
	counts := make([]int64, len(latencyBuckets))
	total  := int64(0)

	for _, s := range r.PerRequest {
		if s.Histogram == nil { continue }
		
		dist := s.Histogram.Distribution()
		for _, bar := range dist {
			// Find which bucket this bar belongs to
			for i, b := range latencyBuckets {
				if bar.From <= b.maxMs {
					counts[i] += bar.Count
					break
				}
			}
		}
		total += s.Histogram.TotalCount()
	}

	if total == 0 {
		fmt.Println("  (no data)")
		return
	}

	// Find max count for scaling
	maxCount := int64(1)
	for _, c := range counts {
		if c > maxCount { maxCount = c }
	}

	const barMax = 30
	for i, b := range latencyBuckets {
		c := counts[i]
		barLen := int(float64(c) / float64(maxCount) * barMax)
		pct := float64(c) / float64(total) * 100

		bar := strings.Repeat("█", barLen) + strings.Repeat("░", barMax-barLen)

		barColored := bar
		switch {
		case i <= 1: // ≤100ms: green
			barColored = color.GreenString(bar)
		case i <= 3: // ≤500ms: yellow
			barColored = color.YellowString(bar)
		default:     // >500ms: red
			barColored = color.RedString(bar)
		}
		fmt.Printf("  %s |%s| %6d  %5.1f%%\n", b.label, barColored, c, pct)
	}
}

// ── Status code distribution ──────────────────────────────────────────────────

func printStatusDistribution(r Report) {
	// Sort codes for deterministic output
	codes := make([]int, 0, len(r.StatusCodes))
	for code := range r.StatusCodes { codes = append(codes, code) }
	sort.Ints(codes)

	total := int64(r.TotalRequests)
	if total == 0 { total = 1 }

	// Find max count for bar scaling
	maxCount := 1
	for _, code := range codes {
		if r.StatusCodes[code] > maxCount { maxCount = r.StatusCodes[code] }
	}

	const barMax = 24
	for _, code := range codes {
		count := r.StatusCodes[code]
		pct   := float64(count) / float64(total) * 100
		barLen := int(float64(count) / float64(maxCount) * barMax)
		bar := strings.Repeat("█", barLen) + strings.Repeat("░", barMax-barLen)

		label := statusLabel(code)
		codeStr := fmt.Sprintf("%d", code)
		barColored := bar
		switch {
		case code < 300:
			codeStr    = color.GreenString("%d", code)
			barColored = color.GreenString(bar)
		case code < 400:
			codeStr    = color.CyanString("%d", code)
			barColored = color.CyanString(bar)
		case code < 500:
			codeStr    = color.YellowString("%d", code)
			barColored = color.YellowString(bar)
		default:
			codeStr    = color.RedString("%d", code)
			barColored = color.RedString(bar)
		}
		fmt.Printf("  %s %-22s |%s| %6d  %5.1f%%\n",
			codeStr, label, barColored, count, pct)
	}
}

func statusLabel(code int) string {
	labels := map[int]string{
		200: "OK",                    201: "Created",
		204: "No Content",           301: "Moved Permanently",
		302: "Found",                304: "Not Modified",
		400: "Bad Request",          401: "Unauthorized",
		403: "Forbidden",            404: "Not Found",
		408: "Request Timeout",      409: "Conflict",
		429: "Too Many Requests",    500: "Internal Server Error",
		502: "Bad Gateway",          503: "Service Unavailable",
		504: "Gateway Timeout",
	}
	if l, ok := labels[code]; ok { return l }
	return ""
}

// ── Formatting helpers ────────────────────────────────────────────────────────

func fmtDur(d interface{ Milliseconds() int64 }) string {
	ms := d.Milliseconds()
	if ms >= 1000 {
		return fmt.Sprintf("%.2fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%dms", ms)
}

// fmtBytes formats a byte count as B, KB, or MB.
func fmtBytes(b int64) string {
	switch {
	case b >= 1_000_000:
		return fmt.Sprintf("%.2f MB", float64(b)/1_000_000)
	case b >= 1_000:
		return fmt.Sprintf("%.1f KB", float64(b)/1_000)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max { return s }
	return s[:max-1] + "…"
}