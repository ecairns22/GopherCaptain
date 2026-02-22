package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"

	_ "github.com/go-sql-driver/mysql"
	"github.com/ecairns22/GopherCaptain/internal/config"
	"github.com/ecairns22/GopherCaptain/internal/creds"
)

var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// Manager handles MariaDB database and user lifecycle.
type Manager struct {
	db *sql.DB
}

// CreateResult holds the details of a newly created database.
type CreateResult struct {
	DBName   string
	DBUser   string
	Password string
}

// New creates a DB manager connecting to MariaDB with the given admin credentials.
func New(host string, port int, user, password string) (*Manager, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, host, port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("connecting to MariaDB: %w", err)
	}
	return &Manager{db: db}, nil
}

// NewFromConfig creates a DB manager from the tool configuration.
func NewFromConfig(cfg *config.Config) (*Manager, error) {
	return New(cfg.MariaDB.Host, cfg.MariaDB.Port, cfg.MariaDB.AdminUser, cfg.MariaDB.AdminPassword)
}

// Close closes the underlying database connection.
func (m *Manager) Close() error {
	return m.db.Close()
}

// Ping tests the MariaDB connection.
func (m *Manager) Ping(ctx context.Context) error {
	if err := m.db.PingContext(ctx); err != nil {
		return fmt.Errorf("MariaDB connection failed: %w; check that MariaDB is running (systemctl status mariadb) and credentials are correct", err)
	}
	return nil
}

// ValidateServiceName checks that a name is safe to use as a DB/user identifier.
func ValidateServiceName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid service name %q: must match %s", name, validName.String())
	}
	return nil
}

// DatabaseExists checks if the database gc_<name> already exists.
func (m *Manager) DatabaseExists(ctx context.Context, name string) (bool, error) {
	dbName := "gc_" + name
	var result string
	err := m.db.QueryRowContext(ctx,
		"SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?", dbName).Scan(&result)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking database existence: %w", err)
	}
	return true, nil
}

// CreateDatabase creates database gc_<name>, user gc_<name>@localhost with a generated password,
// and grants all privileges.
func (m *Manager) CreateDatabase(ctx context.Context, name string) (*CreateResult, error) {
	if err := ValidateServiceName(name); err != nil {
		return nil, err
	}

	dbName := "gc_" + name
	dbUser := "gc_" + name

	// Check if database already exists
	exists, err := m.DatabaseExists(ctx, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("database %q already exists; use --name to choose a different service name", dbName)
	}

	// Generate password
	password, err := creds.Generate(32)
	if err != nil {
		return nil, fmt.Errorf("generating database password: %w", err)
	}

	// Create database — name is validated by regex, safe for identifier use
	if _, err := m.db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE `%s`", dbName)); err != nil {
		return nil, fmt.Errorf("creating database %s: %w", dbName, err)
	}

	// Create user
	if _, err := m.db.ExecContext(ctx, fmt.Sprintf("CREATE USER '%s'@'localhost' IDENTIFIED BY '%s'", dbUser, password)); err != nil {
		// Rollback: drop database
		m.db.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
		return nil, fmt.Errorf("creating user %s: %w", dbUser, err)
	}

	// Grant privileges
	if _, err := m.db.ExecContext(ctx, fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost'", dbName, dbUser)); err != nil {
		// Rollback
		m.db.ExecContext(ctx, fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost'", dbUser))
		m.db.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
		return nil, fmt.Errorf("granting privileges: %w", err)
	}

	if _, err := m.db.ExecContext(ctx, "FLUSH PRIVILEGES"); err != nil {
		return nil, fmt.Errorf("flushing privileges: %w", err)
	}

	return &CreateResult{
		DBName:   dbName,
		DBUser:   dbUser,
		Password: password,
	}, nil
}

// DropDatabase drops the database gc_<name> and user gc_<name>@localhost.
func (m *Manager) DropDatabase(ctx context.Context, name string) error {
	if err := ValidateServiceName(name); err != nil {
		return err
	}

	dbName := "gc_" + name
	dbUser := "gc_" + name

	if _, err := m.db.ExecContext(ctx, fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost'", dbUser)); err != nil {
		return fmt.Errorf("dropping user %s: %w", dbUser, err)
	}

	if _, err := m.db.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName)); err != nil {
		return fmt.Errorf("dropping database %s: %w", dbName, err)
	}

	return nil
}
