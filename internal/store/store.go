// Package store persists boards, tasks and their history in SQLite.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // database/sql driver
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

//go:embed migrations/*.sql
var migrationFS embed.FS

const timeLayout = time.RFC3339

// Store is the SQLite-backed persistence layer.
type Store struct {
	db *sql.DB
}

// Open opens the database file, creating it if it does not exist.
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// A single connection keeps the bot free of SQLITE_BUSY, and the write
	// load here is a handful of rows a day.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// Migrate applies every embedded migration that has not been applied yet.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	var current int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	names, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(names)

	for _, name := range names {
		version, err := migrationVersion(name)
		if err != nil {
			return err
		}
		if version <= current {
			continue
		}

		body, err := migrationFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		err = s.inTx(ctx, func(tx *sql.Tx) error {
			for _, stmt := range splitStatements(string(body)) {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("apply %s: %w", name, err)
				}
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO schema_version (version) VALUES (?)`, version)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func migrationVersion(name string) (int, error) {
	base := path.Base(name)
	digits := base
	if i := strings.IndexByte(base, '_'); i > 0 {
		digits = base[:i]
	}
	version, err := strconv.Atoi(digits)
	if err != nil {
		return 0, fmt.Errorf("migration %s: name must start with a version number", base)
	}
	return version, nil
}

// splitStatements exists because the driver executes one statement per call.
func splitStatements(body string) []string {
	parts := strings.Split(body, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (s *Store) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func formatTime(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(timeLayout)
}

func parseTime(v string) (time.Time, error) {
	t, err := time.Parse(timeLayout, v)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", v, err)
	}
	return t.UTC(), nil
}

func parseNullTime(v sql.NullString) (*time.Time, error) {
	if !v.Valid || v.String == "" {
		return nil, nil
	}
	t, err := parseTime(v.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
