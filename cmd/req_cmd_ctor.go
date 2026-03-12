package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"postman-cli/internal/collection"
	"postman-cli/internal/errs"
	"postman-cli/internal/http_executor"
	"postman-cli/internal/runner"
	"postman-cli/internal/storage"
)

// NewReqCmd constructs the `req` CLI command for single requests.
func NewReqCmd() *cobra.Command {
	var method string
	var headers []string
	var body string
	var envFilePath string

	c := &cobra.Command{
		Use:   "req [url]",
		Short: "Send a single quick HTTP request (curl style)",
		Args:  cobra.ExactArgs(1),
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

			err := engine.Run(dummyColl, ctx)
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

	return c
}