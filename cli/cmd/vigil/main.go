// Package main is the entrypoint for the vigil CLI.
package main

import (
	"os"

	"github.com/uwezukwechibuzor/vigil/cli/internal/cmd"
)

func main() {
	if err := cmd.Root().Execute(); err != nil {
		os.Exit(1)
	}
}
