package commands

import (
	"fmt"
	"strings"

	"github.com/ecairns22/GopherCaptain/internal/orchestrator"
	"github.com/spf13/cobra"
)

func deployCmd() *cobra.Command {
	var (
		version    string
		port       int
		route      string
		routeType  string
		name       string
		envVars    []string
		noDB       bool
		configFile bool
	)

	cmd := &cobra.Command{
		Use:   "deploy <repo>",
		Short: "Deploy a service from a GitHub release",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := args[0]

			// Parse extra env vars
			extraEnv := make(map[string]string)
			for _, e := range envVars {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid env var %q: must be KEY=VALUE", e)
				}
				extraEnv[parts[0]] = parts[1]
			}

			orc, cleanup, err := buildOrchestrator()
			if err != nil {
				return err
			}
			defer cleanup()

			req := orchestrator.DeployRequest{
				Repo:       repo,
				Name:       name,
				Version:    version,
				Port:       port,
				Route:      route,
				RouteType:  routeType,
				ExtraEnv:   extraEnv,
				NoDB:       noDB,
				ConfigFile: configFile,
			}

			result, err := orc.Deploy(cmd.Context(), req)
			if err != nil {
				return fmt.Errorf("deploy failed (rollback completed): %w", err)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "✓ %s %s deployed\n", result.Name, result.Version)
			fmt.Fprintf(w, "  Port:     %d\n", result.Port)
			if result.Route != "" {
				fmt.Fprintf(w, "  Route:    %s → localhost:%d\n", result.Route, result.Port)
			}
			if result.DBName != "" {
				fmt.Fprintf(w, "  Database: %s\n", result.DBName)
			}
			if result.NginxWarn != "" {
				fmt.Fprintf(w, "  Warning:  nginx config failed (%s); service running without routing\n", result.NginxWarn)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", "latest", "Release tag (default: latest)")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "Override port (default: auto-assign)")
	cmd.Flags().StringVarP(&route, "route", "r", "", "Route rule, e.g. \"api.example.com\" or \"/api\"")
	cmd.Flags().StringVar(&routeType, "route-type", "", "\"subdomain\" or \"path\" (inferred from --route)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Service name (default: repo name)")
	cmd.Flags().StringSliceVarP(&envVars, "env", "e", nil, "Extra env vars: -e KEY=VALUE")
	cmd.Flags().BoolVar(&noDB, "no-db", false, "Skip database creation")
	cmd.Flags().BoolVar(&configFile, "config-file", false, "Write config file instead of env vars")

	return cmd
}
