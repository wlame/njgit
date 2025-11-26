package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wlame/nomad-changelog/internal/config"
	"github.com/wlame/nomad-changelog/internal/hcl"
	"github.com/wlame/nomad-changelog/internal/nomad"
)

var (
	// Flags for test-fetch command
	testFetchNamespace string
	testFetchJob       string
)

// testFetchCmd is a test command to verify Nomad integration
// This command fetches a job from Nomad, normalizes it, and outputs HCL
// It's useful for testing the complete pipeline without Git integration
var testFetchCmd = &cobra.Command{
	Use:   "test-fetch",
	Short: "Test fetching and converting a Nomad job to HCL (development only)",
	Long: `Fetch a job from Nomad, normalize it, convert to HCL, and print to stdout.

This is a development/testing command to verify:
  - Nomad authentication works
  - Job fetching works
  - Job normalization works
  - HCL formatting works

Example:
  # Fetch a job named "example" from the "default" namespace
  nomad-changelog test-fetch --job example --namespace default

  # With custom config
  nomad-changelog test-fetch --job example --config ./my-config.toml`,
	RunE: testFetchRun,
}

func init() {
	// Add flags
	testFetchCmd.Flags().StringVar(&testFetchNamespace, "namespace", "default",
		"Nomad namespace")
	testFetchCmd.Flags().StringVar(&testFetchJob, "job", "",
		"Job name to fetch (required)")

	// Mark job as required
	testFetchCmd.MarkFlagRequired("job")

	// Add to root command
	rootCmd.AddCommand(testFetchCmd)
}

// testFetchRun executes the test-fetch command
func testFetchRun(cmd *cobra.Command, args []string) error {
	// Load configuration
	PrintInfo("Loading configuration...")
	cfg, err := config.Load(GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	PrintInfo(fmt.Sprintf("Nomad address: %s", cfg.Nomad.Address))

	// Resolve authentication
	PrintInfo("Resolving Nomad authentication...")
	auth, err := nomad.ResolveAuth(&cfg.Nomad, "", "")
	if err != nil {
		return fmt.Errorf("failed to resolve auth: %w", err)
	}

	if IsVerbose() {
		PrintInfo(fmt.Sprintf("Auth config: %s", auth.String()))
	}

	// Create Nomad client
	PrintInfo("Creating Nomad client...")
	client, err := nomad.NewClient(auth)
	if err != nil {
		return fmt.Errorf("failed to create Nomad client: %w", err)
	}
	defer client.Close()

	// Test connectivity
	PrintInfo("Testing Nomad connectivity...")
	if err := client.Ping(); err != nil {
		return fmt.Errorf("failed to connect to Nomad: %w", err)
	}
	PrintSuccess("Successfully connected to Nomad")

	// Fetch the job
	PrintInfo(fmt.Sprintf("Fetching job %s/%s...", testFetchNamespace, testFetchJob))
	job, err := client.FetchJobSpec(testFetchNamespace, testFetchJob)
	if err != nil {
		// Check if it's a "not found" error
		if _, ok := err.(nomad.JobNotFoundError); ok {
			return fmt.Errorf("job not found: %s/%s", testFetchNamespace, testFetchJob)
		}
		return fmt.Errorf("failed to fetch job: %w", err)
	}

	PrintSuccess(fmt.Sprintf("Fetched job: %s", *job.ID))

	// Show some job details
	if IsVerbose() {
		PrintInfo(fmt.Sprintf("  Type: %s", stringPtrValue(job.Type)))
		PrintInfo(fmt.Sprintf("  Namespace: %s", stringPtrValue(job.Namespace)))
		PrintInfo(fmt.Sprintf("  Region: %s", stringPtrValue(job.Region)))
		if job.TaskGroups != nil {
			PrintInfo(fmt.Sprintf("  Task Groups: %d", len(job.TaskGroups)))
			for _, tg := range job.TaskGroups {
				PrintInfo(fmt.Sprintf("    - %s (count: %d)", stringPtrValue(tg.Name), intPtrValue(tg.Count)))
				if tg.Tasks != nil {
					for _, task := range tg.Tasks {
						PrintInfo(fmt.Sprintf("      - Task: %s (driver: %s)", task.Name, task.Driver))
						if task.Driver == "docker" {
							image := nomad.GetDockerImage(task)
							if image != "" {
								PrintInfo(fmt.Sprintf("        Image: %s", image))
							}
						}
					}
				}
			}
		}
	}

	// Normalize the job
	PrintInfo("Normalizing job (removing metadata)...")
	normalized := nomad.NormalizeJob(job, cfg.Changes.IgnoreFields)
	PrintSuccess("Job normalized")

	// Convert to HCL
	PrintInfo("Converting to HCL...")
	hclBytes, err := hcl.FormatJobAsHCL(normalized)
	if err != nil {
		return fmt.Errorf("failed to convert to HCL: %w", err)
	}

	// Normalize HCL for consistent output
	hclBytes = hcl.NormalizeHCL(hclBytes)

	PrintSuccess("Conversion complete")

	// Print the HCL
	fmt.Println("\n" + string(separator) + " HCL Output " + string(separator))
	fmt.Println(string(hclBytes))
	fmt.Println(string(separator) + separator + separator)

	// Show HCL size
	PrintInfo(fmt.Sprintf("HCL size: %d bytes", len(hclBytes)))

	return nil
}

// Helper functions to safely extract values from pointers

func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intPtrValue(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// separator is a visual separator for output
const separator = "================="
