package db

import (
	"context"
	"os"
	"testing"
)

func TestValidateServiceName(t *testing.T) {
	valid := []string{"api", "my-api", "myApi2", "auth_service", "a"}
	for _, name := range valid {
		if err := ValidateServiceName(name); err != nil {
			t.Errorf("ValidateServiceName(%q) should pass, got: %v", name, err)
		}
	}

	invalid := []string{"", "-bad", "_bad", "has space", "has;semi", "a/b", "../etc"}
	for _, name := range invalid {
		if err := ValidateServiceName(name); err == nil {
			t.Errorf("ValidateServiceName(%q) should fail", name)
		}
	}
}

// Integration tests — only run when GOPHERCAPTAIN_TEST_MARIADB is set.
func TestIntegrationCreateAndDrop(t *testing.T) {
	dsn := os.Getenv("GOPHERCAPTAIN_TEST_MARIADB")
	if dsn == "" {
		t.Skip("set GOPHERCAPTAIN_TEST_MARIADB to run integration tests (format: user:pass@tcp(host:port)/)")
	}

	mgr, err := New("127.0.0.1", 3306, "root", dsn)
	if err != nil {
		t.Fatalf("creating manager: %v", err)
	}
	defer mgr.Close()

	ctx := context.Background()

	if err := mgr.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	result, err := mgr.CreateDatabase(ctx, "integrationtest")
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Logf("created db=%s user=%s", result.DBName, result.DBUser)

	exists, err := mgr.DatabaseExists(ctx, "integrationtest")
	if err != nil {
		t.Fatalf("database exists: %v", err)
	}
	if !exists {
		t.Error("database should exist after creation")
	}

	// Creating again should fail
	_, err = mgr.CreateDatabase(ctx, "integrationtest")
	if err == nil {
		t.Error("expected error creating duplicate database")
	}

	// Clean up
	if err := mgr.DropDatabase(ctx, "integrationtest"); err != nil {
		t.Fatalf("drop database: %v", err)
	}
}
