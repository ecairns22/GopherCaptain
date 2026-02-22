package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "gophercaptain.conf")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writePasswordFile(t *testing.T, dir, password string) string {
	t.Helper()
	path := filepath.Join(dir, "dbpass")
	if err := os.WriteFile(path, []byte(password), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

const validConfigTmpl = `[github]
token = "ghp_testtoken123"
owner = "testowner"

[ports]
range_start = 5000
range_end   = 6000

[mariadb]
host     = "127.0.0.1"
port     = 3306
admin_user = "root"
admin_password_file = "%s"

[nginx]
sites_dir   = "/etc/nginx/sites-available"
enabled_dir = "/etc/nginx/sites-enabled"

[releases]
asset_pattern = "{{.Name}}-linux-amd64"
`

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	pwFile := writePasswordFile(t, dir, "secret123\n")
	content := strings.ReplaceAll(validConfigTmpl, "%s", pwFile)
	path := writeTestConfig(t, dir, content)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitHub.Token != "ghp_testtoken123" {
		t.Errorf("token = %q, want %q", cfg.GitHub.Token, "ghp_testtoken123")
	}
	if cfg.GitHub.Owner != "testowner" {
		t.Errorf("owner = %q, want %q", cfg.GitHub.Owner, "testowner")
	}
	if cfg.Ports.RangeStart != 5000 {
		t.Errorf("range_start = %d, want 5000", cfg.Ports.RangeStart)
	}
	if cfg.Ports.RangeEnd != 6000 {
		t.Errorf("range_end = %d, want 6000", cfg.Ports.RangeEnd)
	}
	if cfg.MariaDB.AdminPassword != "secret123" {
		t.Errorf("admin_password = %q, want %q", cfg.MariaDB.AdminPassword, "secret123")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := LoadFrom("/nonexistent/path/gophercaptain.conf")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "/nonexistent/path/gophercaptain.conf") {
		t.Errorf("error should name the path, got: %v", err)
	}
}

func TestLoadMissingRequiredField(t *testing.T) {
	dir := t.TempDir()
	pwFile := writePasswordFile(t, dir, "secret123")

	// Missing github.token
	content := strings.ReplaceAll(`[github]
owner = "testowner"

[mariadb]
admin_password_file = "%s"
`, "%s", pwFile)

	path := writeTestConfig(t, dir, content)
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	if !strings.Contains(err.Error(), "github.token") {
		t.Errorf("error should mention github.token, got: %v", err)
	}
}

func TestLoadMissingPasswordFile(t *testing.T) {
	dir := t.TempDir()
	content := `[github]
token = "ghp_test"
owner = "testowner"

[mariadb]
admin_password_file = "/nonexistent/dbpass"
`
	path := writeTestConfig(t, dir, content)
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for missing password file")
	}
	if !strings.Contains(err.Error(), "/nonexistent/dbpass") {
		t.Errorf("error should name the password file path, got: %v", err)
	}
}

func TestDefaults(t *testing.T) {
	dir := t.TempDir()
	pwFile := writePasswordFile(t, dir, "secret123")

	// Minimal config — only required fields
	content := strings.ReplaceAll(`[github]
token = "ghp_test"
owner = "testowner"

[mariadb]
admin_password_file = "%s"
`, "%s", pwFile)

	path := writeTestConfig(t, dir, content)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Ports.RangeStart != 3000 {
		t.Errorf("default range_start = %d, want 3000", cfg.Ports.RangeStart)
	}
	if cfg.Ports.RangeEnd != 4000 {
		t.Errorf("default range_end = %d, want 4000", cfg.Ports.RangeEnd)
	}
	if cfg.MariaDB.Host != "127.0.0.1" {
		t.Errorf("default host = %q, want %q", cfg.MariaDB.Host, "127.0.0.1")
	}
	if cfg.MariaDB.Port != 3306 {
		t.Errorf("default port = %d, want 3306", cfg.MariaDB.Port)
	}
	if cfg.Releases.AssetPattern != "{{.Name}}-linux-amd64" {
		t.Errorf("default asset_pattern = %q", cfg.Releases.AssetPattern)
	}
}

func TestTemplateConfig(t *testing.T) {
	tmpl := TemplateConfig()
	if !strings.Contains(tmpl, "[github]") {
		t.Error("template should contain [github] section")
	}
	if !strings.Contains(tmpl, "[mariadb]") {
		t.Error("template should contain [mariadb] section")
	}
	if !strings.Contains(tmpl, "ghp_YOUR_TOKEN_HERE") {
		t.Error("template should contain placeholder token")
	}
}

func TestEnvOverride(t *testing.T) {
	dir := t.TempDir()
	pwFile := writePasswordFile(t, dir, "secret123")
	content := strings.ReplaceAll(`[github]
token = "ghp_envtest"
owner = "envowner"

[mariadb]
admin_password_file = "%s"
`, "%s", pwFile)
	path := writeTestConfig(t, dir, content)

	t.Setenv(envOverride, path)
	got := DefaultPath()
	if got != path {
		t.Errorf("DefaultPath() = %q, want %q", got, path)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GitHub.Token != "ghp_envtest" {
		t.Errorf("token = %q, want %q", cfg.GitHub.Token, "ghp_envtest")
	}
}
