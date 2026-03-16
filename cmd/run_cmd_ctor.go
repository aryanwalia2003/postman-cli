package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"reqx/internal/collection"
	"reqx/internal/environment"
	"reqx/internal/errs"
	"reqx/internal/http_executor"
	"reqx/internal/metrics"
	"reqx/internal/progress"
	"reqx/internal/runner"
	"reqx/internal/storage"
)

func NewRunCmd() *cobra.Command {
	var envFilePath string
	var noCookies, clearCookies, verbose bool
	var quiet bool
	var requestFilters []string
	var iterations int // <-- NEW: Iterations flag variable
	var workers int    // <-- NEW: Workers flag variable
	var exportPath string
	var duration time.Duration
	var rps float64
	var stages string

	// NEW: Variables for Temporary Request Injection
	var injIndex string
	var injName, injMethod, injURL, injBody string
	var injHeaders []string

	c := &cobra.Command{
		Use:   "run [collection.json]",
		Short: "Execute a collection of requests",
		Long: `🏃 Parse and execute a .json collection file with professional load testing capabilities.
The 'run' command handles variable replacement, cookie persistence, and test assertions.

🛠 Advanced Flow Control:
1. Multi-Iteration (-n): Run the entire collection multiple times.
2. Duration-based (-d): Run workers continuously for a set time (e.g. 1m, 5m).
3. Ramping Stages (--stages): Simulate real-world traffic with ramp-up/down.
4. Arrival Rate (--rps): Cap the maximum requests sent per second.
5. Filtering (-f): Execute only requests whose names match a specific substring.
6. Injection: Temporarily insert a brand-new request anywhere in the collection.`,
		Example: `  # Standard execution with environment
  reqx run my-collection.json -e dev-env.json
  
  # Load Testing: Run for 1 minute with 50 concurrent users
  reqx run my-collection.json -d 1m -c 50 -q
  
  # Ramping Test: Ramp up to 20 users, sustain, then ramp down
  reqx run my-collection.json --stages "10s:5,30s:20,10s:0" -q
  
  # Rate Limited: Run with 20 workers but cap at 10 req/s
  reqx run my-collection.json -d 2m -c 20 --rps 10
  
  # Export Results: Save raw metrics for analysis
  reqx run my-collection.json -n 100 --export results.json
  
  # Custom Injection: Add a setup request at the very beginning (index 1)
  reqx run my-api.json --inject-index 1 --inject-name "Auth Setup" --inject-url "http://api.com/auth"
  
  # Stateless: Disable cookie persistence
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

			// 1. Load Collection from File (ONCE for all workers/iterations)
			collBytes, err := storage.ReadJSONFile(collectionPath)
			if err != nil {
				return errs.Wrap(err, errs.KindInvalidInput, "could not read collection file")
			}

			coll, err := storage.ParseCollection(collBytes)
			if err != nil {
				return errs.Wrap(err, errs.KindInvalidInput, "could not parse collection JSON")
			}

			verbosityLevel := runner.VerbosityNormal
			if quiet {
				verbosityLevel = runner.VerbosityQuiet
			} else if verbose {
				verbosityLevel = runner.VerbosityFull
			}

			// ── Phase 3: Scheduler path ──────────────────────────────────────
			// Triggered by any of: --duration, --rps, --stages.
			// Overrides -n / -c entirely.
			if duration > 0 || rps > 0 || stages != "" {
				var parsedStages []runner.Stage
				if stages != "" {
					var parseErr error
					parsedStages, parseErr = runner.ParseStages(stages)
					if parseErr != nil {
						return parseErr
					}
				}

				// Load environment once; Scheduler clones it per worker.
				var baseEnv *environment.Environment
				if envFilePath != "" {
					envBytes, err := storage.ReadJSONFile(envFilePath)
					if err != nil {
						return errs.Wrap(err, errs.KindInvalidInput, "could not read environment file")
					}
					baseEnv, err = storage.ParseEnvironment(envBytes)
					if err != nil {
						return errs.Wrap(err, errs.KindInvalidInput, "could not parse environment JSON")
					}
				}

				cfg := runner.SchedulerConfig{
					Coll:         coll,
					BaseEnv:      baseEnv,
					NoCookies:    noCookies,
					ClearCookies: clearCookies,
					Verbosity:    verbosityLevel,
					Stages:       parsedStages,
					Duration:     duration,
					MaxWorkers:   workers, // -c still sets the fixed concurrency in duration mode
					RPS:          rps,
				}

				printPhase3Header(cfg)

				sched := runner.NewScheduler(cfg)

				// Progress bar: unbounded for duration mode, bounded for stages (if total known).
				bar := progress.NewBar(0, workers)
				bar.Start()

				totalStart := time.Now()
				allResults := sched.Run()
				bar.Stop()

				// Flatten into [][]RequestMetric (one slice per completed iteration).
				allMetrics := make([][]runner.RequestMetric, 0, len(allResults))
				for _, r := range allResults {
					if r.Metrics != nil {
						allMetrics = append(allMetrics, r.Metrics)
					}
				}

				report := metrics.Analyze(allMetrics, time.Since(totalStart))
				metrics.PrintReport(report)

				if exportPath != "" {
					if err := metrics.ExportJSON(allMetrics, exportPath); err != nil {
						color.Red("⚠ Export failed: %v\n", err)
					} else {
						color.Cyan("📄 Results exported to: %s\n", exportPath)
					}
				}
				return nil
			}

			// =========================================================
			// ▼▼▼ NEW: PARALLEL DISPATCH (LOAD TESTING) ▼▼▼
			// =========================================================

			if workers > 1 {
				cfg := runner.WorkerConfig{
					Coll:         coll,
					BaseEnv:      nil, // set below if env file provided
					NoCookies:    noCookies,
					ClearCookies: clearCookies,
					Verbosity:    verbosityLevel,
				}

				if envFilePath != "" {
					envBytes, err := storage.ReadJSONFile(envFilePath)
					if err != nil {
						return errs.Wrap(err, errs.KindInvalidInput, "could not read environment file")
					}
					env, err := storage.ParseEnvironment(envBytes)
					if err != nil {
						return errs.Wrap(err, errs.KindInvalidInput, "could not parse environment JSON")
					}
					cfg.BaseEnv = env
				}

				pool := runner.NewWorkerPool(workers)
				color.Cyan("🚀 Starting load test: %d iterations across %d workers\n", iterations, workers)
				results := pool.Run(cfg, iterations)

				// Flatten results into allMetrics (order by iteration index)
				sort.Slice(results, func(i, j int) bool {
					return results[i].IterationIndex < results[j].IterationIndex
				})
				for _, r := range results {
					if r.Err != nil {
						color.Red("Iteration %d failed: %v\n", r.IterationIndex, r.Err)
					}
					allMetrics = append(allMetrics, r.Metrics)
				}

				report := metrics.Analyze(allMetrics, time.Since(totalStartTime))
				metrics.PrintReport(report)
				if exportPath != "" {
					if err := metrics.ExportJSON(allMetrics, exportPath); err != nil {
						color.Red("⚠ Export failed: %v\n", err)
					} else {
						color.Cyan("📄 Results exported to: %s\n", exportPath)
					}
				}
				return nil
			}

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
				if len(requestFilters) > 0 {
					filtered := []collection.Request{}
					for _, r := range coll.Requests {
						matched := false
						for _, f := range requestFilters {
							if strings.Contains(strings.ToLower(r.Name), strings.ToLower(f)) {
								matched = true
								break
							}
						}
						if matched {
							filtered = append(filtered, r)
						}
					}
					if len(filtered) == 0 {
						color.Yellow("⚠ No requests found matching filters: %v", requestFilters)
						continue // Skip this iteration if filter matches nothing
					}
					coll.Requests = filtered
					color.Cyan("🔍 Filtered collection to %d request(s) matching %v\n", len(filtered), requestFilters)
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
				engine := runner.NewCollectionRunner(exec, nil, nil, nil)
				engine.SetVerbosity(verbosityLevel)
				if clearCookies {
					engine.SetClearCookiesPerRequest(true)
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
				report := metrics.Analyze(allMetrics, time.Since(totalStartTime))
				metrics.PrintReport(report)
			} else if len(allMetrics) > 0 {
				report := metrics.Analyze(allMetrics, time.Since(totalStartTime))
				metrics.PrintReport(report)
			}

			return nil
		},
	}

	// Standard Flags
	c.Flags().IntVarP(&iterations, "iterations", "n", 1, "Number of times to run the collection") // <-- NEW FLAG
	c.Flags().IntVarP(&workers, "workers", "c", 1, "Number of parallel workers (virtual users)")
	c.Flags().StringVarP(&envFilePath, "env", "e", "", "Path to the environment JSON file")
	c.Flags().BoolVar(&noCookies, "no-cookies", false, "Disable cookie persistence for this run")
	c.Flags().BoolVar(&clearCookies, "clear-cookies", false, "Clear cookie jar before each request")
	c.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output to see full request and response")
	c.Flags().StringSliceVarP(&requestFilters, "request", "f", []string{}, "Only run requests matching these names (multiple flags or comma-separated supported)")
	c.Flags().BoolVarP(&quiet, "quiet", "q", false,
		"Suppress per-request logs; show real-time progress bar instead")
	c.Flags().StringVar(&exportPath, "export", "",
		"Path to export raw request metrics as newline-delimited JSON (e.g. results.json)")
	c.Flags().DurationVarP(&duration, "duration", "d", 0,
		"Run duration (e.g. 30s, 2m). Workers run until time elapses.")
	c.Flags().Float64Var(&rps, "rps", 0,
		"Max requests per second to inject (0 = unlimited). Works with -d or --stages.")
	c.Flags().StringVar(&stages, "stages", "",
		`Ramp plan as "<dur>:<workers>" segments, e.g. "10s:5,30s:20,10s:0"`)

	// Injection Flags
	c.Flags().StringVar(&injIndex, "inject-index", "", "Position (1-based) to temporarily insert a new request")
	c.Flags().StringVar(&injName, "inject-name", "", "Name of the temporary request")
	c.Flags().StringVar(&injMethod, "inject-method", "GET", "HTTP method for temporary request")
	c.Flags().StringVar(&injURL, "inject-url", "", "URL for temporary request")
	c.Flags().StringVar(&injBody, "inject-data", "", "Body payload for temporary request")
	c.Flags().StringSliceVar(&injHeaders, "inject-header", []string{}, "Header for temporary request (e.g., 'Key: Value')")

	return c
}

func printPhase3Header(cfg runner.SchedulerConfig) {
	fmt.Println()
	if len(cfg.Stages) > 0 {
		color.Cyan("🎚  Stage-based load test — %d stages\n", len(cfg.Stages))
		for i, s := range cfg.Stages {
			color.Cyan("    Stage %d: %d workers for %v\n", i+1, s.TargetWorkers, s.Duration)
		}
	} else {
		color.Cyan("⏱  Duration-based load test — %v, %d workers", cfg.Duration, cfg.MaxWorkers)
	}
	if cfg.RPS > 0 {
		color.Cyan("  |  Rate: %.1f req/s", cfg.RPS)
	}
	fmt.Println()
}