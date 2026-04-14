package cli

import (
	"github.com/srimon12/qql-go/internal/cli/commands"
	"github.com/srimon12/qql-go/internal/output"
	"github.com/spf13/cobra"
)

func Execute() error {
	return NewRootCmd(output.NewOutputter()).Execute()
}

func ExitCode(err error) int {
	return commands.ExitCode(err)
}

func ErrorPrinted(err error) bool {
	return commands.ErrorPrinted(err)
}

func NewRootCmd(out *output.Outputter) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "qql",
		Short:         "QQL — Qdrant Query Language CLI",
		Long:          `QQL is a query language CLI for Qdrant vector database.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.NewREPLCmd(out).RunE(cmd, args)
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
