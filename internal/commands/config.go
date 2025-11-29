package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wlame/njgit/internal/backend"
	"github.com/wlame/njgit/internal/config"
	"github.com/wlame/njgit/internal/nomad"
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
		if displayCfg.Git.Token != "" {
			displayCfg.Git.Token = "********"
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

		backendType := cfg.Git.Backend
		if backendType == "" {
			backendType = "git"
		}
		if backendType == "git" {
			PrintInfo(fmt.Sprintf("Git repository: %s (local)", cfg.Git.LocalPath))
		} else {
			PrintInfo(fmt.Sprintf("GitHub repository: %s/%s", cfg.Git.Owner, cfg.Git.Repo))
		}
		PrintInfo(fmt.Sprintf("Nomad address: %s", cfg.Nomad.Address))
		PrintInfo(fmt.Sprintf("Tracking %d jobs", len(cfg.Jobs)))

		return nil
	},
}

// configCheckCmd checks the configuration by testing all connections
var configCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check configuration and verify all connections",
	Long: `Perform a comprehensive check of your configuration:
  • Validate configuration file syntax
  • Test connection to Nomad cluster
  • Verify backend access (Git repository or GitHub API)
  • Check if configured jobs exist in Nomad
  • Verify authentication and permissions

This is useful after initial setup to ensure everything is configured correctly.`,
	RunE: configCheckRun,
}

func init() {
	// Add subcommands to config command
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configCheckCmd)

	// Add config command to root command
	rootCmd.AddCommand(configCmd)
}

func configCheckRun(cmd *cobra.Command, args []string) error {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🔍 Checking njgit configuration...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	checksPassed := 0
	checksFailed := 0
	warnings := 0

	// Check 1: Load configuration
	fmt.Println("1️⃣  Loading configuration file...")
	cfg, err := config.Load(GetConfigFile())
	if err != nil {
		PrintError(fmt.Errorf("   ❌ Failed to load configuration: %w", err))
		fmt.Println()
		fmt.Println("💡 Tip: Run 'njgit init' to create a configuration file")
		return err
	}
	PrintSuccess("   ✅ Configuration file loaded")
	checksPassed++

	// Check 2: Validate configuration
	fmt.Println()
	fmt.Println("2️⃣  Validating configuration...")
	if err := cfg.Validate(); err != nil {
		PrintError(fmt.Errorf("   ❌ Configuration validation failed: %w", err))
		fmt.Println()
		fmt.Println("💡 Tip: Check your configuration file for missing or invalid fields")
		return err
	}
	PrintSuccess("   ✅ Configuration is valid")
	checksPassed++

	// Display configuration summary
	fmt.Println()
	fmt.Println("   📋 Configuration Summary:")
	backendType := cfg.Git.Backend
	if backendType == "" {
		backendType = "git"
	}
	fmt.Printf("      Backend: %s\n", backendType)
	if backendType == "git" {
		fmt.Printf("      Local path: %s\n", cfg.Git.LocalPath)
	} else {
		fmt.Printf("      GitHub: %s/%s\n", cfg.Git.Owner, cfg.Git.Repo)
		fmt.Printf("      Branch: %s\n", cfg.Git.Branch)
	}
	fmt.Printf("      Nomad: %s\n", cfg.Nomad.Address)
	fmt.Printf("      Jobs to track: %d\n", len(cfg.Jobs))

	// Check 3: Test Nomad connection
	fmt.Println()
	fmt.Println("3️⃣  Testing Nomad connection...")
	nomadAuth, err := nomad.ResolveAuth(&cfg.Nomad, "", "")
	if err != nil {
		PrintError(fmt.Errorf("   ❌ Failed to resolve Nomad auth: %w", err))
		checksFailed++
	} else {
		nomadClient, err := nomad.NewClient(nomadAuth)
		if err != nil {
			PrintError(fmt.Errorf("   ❌ Failed to create Nomad client: %w", err))
			checksFailed++
		} else {
			defer func() { _ = nomadClient.Close() }()

			if err := nomadClient.Ping(); err != nil {
				PrintError(fmt.Errorf("   ❌ Failed to connect to Nomad: %w", err))
				checksFailed++
				fmt.Println()
				fmt.Println("   💡 Tips:")
				fmt.Println("      • Check if Nomad is running and accessible")
				fmt.Println("      • Verify NOMAD_ADDR is correct")
				fmt.Println("      • Check if ACL token is valid (if using ACLs)")
			} else {
				PrintSuccess("   ✅ Successfully connected to Nomad")
				checksPassed++

				// Check 4: Verify jobs exist in Nomad
				fmt.Println()
				fmt.Println("4️⃣  Checking configured jobs in Nomad...")

				if len(cfg.Jobs) == 0 {
					PrintWarning("   ⚠️  No jobs configured to track")
					warnings++
					fmt.Println("   💡 Add jobs to your configuration file under [[jobs]] section")
				} else {
					jobsFound := 0
					jobsMissing := 0

					for _, jobCfg := range cfg.Jobs {
						jobPath := fmt.Sprintf("%s/%s", jobCfg.Namespace, jobCfg.Name)
						_, err := nomadClient.FetchJobSpec(jobCfg.Namespace, jobCfg.Name)
						if err != nil {
							if _, ok := err.(nomad.JobNotFoundError); ok {
								fmt.Printf("   ⚠️  Job not found: %s\n", jobPath)
								jobsMissing++
							} else {
								fmt.Printf("   ❌ Error checking job %s: %v\n", jobPath, err)
								jobsMissing++
							}
						} else {
							fmt.Printf("   ✅ Job found: %s\n", jobPath)
							jobsFound++
						}
					}

					if jobsMissing > 0 {
						PrintWarning(fmt.Sprintf("   ⚠️  %d job(s) not found in Nomad", jobsMissing))
						warnings++
						fmt.Println("   💡 These jobs will be skipped during sync until they exist")
					}

					if jobsFound > 0 {
						PrintSuccess(fmt.Sprintf("   ✅ %d job(s) found in Nomad", jobsFound))
						checksPassed++
					} else {
						checksFailed++
					}
				}
			}
		}
	}

	// Check 5: Test backend connection
	fmt.Println()
	fmt.Println("5️⃣  Testing backend connection...")

	backend, err := backend.NewBackend(&cfg.Git)
	if err != nil {
		PrintError(fmt.Errorf("   ❌ Failed to create backend: %w", err))
		checksFailed++

		if backendType == "github-api" {
			fmt.Println()
			fmt.Println("   💡 Tips for GitHub API backend:")
			fmt.Println("      • Set GITHUB_TOKEN environment variable")
			fmt.Println("      • Ensure token has 'repo' scope")
			fmt.Println("      • Verify owner and repo names are correct")
		} else {
			fmt.Println()
			fmt.Println("   💡 Tips for Git backend:")
			fmt.Println("      • For SSH: Ensure SSH key is added to your Git provider")
			fmt.Println("      • For HTTPS: Set GITHUB_TOKEN or GH_TOKEN environment variable")
			fmt.Println("      • Verify repository URL is correct")
		}
	} else {
		defer backend.Close()

		if err := backend.Initialize(); err != nil {
			PrintError(fmt.Errorf("   ❌ Failed to initialize backend: %w", err))
			checksFailed++

			if backendType == "github-api" {
				fmt.Println()
				fmt.Println("   💡 Common issues:")
				fmt.Println("      • Invalid or expired GitHub token")
				fmt.Println("      • Repository doesn't exist or is private without access")
				fmt.Println("      • Token missing 'repo' permissions")
			} else {
				fmt.Println()
				fmt.Println("   💡 Common issues:")
				fmt.Println("      • SSH key not authorized")
				fmt.Println("      • Repository doesn't exist")
				fmt.Println("      • Network/firewall issues")
			}
		} else {
			PrintSuccess(fmt.Sprintf("   ✅ Successfully connected to backend (%s)", backend.GetName()))
			checksPassed++
		}
	}

	// Final summary
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 Check Summary")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("   ✅ Passed: %d\n", checksPassed)
	if warnings > 0 {
		fmt.Printf("   ⚠️  Warnings: %d\n", warnings)
	}
	if checksFailed > 0 {
		fmt.Printf("   ❌ Failed: %d\n", checksFailed)
	}
	fmt.Println()

	if checksFailed > 0 {
		fmt.Println("❌ Configuration check failed. Please fix the issues above.")
		return fmt.Errorf("configuration check failed with %d error(s)", checksFailed)
	}

	if warnings > 0 {
		fmt.Println("⚠️  Configuration check passed with warnings.")
		fmt.Println("   The tool will work but some issues should be addressed.")
	} else {
		fmt.Println("✅ All checks passed! You're ready to use njgit.")
		fmt.Println()
		fmt.Println("🚀 Next steps:")
		fmt.Println("   • Run 'njgit sync' to start tracking job changes")
		fmt.Println("   • Run 'njgit sync --dry-run' to preview changes first")
	}

	return nil
}
