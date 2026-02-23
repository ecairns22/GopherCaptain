package commands

import (
	"fmt"

	"github.com/ecairns22/GopherCaptain/internal/config"
	"github.com/ecairns22/GopherCaptain/internal/db"
	ghclient "github.com/ecairns22/GopherCaptain/internal/github"
	"github.com/ecairns22/GopherCaptain/internal/nginx"
	"github.com/ecairns22/GopherCaptain/internal/orchestrator"
	"github.com/ecairns22/GopherCaptain/internal/runner"
	"github.com/ecairns22/GopherCaptain/internal/state"
	"github.com/ecairns22/GopherCaptain/internal/systemd"
)

const stateDBPath = "/var/lib/gophercaptain/state.db"
const unitDir = "/etc/systemd/system"

// buildOrchestrator loads config and constructs all managers into an Orchestrator.
// The caller is responsible for calling the returned cleanup function.
func buildOrchestrator() (*orchestrator.Orchestrator, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}

	store, err := state.Open(stateDBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening state db: %w", err)
	}

	gh, err := ghclient.New(cfg.GitHub.Token, cfg.GitHub.Owner, cfg.Releases.AssetPattern)
	if err != nil {
		store.Close()
		return nil, nil, fmt.Errorf("creating github client: %w", err)
	}

	r := &runner.OSRunner{}
	sys := systemd.New(r, unitDir)
	ngx := nginx.New(r, cfg.Nginx.SitesDir, cfg.Nginx.EnabledDir)

	dbMgr, err := db.NewFromConfig(cfg)
	if err != nil {
		store.Close()
		return nil, nil, fmt.Errorf("connecting to MariaDB: %w", err)
	}

	orc := orchestrator.New(cfg, store, gh, sys, ngx, dbMgr)

	cleanup := func() {
		dbMgr.Close()
		store.Close()
	}

	return orc, cleanup, nil
}

// buildStateOnly opens just the state store (for read-only commands like list/status).
func buildStateOnly() (*state.Store, error) {
	return state.Open(stateDBPath)
}
