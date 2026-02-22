package systemd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ecairns22/GopherCaptain/internal/runner"
)

// Manager handles systemd unit lifecycle operations.
type Manager struct {
	runner  runner.CommandRunner
	unitDir string
}

// New creates a systemd manager with the given command runner and unit directory.
func New(r runner.CommandRunner, unitDir string) *Manager {
	return &Manager{runner: r, unitDir: unitDir}
}

func unitName(name string) string {
	return fmt.Sprintf("gc-%s.service", name)
}

func userName(name string) string {
	return fmt.Sprintf("gc-%s", name)
}

// WriteUnit renders and writes the systemd unit file for a service.
func (m *Manager) WriteUnit(ctx context.Context, name string) error {
	content, err := RenderUnit(ServiceParams{Name: name})
	if err != nil {
		return fmt.Errorf("rendering unit for %s: %w", name, err)
	}

	path := filepath.Join(m.unitDir, unitName(name))
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing unit file %s: %w", path, err)
	}
	return nil
}

// RemoveUnit deletes the systemd unit file for a service.
func (m *Manager) RemoveUnit(name string) error {
	path := filepath.Join(m.unitDir, unitName(name))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing unit file %s: %w", path, err)
	}
	return nil
}

// DaemonReload runs systemctl daemon-reload.
func (m *Manager) DaemonReload(ctx context.Context) error {
	_, stderr, err := m.runner.Run(ctx, "systemctl", "daemon-reload")
	if err != nil {
		return fmt.Errorf("daemon-reload: %s: %w", strings.TrimSpace(stderr), err)
	}
	return nil
}

// Enable enables the service unit.
func (m *Manager) Enable(ctx context.Context, name string) error {
	_, stderr, err := m.runner.Run(ctx, "systemctl", "enable", unitName(name))
	if err != nil {
		return fmt.Errorf("enabling %s: %s: %w", name, strings.TrimSpace(stderr), err)
	}
	return nil
}

// Start starts the service and polls for active status up to 10 seconds.
// On failure, returns an error including journal tail output.
func (m *Manager) Start(ctx context.Context, name string) error {
	_, stderr, err := m.runner.Run(ctx, "systemctl", "start", unitName(name))
	if err != nil {
		return fmt.Errorf("starting %s: %s: %w", name, strings.TrimSpace(stderr), err)
	}

	// Poll for active state up to 10 seconds
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		active, err := m.IsActive(ctx, name)
		if err == nil && active {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Service failed to become active — include journal output in error
	journal, _ := m.JournalTail(ctx, name, 20)
	return fmt.Errorf("service %s failed to become active within 10s; journal:\n%s", name, journal)
}

// Stop stops the service.
func (m *Manager) Stop(ctx context.Context, name string) error {
	_, stderr, err := m.runner.Run(ctx, "systemctl", "stop", unitName(name))
	if err != nil {
		return fmt.Errorf("stopping %s: %s: %w", name, strings.TrimSpace(stderr), err)
	}
	return nil
}

// Disable disables the service unit.
func (m *Manager) Disable(ctx context.Context, name string) error {
	_, stderr, err := m.runner.Run(ctx, "systemctl", "disable", unitName(name))
	if err != nil {
		return fmt.Errorf("disabling %s: %s: %w", name, strings.TrimSpace(stderr), err)
	}
	return nil
}

// IsActive returns true if the service is in the "active" state.
func (m *Manager) IsActive(ctx context.Context, name string) (bool, error) {
	stdout, _, err := m.runner.Run(ctx, "systemctl", "is-active", unitName(name))
	status := strings.TrimSpace(stdout)
	if status == "active" {
		return true, nil
	}
	if err != nil {
		return false, nil // not active
	}
	return false, nil
}

// JournalTail returns the last n lines of journal output for the service.
func (m *Manager) JournalTail(ctx context.Context, name string, lines int) (string, error) {
	stdout, _, err := m.runner.Run(ctx, "journalctl", "-u", unitName(name), "-n", fmt.Sprintf("%d", lines), "--no-pager")
	if err != nil {
		return "", fmt.Errorf("reading journal for %s: %w", name, err)
	}
	return stdout, nil
}

// CreateUser creates a system user for the service.
func (m *Manager) CreateUser(ctx context.Context, name string) error {
	user := userName(name)
	_, stderr, err := m.runner.Run(ctx, "useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin", user)
	if err != nil {
		// Ignore "already exists" errors
		if strings.Contains(stderr, "already exists") {
			return nil
		}
		return fmt.Errorf("creating user %s: %s: %w", user, strings.TrimSpace(stderr), err)
	}
	return nil
}

// RemoveUser removes the system user for the service.
func (m *Manager) RemoveUser(ctx context.Context, name string) error {
	user := userName(name)
	_, stderr, err := m.runner.Run(ctx, "userdel", user)
	if err != nil {
		if strings.Contains(stderr, "does not exist") {
			return nil
		}
		return fmt.Errorf("removing user %s: %s: %w", user, strings.TrimSpace(stderr), err)
	}
	return nil
}
