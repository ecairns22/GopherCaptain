package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func rollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback <service>",
		Short: "Swap back to the previous version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			orc, cleanup, err := buildOrchestrator()
			if err != nil {
				return err
			}
			defer cleanup()

			prevVersion, err := orc.Rollback(cmd.Context(), name)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ %s rolled back to %s\n", name, prevVersion)
			return nil
		},
	}
}
