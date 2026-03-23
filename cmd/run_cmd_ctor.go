package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"reqx/internal/environment"
	"reqx/internal/errs"
	"reqx/internal/history"
	"reqx/internal/http_executor"
	"reqx/internal/metrics"
	"reqx/internal/personas"
	"reqx/internal/planner"
	"reqx/internal/progress"
	"reqx/internal/runner"
	"reqx/internal/storage"
)

func NewRunCmd() *cobra.Command {
	var envFilePath string
	var noCookies, clearCookies, verbose, quiet bool
	var requestFilters []string
	var iterations, workers int
	var exportPath string
	var duration time.Duration
	var rps float64
	var stages, personasPath string

	var injIndex, injName, injMethod, injURL, injBody string
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
  reqx run my-collection.json -n 100 --export results.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if iterations < 1 {
				iterations = 1
			}

			totalStartTime := time.Now()

			collBytes, err := storage.ReadJSONFile(args[0])
			if err != nil {
				return errs.Wrap(err, errs.KindInvalidInput, "could not read collection file")
			}
			coll, err := storage.ParseCollection(collBytes)
			if err != nil {
				return errs.Wrap(err, errs.KindInvalidInput, "could not parse collection JSON")
			}

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

			var loadedPersonas []personas.Persona
			if personasPath != "" {
				loadedPersonas, err = personas.LoadCSV(personasPath)
				if err != nil {
					return err
				}
				if len(loadedPersonas) == 0 {
					color.Yellow("⚠ Personas CSV had no rows; continuing without personas.\n")
				}
			}

			plan, err := planner.BuildExecutionPlan(coll, planner.PlanConfig{
				RequestFilters: requestFilters,
				InjIndex:       injIndex,
				InjName:        injName,
				InjMethod:      injMethod,
				InjURL:         injURL,
				InjBody:        injBody,
				InjHeaders:     injHeaders,
			})
			if err != nil {
				return err
			}

			verbosityLevel := runner.VerbosityNormal
			if quiet {
				verbosityLevel = runner.VerbosityQuiet
			} else if verbose {
				verbosityLevel = runner.VerbosityFull
			}

			allMetrics := make([][]runner.RequestMetric, 0, iterations)

			// 3a. Scheduler (duration / RPS / stages)
			if duration > 0 || rps > 0 || stages != "" {
				var parsedStages []runner.Stage
				if stages != "" {
					parsedStages, err = runner.ParseStages(stages)
					if err != nil {
						return err
					}
				}

				cfg := runner.SchedulerConfig{
					Plan:         plan,
					BaseEnv:      baseEnv,
					NoCookies:    noCookies,
					ClearCookies: clearCookies,
					Verbosity:    verbosityLevel,
					Personas:     loadedPersonas,
					Stages:       parsedStages,
					Duration:     duration,
					MaxWorkers:   workers,
					RPS:          rps,
				}

				printPhase3Header(cfg)

				bar := progress.NewBar(0, workers)
				bar.Start()
				t0 := time.Now()
				allResults := runner.NewScheduler(cfg).Run()
				bar.Stop()

				for _, r := range allResults {
					if r.Metrics != nil {
						allMetrics = append(allMetrics, r.Metrics)
					}
				}
				printAndExport(allMetrics, time.Since(t0), exportPath, filepath.Base(args[0]), plan)
				return nil
			}

			// 3b. WorkerPool (parallel iterations)
			if workers > 1 {
				cfg := runner.WorkerConfig{
					Plan:         plan,
					BaseEnv:      baseEnv,
					NoCookies:    noCookies,
					ClearCookies: clearCookies,
					Verbosity:    verbosityLevel,
					Personas:     loadedPersonas,
				}

				color.Cyan("Starting load test: %d iterations across %d workers\n", iterations, workers)
				results := runner.NewWorkerPool(workers).Run(cfg, iterations)

				sort.Slice(results, func(i, j int) bool {
					return results[i].IterationIndex < results[j].IterationIndex
				})
				for _, r := range results {
					if r.Err != nil {
						color.Red("Iteration %d failed: %v\n", r.IterationIndex, r.Err)
					}
					allMetrics = append(allMetrics, r.Metrics)
				}

				printAndExport(allMetrics, time.Since(totalStartTime), exportPath, filepath.Base(args[0]), plan)
				return nil
			}

			// 3c. Sequential execution
			for i := 1; i <= iterations; i++ {
				if iterations > 1 {
					hdr := fmt.Sprintf("  Iteration %d / %d  ", i, iterations)
					pad := strings.Repeat("=", (70-len(hdr))/2)
					fmt.Printf("\n%s%s%s\n", pad, hdr, pad)
				}

				ctx := runner.NewRuntimeContext()
				if baseEnv != nil {
					ctx.SetEnvironment(baseEnv.Clone())
				}
				if len(loadedPersonas) > 0 {
					applyPersonaToCtx(ctx, loadedPersonas[0])
				}

				exec := http_executor.NewDefaultExecutor()
				if noCookies {
					exec.DisableCookies()
				}

				engine := runner.NewCollectionRunner(exec, nil, nil, nil)
				engine.SetVerbosity(verbosityLevel)
				if clearCookies {
					engine.SetClearCookiesPerRequest(true)
				}

				runMetrics, err := engine.Run(plan, ctx)
				if err != nil {
					color.Red("Iteration %d failed: %v\n", i, err)
				}
				allMetrics = append(allMetrics, runMetrics)

				if i < iterations {
					fmt.Println("\nWaiting 1 second before next iteration...")
					time.Sleep(1 * time.Second)
				}
			}

			printAndExport(allMetrics, time.Since(totalStartTime), exportPath, filepath.Base(args[0]), plan)
			return nil
		},
	}

	c.Flags().IntVarP(&iterations, "iterations", "n", 1, "Number of times to run the collection")
	c.Flags().IntVarP(&workers, "workers", "c", 1, "Number of parallel workers (virtual users)")
	c.Flags().StringVarP(&envFilePath, "env", "e", "", "Path to the environment JSON file")
	c.Flags().BoolVar(&noCookies, "no-cookies", false, "Disable cookie persistence for this run")
	c.Flags().BoolVar(&clearCookies, "clear-cookies", false, "Clear cookie jar before each request")
	c.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output (full headers + body)")
	c.Flags().StringSliceVarP(&requestFilters, "request", "f", []string{}, "Only run requests matching these names")
	c.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress per-request logs; show progress bar")
	c.Flags().StringVar(&exportPath, "export", "", "Export raw metrics as newline-delimited JSON")
	c.Flags().DurationVarP(&duration, "duration", "d", 0, "Run duration (e.g. 30s, 2m)")
	c.Flags().Float64Var(&rps, "rps", 0, "Max requests per second (0 = unlimited)")
	c.Flags().StringVar(&stages, "stages", "", `Ramp plan, e.g. "10s:5,30s:20,10s:0"`)
	c.Flags().StringVar(&personasPath, "personas", "", "CSV file of personas (columns become {{persona.<col>}})")

	c.Flags().StringVar(&injIndex, "inject-index", "", "1-based position to insert a temporary request")
	c.Flags().StringVar(&injName, "inject-name", "", "Name of the temporary request")
	c.Flags().StringVar(&injMethod, "inject-method", "GET", "HTTP method for temporary request")
	c.Flags().StringVar(&injURL, "inject-url", "", "URL for temporary request")
	c.Flags().StringVar(&injBody, "inject-data", "", "Body payload for temporary request")
	c.Flags().StringSliceVar(&injHeaders, "inject-header", []string{}, "Header for temporary request (e.g. 'Key: Value')")

	return c
}

// printAndExport analyzes metrics, prints the report, saves to history, and
// optionally writes raw JSON. plan is used to persist DAG topology when
// the collection used a scenario graph.
func printAndExport(
	allMetrics [][]runner.RequestMetric,
	elapsed time.Duration,
	exportPath string,
	collectionName string,
	plan *planner.ExecutionPlan,
) {
	report := metrics.AnalyzeSharded(allMetrics, elapsed, 0)
	metrics.PrintReport(report)

	if db, err := history.Open(); err == nil {
		if saveErr := db.SaveRunWithDAG(collectionName, report, plan, allMetrics); saveErr != nil {
			color.Yellow("⚠ History save failed: %v\n", saveErr)
		}
		db.Close()
	}

	if exportPath != "" {
		if err := metrics.ExportJSON(allMetrics, exportPath); err != nil {
			color.Red("⚠ Export failed: %v\n", err)
		} else {
			color.Cyan("📄 Results exported to: %s\n", exportPath)
		}
	}
}

func printPhase3Header(cfg runner.SchedulerConfig) {
	fmt.Println()
	if len(cfg.Stages) > 0 {
		color.Cyan("Stage-based load test -- %d stages\n", len(cfg.Stages))
		for i, s := range cfg.Stages {
			color.Cyan("    Stage %d: %d workers for %v\n", i+1, s.TargetWorkers, s.Duration)
		}
	} else {
		color.Cyan("Duration-based load test -- %v, %d workers", cfg.Duration, cfg.MaxWorkers)
	}
	if cfg.RPS > 0 {
		color.Cyan("  |  Rate: %.1f req/s", cfg.RPS)
	}
	fmt.Println()
}

func applyPersonaToCtx(ctx *runner.RuntimeContext, p map[string]string) {
	if ctx.Environment == nil {
		return
	}
	for k, v := range p {
		if k != "" {
			ctx.Environment.Variables["persona."+k] = v
		}
	}
}