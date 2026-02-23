package commands

import (
	"github.com/spf13/cobra"
)

// Root returns the root cobra command with all subcommands attached.
func Root() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gophercaptain",
		Short: "Deploy Go services from GitHub releases",
		Long:  "GopherCaptain deploys Go services from GitHub releases with systemd, nginx, and MariaDB.",
	}

	cmd.AddCommand(initCmd())
	cmd.AddCommand(deployCmd())
	cmd.AddCommand(upgradeCmd())
	cmd.AddCommand(rollbackCmd())
	cmd.AddCommand(removeCmd())
	cmd.AddCommand(listCmd())
	cmd.AddCommand(statusCmd())
	cmd.AddCommand(inspectCmd())
	cmd.AddCommand(versionCmd())

	return cmd
}
