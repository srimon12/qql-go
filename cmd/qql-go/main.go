package main

import (
	"fmt"
	"os"

	"github.com/srimon12/qql-go/internal/cli"
	"github.com/srimon12/qql-go/internal/output"
	"github.com/srimon12/qql-go/server"
)

func main() {
	out := output.NewOutputter()
	rootCmd := cli.NewRootCmd(out)
	rootCmd.AddCommand(server.NewServeCmd())

	if err := rootCmd.Execute(); err != nil {
		if !cli.ErrorPrinted(err) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(cli.ExitCode(err))
	}
}
