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

func (s *Store) Close() error    { return s.global.Close() }
func (s *Store) Global() *sql.DB { return s.global }

func openSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
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
CREATE TABLE IF NOT EXISTS permission_presets (
  id TEXT PRIMARY KEY, life_id TEXT NOT NULL REFERENCES lives(id), name TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  UNIQUE(life_id, name)
);
CREATE TABLE IF NOT EXISTS preset_key_rules (
  preset_id TEXT NOT NULL REFERENCES permission_presets(id), reader_key_id TEXT NOT NULL REFERENCES reader_keys(id),
  allowed INTEGER NOT NULL CHECK(allowed IN (0,1)), updated_at TEXT NOT NULL, PRIMARY KEY(preset_id, reader_key_id)
);
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
	if err != nil {
		return fmt.Errorf("migrate global database: %w", err)
	}
	return nil
}

func (s *Store) EnsureLifeMonth(ctx context.Context, lifeID string, date time.Time) error {
	monthKey := date.In(shanghai()).Format("2006-01")
	lifeDir := filepath.Join(s.dataDir, "lives", lifeID)
	if err := os.MkdirAll(lifeDir, 0o750); err != nil {
		return fmt.Errorf("create life directory: %w", err)
	}
	db, err := openSQLite(filepath.Join(lifeDir, monthKey+".db"))
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err = db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS diary_entries (
 id TEXT PRIMARY KEY, life_id TEXT NOT NULL, entry_date TEXT NOT NULL, content_md TEXT NOT NULL DEFAULT '', secret INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_diary_entries_date ON diary_entries(entry_date);
CREATE TABLE IF NOT EXISTS mood_tags (id TEXT PRIMARY KEY, name TEXT NOT NULL, emoji TEXT NOT NULL, value INTEGER NOT NULL CHECK(value > 0), sort_order INTEGER NOT NULL, created_at TEXT NOT NULL, UNIQUE(name));
CREATE TABLE IF NOT EXISTS mood_records (id TEXT PRIMARY KEY, life_id TEXT NOT NULL, recorded_at TEXT NOT NULL, recorded_date TEXT NOT NULL, value REAL NOT NULL, note TEXT NOT NULL DEFAULT '', tags_json TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_mood_records_date ON mood_records(recorded_date, recorded_at);
CREATE TABLE IF NOT EXISTS body_records (id TEXT PRIMARY KEY, life_id TEXT NOT NULL, recorded_at TEXT NOT NULL, recorded_date TEXT NOT NULL, score INTEGER NOT NULL CHECK(score BETWEEN 0 AND 100), note TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_body_records_date ON body_records(recorded_date, recorded_at);
CREATE TABLE IF NOT EXISTS tasks (id TEXT PRIMARY KEY, life_id TEXT NOT NULL, task_date TEXT NOT NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', priority TEXT NOT NULL DEFAULT 'normal' CHECK(priority IN ('low','normal','high')), done INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_tasks_date ON tasks(task_date, done);
CREATE TABLE IF NOT EXISTS playlists (id TEXT PRIMARY KEY, life_id TEXT NOT NULL, page TEXT NOT NULL CHECK(page IN ('now','past','future')), name TEXT NOT NULL, mode TEXT NOT NULL DEFAULT 'list' CHECK(mode IN ('list','random','single')), volume INTEGER NOT NULL DEFAULT 70 CHECK(volume BETWEEN 0 AND 100), created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS playlist_tracks (id TEXT PRIMARY KEY, playlist_id TEXT NOT NULL REFERENCES playlists(id), file_path TEXT NOT NULL, title TEXT NOT NULL, sort_order INTEGER NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS inbox_messages (id TEXT PRIMARY KEY, life_id TEXT NOT NULL, recipient_type TEXT NOT NULL, recipient_id TEXT NOT NULL, type TEXT NOT NULL, ref_id TEXT, text TEXT NOT NULL, read_at TEXT, created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_inbox_recipient ON inbox_messages(recipient_id,created_at);
CREATE TABLE IF NOT EXISTS plans (id TEXT PRIMARY KEY, life_id TEXT NOT NULL, name TEXT NOT NULL, start_date TEXT NOT NULL, end_date TEXT NOT NULL, intro_md TEXT NOT NULL DEFAULT '', secret INTEGER NOT NULL DEFAULT 0, commentable INTEGER NOT NULL DEFAULT 0, visibility_preset_id TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_plans_end ON plans(end_date);
CREATE TABLE IF NOT EXISTS plan_progress (id TEXT PRIMARY KEY, plan_id TEXT NOT NULL REFERENCES plans(id), date TEXT NOT NULL, percent REAL NOT NULL CHECK(percent BETWEEN 0 AND 100), created_at TEXT NOT NULL, UNIQUE(plan_id,date));
CREATE TABLE IF NOT EXISTS diary_drafts (entry_date TEXT PRIMARY KEY, content_md TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS content_attachments (id TEXT PRIMARY KEY, entry_date TEXT NOT NULL, original_name TEXT NOT NULL, stored_name TEXT NOT NULL UNIQUE, mime_type TEXT NOT NULL, byte_size INTEGER NOT NULL, created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_attachments_date ON content_attachments(entry_date);
CREATE TABLE IF NOT EXISTS comments (id TEXT PRIMARY KEY, target_type TEXT NOT NULL CHECK(target_type IN ('diary','task')), target_id TEXT NOT NULL, parent_id TEXT, author_key_id TEXT NOT NULL, content TEXT NOT NULL, created_at TEXT NOT NULL, deleted_at TEXT);
CREATE INDEX IF NOT EXISTS idx_comments_target ON comments(target_type, target_id, created_at);
CREATE TABLE IF NOT EXISTS milestones (id TEXT PRIMARY KEY, target_type TEXT NOT NULL CHECK(target_type IN ('diary','task')), target_id TEXT NOT NULL UNIQUE, description TEXT NOT NULL, detail TEXT NOT NULL DEFAULT '', visibility_preset_id TEXT, secret INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
`); err != nil {
		return fmt.Errorf("migrate life month database: %w", err)
	}
	for _, change := range []struct{ table, column, definition string }{
		{"diary_entries", "visibility_preset_id", "TEXT"}, {"diary_entries", "commentable", "INTEGER NOT NULL DEFAULT 0"},
		{"tasks", "visibility_preset_id", "TEXT"}, {"tasks", "commentable", "INTEGER NOT NULL DEFAULT 0"}, {"tasks", "secret", "INTEGER NOT NULL DEFAULT 0"},
		{"mood_records", "secret", "INTEGER NOT NULL DEFAULT 0"}, {"body_records", "secret", "INTEGER NOT NULL DEFAULT 0"},
		{"content_attachments", "visibility_preset_id", "TEXT"}, {"content_attachments", "secret", "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := ensureColumn(ctx, db, change.table, change.column, change.definition); err != nil {
			return err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.global.ExecContext(ctx, `INSERT INTO life_months(life_id, month_key, schema_version, created_at, checked_at)
VALUES (?, ?, 1, ?, ?) ON CONFLICT(life_id, month_key) DO UPDATE SET checked_at=excluded.checked_at`, lifeID, monthKey, now, now)
	return err
}

func ensureColumn(ctx context.Context, db *sql.DB, table, column, definition string) error {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+column+" "+definition); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}
	return nil
}

func (s *Store) LifeDBPath(lifeID, monthKey string) string {
	return filepath.Join(s.dataDir, "lives", lifeID, monthKey+".db")
}
func (s *Store) UploadDir(lifeID, monthKey string) string {
	return filepath.Join(s.dataDir, "uploads", lifeID, "diary", monthKey)
}

func shanghai() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return location
}
