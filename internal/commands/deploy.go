package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wlame/nomad-changelog/internal/config"
	gitpkg "github.com/wlame/nomad-changelog/internal/git"
	"github.com/wlame/nomad-changelog/internal/hcl"
	"github.com/wlame/nomad-changelog/internal/nomad"
)

var (
	deployNamespace string
	deployDryRun    bool
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy <commit-hash> <job-name> [flags]",
	Short: "Deploy a specific version of a job to Nomad",
	Long: `Deploy a job configuration from a specific commit to your Nomad cluster.

This is the rollback/redeploy feature that allows you to:
  • Restore a previous version of a job
  • Deploy a specific historical configuration
  • Recover from bad deployments

The command will:
  1. Retrieve the job specification from the specified commit
  2. Parse the HCL configuration
  3. Submit it to Nomad for deployment

IMPORTANT: This will deploy the exact configuration from that commit,
potentially overwriting current job settings.

Examples:
  # Deploy a previous version
  nomad-changelog deploy a1b2c3d4 web-app

  # Deploy with specific namespace
  nomad-changelog deploy a1b2c3d4 web-app --namespace production

  # Preview what would be deployed (dry run)
  nomad-changelog deploy a1b2c3d4 web-app --dry-run

Workflow:
  1. Find the commit: nomad-changelog history
  2. Review the version: nomad-changelog show <commit>
  3. Deploy it: nomad-changelog deploy <commit> <job>`,
	Args: cobra.ExactArgs(2),
	RunE: deployRun,
}

func init() {
	deployCmd.Flags().StringVar(&deployNamespace, "namespace", "default", "Nomad namespace")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Show what would be deployed without actually deploying")

	rootCmd.AddCommand(deployCmd)
}

func deployRun(cmd *cobra.Command, args []string) error {
	commitHash := args[0]
	jobName := args[1]

	if deployNamespace == "" {
		deployNamespace = "default"
	}

	// Load configuration
	cfg, err := config.Load(GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get job HCL from the commit
	PrintInfo(fmt.Sprintf("Loading job %s/%s from commit %s...", deployNamespace, jobName, commitHash))

	jobHCL, err := getJobFromCommit(cfg, commitHash, deployNamespace, jobName)
	if err != nil {
		return err
	}

	// Parse HCL to Job struct
	// We need to pass the Nomad address because ParseHCL makes a request to Nomad
	PrintInfo("Parsing job specification...")
	job, err := hcl.ParseHCL(jobHCL, cfg.Nomad.Address)
	if err != nil {
		return fmt.Errorf("failed to parse HCL: %w", err)
	}

	// Ensure namespace is set
	if job.Namespace == nil || *job.Namespace == "" {
		job.Namespace = &deployNamespace
	}

	if deployDryRun {
		// Dry run - just show what would be deployed
		fmt.Println()
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("🔍 DRY RUN - Would deploy:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		fmt.Printf("Job:       %s\n", *job.ID)
		fmt.Printf("Namespace: %s\n", *job.Namespace)
		fmt.Printf("Type:      %s\n", *job.Type)
		if job.Region != nil {
			fmt.Printf("Region:    %s\n", *job.Region)
		}
		if job.Datacenters != nil && len(job.Datacenters) > 0 {
			fmt.Printf("Datacenters: %v\n", job.Datacenters)
		}
		fmt.Println()
		fmt.Println("Job specification:")
		fmt.Println(string(jobHCL))
		fmt.Println()
		PrintInfo("This is a dry run - no changes were made to Nomad")
		fmt.Println("Remove --dry-run flag to actually deploy")
		return nil
	}

	// Connect to Nomad
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

	// Deploy to Nomad
	PrintInfo(fmt.Sprintf("Deploying %s/%s to Nomad...", *job.Namespace, *job.ID))

	evalID, err := nomadClient.DeployJob(job)
	if err != nil {
		return fmt.Errorf("failed to deploy job: %w", err)
	}

	// Success!
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	PrintSuccess(fmt.Sprintf("✅ Successfully deployed %s/%s", *job.Namespace, *job.ID))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("Evaluation ID: %s\n", evalID)
	fmt.Printf("Commit:        %s\n", commitHash)
	fmt.Println()
	fmt.Println("💡 Monitor deployment:")
	fmt.Printf("  nomad eval status %s\n", evalID)
	fmt.Printf("  nomad job status %s\n", *job.ID)
	fmt.Println()

	return nil
}

func getJobFromCommit(cfg *config.Config, commitHash, namespace, jobName string) ([]byte, error) {
	// Check backend type
	backendType := cfg.Git.Backend
	if backendType == "" {
		backendType = "git"
	}

	if backendType != "git" {
		return nil, fmt.Errorf("deploy command currently only supports git backend\n\n"+
			"For GitHub API backend:\n"+
			"  1. View the file on GitHub: nomad-changelog show %s --job %s\n"+
			"  2. Copy the job specification\n"+
			"  3. Deploy manually: nomad job run <file>", commitHash, jobName)
	}

	// Create Git client
	gitClient, err := gitpkg.NewClient(&cfg.Git)
	if err != nil {
		return nil, fmt.Errorf("failed to create Git client: %w", err)
	}
	defer gitClient.Close()

	// Open repository
	repo, err := gitClient.OpenOrClone()
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// Get commits to find full hash
	commits, err := repo.GetHistory("", 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	// Find the commit
	var fullHash string
	for _, c := range commits {
		if c.Hash == commitHash || c.FullHash == commitHash {
			fullHash = c.FullHash
			break
		}
	}

	if fullHash == "" {
		return nil, fmt.Errorf("commit %s not found", commitHash)
	}

	// Build file path
	filePath := filepath.Join(namespace, jobName+".hcl")

	// Get file content at this commit
	content, err := repo.GetFileAtCommit(fullHash, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file %s at commit %s: %w", filePath, commitHash, err)
	}

	return content, nil
}
