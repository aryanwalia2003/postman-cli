package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"reqx/internal/collection"
	"reqx/internal/errs"
	"reqx/internal/http_executor"
	"reqx/internal/runner"
	"reqx/internal/storage"
)

func NewRunCmd() *cobra.Command {
	var envFilePath string
	var noCookies, clearCookies, verbose bool
	var requestFilter string
	var iterations int // <-- NEW: Iterations flag variable

	// NEW: Variables for Temporary Request Injection
	var injIndex string
	var injName, injMethod, injURL, injBody string
	var injHeaders []string

	c := &cobra.Command{
		Use:   "run [collection.json]",
		Short: "Execute a collection of requests",
		Long: `🏃 Parse and execute a .json collection file sequentially.
The 'run' command is the heart of ReqX. It handles variable replacement, 
cookie persistence, pre-request scripts, and test assertions.

🛠 Advanced Flow Control:
1. Multi-Iteration (-n): Run the entire collection multiple times for load testing.
2. Filtering (-f): Execute only requests whose names match a specific substring.
3. Injection: Temporarily insert a brand-new request (like a one-time auth setup) 
   at a specific position without modifying your source collection file.`,
		Example: `  # Standard execution with environment
  reqx run my-collection.json -e dev-env.json
  
  # Load Testing: Run 20 iterations and view aggregated stats
  reqx run my-collection.json -n 20
  
  # Targeted Testing: Run only requests with "User" in the name
  reqx run my-collection.json -f "User"
  
  # Debugging: Verbose output showing full request and response bodies
  reqx run my-collection.json -v
  
  # Custom Injection: Add a setup request at the very beginning (index 1)
  reqx run my-api.json --inject-index 1 --inject-name "Auth Setup" --inject-url "http://api.com/auth"
  
  # Stateless: Disable cookie persistence for a clean run
  reqx run my-api.json --no-cookies`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			collectionPath := args[0]
			
			if iterations < 1 {
				iterations = 1
			}

			// This slice will hold ALL metrics from ALL iterations
			allMetrics := make([][]runner.RequestMetric, 0, iterations)
			totalStartTime := time.Now()

			// =========================================================
			// ▼▼▼ NEW: ITERATION LOOP STARTS HERE (OUTERMOST) ▼▼▼
			// =========================================================
			for i := 1; i <= iterations; i++ {
				if iterations > 1 {
					iterationHeader := fmt.Sprintf("  Iteration %d / %d  ", i, iterations)
					padding := strings.Repeat("=", (70-len(iterationHeader))/2)
					fmt.Printf("\n%s%s%s\n", padding, iterationHeader, padding)
				}
				
				// All logic below this is now inside the iteration loop,
				// ensuring a clean state for every run.

				// 1. Load Collection from File
				collBytes, err := storage.ReadJSONFile(collectionPath)
				if err != nil {
					return errs.Wrap(err, errs.KindInvalidInput, "could not read collection file")
				}

				coll, err := storage.ParseCollection(collBytes)
				if err != nil {
					return errs.Wrap(err, errs.KindInvalidInput, "could not parse collection JSON")
				}

				// Injection Logic
				if injIndex != "" && injName != "" && injURL != "" {
					idx, err := strconv.Atoi(injIndex)
					if err != nil || idx < 1 { return errs.InvalidInput("Invalid --inject-index.") }
					insertPos := idx - 1
					if insertPos > len(coll.Requests) { insertPos = len(coll.Requests) }
					headerMap := make(map[string]string)
					for _, h := range injHeaders {
						parts := strings.SplitN(h, ":", 2)
						if len(parts) == 2 {
							headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
						}
					}
					tempReq := collection.Request{
						Name:    color.New(color.FgHiMagenta).Sprintf("[INJECTED] %s", injName),
						Method:  strings.ToUpper(injMethod),
						URL:     injURL,
						Headers: headerMap,
						Body:    injBody,
					}
					color.Magenta("💉 Injecting temporary request '%s' at position %d...\n", injName, idx)
					if insertPos == len(coll.Requests) {
						coll.Requests = append(coll.Requests, tempReq)
					} else {
						coll.Requests = append(coll.Requests[:insertPos+1], coll.Requests[insertPos:]...)
						coll.Requests[insertPos] = tempReq
					}
				} else if (injName != "" || injURL != "") && injIndex == "" {
					color.Yellow("⚠ Warning: Ignored temporary request injection. Missing --inject-index.\n")
				}

				// Filtering Logic
				if requestFilter != "" {
					filtered := []collection.Request{}
					for _, r := range coll.Requests {
						if strings.Contains(strings.ToLower(r.Name), strings.ToLower(requestFilter)) {
							filtered = append(filtered, r)
						}
					}
					if len(filtered) == 0 {
						color.Yellow("⚠ No requests found matching filter: %s", requestFilter)
						continue // Skip this iteration if filter matches nothing
					}
					coll.Requests = filtered
					color.Cyan("🔍 Filtered collection to %d request(s) matching '%s'\n", len(filtered), requestFilter)
				}

				// A fresh context for each iteration is crucial!
				ctx := runner.NewRuntimeContext()

				// Load Environment
				if envFilePath != "" {
					envBytes, err := storage.ReadJSONFile(envFilePath)
					if err != nil {
						return errs.Wrap(err, errs.KindInvalidInput, "could not read environment file")
					}
					env, err := storage.ParseEnvironment(envBytes)
					if err != nil {
						return errs.Wrap(err, errs.KindInvalidInput, "could not parse environment JSON")
					}
					ctx.SetEnvironment(env)
				}

				// Build executor
				exec := http_executor.NewDefaultExecutor()
				if noCookies {
					exec.DisableCookies()
				}

				// Run Collection for this iteration
				engine := runner.NewCollectionRunner(exec, nil, nil)
				if clearCookies {
					engine.SetClearCookiesPerRequest(true)
				}
				if verbose {
					engine.SetVerbose(true)
				}

				runMetrics, err := engine.Run(coll, ctx)
				if err != nil {
					color.Red("Iteration %d failed with error: %v\n", i, err)
					// We continue to the next iteration even on failure
				}

				// Add this iteration's metrics to the master list
				allMetrics = append(allMetrics, runMetrics)

				// Add a small delay between iterations
				if i < iterations {
					fmt.Println("\nWaiting 1 second before next iteration...")
					time.Sleep(1 * time.Second)
				}
			} // <-- ITERATION LOOP ENDS HERE

			// ==========================================
			// NEW: Print the Final Aggregated Summary
			// ==========================================
			if iterations > 1 {
				printAggregatedSummary(allMetrics, time.Since(totalStartTime))
			} else if len(allMetrics) > 0 {
				// If only one iteration, print the simple summary
				printSimpleSummary(allMetrics[0], time.Since(totalStartTime))
			}

			return nil
		},
	}

	// Standard Flags
	c.Flags().IntVarP(&iterations, "iterations", "n", 1, "Number of times to run the collection") // <-- NEW FLAG
	c.Flags().StringVarP(&envFilePath, "env", "e", "", "Path to the environment JSON file")
	c.Flags().BoolVar(&noCookies, "no-cookies", false, "Disable cookie persistence for this run")
	c.Flags().BoolVar(&clearCookies, "clear-cookies", false, "Clear cookie jar before each request")
	c.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output to see full request and response")
	c.Flags().StringVarP(&requestFilter, "request", "f", "", "Only run requests matching this name (substring match)")

	// Injection Flags
	c.Flags().StringVar(&injIndex, "inject-index", "", "Position (1-based) to temporarily insert a new request")
	c.Flags().StringVar(&injName, "inject-name", "", "Name of the temporary request")
	c.Flags().StringVar(&injMethod, "inject-method", "GET", "HTTP method for temporary request")
	c.Flags().StringVar(&injURL, "inject-url", "", "URL for temporary request")
	c.Flags().StringVar(&injBody, "inject-data", "", "Body payload for temporary request")
	c.Flags().StringSliceVar(&injHeaders, "inject-header", []string{}, "Header for temporary request (e.g., 'Key: Value')")

	return c
}

// =========================================================
// ▼▼▼ NEW HELPER FUNCTIONS ▼▼▼
// =========================================================

// printSimpleSummary prints a summary for a single run.
func printSimpleSummary(metrics []runner.RequestMetric, totalTime time.Duration) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	color.New(color.FgHiCyan, color.Bold).Println("  EXECUTION SUMMARY")
	fmt.Println(strings.Repeat("=", 70))

	var totalHttp, successHttp, failedHttp int
	var maxTime, minTime, totalHttpTime time.Duration
	var slowestReq string
	minTime = time.Hour

	for i, m := range metrics {
		statusCol := color.New(color.FgHiGreen).SprintFunc()
		if m.Error != nil || (m.StatusCode != 0 && m.StatusCode >= 400) {
			statusCol = color.New(color.FgHiRed).SprintFunc()
		}

		if m.Protocol == "SOCKET" {
			statusTxt := "OK"
			if m.Error != nil { statusTxt = "ERR" }
			fmt.Printf("  [%2d] %-8s %-20s %s\n", i+1, color.BlueString("SOCKET"), statusCol(statusTxt), m.Name)
		} else {
			totalHttp++
			totalHttpTime += m.Duration
			if m.Error != nil || m.StatusCode >= 400 { failedHttp++ } else { successHttp++ }
			if m.Duration > maxTime { maxTime = m.Duration; slowestReq = m.Name }
			if m.Duration < minTime && m.Duration > 0 { minTime = m.Duration }
			statusTxt := m.StatusString
			if m.Error != nil { statusTxt = "ERR" }
			fmt.Printf("  [%2d] %-8s %-20s %-8s %s\n", i+1, "HTTP", statusCol(statusTxt), m.Duration.Round(time.Millisecond).String(), m.Name)
		}
	}

	fmt.Println(strings.Repeat("-", 70))
	if totalHttp > 0 {
		avgTime := totalHttpTime / time.Duration(totalHttp)
		fmt.Printf("  HTTP Requests : %d Total | %s | %s\n", totalHttp, color.GreenString("%d Success", successHttp), color.RedString("%d Failed", failedHttp))
		fmt.Printf("  Avg Latency   : %s\n", color.CyanString(avgTime.Round(time.Millisecond).String()))
		fmt.Printf("  Min Latency   : %s\n", minTime.Round(time.Millisecond).String())
		fmt.Printf("  Max Latency   : %s (%s)\n", color.YellowString(maxTime.Round(time.Millisecond).String()), slowestReq)
	}
	fmt.Printf("  Total Run Time: %v\n", totalTime.Round(time.Millisecond))
	fmt.Println(strings.Repeat("=", 70))
}

// printAggregatedSummary prints stats across multiple iterations.
func printAggregatedSummary(allMetrics [][]runner.RequestMetric, totalTime time.Duration) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	color.New(color.FgHiCyan, color.Bold).Println("  AGGREGATED SUMMARY")
	fmt.Println(strings.Repeat("=", 70))

	totalRuns := len(allMetrics)
	var totalReqs, totalSuccess, totalFailed int
	var totalLatency time.Duration

	for _, runMetrics := range allMetrics {
		for _, m := range runMetrics {
			if m.Protocol == "HTTP" || m.Protocol == "" { 
				totalReqs++
				totalLatency += m.Duration
				if m.Error != nil || m.StatusCode >= 400 {
					totalFailed++
				} else {
					totalSuccess++
				}
			}
		}
	}

	var avgLatency time.Duration
	if totalReqs > 0 {
		avgLatency = totalLatency / time.Duration(totalReqs)
	}
	
	fmt.Printf("  Iterations    : %d\n", totalRuns)
	fmt.Printf("  HTTP Requests : %d Total (%d per iteration)\n", totalReqs, totalReqs/totalRuns)
	if totalReqs > 0 {
		fmt.Printf("  Success Rate  : %.2f%% (%s / %s)\n", 
			float64(totalSuccess)/float64(totalReqs)*100,
			color.GreenString("%d", totalSuccess),
			color.RedString("%d", totalFailed))
	}
	fmt.Printf("  Avg Latency   : %s\n", color.CyanString(avgLatency.Round(time.Millisecond).String()))
	fmt.Printf("  Total Run Time: %v\n", totalTime.Round(time.Millisecond))
	fmt.Println(strings.Repeat("=", 70))
}