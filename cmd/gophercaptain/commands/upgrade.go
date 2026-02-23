package commands

import (
	"fmt"

	"github.com/ecairns22/GopherCaptain/internal/orchestrator"
	"github.com/spf13/cobra"
)

func upgradeCmd() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "upgrade <service>",
		Short: "Upgrade to a new version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			orc, cleanup, err := buildOrchestrator()
			if err != nil {
				return err
			}
			defer cleanup()

			req := orchestrator.UpgradeRequest{
				Name:    name,
				Version: version,
			}

			result, err := orc.Upgrade(cmd.Context(), req)
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			if result.RolledBack {
				fmt.Fprintf(w, "✗ %s upgrade to %s failed: %s\n", result.Name, result.NewVersion, result.RollbackMsg)
				return fmt.Errorf("upgrade failed, rolled back to %s", result.OldVersion)
			}

			fmt.Fprintf(w, "✓ %s upgraded %s → %s\n", result.Name, result.OldVersion, result.NewVersion)
			return nil
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", "latest", "Target version (default: latest)")

	return cmd
}
