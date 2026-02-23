package commands

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/ecairns22/GopherCaptain/internal/orchestrator"
	"github.com/spf13/cobra"
)

func removeCmd() *cobra.Command {
	var (
		dropDB bool
		yes    bool
	)

	cmd := &cobra.Command{
		Use:   "remove <service>",
		Short: "Stop and remove a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			orc, cleanup, err := buildOrchestrator()
			if err != nil {
				return err
			}
			defer cleanup()

			// If drop-db, confirm unless --yes
			if dropDB && !yes {
				svc, err := orc.GetService(cmd.Context(), name)
				if err != nil {
					return err
				}
				if svc.DBName != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "This will drop database %q and user %q. Continue? [y/N] ", svc.DBName, svc.DBUser)
					reader := bufio.NewReader(cmd.InOrStdin())
					answer, _ := reader.ReadString('\n')
					answer = strings.TrimSpace(strings.ToLower(answer))
					if answer != "y" && answer != "yes" {
						fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
						return nil
					}
				}
			}

			w := cmd.OutOrStdout()
			step := func(msg string) {
				fmt.Fprintln(w, msg)
			}

			req := orchestrator.RemoveRequest{
				Name:   name,
				DropDB: dropDB,
				Yes:    yes,
			}

			if err := orc.Remove(cmd.Context(), req, step); err != nil {
				return err
			}

			fmt.Fprintf(w, "✓ %s removed\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dropDB, "drop-db", false, "Also drop the MariaDB database and user")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")

	return cmd
}
