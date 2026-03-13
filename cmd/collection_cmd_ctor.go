package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"postman-cli/internal/collection"
	"postman-cli/internal/errs"
	"postman-cli/internal/storage"
)

// NewCollectionCmd creates the base `collection` command
func NewCollectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "collection",
		Aliases: []string{"coll"},
		Short:   "Manage requests inside a collection JSON file",
		Long:    "View, add, or reorder requests permanently within a collection file without opening an editor.",
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newMoveCmd())

	return cmd
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [collection.json]",
		Short: "List all requests in a collection with their index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			coll, err := loadCollection(filePath)
			if err != nil {
				return err
			}

			color.Cyan("Requests in '%s' (%s):", coll.Name, filePath)
			for i, req := range coll.Requests {
				methodColor := color.New(color.FgWhite).SprintFunc()
				switch strings.ToUpper(req.Method) {
				case "GET":
					methodColor = color.New(color.FgGreen).SprintFunc()
				case "POST":
					methodColor = color.New(color.FgYellow).SprintFunc()
				case "PUT", "PATCH":
					methodColor = color.New(color.FgBlue).SprintFunc()
				case "DELETE":
					methodColor = color.New(color.FgRed).SprintFunc()
				}
				
				fmt.Printf("  [%d] %s %s\n", i+1, methodColor(fmt.Sprintf("%-6s", req.Method)), req.Name)
			}
			return nil
		},
	}
}

func newAddCmd() *cobra.Command {
	var name, method, url, body string
	var headers []string

	cmd := &cobra.Command{
		Use:   "add [collection.json]",
		Short: "Add a new request to the end of a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			coll, err := loadCollection(filePath)
			if err != nil {
				return err
			}

			if name == "" || url == "" {
				return errs.InvalidInput("--name and --url are required")
			}

			headerMap := make(map[string]string)
			for _, h := range headers {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) == 2 {
					headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}

			newReq := collection.Request{
				Name:    name,
				Method:  strings.ToUpper(method),
				URL:     url,
				Headers: headerMap,
				Body:    body,
			}

			coll.Requests = append(coll.Requests, newReq)

			if err := saveCollection(filePath, coll); err != nil {
				return err
			}

			color.Green("✔ Successfully added request '%s' to '%s'\n", name, filePath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Name of the new request (Required)")
	cmd.Flags().StringVarP(&method, "method", "X", "GET", "HTTP method")
	cmd.Flags().StringVarP(&url, "url", "u", "", "Request URL (Required)")
	cmd.Flags().StringVarP(&body, "data", "d", "", "Request body payload")
	cmd.Flags().StringSliceVarP(&headers, "header", "H", []string{}, "Custom headers (e.g., 'Content-Type: application/json')")

	return cmd
}

func newMoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "move [collection.json] [from_index] [to_index]",
		Short: "Change the execution order of a request (1-based index)",
		Long:  "Example: 'move coll.json 5 2' moves the 5th request to the 2nd position.",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			
			fromIdx, err := strconv.Atoi(args[1])
			if err != nil || fromIdx < 1 {
				return errs.InvalidInput("Invalid from_index")
			}
			toIdx, err := strconv.Atoi(args[2])
			if err != nil || toIdx < 1 {
				return errs.InvalidInput("Invalid to_index")
			}

			coll, err := loadCollection(filePath)
			if err != nil {
				return err
			}

			// Convert 1-based CLI index to 0-based array index
			from := fromIdx - 1
			to := toIdx - 1

			if from >= len(coll.Requests) || to >= len(coll.Requests) {
				return errs.InvalidInput(fmt.Sprintf("Index out of bounds. Collection has %d requests.", len(coll.Requests)))
			}

			if from == to {
				color.Yellow("⚠ Request is already at position %d", toIdx)
				return nil
			}

			// Slice manipulation logic
			reqToMove := coll.Requests[from]
			// Remove from original position
			coll.Requests = append(coll.Requests[:from], coll.Requests[from+1:]...)
			
			// Insert at new position
			if to == len(coll.Requests) { // Moving to the very end
				coll.Requests = append(coll.Requests, reqToMove)
			} else {
				// Shift elements and insert
				coll.Requests = append(coll.Requests[:to+1], coll.Requests[to:]...)
				coll.Requests[to] = reqToMove
			}

			if err := saveCollection(filePath, coll); err != nil {
				return err
			}

			color.Green("✔ Successfully moved '%s' from position %d to %d\n", reqToMove.Name, fromIdx, toIdx)
			return nil
		},
	}
}

// --- Helper Functions ---

func loadCollection(path string) (*collection.Collection, error) {
	bytes, err := storage.ReadJSONFile(path)
	if err != nil {
		return nil, errs.Wrap(err, errs.KindInvalidInput, "Failed to read collection file")
	}
	coll, err := storage.ParseCollection(bytes)
	if err != nil {
		return nil, errs.Wrap(err, errs.KindInvalidInput, "Failed to parse collection JSON")
	}
	return coll, nil
}

func saveCollection(path string, coll *collection.Collection) error {
	// Pretty print JSON with 2 spaces
	bytes, err := json.MarshalIndent(coll, "", "  ")
	if err != nil {
		return errs.Wrap(err, errs.KindInternal, "Failed to generate JSON")
	}
	
	if err := storage.WriteJSONFile(path, bytes); err != nil {
		return errs.Wrap(err, errs.KindInternal, "Failed to save collection file")
	}
	return nil
}