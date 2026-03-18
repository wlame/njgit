package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wlame/njgit/internal/config"
	gitpkg "github.com/wlame/njgit/internal/git"
	"github.com/wlame/njgit/internal/web"
)

var (
	serveBind string
	servePort int
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start web dashboard",
	Long:  "Start a web UI dashboard to browse commit history and view HCL configs",
	RunE:  serveRun,
}

func init() {
	serveCmd.Flags().StringVar(&serveBind, "bind", "0.0.0.0", "Address to bind the HTTP server")
	serveCmd.Flags().IntVar(&servePort, "port", 8800, "Port for the HTTP server")
	rootCmd.AddCommand(serveCmd)
}

func serveRun(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Git.Backend != "git" {
		return fmt.Errorf("web dashboard only supports git backend (current: %s)", cfg.Git.Backend)
	}

	repo, err := gitpkg.NewLocalRepository(cfg.Git.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	srv := web.NewServer(repo, serveBind, servePort)
	PrintInfo(fmt.Sprintf("Starting njgit dashboard at http://%s:%d", serveBind, servePort))
	return srv.Start()
}
