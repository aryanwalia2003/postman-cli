package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"postman-cli/internal/collection"
	"postman-cli/internal/errs"
	"postman-cli/internal/http_executor"
	"postman-cli/internal/runner"
	"postman-cli/internal/storage"
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
		Long: `🏃 Execute all or specific requests from a collection file.
This command parses the provided JSON collection and runs each request sequentially.
It supports environment variable substitution, cookie persistence, and real-time scripting.

Advanced Features:
1. Substring Filtering: Use --request to run only a subset of requests.
2. In-Memory Injection: Use the --inject flags to temporarily add a request 
   at a specific position without modifying the source JSON file. 
   This is perfect for adding a 'setup' or 'cleanup' task for a single run.`,
		Example: `  # Run everything in a collection
  postman-cli run my-api.json
  
  # Run a collection 5 times
  postman-cli run my-api.json -n 5
  
  # Run with an environment file
  postman-cli run my-api.json --env prod.json
  
  # Run only requests containing "Login" or "Auth"
  postman-cli run my-api.json -f "Login"
  
  # Inject a temporary 'Health Check' at the start (index 1)
  postman-cli run my-api.json --inject-index 1 --inject-name "Health" --inject-url "http://api/health"
  
  # Disable cookies for a stateless run
  postman-cli run my-api.json --no-cookies`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			collectionPath := args[0]
			
			if iterations < 1 {
				iterations = 1
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

				err = engine.Run(coll, ctx)
				if err != nil {
					color.Red("Iteration %d failed with error: %v\n", i, err)
					// We continue to the next iteration even on failure
				}

				// Add a small delay between iterations
				if i < iterations {
					fmt.Println("\nWaiting 1 second before next iteration...")
					time.Sleep(1 * time.Second)
				}
			} // <-- ITERATION LOOP ENDS HERE

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