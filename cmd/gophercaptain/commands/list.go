package commands

import (
	"fmt"
	"text/tabwriter"

	"github.com/ecairns22/GopherCaptain/internal/runner"
	"github.com/ecairns22/GopherCaptain/internal/systemd"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show all deployed services",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := buildStateOnly()
			if err != nil {
				return err
			}
			defer store.Close()

			services, err := store.ListServices(cmd.Context())
			if err != nil {
				return err
			}

			if len(services) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No services deployed. Run 'gophercaptain deploy <repo>' to get started.")
				return nil
			}

			// Query live status via systemd
			r := &runner.OSRunner{}
			sys := systemd.New(r, unitDir)

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tPORT\tROUTE\tSTATUS")

			for _, svc := range services {
				route := svc.RouteValue
				if route == "" {
					route = "—"
				}

				status := "stopped"
				if active, _ := sys.IsActive(cmd.Context(), svc.Name); active {
					status = "running"
				}

				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", svc.Name, svc.Version, svc.Port, route, status)
			}

			w.Flush()
			return nil
		},
	}
}
