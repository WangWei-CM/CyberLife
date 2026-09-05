package interaction

import (
	"context"
	"testing"
	"time"

	"cyberlife/server/internal/storage"
)

// TestCommentOnEarlierMonthTarget: comments and milestones follow their target into its month database.
func TestCommentOnEarlierMonthTarget(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.Global().Exec(`INSERT INTO writers(id,nickname,master_key_hash,created_at) VALUES('writer-1','Tester','hash',?)`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Global().Exec(`INSERT INTO lives(id,owner_id,status,last_active_at,created_at) VALUES('life-1','writer-1','active',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	old, err := store.OpenLifeMonth(ctx, "life-1", "2000-01")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(`INSERT INTO diary_entries(id,life_id,entry_date,content_md,secret,commentable,created_at,updated_at) VALUES('d-old','life-1','2000-01-15','旧日记',0,1,?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	old.Close()
	service := New(store)
	comment, target, err := service.AddComment(ctx, "life-1", "reader-1", "diary", "d-old", "写得真好")
	if err != nil {
		t.Fatal(err)
	}
	if target.Date != "2000-01-15" || !target.Commentable {
		t.Fatalf("unexpected target %+v", target)
	}
	comments, err := service.ListComments(ctx, "life-1", "diary", "d-old")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].ID != comment.ID {
		t.Fatalf("expected the comment to be listed, got %+v", comments)
	}
	if _, err := service.AddMilestone(ctx, "life-1", "diary", "d-old", "里程碑", "", "", false); err != nil {
		t.Fatal(err)
	}
	milestones, err := service.ListMilestones(ctx, "life-1", "diary", "d-old")
	if err != nil {
		t.Fatal(err)
	}
	if len(milestones) != 1 {
		t.Fatalf("expected one milestone, got %+v", milestones)
	}
	if _, _, err := service.AddComment(ctx, "life-1", "reader-1", "diary", "missing", "x"); err == nil {
		t.Fatal("expected a missing target to fail")
	}
}
