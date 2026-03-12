package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewRootCmd constructs the base CLI command.
func NewRootCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "postman-cli", //this is the name of the command that will be used to run the CLI
		Short: "A fast, scriptable API client for the command line",
		Long:  "postman-cli is a lightweight developer tool for running requests and debugging APIs directly from the terminal.",
	}

	c.AddCommand(NewRunCmd())
	c.AddCommand(NewSampleCmd())

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
