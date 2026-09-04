package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	dataDir string
	global  *sql.DB
}

func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "lives"), 0o750); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	globalPath := filepath.Join(dataDir, "global.db")
	db, err := openSQLite(globalPath)
	if err != nil {
		return nil, err
	}
	store := &Store{dataDir: dataDir, global: db}
	if err := store.migrateGlobal(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.global.Close() }
func (s *Store) Global() *sql.DB { return s.global }

func openSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil { return nil, fmt.Errorf("open sqlite: %w", err) }
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil { _ = db.Close(); return nil, fmt.Errorf("ping sqlite: %w", err) }
	return db, nil
}

func (s *Store) migrateGlobal(ctx context.Context) error {
	_, err := s.global.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS admins (
  id TEXT PRIMARY KEY, password_hash TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS writers (
  id TEXT PRIMARY KEY, nickname TEXT NOT NULL, master_key_hash TEXT NOT NULL, key_version INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL, revoked_at TEXT
);
CREATE TABLE IF NOT EXISTS lives (
  id TEXT PRIMARY KEY, owner_id TEXT NOT NULL UNIQUE REFERENCES writers(id), status TEXT NOT NULL DEFAULT 'active'
    CHECK(status IN ('active','lost','stopped')), last_active_at TEXT NOT NULL, created_at TEXT NOT NULL, stopped_at TEXT
);
CREATE TABLE IF NOT EXISTS reader_keys (
  id TEXT PRIMARY KEY, life_id TEXT NOT NULL REFERENCES lives(id), key_hash TEXT NOT NULL, nickname TEXT NOT NULL,
  anchor_at_utc TEXT NOT NULL, anchor_local_date TEXT NOT NULL, expires_at TEXT, revoked_at TEXT, note TEXT,
  key_version INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_reader_keys_life ON reader_keys(life_id);
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY, token_hash TEXT NOT NULL UNIQUE, actor_type TEXT NOT NULL CHECK(actor_type IN ('admin','writer','reader','mobile')),
  actor_id TEXT NOT NULL, life_id TEXT, key_version INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL,
  last_active_at TEXT NOT NULL, revoked_at TEXT, user_agent TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token_hash);
CREATE TABLE IF NOT EXISTS life_months (
  life_id TEXT NOT NULL REFERENCES lives(id), month_key TEXT NOT NULL, schema_version INTEGER NOT NULL,
  created_at TEXT NOT NULL, checked_at TEXT NOT NULL, PRIMARY KEY(life_id, month_key)
);
`)
	if err != nil { return fmt.Errorf("migrate global database: %w", err) }
	return nil
}

func (s *Store) EnsureLifeMonth(ctx context.Context, lifeID string, date time.Time) error {
	monthKey := date.In(shanghai()).Format("2006-01")
	lifeDir := filepath.Join(s.dataDir, "lives", lifeID)
	if err := os.MkdirAll(lifeDir, 0o750); err != nil { return fmt.Errorf("create life directory: %w", err) }
	db, err := openSQLite(filepath.Join(lifeDir, monthKey+".db"))
	if err != nil { return err }
	defer db.Close()
	if _, err = db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS diary_entries (
 id TEXT PRIMARY KEY, life_id TEXT NOT NULL, entry_date TEXT NOT NULL, content_md TEXT NOT NULL DEFAULT '', secret INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_diary_entries_date ON diary_entries(entry_date);
`); err != nil { return fmt.Errorf("migrate life month database: %w", err) }
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.global.ExecContext(ctx, `INSERT INTO life_months(life_id, month_key, schema_version, created_at, checked_at)
VALUES (?, ?, 1, ?, ?) ON CONFLICT(life_id, month_key) DO UPDATE SET checked_at=excluded.checked_at`, lifeID, monthKey, now, now)
	return err
}

func (s *Store) LifeDBPath(lifeID, monthKey string) string { return filepath.Join(s.dataDir, "lives", lifeID, monthKey+".db") }

func shanghai() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil { return time.FixedZone("CST", 8*3600) }
	return location
}
