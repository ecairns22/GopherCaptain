package nginx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ecairns22/GopherCaptain/internal/runner"
)

// Manager handles nginx config lifecycle operations.
type Manager struct {
	runner     runner.CommandRunner
	sitesDir   string
	enabledDir string
}

// New creates an nginx manager.
func New(r runner.CommandRunner, sitesDir, enabledDir string) *Manager {
	return &Manager{
		runner:     r,
		sitesDir:   sitesDir,
		enabledDir: enabledDir,
	}
}

func configName(name string) string {
	return fmt.Sprintf("gc-%s.conf", name)
}

// WriteConfig renders the nginx config, writes it, creates the enabled symlink,
// tests with nginx -t, and reloads nginx. On test failure, rolls back the config.
func (m *Manager) WriteConfig(ctx context.Context, params RouteParams) error {
	content, err := RenderConfig(params)
	if err != nil {
		return err
	}

	filename := configName(params.Name)
	sitesPath := filepath.Join(m.sitesDir, filename)
	enabledPath := filepath.Join(m.enabledDir, filename)

	// Write config to sites-available
	if err := os.WriteFile(sitesPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing nginx config %s: %w", sitesPath, err)
	}

	// Create symlink in sites-enabled
	os.Remove(enabledPath) // remove stale symlink if any
	if err := os.Symlink(sitesPath, enabledPath); err != nil {
		os.Remove(sitesPath)
		return fmt.Errorf("creating symlink %s: %w", enabledPath, err)
	}

	// Test config
	_, stderr, err := m.runner.Run(ctx, "nginx", "-t")
	if err != nil {
		// Rollback: remove config and symlink
		os.Remove(enabledPath)
		os.Remove(sitesPath)
		return fmt.Errorf("nginx config test failed (config rolled back): %s", strings.TrimSpace(stderr))
	}

	// Reload nginx
	_, stderr, err = m.runner.Run(ctx, "systemctl", "reload", "nginx")
	if err != nil {
		return fmt.Errorf("reloading nginx: %s: %w", strings.TrimSpace(stderr), err)
	}

	return nil
}

// RemoveConfig removes the nginx config and symlink, then reloads nginx.
func (m *Manager) RemoveConfig(ctx context.Context, name string) error {
	filename := configName(name)
	enabledPath := filepath.Join(m.enabledDir, filename)
	sitesPath := filepath.Join(m.sitesDir, filename)

	os.Remove(enabledPath)
	os.Remove(sitesPath)

	_, stderr, err := m.runner.Run(ctx, "systemctl", "reload", "nginx")
	if err != nil {
		return fmt.Errorf("reloading nginx after removing %s: %s: %w", name, strings.TrimSpace(stderr), err)
	}
	return nil
}

// InferRouteType returns "subdomain" if value contains a dot, "path" if it starts with "/".
func InferRouteType(value string) string {
	if strings.HasPrefix(value, "/") {
		return "path"
	}
	if strings.Contains(value, ".") {
		return "subdomain"
	}
	return "subdomain" // default
}
