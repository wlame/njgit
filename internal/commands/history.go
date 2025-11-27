package commands

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wlame/nomad-changelog/internal/config"
	gitpkg "github.com/wlame/nomad-changelog/internal/git"
)

var (
	historyJob       string
	historyNamespace string
	historyLimit     int
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show commit history for tracked jobs",
	Long: `Display the Git commit history for Nomad job configurations.

This shows all changes made to your job configurations over time, allowing you to:
  • See when jobs were changed
  • View who made the changes
  • Identify specific versions for rollback

You can filter by job name/namespace or show all changes.

Examples:
  # Show all history
  nomad-changelog history

  # Show last 10 commits
  nomad-changelog history --limit 10

  # Show history for specific job
  nomad-changelog history --job web-app --namespace default

  # Verbose output with file names
  nomad-changelog history --verbose`,
	RunE: historyRun,
}

func init() {
	historyCmd.Flags().StringVar(&historyJob, "job", "", "Filter by job name")
	historyCmd.Flags().StringVar(&historyNamespace, "namespace", "default", "Job namespace (used with --job)")
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

	// Create Git client
	gitClient, err := gitpkg.NewClient(&cfg.Git)
	if err != nil {
		return fmt.Errorf("failed to create Git client: %w", err)
	}
	defer gitClient.Close()

	// Open or clone repository
	repo, err := gitClient.OpenOrClone()
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Build file path filter if job specified
	var filePath string
	if historyJob != "" {
		if historyNamespace == "" {
			historyNamespace = "default"
		}
		filePath = filepath.Join(historyNamespace, historyJob+".hcl")
		PrintInfo(fmt.Sprintf("Filtering by job: %s/%s", historyNamespace, historyJob))
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
			fmt.Println("💡 Tip: Run 'nomad-changelog sync' to start tracking changes")
		}
		return nil
	}

	// Display history
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📜 Commit History")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	for i, commit := range commits {
		// Format date
		dateStr := formatDate(commit.Date)

		// Print commit info
		fmt.Printf("commit %s\n", commit.Hash)
		fmt.Printf("Author: %s <%s>\n", commit.Author, commit.Email)
		fmt.Printf("Date:   %s\n", dateStr)
		fmt.Println()

		// Print commit message (indent)
		messageLines := strings.Split(commit.Message, "\n")
		for _, line := range messageLines {
			fmt.Printf("    %s\n", line)
		}

		// Print files if verbose
		if IsVerbose() && len(commit.Files) > 0 {
			fmt.Println()
			for _, file := range commit.Files {
				fmt.Printf("    📄 %s\n", file)
			}
		}

		// Separator between commits
		if i < len(commits)-1 {
			fmt.Println()
			fmt.Println("────────────────────────────────────────────")
			fmt.Println()
		}
	}

	// Show summary
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Showing %d commit(s)\n", len(commits))
	fmt.Println()

	// Show next steps
	fmt.Println("💡 Next steps:")
	fmt.Println("  • View a specific version: nomad-changelog show <commit-hash>")
	fmt.Println("  • Deploy a previous version: nomad-changelog deploy <commit-hash> <job>")
	if !IsVerbose() {
		fmt.Println("  • Show changed files: nomad-changelog history --verbose")
	}
	fmt.Println()

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
	fmt.Println("  • Use 'nomad-changelog show <commit-hash>' to view specific versions")
	fmt.Println("  • Use 'nomad-changelog deploy <commit-hash> <job>' to rollback")
	fmt.Println()

	return nil
}

// formatDate formats a time.Time into a human-readable string
func formatDate(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	// If less than 24 hours ago, show relative time
	if diff < 24*time.Hour {
		if diff < time.Hour {
			minutes := int(diff.Minutes())
			if minutes <= 1 {
				return "1 minute ago"
			}
			return fmt.Sprintf("%d minutes ago", minutes)
		}
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	// If less than 7 days ago, show days
	if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}

	// Otherwise show full date
	return t.Format("Mon Jan 2 15:04:05 2006 -0700")
}
