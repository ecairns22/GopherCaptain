package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var credentialKeys = regexp.MustCompile(`(?i)(PASSWORD|SECRET|TOKEN|KEY)`)

func inspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <service>",
		Short: "Print all generated config for a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			store, err := buildStateOnly()
			if err != nil {
				return err
			}
			defer store.Close()

			// Verify service exists in state
			if _, err := store.GetService(cmd.Context(), name); err != nil {
				return fmt.Errorf("service %q not found; run 'gophercaptain list' to see deployed services", name)
			}

			w := cmd.OutOrStdout()

			// Systemd unit
			unitPath := filepath.Join(unitDir, fmt.Sprintf("gc-%s.service", name))
			fmt.Fprintf(w, "=== Systemd Unit (%s) ===\n", unitPath)
			if data, err := os.ReadFile(unitPath); err == nil {
				fmt.Fprintln(w, string(data))
			} else {
				fmt.Fprintf(w, "(not found: %v)\n\n", err)
			}

			// Nginx config
			// Try common locations
			for _, dir := range []string{"/etc/nginx/sites-available", "/etc/nginx/sites-enabled"} {
				confPath := filepath.Join(dir, fmt.Sprintf("gc-%s.conf", name))
				if data, err := os.ReadFile(confPath); err == nil {
					fmt.Fprintf(w, "=== Nginx Config (%s) ===\n", confPath)
					fmt.Fprintln(w, string(data))
					break
				}
			}

			// Env file (with redaction)
			envPath := filepath.Join("/etc/gophercaptain", name, "env")
			fmt.Fprintf(w, "=== Env File (%s) ===\n", envPath)
			if data, err := os.ReadFile(envPath); err == nil {
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					if line == "" {
						continue
					}
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 && credentialKeys.MatchString(parts[0]) {
						fmt.Fprintf(w, "%s=****\n", parts[0])
					} else {
						fmt.Fprintln(w, line)
					}
				}
			} else {
				fmt.Fprintf(w, "(not found: %v)\n", err)
			}

			return nil
		},
	}
}
