package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wlame/nomad-changelog/internal/config"
	gitpkg "github.com/wlame/nomad-changelog/internal/git"
)

var (
	showJob       string
	showNamespace string
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show <commit-hash> [flags]",
	Short: "Show a specific version of a job",
	Long: `Display the job configuration at a specific commit.

This command retrieves and displays the job specification from a specific
point in history. You can use this to:
  • Review what changed in a specific commit
  • See the exact configuration before deploying
  • Compare different versions

For Git backend: Shows the file content from that commit
For GitHub API backend: Opens the commit view on GitHub

Examples:
  # Show a specific commit (interactive job selection)
  nomad-changelog show a1b2c3d4

  # Show specific job at a commit
  nomad-changelog show a1b2c3d4 --job web-app --namespace default

  # View on GitHub (if using GitHub API backend)
  nomad-changelog show a1b2c3d4`,
	Args: cobra.ExactArgs(1),
	RunE: showRun,
}

func init() {
	showCmd.Flags().StringVar(&showJob, "job", "", "Job name to show")
	showCmd.Flags().StringVar(&showNamespace, "namespace", "default", "Job namespace (used with --job)")

	rootCmd.AddCommand(showCmd)
}

func showRun(cmd *cobra.Command, args []string) error {
	commitHash := args[0]

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
		return showGitHubCommit(cfg, commitHash)
	}

	// For Git backend, show file content
	return showGitCommit(cfg, commitHash)
}

func showGitCommit(cfg *config.Config, commitHash string) error {
	PrintInfo(fmt.Sprintf("Loading commit %s...", commitHash))

	// Create Git client
	gitClient, err := gitpkg.NewClient(&cfg.Git)
	if err != nil {
		return fmt.Errorf("failed to create Git client: %w", err)
	}
	defer gitClient.Close()

	// Open repository
	repo, err := gitClient.OpenOrClone()
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Get commit info
	commits, err := repo.GetHistory("", 0)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	// Find the commit
	var matchingCommit *gitpkg.CommitInfo
	for _, c := range commits {
		if c.Hash == commitHash || c.FullHash == commitHash {
			matchingCommit = &c
			break
		}
	}

	if matchingCommit == nil {
		return fmt.Errorf("commit %s not found", commitHash)
	}

	// Display commit header
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📝 Commit %s\n", matchingCommit.Hash)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("Author: %s <%s>\n", matchingCommit.Author, matchingCommit.Email)
	fmt.Printf("Date:   %s\n", formatDate(matchingCommit.Date))
	fmt.Println()
	fmt.Printf("    %s\n", matchingCommit.Message)
	fmt.Println()

	// Determine which file to show
	var filePath string
	if showJob != "" {
		// User specified a job
		if showNamespace == "" {
			showNamespace = "default"
		}
		filePath = filepath.Join(showNamespace, showJob+".hcl")
	} else {
		// Show all files changed in this commit
		if len(matchingCommit.Files) == 0 {
			PrintWarning("No files changed in this commit")
			return nil
		}

		if len(matchingCommit.Files) == 1 {
			// Only one file, show it
			filePath = matchingCommit.Files[0]
		} else {
			// Multiple files, let user know
			fmt.Println("Files changed in this commit:")
			for i, file := range matchingCommit.Files {
				fmt.Printf("  %d) %s\n", i+1, file)
			}
			fmt.Println()
			fmt.Println("💡 Use --job and --namespace flags to view a specific file")
			fmt.Println()
			fmt.Println("Examples:")
			for _, file := range matchingCommit.Files {
				// Parse namespace/job from file path
				namespace := filepath.Dir(file)
				if namespace == "." {
					namespace = "default"
				}
				job := filepath.Base(file)
				job = job[:len(job)-4] // Remove .hcl extension

				fmt.Printf("  nomad-changelog show %s --job %s --namespace %s\n", commitHash, job, namespace)
			}
			return nil
		}
	}

	// Get file content at this commit
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📄 File: %s\n", filePath)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	content, err := repo.GetFileAtCommit(matchingCommit.FullHash, filePath)
	if err != nil {
		return fmt.Errorf("failed to get file at commit: %w", err)
	}

	// Display content
	fmt.Println(string(content))

	// Show deployment option
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💡 To deploy this version:")

	// Parse namespace/job from file path
	namespace := filepath.Dir(filePath)
	if namespace == "." {
		namespace = "default"
	}
	job := filepath.Base(filePath)
	job = job[:len(job)-4] // Remove .hcl extension

	fmt.Printf("  nomad-changelog deploy %s %s --namespace %s\n", commitHash, job, namespace)
	fmt.Println()

	return nil
}

func showGitHubCommit(cfg *config.Config, commitHash string) error {
	owner := cfg.Git.Owner
	repo := cfg.Git.Repo

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📝 Commit %s\n", commitHash)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	PrintInfo("Using GitHub API backend - viewing on GitHub")
	fmt.Println()

	if showJob != "" {
		if showNamespace == "" {
			showNamespace = "default"
		}
		filePath := filepath.Join(showNamespace, showJob+".hcl")

		// Link to specific file in commit
		fileURL := fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, commitHash, filePath)

		fmt.Printf("📄 Job: %s/%s\n", showNamespace, showJob)
		fmt.Printf("🔗 View on GitHub: %s\n", fileURL)
	} else {
		// Link to commit
		commitURL := fmt.Sprintf("https://github.com/%s/%s/commit/%s", owner, repo, commitHash)
		fmt.Printf("🔗 View commit on GitHub: %s\n", commitURL)
	}

	fmt.Println()
	fmt.Println("💡 To deploy this version:")
	if showJob != "" {
		fmt.Printf("  nomad-changelog deploy %s %s --namespace %s\n", commitHash, showJob, showNamespace)
	} else {
		fmt.Println("  nomad-changelog deploy <commit> <job> --namespace <namespace>")
	}
	fmt.Println()

	return nil
}
