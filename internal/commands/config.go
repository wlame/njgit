package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wlame/nomad-changelog/internal/config"
)

// configCmd represents the config command and its subcommands
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `Display and validate configuration settings.`,
}

// configShowCmd shows the current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	Long:  `Load and display the current configuration from file and environment variables.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.Load(GetConfigFile())
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Redact sensitive information for display
		displayCfg := *cfg
		if displayCfg.Nomad.Token != "" {
			displayCfg.Nomad.Token = "********"
		}
		if displayCfg.Git.GitHubToken != "" {
			displayCfg.Git.GitHubToken = "********"
		}

		// Display configuration
		PrintInfo("Configuration loaded successfully")
		fmt.Println()

		// Pretty print as JSON for readability
		jsonBytes, err := json.MarshalIndent(displayCfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format config: %w", err)
		}

		fmt.Println(string(jsonBytes))
		return nil
	},
}

// configValidateCmd validates the configuration
var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration",
	Long:  `Load and validate the configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.Load(GetConfigFile())
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Validate configuration
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("configuration is invalid: %w", err)
		}

		PrintSuccess("Configuration is valid")
		PrintInfo(fmt.Sprintf("Git repository: %s", cfg.Git.URL))
		PrintInfo(fmt.Sprintf("Nomad address: %s", cfg.Nomad.Address))
		PrintInfo(fmt.Sprintf("Tracking %d jobs", len(cfg.Jobs)))

		return nil
	},
}

func init() {
	// Add subcommands to config command
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)

	// Add config command to root command
	rootCmd.AddCommand(configCmd)
}
