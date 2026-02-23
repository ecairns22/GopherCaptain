package commands

import (
	"fmt"

	"github.com/ecairns22/GopherCaptain/internal/runner"
	"github.com/ecairns22/GopherCaptain/internal/systemd"
	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <service>",
		Short: "Detailed status for a deployed service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			store, err := buildStateOnly()
			if err != nil {
				return err
			}
			defer store.Close()

			svc, err := store.GetService(cmd.Context(), name)
			if err != nil {
				return fmt.Errorf("service %q not found; run 'gophercaptain list' to see deployed services", name)
			}

			// Live systemd status
			r := &runner.OSRunner{}
			sys := systemd.New(r, unitDir)
			status := "stopped"
			if active, _ := sys.IsActive(cmd.Context(), svc.Name); active {
				status = "running"
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Service:     %s\n", svc.Name)
			fmt.Fprintf(w, "Repo:        %s\n", svc.Repo)
			fmt.Fprintf(w, "Version:     %s\n", svc.Version)
			if svc.PrevVersion != "" {
				fmt.Fprintf(w, "Previous:    %s\n", svc.PrevVersion)
			}
			fmt.Fprintf(w, "Port:        %d\n", svc.Port)
			if svc.RouteValue != "" {
				fmt.Fprintf(w, "Route:       %s (%s)\n", svc.RouteValue, svc.RouteType)
			}
			if svc.DBName != "" {
				fmt.Fprintf(w, "Database:    %s\n", svc.DBName)
			}
			fmt.Fprintf(w, "Status:      %s\n", status)
			fmt.Fprintf(w, "Deployed:    %s\n", svc.DeployedAt.Format("2006-01-02 15:04:05"))
			fmt.Fprintf(w, "Updated:     %s\n", svc.UpdatedAt.Format("2006-01-02 15:04:05"))

			return nil
		},
	}
}
