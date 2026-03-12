package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"postman-cli/internal/storage"
)

// NewSampleCmd constructs the `sample` CLI command.
func NewSampleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sample",
		Short: "Generate sample collection and environment files",
		RunE: func(cmd *cobra.Command, args []string) error {
			err1 := os.WriteFile("sample-collection.json", []byte(storage.SampleCollectionJSON), 0644)
			err2 := os.WriteFile("sample-env.json", []byte(storage.SampleEnvJSON), 0644)

			if err1 != nil || err2 != nil {
				return fmt.Errorf("failed to write sample files")
			}

			fmt.Println("Created sample-collection.json and sample-env.json")
			return nil
		},
	}
}
