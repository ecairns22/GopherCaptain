package commands

import (
	"fmt"
	"os"

	"github.com/ecairns22/GopherCaptain/internal/config"
	"github.com/ecairns22/GopherCaptain/internal/db"
	"github.com/ecairns22/GopherCaptain/internal/state"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "First-time setup: create directories, write config template, test connections",
		RunE:  runInit,
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// 1. Create directories
	dirs := []string{
		"/opt/gophercaptain/bin",
		"/etc/gophercaptain",
		"/var/lib/gophercaptain",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  directory %s\n", d)
	}

	// 2. Write template config if missing
	configPath := config.DefaultPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte(config.TemplateConfig()), 0600); err != nil {
			return fmt.Errorf("writing config template: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  wrote config template to %s\n", configPath)
		fmt.Fprintf(cmd.OutOrStdout(), "\nEdit %s with your settings, then run 'gophercaptain init' again.\n", configPath)
		return nil
	}

	// 3. Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  config loaded from %s\n", configPath)

	// 4. Test MariaDB connection
	dbMgr, err := db.NewFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("connecting to MariaDB: %w", err)
	}
	defer dbMgr.Close()

	if err := dbMgr.Ping(ctx); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  MariaDB: FAILED (%v)\n", err)
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  MariaDB: OK\n")

	// 5. Verify nginx directories
	for _, d := range []string{cfg.Nginx.SitesDir, cfg.Nginx.EnabledDir} {
		info, err := os.Stat(d)
		if err != nil {
			return fmt.Errorf("nginx directory %s: %w", d, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("nginx path %s is not a directory", d)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  nginx dir %s: OK\n", d)
	}

	// 6. Initialize state database
	store, err := state.Open("/var/lib/gophercaptain/state.db")
	if err != nil {
		return fmt.Errorf("initializing state database: %w", err)
	}
	store.Close()
	fmt.Fprintf(cmd.OutOrStdout(), "  state database: OK\n")

	fmt.Fprintf(cmd.OutOrStdout(), "\nGopherCaptain initialized successfully.\n")
	return nil
}
