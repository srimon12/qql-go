package main

import (
	"fmt"
	"os"

	"github.com/qdrant/qql-go/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		if !cli.ErrorPrinted(err) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(cli.ExitCode(err))
	}
}
