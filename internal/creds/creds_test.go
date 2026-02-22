package creds

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

func TestGenerateLength(t *testing.T) {
	pw, err := Generate(32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pw) != 32 {
		t.Errorf("length = %d, want 32", len(pw))
	}
}

func TestGenerateAlphanumeric(t *testing.T) {
	pw, err := Generate(100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	re := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	if !re.MatchString(pw) {
		t.Errorf("password contains non-alphanumeric characters: %q", pw)
	}
}

func TestGenerateUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		pw, err := Generate(32)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if seen[pw] {
			t.Fatalf("duplicate password generated: %q", pw)
		}
		seen[pw] = true
	}
}

func TestWriteEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env")

	content := &EnvFileContent{
		Entries: map[string]string{
			"PORT":        "3000",
			"DB_NAME":     "gc_api",
			"DB_PASSWORD": "secret",
		},
	}

	if err := WriteEnvFile(path, content); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading env file: %v", err)
	}

	text := string(data)
	if !strings.Contains(text, "PORT=3000") {
		t.Errorf("env file should contain PORT=3000, got:\n%s", text)
	}
	if !strings.Contains(text, "DB_PASSWORD=secret") {
		t.Errorf("env file should contain DB_PASSWORD=secret, got:\n%s", text)
	}

	// Check permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("permissions = %o, want 0600", perm)
	}
}

func TestWriteTOMLConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := &EnvFileContent{
		Entries: map[string]string{
			"port":        "3000",
			"db_name":     "gc_api",
			"db_password": "secret",
		},
	}

	if err := WriteTOMLConfigFile(path, content); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading TOML file: %v", err)
	}

	// Verify it's valid TOML
	var parsed map[string]string
	if err := toml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("TOML parse error: %v\nContent:\n%s", err, string(data))
	}
	if parsed["port"] != "3000" {
		t.Errorf("parsed[port] = %q, want %q", parsed["port"], "3000")
	}

	// Check permissions
	info, _ := os.Stat(path)
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("permissions = %o, want 0600", perm)
	}
}
