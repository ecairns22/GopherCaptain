package systemd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ecairns22/GopherCaptain/internal/runner"
)

func TestWriteUnitCreatesFile(t *testing.T) {
	dir := t.TempDir()
	fake := runner.NewFakeRunner()
	mgr := New(fake, dir)

	ctx := context.Background()
	if err := mgr.WriteUnit(ctx, "api"); err != nil {
		t.Fatalf("WriteUnit: %v", err)
	}

	path := filepath.Join(dir, "gc-api.service")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading unit file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Description=GopherCaptain: api") {
		t.Error("unit should contain Description")
	}
	if !strings.Contains(content, "ExecStart=/opt/gophercaptain/bin/api/api") {
		t.Error("unit should contain ExecStart")
	}
	if !strings.Contains(content, "User=gc-api") {
		t.Error("unit should contain User=gc-api")
	}
}

func TestCreateUser(t *testing.T) {
	fake := runner.NewFakeRunner()
	mgr := New(fake, t.TempDir())

	ctx := context.Background()
	if err := mgr.CreateUser(ctx, "api"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if !fake.Called("useradd") {
		t.Error("expected useradd to be called")
	}
}

func TestCreateUserAlreadyExists(t *testing.T) {
	fake := runner.NewFakeRunner()
	fake.SetResponse("useradd", runner.Response{
		Stderr: "useradd: user 'gc-api' already exists",
		Err:    fmt.Errorf("exit status 9"),
	})
	mgr := New(fake, t.TempDir())

	ctx := context.Background()
	if err := mgr.CreateUser(ctx, "api"); err != nil {
		t.Fatalf("CreateUser should not error for existing user: %v", err)
	}
}

func TestDaemonReload(t *testing.T) {
	fake := runner.NewFakeRunner()
	mgr := New(fake, t.TempDir())

	if err := mgr.DaemonReload(context.Background()); err != nil {
		t.Fatalf("DaemonReload: %v", err)
	}
	if !fake.Called("systemctl daemon-reload") {
		t.Error("expected systemctl daemon-reload to be called")
	}
}

func TestStartImmediate(t *testing.T) {
	fake := runner.NewFakeRunner()
	// systemctl start succeeds
	fake.SetResponse("systemctl start gc-api.service", runner.Response{})
	// systemctl is-active returns "active" immediately
	fake.SetResponse("systemctl is-active gc-api.service", runner.Response{Stdout: "active\n"})
	mgr := New(fake, t.TempDir())

	if err := mgr.Start(context.Background(), "api"); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

func TestStartTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test")
	}

	fake := runner.NewFakeRunner()
	fake.SetResponse("systemctl start gc-api.service", runner.Response{})
	// is-active always returns inactive
	fake.SetResponse("systemctl is-active gc-api.service", runner.Response{
		Stdout: "inactive\n",
		Err:    fmt.Errorf("exit status 3"),
	})
	// journal output for error message
	fake.SetResponse("journalctl", runner.Response{Stdout: "some error log\n"})
	mgr := New(fake, t.TempDir())

	err := mgr.Start(context.Background(), "api")
	if err == nil {
		t.Fatal("expected error for start timeout")
	}
	if !strings.Contains(err.Error(), "failed to become active") {
		t.Errorf("error should mention failure, got: %v", err)
	}
}

func TestRenderUnitOutput(t *testing.T) {
	content, err := RenderUnit(ServiceParams{Name: "myapi"})
	if err != nil {
		t.Fatalf("RenderUnit: %v", err)
	}

	checks := []string{
		"Description=GopherCaptain: myapi",
		"ExecStart=/opt/gophercaptain/bin/myapi/myapi",
		"EnvironmentFile=/etc/gophercaptain/myapi/env",
		"User=gc-myapi",
		"Group=gc-myapi",
		"NoNewPrivileges=true",
		"WantedBy=multi-user.target",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("unit should contain %q", check)
		}
	}
}

func TestRemoveUnit(t *testing.T) {
	dir := t.TempDir()
	fake := runner.NewFakeRunner()
	mgr := New(fake, dir)

	// Create then remove
	path := filepath.Join(dir, "gc-api.service")
	os.WriteFile(path, []byte("test"), 0644)

	if err := mgr.RemoveUnit("api"); err != nil {
		t.Fatalf("RemoveUnit: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("unit file should be removed")
	}
}

func TestRemoveUnitNotExists(t *testing.T) {
	mgr := New(runner.NewFakeRunner(), t.TempDir())

	// Should not error on non-existent file
	if err := mgr.RemoveUnit("ghost"); err != nil {
		t.Fatalf("RemoveUnit non-existent: %v", err)
	}
}
