package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a queried service does not exist.
var ErrNotFound = errors.New("service not found")

// Service represents a deployed service in the state store.
type Service struct {
	Name        string
	Repo        string
	Version     string
	PrevVersion string
	Port        int
	RouteType   string
	RouteValue  string
	DBName      string
	DBUser      string
	ExtraEnv    map[string]string
	DeployedAt  time.Time
	UpdatedAt   time.Time
}

// HistoryEntry represents an action recorded in the history table.
type HistoryEntry struct {
	ID        int64
	Service   string
	Action    string
	Version   string
	Timestamp time.Time
	Detail    map[string]string
}

// Store wraps a SQLite database for state management.
type Store struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at the given path with WAL mode.
// Use ":memory:" for in-memory databases in tests.
func Open(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening state db %s: %w", dbPath, err)
	}

	// WAL mode for better concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	// Limit connections — SQLite handles one writer at a time
	db.SetMaxOpenConns(1)

	// Create schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

// InsertService adds a new service to the store.
func (s *Store) InsertService(ctx context.Context, svc *Service) error {
	extraEnv, err := marshalJSON(svc.ExtraEnv)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO services (name, repo, version, prev_version, port, route_type, route_value, db_name, db_user, extra_env, deployed_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		svc.Name, svc.Repo, svc.Version, nullString(svc.PrevVersion),
		svc.Port, svc.RouteType, svc.RouteValue,
		svc.DBName, svc.DBUser, extraEnv,
		svc.DeployedAt.Unix(), svc.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("inserting service %s: %w", svc.Name, err)
	}
	return nil
}

// GetService retrieves a service by name.
func (s *Store) GetService(ctx context.Context, name string) (*Service, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT name, repo, version, prev_version, port, route_type, route_value, db_name, db_user, extra_env, deployed_at, updated_at
		 FROM services WHERE name = ?`, name)

	svc, err := scanService(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting service %s: %w", name, err)
	}
	return svc, nil
}

// ListServices returns all services.
func (s *Store) ListServices(ctx context.Context) ([]*Service, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, repo, version, prev_version, port, route_type, route_value, db_name, db_user, extra_env, deployed_at, updated_at
		 FROM services ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}
	defer rows.Close()

	var services []*Service
	for rows.Next() {
		svc, err := scanServiceFromRows(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, svc)
	}
	return services, rows.Err()
}

// UpdateService updates a service record.
func (s *Store) UpdateService(ctx context.Context, svc *Service) error {
	extraEnv, err := marshalJSON(svc.ExtraEnv)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE services SET repo=?, version=?, prev_version=?, port=?, route_type=?, route_value=?, db_name=?, db_user=?, extra_env=?, updated_at=?
		 WHERE name=?`,
		svc.Repo, svc.Version, nullString(svc.PrevVersion),
		svc.Port, svc.RouteType, svc.RouteValue,
		svc.DBName, svc.DBUser, extraEnv,
		svc.UpdatedAt.Unix(), svc.Name,
	)
	if err != nil {
		return fmt.Errorf("updating service %s: %w", svc.Name, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteService removes a service by name.
func (s *Store) DeleteService(ctx context.Context, name string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM services WHERE name=?`, name)
	if err != nil {
		return fmt.Errorf("deleting service %s: %w", name, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UsedPorts returns all ports currently assigned to services.
func (s *Store) UsedPorts(ctx context.Context) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT port FROM services ORDER BY port`)
	if err != nil {
		return nil, fmt.Errorf("querying used ports: %w", err)
	}
	defer rows.Close()

	var ports []int
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		ports = append(ports, p)
	}
	return ports, rows.Err()
}

// PortOwner returns the name of the service using the given port, or empty string if free.
func (s *Store) PortOwner(ctx context.Context, port int) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT name FROM services WHERE port=?`, port).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("querying port owner for %d: %w", port, err)
	}
	return name, nil
}

// AppendHistory records an action in the history table.
func (s *Store) AppendHistory(ctx context.Context, entry *HistoryEntry) error {
	detail, err := marshalJSON(entry.Detail)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO history (service, action, version, timestamp, detail) VALUES (?, ?, ?, ?, ?)`,
		entry.Service, entry.Action, nullString(entry.Version),
		entry.Timestamp.Unix(), detail,
	)
	if err != nil {
		return fmt.Errorf("appending history: %w", err)
	}
	return nil
}

// ListHistory returns history entries for a service, newest first.
func (s *Store) ListHistory(ctx context.Context, service string) ([]*HistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, service, action, version, timestamp, detail FROM history WHERE service=? ORDER BY id DESC`, service)
	if err != nil {
		return nil, fmt.Errorf("listing history for %s: %w", service, err)
	}
	defer rows.Close()

	var entries []*HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		var version sql.NullString
		var detail sql.NullString
		var ts int64
		if err := rows.Scan(&e.ID, &e.Service, &e.Action, &version, &ts, &detail); err != nil {
			return nil, err
		}
		e.Version = version.String
		e.Timestamp = time.Unix(ts, 0)
		if detail.Valid && detail.String != "" {
			if err := json.Unmarshal([]byte(detail.String), &e.Detail); err != nil {
				return nil, fmt.Errorf("unmarshaling history detail: %w", err)
			}
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

// scanService scans a single row into a Service.
func scanService(row *sql.Row) (*Service, error) {
	var svc Service
	var prevVersion sql.NullString
	var extraEnv sql.NullString
	var deployedAt, updatedAt int64

	err := row.Scan(
		&svc.Name, &svc.Repo, &svc.Version, &prevVersion,
		&svc.Port, &svc.RouteType, &svc.RouteValue,
		&svc.DBName, &svc.DBUser, &extraEnv,
		&deployedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	svc.PrevVersion = prevVersion.String
	svc.DeployedAt = time.Unix(deployedAt, 0)
	svc.UpdatedAt = time.Unix(updatedAt, 0)

	if extraEnv.Valid && extraEnv.String != "" {
		if err := json.Unmarshal([]byte(extraEnv.String), &svc.ExtraEnv); err != nil {
			return nil, fmt.Errorf("unmarshaling extra_env: %w", err)
		}
	}

	return &svc, nil
}

// scanServiceFromRows scans a row from *sql.Rows into a Service.
func scanServiceFromRows(rows *sql.Rows) (*Service, error) {
	var svc Service
	var prevVersion sql.NullString
	var extraEnv sql.NullString
	var deployedAt, updatedAt int64

	err := rows.Scan(
		&svc.Name, &svc.Repo, &svc.Version, &prevVersion,
		&svc.Port, &svc.RouteType, &svc.RouteValue,
		&svc.DBName, &svc.DBUser, &extraEnv,
		&deployedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	svc.PrevVersion = prevVersion.String
	svc.DeployedAt = time.Unix(deployedAt, 0)
	svc.UpdatedAt = time.Unix(updatedAt, 0)

	if extraEnv.Valid && extraEnv.String != "" {
		if err := json.Unmarshal([]byte(extraEnv.String), &svc.ExtraEnv); err != nil {
			return nil, fmt.Errorf("unmarshaling extra_env: %w", err)
		}
	}

	return &svc, nil
}

func marshalJSON(v interface{}) (sql.NullString, error) {
	if v == nil {
		return sql.NullString{}, nil
	}
	// Check for empty map
	if m, ok := v.(map[string]string); ok && len(m) == 0 {
		return sql.NullString{}, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("marshaling JSON: %w", err)
	}
	return sql.NullString{String: string(data), Valid: true}, nil
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
