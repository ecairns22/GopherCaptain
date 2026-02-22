package nginx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ecairns22/GopherCaptain/internal/runner"
)

func TestSubdomainConfig(t *testing.T) {
	sitesDir := t.TempDir()
	enabledDir := t.TempDir()
	fake := runner.NewFakeRunner()
	mgr := New(fake, sitesDir, enabledDir)

	params := RouteParams{
		Name:       "api",
		RouteType:  "subdomain",
		RouteValue: "api.example.com",
		Port:       3000,
	}

	if err := mgr.WriteConfig(context.Background(), params); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// Check sites-available file
	data, err := os.ReadFile(filepath.Join(sitesDir, "gc-api.conf"))
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "server_name api.example.com") {
		t.Error("config should contain server_name")
	}
	if !strings.Contains(content, "proxy_pass http://127.0.0.1:3000") {
		t.Error("config should contain proxy_pass")
	}

	// Check symlink exists
	link := filepath.Join(enabledDir, "gc-api.conf")
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("symlink should exist: %v", err)
	}

	// Check nginx -t and reload were called
	if !fake.Called("nginx -t") {
		t.Error("expected nginx -t to be called")
	}
	if !fake.Called("systemctl reload nginx") {
		t.Error("expected systemctl reload nginx to be called")
	}
}

func TestPathConfig(t *testing.T) {
	sitesDir := t.TempDir()
	enabledDir := t.TempDir()
	fake := runner.NewFakeRunner()
	mgr := New(fake, sitesDir, enabledDir)

	params := RouteParams{
		Name:       "auth",
		RouteType:  "path",
		RouteValue: "/auth",
		Port:       3001,
	}

	if err := mgr.WriteConfig(context.Background(), params); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(sitesDir, "gc-auth.conf"))
	content := string(data)
	if !strings.Contains(content, "location /auth") {
		t.Error("config should contain location /auth")
	}
	if !strings.Contains(content, "proxy_pass http://127.0.0.1:3001") {
		t.Error("config should contain proxy_pass with port 3001")
	}
}

func TestTestFailureRollback(t *testing.T) {
	sitesDir := t.TempDir()
	enabledDir := t.TempDir()
	fake := runner.NewFakeRunner()
	fake.SetResponse("nginx -t", runner.Response{
		Stderr: "nginx: configuration file syntax is invalid",
		Err:    fmt.Errorf("exit status 1"),
	})
	mgr := New(fake, sitesDir, enabledDir)

	params := RouteParams{
		Name:       "bad",
		RouteType:  "subdomain",
		RouteValue: "bad.example.com",
		Port:       3002,
	}

	err := mgr.WriteConfig(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for failed nginx -t")
	}
	if !strings.Contains(err.Error(), "rolled back") {
		t.Errorf("error should mention rollback, got: %v", err)
	}

	// Config should be cleaned up
	if _, err := os.Stat(filepath.Join(sitesDir, "gc-bad.conf")); !os.IsNotExist(err) {
		t.Error("sites-available config should be removed on rollback")
	}
	if _, err := os.Stat(filepath.Join(enabledDir, "gc-bad.conf")); !os.IsNotExist(err) {
		t.Error("sites-enabled symlink should be removed on rollback")
	}
}

func TestRemoveConfig(t *testing.T) {
	sitesDir := t.TempDir()
	enabledDir := t.TempDir()
	fake := runner.NewFakeRunner()
	mgr := New(fake, sitesDir, enabledDir)

	// Create files to remove
	sitesPath := filepath.Join(sitesDir, "gc-api.conf")
	enabledPath := filepath.Join(enabledDir, "gc-api.conf")
	os.WriteFile(sitesPath, []byte("test"), 0644)
	os.Symlink(sitesPath, enabledPath)

	if err := mgr.RemoveConfig(context.Background(), "api"); err != nil {
		t.Fatalf("RemoveConfig: %v", err)
	}

	if _, err := os.Stat(sitesPath); !os.IsNotExist(err) {
		t.Error("sites-available config should be removed")
	}
	if _, err := os.Stat(enabledPath); !os.IsNotExist(err) {
		t.Error("sites-enabled symlink should be removed")
	}
	if !fake.Called("systemctl reload nginx") {
		t.Error("expected nginx reload after removal")
	}
}

func TestInferRouteType(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"api.example.com", "subdomain"},
		{"/api", "path"},
		{"/auth/v2", "path"},
		{"sub.domain.io", "subdomain"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := InferRouteType(tt.value)
			if got != tt.want {
				t.Errorf("InferRouteType(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestRenderSubdomainOutput(t *testing.T) {
	content, err := RenderConfig(RouteParams{
		Name:       "myapi",
		RouteType:  "subdomain",
		RouteValue: "myapi.example.com",
		Port:       3000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "server_name myapi.example.com") {
		t.Error("should contain server_name")
	}
	if !strings.Contains(content, "listen 80") {
		t.Error("should contain listen 80")
	}
}

func TestRenderPathOutput(t *testing.T) {
	content, err := RenderConfig(RouteParams{
		Name:       "myapi",
		RouteType:  "path",
		RouteValue: "/api",
		Port:       3000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "location /api") {
		t.Error("should contain location /api")
	}
	// Path template should NOT have a server block
	if strings.Contains(content, "listen 80") {
		t.Error("path template should not contain listen directive")
	}
}
