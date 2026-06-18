package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/srimon12/qql-go/internal/convert"
	"github.com/srimon12/qql-go/internal/output"
)

func NewConvertCmd(out *output.Outputter) *cobra.Command {
	var (
		filePath string
		validate bool
		quiet    bool
		jsonOut  bool
	)

	cmd := &cobra.Command{
		Use:   "convert [file]",
		Short: "Convert Qdrant REST API JSON to QQL",
		Long: `Convert Qdrant REST API JSON payloads to native QQL statements.

Accepts JSON from stdin, a file path, or as a direct argument.
Auto-detects the operation type from the JSON structure and outputs
equivalent QQL statements that can be piped to qql-go execute.

Supported operations:
  - Upsert points      → INSERT INTO
  - Search             → QUERY
  - Recommend          → QUERY RECOMMEND
  - Discover           → QUERY DISCOVER
  - Scroll             → SCROLL FROM
  - Get points         → SELECT * FROM ... WHERE id IN
  - Delete points      → DELETE FROM
  - Create collection  → CREATE COLLECTION
  - Create index       → CREATE INDEX

Examples:
  qql-go convert payload.json
  qql-go convert --validate payload.json
  cat payload.json | qql-go convert
  echo '{"points":[{"id":1,"payload":{"text":"hi"}}]}' | qql-go convert`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var input []byte
			var err error

			if len(args) > 0 {
				filePath = args[0]
			}

			if filePath != "" {
				input, err = os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("cannot read file: %w", err)
				}
			} else {
				input, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("cannot read stdin: %w", err)
				}
			}

			inputStr := strings.TrimSpace(string(input))
			if len(inputStr) == 0 {
				return fmt.Errorf("no input provided")
			}

			statements, err := convert.JSONToQQL([]byte(inputStr))
			if err != nil {
				return fmt.Errorf("conversion error: %w", err)
			}

			if validate {
				exec := NewExecutor(nil, nil)
				for i, stmt := range statements {
					_, err := exec.Explain(stmt)
					if err != nil {
						return fmt.Errorf("statement %d failed validation: %w\n%s", i+1, err, stmt)
					}
				}
			}

			outputStr := strings.Join(statements, "\n\n")
			if quiet {
				out.Print(outputStr)
			} else if jsonOut {
				out.PrintJSON(map[string]any{
					"ok":         true,
					"statements": statements,
					"count":      len(statements),
				}, true)
			} else {
				for i, stmt := range statements {
					if i > 0 {
						out.Print("")
					}
					out.Print(stmt)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&validate, "validate", false, "Validate generated QQL with explain")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Output only the QQL statements")
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "Output as JSON")

	return cmd
}
