package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/spf13/cobra"
	"github.com/wlame/ndiff/internal/backend"
	"github.com/wlame/ndiff/internal/config"
	"github.com/wlame/ndiff/internal/hcl"
	"github.com/wlame/ndiff/internal/nomad"
)

var (
	// Flags for sync command
	syncDryRun bool
	syncNoPush bool
	syncJobs   string // Comma-separated list of jobs to sync
)

// syncCmd represents the sync command
// This is the main command that syncs Nomad jobs to Git
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync Nomad jobs to Git repository",
	Long: `Fetch job specifications from Nomad, convert them to HCL, and store in Git.

This command:
  1. Connects to Nomad and fetches job specifications
  2. Normalizes jobs (removes Nomad metadata)
  3. Converts jobs to HCL format
  4. Compares with existing files in Git
  5. For changed jobs: writes files, creates commits, and pushes

Each changed job gets its own commit with a detailed message showing what changed.

Examples:
  # Sync all configured jobs
  ndiff sync

  # Sync specific jobs only
  ndiff sync --jobs web-server,api-server

  # Dry run (show what would change without committing)
  ndiff sync --dry-run

  # Commit locally but don't push to remote
  ndiff sync --no-push`,
	RunE: syncRun,
}

func init() {
	// Add flags
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false,
		"Show what would change without making any commits")
	syncCmd.Flags().BoolVar(&syncNoPush, "no-push", false,
		"Commit changes locally but don't push to remote")
	syncCmd.Flags().StringVar(&syncJobs, "jobs", "",
		"Comma-separated list of jobs to sync (default: all configured jobs)")

	// Add to root command
	rootCmd.AddCommand(syncCmd)
}

// syncRun executes the sync command
func syncRun(cmd *cobra.Command, args []string) error {
	PrintInfo("Starting sync...")

	// 1. Load configuration
	PrintInfo("Loading configuration...")
	cfg, err := config.Load(GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	PrintInfo(fmt.Sprintf("Nomad: %s", cfg.Nomad.Address))

	// Display backend info
	backendType := cfg.Git.Backend
	if backendType == "" {
		backendType = "git"
	}
	if backendType == "git" {
		PrintInfo(fmt.Sprintf("Backend: git (%s)", cfg.Git.URL))
	} else {
		PrintInfo(fmt.Sprintf("Backend: github-api (%s/%s)", cfg.Git.Owner, cfg.Git.Repo))
	}

	// 2. Create Nomad client
	PrintInfo("Connecting to Nomad...")
	nomadAuth, err := nomad.ResolveAuth(&cfg.Nomad, "", "")
	if err != nil {
		return fmt.Errorf("failed to resolve Nomad auth: %w", err)
	}

	nomadClient, err := nomad.NewClient(nomadAuth)
	if err != nil {
		return fmt.Errorf("failed to create Nomad client: %w", err)
	}
	defer nomadClient.Close()

	// Test Nomad connectivity
	if err := nomadClient.Ping(); err != nil {
		return fmt.Errorf("failed to connect to Nomad: %w", err)
	}
	PrintSuccess("Connected to Nomad")

	// 3. Create backend
	if !syncDryRun {
		PrintInfo("Setting up backend...")
		backend, err := backend.NewBackend(&cfg.Git)
		if err != nil {
			return fmt.Errorf("failed to create backend: %w", err)
		}
		defer backend.Close()

		if err := backend.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize backend: %w", err)
		}

		PrintSuccess(fmt.Sprintf("Backend ready (%s)", backend.GetName()))

		// Perform the sync
		return performSync(cfg, nomadClient, backend)
	} else {
		// Dry run - no backend operations
		return performDryRun(cfg, nomadClient)
	}
}

// performSync performs the actual sync with backend operations
func performSync(cfg *config.Config, nomadClient *nomad.Client, backend backend.Backend) error {
	// Filter jobs if --jobs flag was provided
	jobsToSync := getJobsToSync(cfg)

	PrintInfo(fmt.Sprintf("Syncing %d jobs...", len(jobsToSync)))

	var changedJobs []string
	var errors []error

	// Process each job
	for _, jobCfg := range jobsToSync {
		changed, err := syncJob(cfg, nomadClient, backend, jobCfg)
		if err != nil {
			// Log error but continue with other jobs
			PrintError(fmt.Errorf("job %s/%s: %w", jobCfg.Namespace, jobCfg.Name, err))
			errors = append(errors, err)
			continue
		}

		if changed {
			changedJobs = append(changedJobs, fmt.Sprintf("%s/%s", jobCfg.Namespace, jobCfg.Name))
		}
	}

	// Report results
	if len(changedJobs) > 0 {
		PrintSuccess(fmt.Sprintf("Synced %d jobs with changes:", len(changedJobs)))
		for _, job := range changedJobs {
			fmt.Printf("  - %s\n", job)
		}
	} else {
		PrintInfo("No changes detected")
	}

	if len(errors) > 0 {
		PrintWarning(fmt.Sprintf("%d jobs had errors", len(errors)))
		return fmt.Errorf("sync completed with %d errors", len(errors))
	}

	return nil
}

// syncJob syncs a single job
// Returns true if the job changed, false otherwise
func syncJob(cfg *config.Config, nomadClient *nomad.Client, backend backend.Backend, jobCfg config.JobConfig) (bool, error) {
	jobPath := fmt.Sprintf("%s/%s", jobCfg.Namespace, jobCfg.Name)
	PrintInfo(fmt.Sprintf("Checking %s...", jobPath))

	// 1. Fetch job from Nomad
	job, err := nomadClient.FetchJobSpec(jobCfg.Namespace, jobCfg.Name)
	if err != nil {
		if _, ok := err.(nomad.JobNotFoundError); ok {
			// Job not found - warn but don't error
			PrintWarning(fmt.Sprintf("%s: Job not found in Nomad (skipping)", jobPath))
			return false, nil
		}
		return false, fmt.Errorf("failed to fetch job: %w", err)
	}

	// 2. Normalize the job
	normalized := nomad.NormalizeJob(job, cfg.Changes.IgnoreFields)

	// 3. Convert to HCL
	hclBytes, err := hcl.FormatJobAsHCL(normalized)
	if err != nil {
		return false, fmt.Errorf("failed to convert to HCL: %w", err)
	}

	// Normalize HCL for consistent comparison
	hclBytes = hcl.NormalizeHCL(hclBytes)

	// 4. Check if file exists and compare
	filePath := filepath.Join(jobCfg.Namespace, jobCfg.Name+".hcl")
	fileExists, err := backend.FileExists(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to check if file exists: %w", err)
	}

	var hasChanges bool
	var changeDescription string

	if fileExists {
		// Read existing file
		existingContent, err := backend.ReadFile(filePath)
		if err != nil {
			return false, fmt.Errorf("failed to read existing file: %w", err)
		}

		// Compare
		if hcl.CompareHCL(existingContent, hclBytes) {
			// No changes
			if IsVerbose() {
				PrintInfo(fmt.Sprintf("  %s: No changes", jobPath))
			}
			return false, nil
		}

		hasChanges = true
		changeDescription = detectChanges(existingContent, hclBytes, job)
	} else {
		// New file
		hasChanges = true
		changeDescription = "Initial version"
	}

	if !hasChanges {
		return false, nil
	}

	// 5. Write the new HCL file
	PrintInfo(fmt.Sprintf("  %s: CHANGED", jobPath))
	if IsVerbose() && changeDescription != "" {
		fmt.Printf("    %s\n", changeDescription)
	}

	if err := backend.WriteFile(filePath, hclBytes); err != nil {
		return false, fmt.Errorf("failed to write file: %w", err)
	}

	// 6. Create commit
	commitMsg := buildCommitMessage(jobPath, changeDescription)
	hash, err := backend.Commit(commitMsg)
	if err != nil {
		return false, fmt.Errorf("failed to commit: %w", err)
	}

	if IsVerbose() && hash != "" {
		PrintInfo(fmt.Sprintf("  Committed: %s", hash[:8]))
	}

	// 7. Push (unless --no-push)
	if !syncNoPush {
		if err := backend.Push(); err != nil {
			return false, fmt.Errorf("failed to push: %w", err)
		}
		if IsVerbose() {
			PrintInfo("  Pushed to remote")
		}
	}

	return true, nil
}

// performDryRun performs a dry run (no Git operations)
func performDryRun(cfg *config.Config, nomadClient *nomad.Client) error {
	jobsToSync := getJobsToSync(cfg)

	PrintInfo(fmt.Sprintf("DRY RUN: Checking %d jobs...", len(jobsToSync)))

	var changes []string

	for _, jobCfg := range jobsToSync {
		jobPath := fmt.Sprintf("%s/%s", jobCfg.Namespace, jobCfg.Name)

		// Fetch and normalize
		job, err := nomadClient.FetchJobSpec(jobCfg.Namespace, jobCfg.Name)
		if err != nil {
			if _, ok := err.(nomad.JobNotFoundError); ok {
				PrintWarning(fmt.Sprintf("%s: Not found in Nomad", jobPath))
				continue
			}
			PrintError(fmt.Errorf("%s: %w", jobPath, err))
			continue
		}

		normalized := nomad.NormalizeJob(job, cfg.Changes.IgnoreFields)
		hclBytes, err := hcl.FormatJobAsHCL(normalized)
		if err != nil {
			PrintError(fmt.Errorf("%s: %w", jobPath, err))
			continue
		}

		// In dry run, we just report that the job would be synced
		changes = append(changes, jobPath)
		PrintInfo(fmt.Sprintf("  %s: Would sync (%d bytes HCL)", jobPath, len(hclBytes)))
	}

	if len(changes) > 0 {
		PrintSuccess(fmt.Sprintf("DRY RUN: Would sync %d jobs", len(changes)))
	} else {
		PrintInfo("DRY RUN: No jobs to sync")
	}

	return nil
}

// getJobsToSync returns the list of jobs to sync based on --jobs flag
func getJobsToSync(cfg *config.Config) []config.JobConfig {
	if syncJobs == "" {
		// Sync all configured jobs
		return cfg.Jobs
	}

	// Filter to only specified jobs
	jobNames := strings.Split(syncJobs, ",")
	jobSet := make(map[string]bool)
	for _, name := range jobNames {
		jobSet[strings.TrimSpace(name)] = true
	}

	var filtered []config.JobConfig
	for _, jobCfg := range cfg.Jobs {
		if jobSet[jobCfg.Name] {
			filtered = append(filtered, jobCfg)
		}
	}

	return filtered
}

// buildCommitMessage builds a commit message for a job change
func buildCommitMessage(jobPath, changeDescription string) string {
	var msg strings.Builder

	msg.WriteString(fmt.Sprintf("Update %s", jobPath))

	if changeDescription != "" && changeDescription != "Initial version" {
		msg.WriteString("\n\n")
		msg.WriteString("Changes:\n")
		msg.WriteString(changeDescription)
	} else if changeDescription == "Initial version" {
		msg.WriteString("\n\nInitial version")
	}

	return msg.String()
}

// detectChanges attempts to identify what changed in a job
// This is a simple implementation - just mentions that something changed
// A more sophisticated version would parse the HCL and identify specific changes
func detectChanges(oldContent, newContent []byte, job *api.Job) string {
	// Simple implementation: just note that something changed
	// TODO: Implement detailed change detection
	return "Job configuration updated"
}
