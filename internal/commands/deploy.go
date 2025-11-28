package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wlame/njgit/internal/config"
	gitpkg "github.com/wlame/njgit/internal/git"
	"github.com/wlame/njgit/internal/hcl"
	"github.com/wlame/njgit/internal/nomad"
)

var (
	deployNamespace string
	deployRegion    string
	deployDryRun    bool
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy <commit-hash> [job-name] [flags]",
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

If job-name is not provided, it will be automatically detected from the files
changed in the commit. This works when the commit only affects a single job.

IMPORTANT: This will deploy the exact configuration from that commit,
potentially overwriting current job settings.

Examples:
  # Deploy with auto-detected job name (typical usage)
  njgit deploy a1b2c3d4

  # Deploy a specific job (useful if commit was made manually with multiple jobs)
  njgit deploy a1b2c3d4 web-app

  # Deploy with specific namespace
  njgit deploy a1b2c3d4 web-app --namespace production

  # Preview what would be deployed (dry run)
  njgit deploy a1b2c3d4 --dry-run

Workflow:
  1. Find the commit: njgit history
  2. Review the version: njgit show <commit>
  3. Deploy it: njgit deploy <commit>`,
	Args: cobra.RangeArgs(1, 2),
	RunE: deployRun,
}

func init() {
	deployCmd.Flags().StringVar(&deployNamespace, "namespace", "default", "Nomad namespace")
	deployCmd.Flags().StringVar(&deployRegion, "region", "global", "Nomad region")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Show what would be deployed without actually deploying")

	rootCmd.AddCommand(deployCmd)
}

func deployRun(cmd *cobra.Command, args []string) error {
	commitHash := args[0]
	var jobName string

	// Job name can be provided or auto-detected
	if len(args) >= 2 {
		jobName = args[1]
	}

	if deployNamespace == "" {
		deployNamespace = "default"
	}
	if deployRegion == "" {
		deployRegion = "global"
	}

	// Load configuration
	cfg, err := config.Load(GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Auto-detect job name if not provided
	if jobName == "" {
		PrintInfo(fmt.Sprintf("Auto-detecting job name from commit %s...", commitHash))
		detectedJob, err := detectJobFromCommit(cfg, commitHash, deployRegion, deployNamespace)
		if err != nil {
			return err
		}
		jobName = detectedJob
		PrintInfo(fmt.Sprintf("Detected job: %s", jobName))
	}

	// Get job HCL from the commit
	PrintInfo(fmt.Sprintf("Loading job %s/%s/%s from commit %s...", deployRegion, deployNamespace, jobName, commitHash))

	jobHCL, err := getJobFromCommit(cfg, commitHash, deployRegion, deployNamespace, jobName)
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

func detectJobFromCommit(cfg *config.Config, commitHash, region, namespace string) (string, error) {
	// Check backend type
	backendType := cfg.Git.Backend
	if backendType == "" {
		backendType = "git"
	}

	if backendType != "git" {
		return "", fmt.Errorf("auto-detection only supports git backend\n\n"+
			"Please specify job name explicitly:\n"+
			"  njgit deploy %s <job-name>", commitHash)
	}

	// Open local repository
	repo, err := gitpkg.NewLocalRepository(cfg.Git.LocalPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository at %s: %w", cfg.Git.LocalPath, err)
	}

	// Get commits to find full hash and commit info
	commits, err := repo.GetHistory("", 0)
	if err != nil {
		return "", fmt.Errorf("failed to get history: %w", err)
	}

	// Find the commit
	var commitInfo *gitpkg.CommitInfo
	for i := range commits {
		if commits[i].Hash == commitHash || commits[i].FullHash == commitHash {
			commitInfo = &commits[i]
			break
		}
	}

	if commitInfo == nil {
		return "", fmt.Errorf("commit %s not found", commitHash)
	}

	// Extract job names from changed files
	// Files follow pattern: <region>/<namespace>/<job-name>.hcl
	jobNames := make(map[string]bool)
	targetPath := filepath.Join(region, namespace)

	for _, file := range commitInfo.Files {
		// Parse the file path
		dir := filepath.Dir(file)
		base := filepath.Base(file)

		// Check if it's in the target region/namespace
		if dir != targetPath {
			continue
		}

		// Extract job name (remove .hcl extension)
		if filepath.Ext(base) == ".hcl" {
			jobName := base[:len(base)-4]
			jobNames[jobName] = true
		}
	}

	// Check how many unique jobs we found
	if len(jobNames) == 0 {
		return "", fmt.Errorf("no job files found in %s/%s for commit %s\n\n"+
			"Changed files:\n"+
			"%v\n\n"+
			"Please specify job name explicitly:\n"+
			"  njgit deploy %s <job-name> --region %s --namespace %s",
			region, namespace, commitHash, commitInfo.Files, commitHash, region, namespace)
	}

	if len(jobNames) > 1 {
		// Convert map to slice for display
		jobs := make([]string, 0, len(jobNames))
		for job := range jobNames {
			jobs = append(jobs, job)
		}

		return "", fmt.Errorf("commit affects multiple jobs: %v\n\n"+
			"Please specify which job to deploy:\n"+
			"  njgit deploy %s <job-name> --region %s --namespace %s",
			jobs, commitHash, region, namespace)
	}

	// Return the single job name
	for jobName := range jobNames {
		return jobName, nil
	}

	return "", fmt.Errorf("unexpected error detecting job name")
}

func getJobFromCommit(cfg *config.Config, commitHash, region, namespace, jobName string) ([]byte, error) {
	// Check backend type
	backendType := cfg.Git.Backend
	if backendType == "" {
		backendType = "git"
	}

	if backendType != "git" {
		return nil, fmt.Errorf("deploy command currently only supports git backend\n\n"+
			"For GitHub API backend:\n"+
			"  1. View the file on GitHub: njgit show %s --job %s\n"+
			"  2. Copy the job specification\n"+
			"  3. Deploy manually: nomad job run <file>", commitHash, jobName)
	}

	// Open local repository
	repo, err := gitpkg.NewLocalRepository(cfg.Git.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository at %s: %w", cfg.Git.LocalPath, err)
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
	filePath := filepath.Join(region, namespace, jobName+".hcl")

	// Get file content at this commit
	content, err := repo.GetFileAtCommit(fullHash, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file %s at commit %s: %w", filePath, commitHash, err)
	}

	return content, nil
}
