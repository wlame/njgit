// Package main is the entry point for the njgit CLI application.
// In Go, the main package and main() function are special - this is where execution starts.
//
// The main package should be kept minimal. All the actual logic lives in other packages
// (especially internal/commands). This is a Go best practice.
package main

import (
	"os"

	"github.com/wlame/njgit/internal/commands"
)

// main is the entry point of the application
// It's automatically called when the binary is executed
func main() {
	// Execute the root command (which handles all subcommands)
	// This delegates to the Cobra command structure defined in internal/commands
	if err := commands.Execute(); err != nil {
		// If there's an error, print it and exit with a non-zero status code
		// Non-zero exit codes indicate failure to the shell/calling process
		commands.PrintError(err)
		os.Exit(1)
	}

	// If we get here, the command executed successfully
	// The program will exit with status code 0 (success)
}
