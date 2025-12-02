package commands

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wlame/njgit/internal/config"
	gitpkg "github.com/wlame/njgit/internal/git"
)

var (
	historyJob       string
	historyNamespace string
	historyRegion    string
	historyLimit     int
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show commit history for tracked jobs",
	Long: `Display the Git commit history for Nomad job configurations.

This shows all changes made to your job configurations over time, allowing you to:
  • See when jobs were changed
  • Identify specific versions for rollback

You can filter by job name/namespace or show all changes.

Examples:
  # Show all history
  njgit history

  # Show last 10 commits
  njgit history --limit 10

  # Show history for specific job
  njgit history --job web-app --namespace default`,
	RunE: historyRun,
}

func init() {
	historyCmd.Flags().StringVar(&historyJob, "job", "", "Filter by job name")
	historyCmd.Flags().StringVar(&historyNamespace, "namespace", "default", "Job namespace (used with --job)")
	historyCmd.Flags().StringVar(&historyRegion, "region", "global", "Job region (used with --job)")
	historyCmd.Flags().IntVar(&historyLimit, "limit", 20, "Maximum number of commits to show (0 for unlimited)")

	rootCmd.AddCommand(historyCmd)
}

func historyRun(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check backend type
	backendType := cfg.Git.Backend
	if backendType == "" {
		backendType = "git"
	}

	if backendType != "git" {
		// For GitHub API backend, show link to GitHub
		return showGitHubHistory(cfg)
	}

	// For Git backend, show local history
	return showGitHistory(cfg)
}

func showGitHistory(cfg *config.Config) error {
	PrintInfo("Loading Git repository history...")

	// Open local repository
	repo, err := gitpkg.NewLocalRepository(cfg.Git.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to open repository at %s: %w", cfg.Git.LocalPath, err)
	}

	// Build file path filter if job specified
	var filePath string
	if historyJob != "" {
		if historyNamespace == "" {
			historyNamespace = "default"
		}
		if historyRegion == "" {
			historyRegion = "global"
		}
		filePath = filepath.Join(historyRegion, historyNamespace, historyJob+".hcl")
		PrintInfo(fmt.Sprintf("Filtering by job: %s/%s/%s", historyRegion, historyNamespace, historyJob))
	}

	// Get history
	commits, err := repo.GetHistory(filePath, historyLimit)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	if len(commits) == 0 {
		if filePath != "" {
			PrintWarning(fmt.Sprintf("No commits found for %s", filePath))
			fmt.Println()
			fmt.Println("💡 Tips:")
			fmt.Println("  • Check if the job name and namespace are correct")
			fmt.Println("  • Verify the job has been synced at least once")
		} else {
			PrintWarning("No commits found in repository")
			fmt.Println()
			fmt.Println("💡 Tip: Run 'njgit sync' to start tracking changes")
		}
		return nil
	}

	// Display history - one line per commit
	for _, commit := range commits {
		// Format date
		dateStr := formatDate(commit.Date)

		// Extract job name from files if available
		jobName := ""
		if len(commit.Files) > 0 {
			// Files are in format: region/namespace/jobname.hcl
			// Extract the full job path and remove .hcl extension
			filePath := commit.Files[0]
			jobName = strings.TrimSuffix(filePath, ".hcl")
		}

		// Get first line of commit message
		messageLines := strings.Split(commit.Message, "\n")
		firstLine := messageLines[0]

		// Print one-line format: date hash job message
		if jobName != "" {
			fmt.Printf("%s %s %s %s\n", dateStr, commit.Hash, jobName, firstLine)
		} else {
			fmt.Printf("%s %s %s\n", dateStr, commit.Hash, firstLine)
		}
	}

	return nil
}

func showGitHubHistory(cfg *config.Config) error {
	owner := cfg.Git.Owner
	repo := cfg.Git.Repo
	branch := cfg.Git.Branch
	if branch == "" {
		branch = "main"
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📜 GitHub History")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	PrintInfo("Using GitHub API backend - history available on GitHub")
	fmt.Println()

	// Build GitHub URLs
	if historyJob != "" {
		if historyNamespace == "" {
			historyNamespace = "default"
		}
		filePath := filepath.Join(historyNamespace, historyJob+".hcl")
		fileURL := fmt.Sprintf("https://github.com/%s/%s/commits/%s/%s", owner, repo, branch, filePath)

		fmt.Printf("📄 Job: %s/%s\n", historyNamespace, historyJob)
		fmt.Printf("🔗 View history: %s\n", fileURL)
	} else {
		repoURL := fmt.Sprintf("https://github.com/%s/%s/commits/%s", owner, repo, branch)
		fmt.Printf("🔗 View all commits: %s\n", repoURL)
	}

	fmt.Println()
	fmt.Println("💡 Using GitHub API backend:")
	fmt.Println("  • History is stored on GitHub")
	fmt.Println("  • Click the link above to view commits in your browser")
	fmt.Println("  • Use 'njgit show <commit-hash>' to view specific versions")
	fmt.Println("  • Use 'njgit deploy <commit-hash> <job>' to rollback")
	fmt.Println()

	return nil
}

// formatDate formats a time.Time into YYYY-MM-DD HH:MM format
func formatDate(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}
