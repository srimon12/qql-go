package cli

import (
	"fmt"
	"os"

	"github.com/qdrant/qql-go/internal/cli/commands"
	"github.com/qdrant/qql-go/internal/config"
	"github.com/qdrant/qql-go/internal/output"
	"github.com/qdrant/qql-go/internal/repl"
	"github.com/spf13/cobra"
)

func Execute() error {
	return NewRootCmd(output.NewOutputter()).Execute()
}

func NewRootCmd(out *output.Outputter) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "qql",
		Short: "QQL — Qdrant Query Language CLI",
		Long:  `QQL is a query language CLI for Qdrant vector database.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				out.PrintError(fmt.Sprintf("Failed to load config: %v", err))
				os.Exit(1)
			}

			if cfg == nil || cfg.URL == "" {
				out.PrintError("Not connected. Run: qql connect --url <url>")
				os.Exit(1)
			}

			client, err := commands.NewClient(cfg)
			if err != nil {
				out.PrintError(fmt.Sprintf("Connection failed: %v", err))
				os.Exit(1)
			}

			executor := commands.NewExecutor(client, cfg)
			repl := repl.NewREPL(cfg, executor)
			return repl.Run()
		},
	}

	rootCmd.AddCommand(
		commands.NewConnectCmd(out),
		commands.NewDisconnectCmd(out),
		commands.NewREPLCmd(out),
		commands.NewExecCmd(out),
		commands.NewExplainCmd(out),
		commands.NewDoctorCmd(out),
		commands.NewVersionCmd(out),
	)

	return rootCmd
}
