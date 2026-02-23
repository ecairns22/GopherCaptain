package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ecairns22/GopherCaptain/internal/config"
	"github.com/ecairns22/GopherCaptain/internal/creds"
	"github.com/ecairns22/GopherCaptain/internal/db"
	ghclient "github.com/ecairns22/GopherCaptain/internal/github"
	"github.com/ecairns22/GopherCaptain/internal/health"
	"github.com/ecairns22/GopherCaptain/internal/nginx"
	"github.com/ecairns22/GopherCaptain/internal/ports"
	"github.com/ecairns22/GopherCaptain/internal/state"
	"github.com/ecairns22/GopherCaptain/internal/systemd"
)

const (
	binBase    = "/opt/gophercaptain/bin"
	configBase = "/etc/gophercaptain"
	stateDB    = "/var/lib/gophercaptain/state.db"
)

// Orchestrator coordinates all managers for deploy/upgrade/rollback/remove flows.
type Orchestrator struct {
	cfg     *config.Config
	store   *state.Store
	gh      *ghclient.Client
	systemd *systemd.Manager
	nginx   *nginx.Manager
	db      *db.Manager
	ports   *ports.Allocator
}

// New creates an Orchestrator from the loaded config and initialized managers.
func New(cfg *config.Config, store *state.Store, gh *ghclient.Client, sys *systemd.Manager, ngx *nginx.Manager, dbMgr *db.Manager) *Orchestrator {
	alloc := ports.New(cfg.Ports.RangeStart, cfg.Ports.RangeEnd, store)
	return &Orchestrator{
		cfg:     cfg,
		store:   store,
		gh:      gh,
		systemd: sys,
		nginx:   ngx,
		db:      dbMgr,
		ports:   alloc,
	}
}

// DeployRequest holds all parameters for a deploy.
type DeployRequest struct {
	Repo       string
	Name       string // derived from repo if empty
	Version    string // "latest" or explicit tag
	Port       int    // 0 = auto-assign
	Route      string // e.g. "api.example.com" or "/api", empty = no routing
	RouteType  string // "subdomain" or "path", inferred if empty
	ExtraEnv   map[string]string
	NoDB       bool
	ConfigFile bool // write TOML instead of env
	Owner      string
}

// DeployResult holds the output of a successful deploy.
type DeployResult struct {
	Name       string
	Version    string
	Port       int
	Route      string
	RouteType  string
	DBName     string
	NginxSkip  bool   // true if nginx was skipped or failed (non-fatal)
	NginxWarn  string // warning message if nginx failed
}

// Deploy executes the full deploy flow with rollback on failure.
func (o *Orchestrator) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	// Derive service name from repo if not provided
	name := req.Name
	if name == "" {
		parts := strings.Split(req.Repo, "/")
		name = parts[len(parts)-1]
	}

	if err := db.ValidateServiceName(name); err != nil {
		return nil, err
	}

	// Check name not already taken
	if _, err := o.store.GetService(ctx, name); err == nil {
		return nil, fmt.Errorf("service %q already exists; use a different --name or remove it first", name)
	}

	// Resolve owner
	owner := req.Owner
	repo := req.Repo
	if strings.Contains(repo, "/") {
		parts := strings.SplitN(repo, "/", 2)
		owner = parts[0]
		repo = parts[1]
	}

	// Resolve version
	version, err := o.gh.ResolveVersion(ctx, owner, repo, req.Version)
	if err != nil {
		return nil, fmt.Errorf("resolving version: %w", err)
	}

	// Allocate port
	port := req.Port
	if port == 0 {
		p, err := o.ports.Next(ctx)
		if err != nil {
			return nil, err
		}
		port = p
	} else {
		if err := o.ports.Request(ctx, port); err != nil {
			return nil, err
		}
	}

	// Infer route type
	routeType := req.RouteType
	if routeType == "" && req.Route != "" {
		routeType = nginx.InferRouteType(req.Route)
	}

	// Track completed steps for rollback
	var completed []string

	rollback := func(originalErr error) error {
		var rollbackErrs []string
		for i := len(completed) - 1; i >= 0; i-- {
			step := completed[i]
			var rbErr error
			switch step {
			case "binary":
				rbErr = removeBinary(name, version)
			case "database":
				rbErr = o.db.DropDatabase(ctx, name)
			case "envfile":
				rbErr = removeEnvDir(name)
			case "systemd":
				o.systemd.Stop(ctx, name)
				o.systemd.Disable(ctx, name)
				o.systemd.RemoveUnit(name)
				o.systemd.DaemonReload(ctx)
				rbErr = o.systemd.RemoveUser(ctx, name)
			case "nginx":
				rbErr = o.nginx.RemoveConfig(ctx, name)
			}
			if rbErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("rollback %s: %v", step, rbErr))
			}
		}
		if len(rollbackErrs) > 0 {
			return fmt.Errorf("%w; rollback issues (may need manual cleanup): %s", originalErr, strings.Join(rollbackErrs, "; "))
		}
		return originalErr
	}

	// Step 1: Fetch binary
	_, err = o.gh.DownloadAsset(ctx, owner, repo, version, name)
	if err != nil {
		return nil, fmt.Errorf("fetching binary: %w", err)
	}
	completed = append(completed, "binary")

	// Step 2: Create database (unless --no-db)
	var dbResult *db.CreateResult
	if !req.NoDB {
		dbResult, err = o.db.CreateDatabase(ctx, name)
		if err != nil {
			return nil, rollback(fmt.Errorf("creating database: %w", err))
		}
		completed = append(completed, "database")
	}

	// Step 3: Write env/config file
	envEntries := map[string]string{
		"PORT": fmt.Sprintf("%d", port),
	}
	if dbResult != nil {
		envEntries["DB_HOST"] = o.cfg.MariaDB.Host
		envEntries["DB_PORT"] = fmt.Sprintf("%d", o.cfg.MariaDB.Port)
		envEntries["DB_NAME"] = dbResult.DBName
		envEntries["DB_USER"] = dbResult.DBUser
		envEntries["DB_PASSWORD"] = dbResult.Password
	}
	for k, v := range req.ExtraEnv {
		envEntries[k] = v
	}

	envDir := filepath.Join(configBase, name)
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return nil, rollback(fmt.Errorf("creating env dir: %w", err))
	}

	envContent := &creds.EnvFileContent{Entries: envEntries}
	if req.ConfigFile {
		err = creds.WriteTOMLConfigFile(filepath.Join(envDir, "env"), envContent)
	} else {
		err = creds.WriteEnvFile(filepath.Join(envDir, "env"), envContent)
	}
	if err != nil {
		return nil, rollback(fmt.Errorf("writing env file: %w", err))
	}
	completed = append(completed, "envfile")

	// Step 4: Write systemd unit, create user, enable, start
	if err := o.systemd.CreateUser(ctx, name); err != nil {
		return nil, rollback(fmt.Errorf("creating system user: %w", err))
	}
	if err := o.systemd.WriteUnit(ctx, name); err != nil {
		return nil, rollback(fmt.Errorf("writing systemd unit: %w", err))
	}
	if err := o.systemd.DaemonReload(ctx); err != nil {
		return nil, rollback(fmt.Errorf("daemon-reload: %w", err))
	}
	if err := o.systemd.Enable(ctx, name); err != nil {
		return nil, rollback(fmt.Errorf("enabling service: %w", err))
	}
	if err := o.systemd.Start(ctx, name); err != nil {
		return nil, rollback(fmt.Errorf("starting service: %w", err))
	}
	completed = append(completed, "systemd")

	// Step 5: Write nginx config (non-fatal per design)
	result := &DeployResult{
		Name:    name,
		Version: version,
		Port:    port,
	}
	if dbResult != nil {
		result.DBName = dbResult.DBName
	}

	if req.Route != "" {
		params := nginx.RouteParams{
			Name:       name,
			RouteType:  routeType,
			RouteValue: req.Route,
			Port:       port,
		}
		if err := o.nginx.WriteConfig(ctx, params); err != nil {
			// Nginx failure is non-fatal — warn but continue
			result.NginxSkip = true
			result.NginxWarn = err.Error()
		} else {
			result.Route = req.Route
			result.RouteType = routeType
			completed = append(completed, "nginx")
		}
	} else {
		result.NginxSkip = true
	}

	// Step 6: Record state
	now := time.Now()
	svc := &state.Service{
		Name:       name,
		Repo:       req.Repo,
		Version:    version,
		Port:       port,
		RouteType:  routeType,
		RouteValue: req.Route,
		DBName:     result.DBName,
		DBUser:     result.DBName, // gc_<name> for both
		ExtraEnv:   req.ExtraEnv,
		DeployedAt: now,
		UpdatedAt:  now,
	}
	if dbResult != nil {
		svc.DBUser = dbResult.DBUser
	}
	if err := o.store.InsertService(ctx, svc); err != nil {
		return nil, rollback(fmt.Errorf("recording state: %w", err))
	}

	o.store.AppendHistory(ctx, &state.HistoryEntry{
		Service:   name,
		Action:    "deploy",
		Version:   version,
		Timestamp: now,
		Detail:    map[string]string{"port": fmt.Sprintf("%d", port)},
	})

	return result, nil
}

// UpgradeRequest holds parameters for an upgrade.
type UpgradeRequest struct {
	Name    string
	Version string // "latest" or explicit tag
	Owner   string
}

// UpgradeResult holds the output of a successful upgrade.
type UpgradeResult struct {
	Name        string
	OldVersion  string
	NewVersion  string
	RolledBack  bool
	RollbackMsg string
}

// Upgrade executes the upgrade flow: fetch, stop, swap symlink, start, health check.
func (o *Orchestrator) Upgrade(ctx context.Context, req UpgradeRequest) (*UpgradeResult, error) {
	svc, err := o.store.GetService(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("service %q not found; run 'gophercaptain list' to see deployed services", req.Name)
	}

	// Resolve owner/repo
	owner := req.Owner
	repo := svc.Repo
	if strings.Contains(repo, "/") {
		parts := strings.SplitN(repo, "/", 2)
		if owner == "" {
			owner = parts[0]
		}
		repo = parts[1]
	}

	// Resolve version
	version, err := o.gh.ResolveVersion(ctx, owner, repo, req.Version)
	if err != nil {
		return nil, fmt.Errorf("resolving version: %w", err)
	}

	if version == svc.Version {
		return nil, fmt.Errorf("service %q is already at version %s", req.Name, version)
	}

	oldVersion := svc.Version

	// Step 1: Fetch new binary
	_, err = o.gh.DownloadAsset(ctx, owner, repo, version, req.Name)
	if err != nil {
		return nil, fmt.Errorf("fetching binary: %w", err)
	}

	// Step 2: Stop service
	if err := o.systemd.Stop(ctx, req.Name); err != nil {
		return nil, fmt.Errorf("stopping service: %w", err)
	}

	// Step 3: Update symlink
	if err := updateSymlink(req.Name, version); err != nil {
		// Try to restart with old version
		updateSymlink(req.Name, oldVersion)
		o.systemd.Start(ctx, req.Name)
		return nil, fmt.Errorf("updating symlink: %w", err)
	}

	// Step 4: Start service
	if err := o.systemd.Start(ctx, req.Name); err != nil {
		// Rollback: swap symlink back and restart
		updateSymlink(req.Name, oldVersion)
		o.systemd.Start(ctx, req.Name)
		return &UpgradeResult{
			Name:        req.Name,
			OldVersion:  oldVersion,
			NewVersion:  version,
			RolledBack:  true,
			RollbackMsg: fmt.Sprintf("service failed to start with %s, rolled back to %s", version, oldVersion),
		}, nil
	}

	// Step 5: Health check
	if err := health.WaitForPort(svc.Port, 10*time.Second); err != nil {
		// Rollback: swap symlink back, restart
		o.systemd.Stop(ctx, req.Name)
		updateSymlink(req.Name, oldVersion)
		o.systemd.Start(ctx, req.Name)
		return &UpgradeResult{
			Name:        req.Name,
			OldVersion:  oldVersion,
			NewVersion:  version,
			RolledBack:  true,
			RollbackMsg: fmt.Sprintf("health check failed for %s, rolled back to %s", version, oldVersion),
		}, nil
	}

	// Step 6: Prune old versions (keep current + previous)
	pruneVersions(req.Name, version, oldVersion)

	// Step 7: Update state
	now := time.Now()
	svc.PrevVersion = oldVersion
	svc.Version = version
	svc.UpdatedAt = now
	if err := o.store.UpdateService(ctx, svc); err != nil {
		return nil, fmt.Errorf("updating state: %w", err)
	}

	o.store.AppendHistory(ctx, &state.HistoryEntry{
		Service:   req.Name,
		Action:    "upgrade",
		Version:   version,
		Timestamp: now,
		Detail:    map[string]string{"from": oldVersion},
	})

	return &UpgradeResult{
		Name:       req.Name,
		OldVersion: oldVersion,
		NewVersion: version,
	}, nil
}

// Rollback swaps back to the previous version.
func (o *Orchestrator) Rollback(ctx context.Context, name string) (string, error) {
	svc, err := o.store.GetService(ctx, name)
	if err != nil {
		return "", fmt.Errorf("service %q not found; run 'gophercaptain list' to see deployed services", name)
	}

	if svc.PrevVersion == "" {
		return "", fmt.Errorf("service %q has no previous version to roll back to", name)
	}

	prevVersion := svc.PrevVersion

	// Stop service
	if err := o.systemd.Stop(ctx, name); err != nil {
		return "", fmt.Errorf("stopping service: %w", err)
	}

	// Swap symlink
	if err := updateSymlink(name, prevVersion); err != nil {
		o.systemd.Start(ctx, name)
		return "", fmt.Errorf("updating symlink: %w", err)
	}

	// Start service
	if err := o.systemd.Start(ctx, name); err != nil {
		return "", fmt.Errorf("starting service after rollback: %w", err)
	}

	// Update state
	now := time.Now()
	svc.PrevVersion = svc.Version
	svc.Version = prevVersion
	svc.UpdatedAt = now
	o.store.UpdateService(ctx, svc)

	o.store.AppendHistory(ctx, &state.HistoryEntry{
		Service:   name,
		Action:    "rollback",
		Version:   prevVersion,
		Timestamp: now,
	})

	return prevVersion, nil
}

// RemoveRequest holds parameters for a remove.
type RemoveRequest struct {
	Name   string
	DropDB bool
	Yes    bool // skip confirmation
}

// RemoveStep is a callback for reporting progress.
type RemoveStep func(msg string)

// Remove executes the full remove flow.
func (o *Orchestrator) Remove(ctx context.Context, req RemoveRequest, step RemoveStep) error {
	svc, err := o.store.GetService(ctx, req.Name)
	if err != nil {
		return fmt.Errorf("service %q not found; run 'gophercaptain list' to see deployed services", req.Name)
	}

	// Stop systemd service
	step(fmt.Sprintf("Stopping gc-%s...", req.Name))
	o.systemd.Stop(ctx, req.Name)

	// Disable and remove unit
	step("Removing systemd unit...")
	o.systemd.Disable(ctx, req.Name)
	o.systemd.RemoveUnit(req.Name)
	o.systemd.DaemonReload(ctx)
	o.systemd.RemoveUser(ctx, req.Name)

	// Remove nginx config
	if svc.RouteValue != "" {
		step("Removing nginx config...")
		o.nginx.RemoveConfig(ctx, req.Name)
	}

	// Remove env file and config directory
	step("Removing env and binaries...")
	removeEnvDir(req.Name)

	// Remove binaries
	binDir := filepath.Join(binBase, req.Name)
	os.RemoveAll(binDir)

	// Drop database if requested
	if req.DropDB && svc.DBName != "" {
		step(fmt.Sprintf("Dropping database %s...", svc.DBName))
		if err := o.db.DropDatabase(ctx, req.Name); err != nil {
			return fmt.Errorf("dropping database: %w", err)
		}
	}

	// Delete from state, record history
	now := time.Now()
	o.store.AppendHistory(ctx, &state.HistoryEntry{
		Service:   req.Name,
		Action:    "remove",
		Version:   svc.Version,
		Timestamp: now,
	})
	o.store.DeleteService(ctx, req.Name)

	return nil
}

// GetService returns a service from state (used by CLI commands).
func (o *Orchestrator) GetService(ctx context.Context, name string) (*state.Service, error) {
	return o.store.GetService(ctx, name)
}

// ListServices returns all services from state (used by CLI commands).
func (o *Orchestrator) ListServices(ctx context.Context) ([]*state.Service, error) {
	return o.store.ListServices(ctx)
}

// IsActive returns the live systemd status for a service.
func (o *Orchestrator) IsActive(ctx context.Context, name string) (bool, error) {
	return o.systemd.IsActive(ctx, name)
}

// --- helpers ---

func updateSymlink(name, version string) error {
	dir := filepath.Join(binBase, name)
	symlinkPath := filepath.Join(dir, name)
	target := fmt.Sprintf("%s-%s", name, version)
	os.Remove(symlinkPath)
	return os.Symlink(target, symlinkPath)
}

func removeBinary(name, version string) error {
	dir := filepath.Join(binBase, name)
	os.Remove(filepath.Join(dir, name)) // symlink
	return os.Remove(filepath.Join(dir, fmt.Sprintf("%s-%s", name, version)))
}

func removeEnvDir(name string) error {
	return os.RemoveAll(filepath.Join(configBase, name))
}

func pruneVersions(name, current, previous string) {
	dir := filepath.Join(binBase, name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	keep := map[string]bool{
		name:                                    true, // symlink
		fmt.Sprintf("%s-%s", name, current):     true,
		fmt.Sprintf("%s-%s", name, previous):    true,
	}

	// Sort to get deterministic pruning order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, e := range entries {
		if !keep[e.Name()] {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
