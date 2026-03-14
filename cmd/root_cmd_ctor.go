package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewRootCmd constructs the base CLI command.
func NewRootCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "reqx", 
		Short: "A fast, scriptable API client for the terminal",
		Long: `🚀 ReqX: The high-performance, developer-first API client.
Built for speed and automation, ReqX allows you to run Postman-style collections, 
individual HTTP requests, and interactive Socket.IO events directly from your 
terminal. It is a powerful, lightweight alternative for teams that value 
terminal-centric workflows and fast execution.

⚡ Key Capabilities:
- Stateful Collections: Use environment variables to pass data between requests.
- Advanced Scripting: JavaScript-based tests and pre-request logic (Goja engine).
- Real-time Debugging: First-class support for Socket.IO v4 with async listeners.
- Performance Ready: Built-in support for multi-iteration runs and aggregated summaries.
- Zero GUI Required: Manage collection files entirely via the 'collection' command.`,
		Example: `  # 📡 RUN a collection with environment variables
  reqx run vuc-collection.json -e prod-env.json
  
  # 🚀 PERFORMANCE test with 10 iterations and aggregated summary
  reqx run vuc-collection.json -n 10
  
  # 🔍 FILTER to run only the "Login" request
  reqx run vuc-collection.json -f "Login"
  
  # 💡 Ad-hoc HTTP request (curl style)
  reqx req https://api.github.com/users/aryanwalia2003
  
  # 📂 MANAGE collection content without opening an editor
  reqx collection add my-api.json -n "Health Check" -u "{{base_url}}/health"`,
	}

	c.AddCommand(NewRunCmd())
	c.AddCommand(NewSampleCmd())
	c.AddCommand(NewReqCmd())
	c.AddCommand(NewSioCmd())
	c.AddCommand(NewCollectionCmd())

	return c
}

// Execute is the main entrypoint called by main.go.
func Execute() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
