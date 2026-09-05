package auth

import (
	"context"
	"testing"
	"time"

	"cyberlife/server/internal/storage"
)

// TestKeyLoginDoesNotDeadlock guards the single-connection SQLite pool: KeyLogin used to hold the
// writers cursor open while inserting the session, which blocked forever.
func TestKeyLoginDoesNotDeadlock(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hash, err := hashSecret("cl_test-master-key")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.Global().Exec(`INSERT INTO writers(id,nickname,master_key_hash,created_at) VALUES('writer-1','Tester',?,?)`, hash, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Global().Exec(`INSERT INTO lives(id,owner_id,status,last_active_at,created_at) VALUES('life-1','writer-1','active',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	service := New(store.Global())
	token, actor, err := service.KeyLogin(ctx, "cl_test-master-key", "test")
	if err != nil {
		t.Fatalf("key login failed: %v", err)
	}
	if token == "" || actor.Type != "writer" || actor.LifeID != "life-1" {
		t.Fatalf("unexpected actor %+v", actor)
	}
	if actor.Nickname != "Tester" || actor.LifeCreatedAt == "" {
		t.Fatalf("actor not enriched: %+v", actor)
	}
	resolved, err := service.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}
	if resolved.Nickname != "Tester" || resolved.SessionID != actor.SessionID {
		t.Fatalf("unexpected resolved actor %+v", resolved)
	}
	if _, _, err := service.KeyLogin(ctx, "wrong-key", "test"); err == nil {
		t.Fatal("expected wrong key to be rejected")
	}
}
