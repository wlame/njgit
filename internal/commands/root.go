// Package commands implements all CLI commands for ndiff.
// It uses the Cobra library which is the standard for CLI applications in Go.
package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wlame/ndiff/pkg/version"
)

var (
	// cfgFile holds the path to the configuration file
	// This is set by the --config flag
	cfgFile string

	// verbose enables verbose output
	// This is set by the --verbose flag
	verbose bool
)

// rootCmd represents the base command when called without any subcommands
// In Cobra, the root command is the entry point for the CLI
var rootCmd = &cobra.Command{
	// Use defines the command name
	Use: "ndiff",

	// Short is a brief description shown in help output
	Short: "Track Nomad job configuration changes in Git",

	// Long is the detailed description shown in 'help' output
	Long: `ndiff is a stateless CLI tool that tracks Nomad job
configuration changes by syncing them to a Git repository.

It provides version history and rollback capabilities for your Nomad jobs
by storing job specifications as HCL files in Git.

Key features:
  - Automatic change detection (ignores Nomad internal metadata)
  - Git-based version history with detailed commit messages
  - Support for multiple authentication methods (SSH, tokens)
  - Stateless operation (no local database)
  - Can run on a schedule (cron, GitHub Actions)

Example usage:
  # Sync all configured jobs
  ndiff sync

  # Initialize a new repository
  ndiff init

  # Show pending changes
  ndiff diff

For more information, see: https://github.com/wlame/ndiff`,

	// SilenceUsage prevents showing usage on errors
	// We don't want to show the full usage every time there's an error
	SilenceUsage: true,

	// SilenceErrors prevents Cobra from printing errors
	// We'll handle error printing ourselves for better control
	SilenceErrors: true,
}

// Execute is the main entry point for the CLI
// It's called from main.go and executes the root command
// Returns an error if command execution fails
func Execute() error {
	return rootCmd.Execute()
}

// init is a special Go function that runs automatically when the package is imported
// We use it to set up the command structure and flags
func init() {
	// Cobra supports persistent flags, which are available to all subcommands
	// These flags are defined on the root command

	// --config flag: Path to config file
	// The flag is bound to the cfgFile variable
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "",
		"config file (default: ./ndiff.toml)")

	// --verbose flag: Enable verbose output
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"verbose output")

	// Add version command
	// This is a built-in command that shows version information
	rootCmd.AddCommand(versionCmd)

	// TODO: Add other subcommands here as we implement them:
	// rootCmd.AddCommand(syncCmd)
	// rootCmd.AddCommand(initCmd)
	// rootCmd.AddCommand(addCmd)
	// rootCmd.AddCommand(removeCmd)
	// rootCmd.AddCommand(diffCmd)
	// rootCmd.AddCommand(statusCmd)
	// rootCmd.AddCommand(configCmd)
}

// versionCmd represents the version command
// It displays version information about the binary
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Display the version, commit hash, and build time of ndiff.`,
	Run: func(cmd *cobra.Command, args []string) {
		// The Run function is called when the command is executed
		// cmd: The command being run
		// args: Command-line arguments (not used here)

		// Print version information
		fmt.Println(version.String())

		// If verbose flag is set, also print structured info
		if verbose {
			fmt.Println()
			info := version.Get()
			fmt.Printf("Version:    %s\n", info.Version)
			fmt.Printf("Commit:     %s\n", info.Commit)
			fmt.Printf("Build Time: %s\n", info.BuildTime)
		}
	},
}

// GetConfigFile returns the path to the configuration file
// This is used by subcommands to load the configuration
func GetConfigFile() string {
	return cfgFile
}

// IsVerbose returns true if verbose mode is enabled
// This is used by subcommands to determine output verbosity
func IsVerbose() bool {
	return verbose
}

// PrintError prints an error message to stderr
// This is a helper function for consistent error formatting
func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
}

// PrintWarning prints a warning message to stderr
// This is a helper function for consistent warning formatting
func PrintWarning(msg string) {
	fmt.Fprintf(os.Stderr, "[WARN] %s\n", msg)
}

// PrintInfo prints an info message to stdout
// This is a helper function for consistent info formatting
func PrintInfo(msg string) {
	fmt.Printf("[INFO] %s\n", msg)
}

// PrintSuccess prints a success message to stdout
// This is a helper function for consistent success formatting
func PrintSuccess(msg string) {
	fmt.Printf("[SUCCESS] %s\n", msg)
}
