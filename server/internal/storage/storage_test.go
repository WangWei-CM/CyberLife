package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureLifeMonthCreatesDatabaseAndIndex(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory)
	if err != nil { t.Fatal(err) }
	defer store.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.Global().Exec(`INSERT INTO writers(id,nickname,master_key_hash,created_at) VALUES('writer-test','Tester','hash',?)`, now); err != nil { t.Fatal(err) }
	if _, err := store.Global().Exec(`INSERT INTO lives(id,owner_id,status,last_active_at,created_at) VALUES('life-test','writer-test','active',?,?)`, now, now); err != nil { t.Fatal(err) }
	date := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	if err := store.EnsureLifeMonth(context.Background(), "life-test", date); err != nil { t.Fatal(err) }
	if _, err := os.Stat(filepath.Join(directory, "lives", "life-test", "2026-09.db")); err != nil { t.Fatalf("month database not created: %v", err) }
	var count int
	if err := store.Global().QueryRow(`SELECT COUNT(*) FROM life_months WHERE life_id=? AND month_key=?`, "life-test", "2026-09").Scan(&count); err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("expected one month index, got %d", count) }
}
