package admin

import (
	"context"
	"testing"
	"time"

	"cyberlife/server/internal/storage"
)

func TestReaderKeyIsOwnedByLifeWhenRevoked(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, writer := range []struct{ id, life string }{{"writer-a", "life-a"}, {"writer-b", "life-b"}} {
		if _, err := store.Global().Exec(`INSERT INTO writers(id,nickname,master_key_hash,created_at) VALUES(?,?,?,?)`, writer.id, writer.id, "hash", now); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Global().Exec(`INSERT INTO lives(id,owner_id,status,last_active_at,created_at) VALUES(?,?,?,?,?)`, writer.life, writer.id, "active", now, now); err != nil {
			t.Fatal(err)
		}
	}

	service := New(store.Global(), store)
	ctx := context.Background()
	item, key, err := service.CreateReaderKey(ctx, "life-a", "Reader A", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if key == "" || item.ID == "" {
		t.Fatalf("expected generated reader key, got item=%+v key=%q", item, key)
	}
	if _, _, err := service.CreateReaderKey(ctx, "life-a", "", "", nil); err == nil {
		t.Fatal("expected empty nickname to be rejected")
	}
	if err := service.RevokeReaderKey(ctx, "life-b", item.ID); err == nil {
		t.Fatal("expected another life to be unable to revoke the key")
	}
	var revokedAt *string
	if err := store.Global().QueryRow(`SELECT revoked_at FROM reader_keys WHERE id=?`, item.ID).Scan(&revokedAt); err != nil {
		t.Fatal(err)
	}
	if revokedAt != nil {
		t.Fatalf("key was revoked by the wrong life: %v", *revokedAt)
	}
	if err := service.RevokeReaderKey(ctx, "life-a", item.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.RevokeReaderKey(ctx, "life-a", item.ID); err == nil {
		t.Fatal("expected a revoked key to be rejected")
	}
}
