package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const currentVersion = 1

type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at dbPath and runs migrations.
func New(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	// Configure pragmas.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec pragma %q: %w", p, err)
		}
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// NewMemory creates an in-memory store for testing.
func NewMemory() (*Store, error) {
	return New(":memory:")
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	var version int
	err := s.db.QueryRow("PRAGMA user_version").Scan(&version)
	if err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	if version >= currentVersion {
		return nil
	}

	if version < 1 {
		if err := s.migrateV1(); err != nil {
			return err
		}
	}

	_, err = s.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", currentVersion))
	return err
}

func (s *Store) migrateV1() error {
	const ddl = `
	CREATE TABLE IF NOT EXISTS projects (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL UNIQUE,
		color       TEXT NOT NULL DEFAULT '#6C63FF',
		category    TEXT NOT NULL DEFAULT 'work',
		archived    INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id  INTEGER NOT NULL REFERENCES projects(id),
		name        TEXT NOT NULL,
		tags        TEXT NOT NULL DEFAULT '',
		archived    INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		UNIQUE(project_id, name)
	);

	CREATE TABLE IF NOT EXISTS time_entries (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id  INTEGER NOT NULL REFERENCES projects(id),
		task_id     INTEGER REFERENCES tasks(id),
		start_time  TEXT NOT NULL,
		end_time    TEXT,
		duration    INTEGER NOT NULL DEFAULT 0,
		notes       TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	);

	CREATE INDEX IF NOT EXISTS idx_entries_project ON time_entries(project_id);
	CREATE INDEX IF NOT EXISTS idx_entries_start   ON time_entries(start_time);

	CREATE TABLE IF NOT EXISTS pomodoro_sessions (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		time_entry_id   INTEGER REFERENCES time_entries(id),
		work_duration   INTEGER NOT NULL DEFAULT 1500,
		break_duration  INTEGER NOT NULL DEFAULT 300,
		completed_count INTEGER NOT NULL DEFAULT 0,
		target_count    INTEGER NOT NULL DEFAULT 4,
		status          TEXT NOT NULL DEFAULT 'idle',
		started_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		completed_at    TEXT
	);

	CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	INSERT OR IGNORE INTO settings (key, value) VALUES
		('pomodoro_work',       '1500'),
		('pomodoro_break',      '300'),
		('pomodoro_long_break', '900'),
		('pomodoro_count',      '4'),
		('idle_timeout',        '300'),
		('idle_action',         'pause'),
		('daily_goal',          '28800'),
		('week_start',          'monday');
	`
	_, err := s.db.Exec(ddl)
	return err
}

// DefaultDBPath returns ~/.config/trackr/trackr.db
func DefaultDBPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "trackr", "trackr.db"), nil
}
