package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"reqx/internal/collection"
	"reqx/internal/errs"
	"reqx/internal/http_executor"
	"reqx/internal/runner"
	"reqx/internal/storage"
)

// NewReqCmd constructs the `req` CLI command for single requests.
func NewReqCmd() *cobra.Command {
	var method string
	var headers []string
	var body string
	var envFilePath string
	var verbose bool

	c := &cobra.Command{
		Use:   "req [url]",
		Short: "Send a single quick HTTP request (curl style)",
		Long: `🚀 Execute a standalone HTTP request instantly.
No collection file required. 'req' is designed for speed—giving you a 
curl-like experience but with the added power of variables, environment 
injection, and formatted output.

Perfect for:
- Testing a single endpoint before adding it to a collection.
- Quick debugging of external APIs using predefined environmental secrets.`,
		Example: `  # 📡 Simple GET request
  reqx req https://api.github.com/users/aryanwalia2003
  
  # 🔐 POST with body and variables from an environment file
  reqx req "{{base_url}}/auth/login" -e prod.json -X POST -d '{"user":"test"}'
  
  # 🛠 Adding custom headers and using verbose output
  reqx req http://localhost:8080/api -H "Authorization: Bearer my-token" -v`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]

			// 1. Parse Headers from CLI flags (e.g., "Content-Type: application/json")
			headerMap := make(map[string]string)
			for _, h := range headers {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) == 2 {
					headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}

			// 2. Build an in-memory Single-Request Collection
			singleReq := collection.Request{
				Name:    "Ad-hoc Request",
				Method:  strings.ToUpper(method),
				URL:     targetURL,
				Headers: headerMap,
				Body:    body,
			}

			dummyColl := &collection.Collection{
				Name:     "Ad-hoc Collection",
				Requests: []collection.Request{singleReq},
			}

			// 3. Init Runtime Context (load env if provided)
			ctx := runner.NewRuntimeContext()
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

			// 4. Execute using the standard Collection Runner
			exec := http_executor.NewDefaultExecutor()
			engine := runner.NewCollectionRunner(exec, nil, nil)

			if verbose {
				engine.SetVerbose(true)
			}
			_, err := engine.Run(dummyColl, ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Request Failed: %v\n", err)
				os.Exit(1)
			}

			return nil
		},
	}

	// Flags (similar to curl)
	c.Flags().StringVarP(&method, "request", "X", "GET", "HTTP method (GET, POST, PUT, etc.)")
	c.Flags().StringSliceVarP(&headers, "header", "H", []string{}, "Custom headers (can specify multiple times, e.g., 'Key: Value')")
	c.Flags().StringVarP(&body, "data", "d", "", "HTTP POST/PUT data body")
	c.Flags().StringVarP(&envFilePath, "env", "e", "", "Path to environment JSON file (for variable replacement)")
	c.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output to see full request and response")

	return c
}