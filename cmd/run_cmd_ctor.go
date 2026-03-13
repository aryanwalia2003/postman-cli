package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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

	// NEW: Variables for Temporary Request Injection
	var injIndex string
	var injName, injMethod, injURL, injBody string
	var injHeaders []string

	c := &cobra.Command{
		Use:   "run [collection.json]",
		Short: "Execute a collection of requests",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			collectionPath := args[0]

			// 1. Load Collection from File
			collBytes, err := storage.ReadJSONFile(collectionPath)
			if err != nil {
				return errs.Wrap(err, errs.KindInvalidInput, "could not read collection file")
			}

			coll, err := storage.ParseCollection(collBytes)
			if err != nil {
				return errs.Wrap(err, errs.KindInvalidInput, "could not parse collection JSON")
			}

			// =================================================================
			// ▼▼▼ NEW: TEMPORARY INJECTION LOGIC ▼▼▼
			// =================================================================
			if injIndex != "" && injName != "" && injURL != "" {
				idx, err := strconv.Atoi(injIndex)
				if err != nil || idx < 1 {
					return errs.InvalidInput("Invalid --inject-index. It must be a 1-based number.")
				}
				
				// Convert 1-based CLI index to 0-based slice index
				insertPos := idx - 1
				if insertPos > len(coll.Requests) {
					insertPos = len(coll.Requests) // If index is too large, append to end
				}

				headerMap := make(map[string]string)
				for _, h := range injHeaders {
					parts := strings.SplitN(h, ":", 2)
					if len(parts) == 2 {
						headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				}

				tempReq := collection.Request{
					Name:    color.New(color.FgHiMagenta).Sprintf("[INJECTED] %s", injName), // Highlight it!
					Method:  strings.ToUpper(injMethod),
					URL:     injURL,
					Headers: headerMap,
					Body:    injBody,
				}

				color.Magenta("💉 Injecting temporary request '%s' at position %d...\n", injName, idx)

				// Insert into the slice IN-MEMORY ONLY
				if insertPos == len(coll.Requests) {
					coll.Requests = append(coll.Requests, tempReq)
				} else {
					coll.Requests = append(coll.Requests[:insertPos+1], coll.Requests[insertPos:]...)
					coll.Requests[insertPos] = tempReq
				}
			} else if (injName != "" || injURL != "") && injIndex == "" {
				color.Yellow("⚠ Warning: Ignored temporary request injection. Missing --inject-index.\n")
			}
			// =================================================================
			// ▲▲▲ END INJECTION LOGIC ▲▲▲
			// =================================================================


			// 2. Filter requests if --request is provided
			if requestFilter != "" {
				filtered := []collection.Request{}
				for _, r := range coll.Requests {
					if strings.Contains(strings.ToLower(r.Name), strings.ToLower(requestFilter)) {
						filtered = append(filtered, r)
					}
				}
				if len(filtered) == 0 {
					return errs.InvalidInput(fmt.Sprintf("No requests found matching: %s", requestFilter))
				}
				coll.Requests = filtered
				color.Cyan("🔍 Filtered collection to %d request(s) matching '%s'\n", len(filtered), requestFilter)
			}

			// 3. Init Runtime Context
			ctx := runner.NewRuntimeContext()

			// 3. Load Environment if provided
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

			// 4. Build executor with cookie jar wired in
			exec := http_executor.NewDefaultExecutor()
			if noCookies {
				exec.DisableCookies()
			}

			// 5. Run Collection
			engine := runner.NewCollectionRunner(exec, nil, nil)
			if clearCookies {
				engine.SetClearCookiesPerRequest(true)
			}
			if verbose {
				engine.SetVerbose(true)
			}

			err = engine.Run(coll, ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nExecution Failed: %v\n", err)
				os.Exit(1)
			}

			return nil
		},
	}

	// Standard Flags
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