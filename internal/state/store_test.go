package state

import (
	"context"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening test store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func testService(name string, port int) *Service {
	now := time.Now().Truncate(time.Second)
	return &Service{
		Name:       name,
		Repo:       "testowner/" + name,
		Version:    "v1.0.0",
		Port:       port,
		RouteType:  "subdomain",
		RouteValue: name + ".example.com",
		DBName:     "gc_" + name,
		DBUser:     "gc_" + name,
		DeployedAt: now,
		UpdatedAt:  now,
	}
}

func TestSchemaCreation(t *testing.T) {
	s := openTestStore(t)
	// Verify we can list (tables exist)
	svcs, err := s.ListServices(context.Background())
	if err != nil {
		t.Fatalf("listing services: %v", err)
	}
	if len(svcs) != 0 {
		t.Errorf("expected 0 services, got %d", len(svcs))
	}
}

func TestIdempotentOpen(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/state.db"

	s1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	s1.Close()

	// Second open should succeed without error
	s2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	s2.Close()
}

func TestCRUDRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	svc := testService("api", 3000)
	svc.ExtraEnv = map[string]string{"LOG_LEVEL": "info"}

	// Insert
	if err := s.InsertService(ctx, svc); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Get
	got, err := s.GetService(ctx, "api")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "api" {
		t.Errorf("name = %q, want %q", got.Name, "api")
	}
	if got.Port != 3000 {
		t.Errorf("port = %d, want 3000", got.Port)
	}
	if got.ExtraEnv["LOG_LEVEL"] != "info" {
		t.Errorf("extra_env[LOG_LEVEL] = %q, want %q", got.ExtraEnv["LOG_LEVEL"], "info")
	}
	if !got.DeployedAt.Equal(svc.DeployedAt) {
		t.Errorf("deployed_at = %v, want %v", got.DeployedAt, svc.DeployedAt)
	}

	// List
	all, err := s.ListServices(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("list length = %d, want 1", len(all))
	}

	// Update
	got.Version = "v2.0.0"
	got.PrevVersion = "v1.0.0"
	got.UpdatedAt = time.Now().Truncate(time.Second)
	if err := s.UpdateService(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, err := s.GetService(ctx, "api")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if updated.Version != "v2.0.0" {
		t.Errorf("version after update = %q, want %q", updated.Version, "v2.0.0")
	}
	if updated.PrevVersion != "v1.0.0" {
		t.Errorf("prev_version after update = %q, want %q", updated.PrevVersion, "v1.0.0")
	}

	// Delete
	if err := s.DeleteService(ctx, "api"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetService(ctx, "api")
	if err != ErrNotFound {
		t.Errorf("get after delete: expected ErrNotFound, got %v", err)
	}
}

func TestGetNotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.GetService(context.Background(), "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	s := openTestStore(t)
	err := s.DeleteService(context.Background(), "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateNotFound(t *testing.T) {
	s := openTestStore(t)
	svc := testService("ghost", 9999)
	err := s.UpdateService(context.Background(), svc)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPortQueries(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	s.InsertService(ctx, testService("api", 3000))
	s.InsertService(ctx, testService("auth", 3001))

	ports, err := s.UsedPorts(ctx)
	if err != nil {
		t.Fatalf("used ports: %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}
	if ports[0] != 3000 || ports[1] != 3001 {
		t.Errorf("ports = %v, want [3000, 3001]", ports)
	}

	owner, err := s.PortOwner(ctx, 3000)
	if err != nil {
		t.Fatalf("port owner: %v", err)
	}
	if owner != "api" {
		t.Errorf("port 3000 owner = %q, want %q", owner, "api")
	}

	owner, err = s.PortOwner(ctx, 9999)
	if err != nil {
		t.Fatalf("port owner free: %v", err)
	}
	if owner != "" {
		t.Errorf("port 9999 owner = %q, want empty", owner)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	svc := testService("api", 3000)
	svc.ExtraEnv = map[string]string{
		"KEY1": "val1",
		"KEY2": "val2",
	}
	s.InsertService(ctx, svc)

	got, _ := s.GetService(ctx, "api")
	if len(got.ExtraEnv) != 2 {
		t.Fatalf("extra_env length = %d, want 2", len(got.ExtraEnv))
	}
	if got.ExtraEnv["KEY1"] != "val1" {
		t.Errorf("KEY1 = %q, want %q", got.ExtraEnv["KEY1"], "val1")
	}
}

func TestNilExtraEnv(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	svc := testService("api", 3000)
	svc.ExtraEnv = nil
	s.InsertService(ctx, svc)

	got, _ := s.GetService(ctx, "api")
	if got.ExtraEnv != nil {
		t.Errorf("extra_env should be nil, got %v", got.ExtraEnv)
	}
}

func TestHistory(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)

	err := s.AppendHistory(ctx, &HistoryEntry{
		Service:   "api",
		Action:    "deploy",
		Version:   "v1.0.0",
		Timestamp: now,
		Detail:    map[string]string{"port": "3000"},
	})
	if err != nil {
		t.Fatalf("append history: %v", err)
	}

	err = s.AppendHistory(ctx, &HistoryEntry{
		Service:   "api",
		Action:    "upgrade",
		Version:   "v2.0.0",
		Timestamp: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("append history 2: %v", err)
	}

	entries, err := s.ListHistory(ctx, "api")
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(entries))
	}
	// Newest first
	if entries[0].Action != "upgrade" {
		t.Errorf("first entry action = %q, want %q", entries[0].Action, "upgrade")
	}
	if entries[1].Detail["port"] != "3000" {
		t.Errorf("second entry detail[port] = %q, want %q", entries[1].Detail["port"], "3000")
	}
}
