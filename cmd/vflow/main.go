package main

import (
	"os"

	"github.com/nerveband/vflow/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		if exitErr, ok := err.(interface{ ExitCode() int }); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
