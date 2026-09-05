package future

import (
	"context"
	"testing"
	"time"

	"cyberlife/server/internal/storage"
)

func newLife(t *testing.T) (*storage.Store, string) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.Global().Exec(`INSERT INTO writers(id,nickname,master_key_hash,created_at) VALUES('writer-1','Tester','hash',?)`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Global().Exec(`INSERT INTO lives(id,owner_id,status,last_active_at,created_at) VALUES('life-1','writer-1','active',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	return store, "life-1"
}

// TestPlansSurviveMonthRollover: a plan written to an earlier month database must still be listed,
// editable and markable from the current month.
func TestPlansSurviveMonthRollover(t *testing.T) {
	store, life := newLife(t)
	ctx := context.Background()
	old, err := store.OpenLifeMonth(ctx, life, "2000-01")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := old.Exec(`INSERT INTO plans(id,life_id,name,start_date,end_date,intro_md,secret,commentable,created_at,updated_at) VALUES('plan-old',?,'旧规划','2000-01-01','2000-12-31','',0,0,?,?)`, life, now, now); err != nil {
		t.Fatal(err)
	}
	old.Close()
	service := New(store)
	plans, err := service.ListPlans(ctx, life)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].ID != "plan-old" {
		t.Fatalf("expected the old plan to be listed, got %+v", plans)
	}
	updated, err := service.UpdatePlan(ctx, life, "plan-old", "旧规划（改）", "2000-01-01", "2001-06-30", "# 简介")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "旧规划（改）" || updated.EndDate != "2001-06-30" {
		t.Fatalf("unexpected update result %+v", updated)
	}
	marked, err := service.SetProgress(ctx, life, "plan-old", "2000-06-01", 40)
	if err != nil {
		t.Fatal(err)
	}
	if marked.Progress != 40 {
		t.Fatalf("expected progress 40, got %v", marked.Progress)
	}
	created, err := service.CreatePlan(ctx, life, "新规划", "2030-01-01", "2030-02-01", "")
	if err != nil {
		t.Fatal(err)
	}
	plans, err = service.ListPlans(ctx, life)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || plans[0].ID != "plan-old" || plans[1].ID != created.ID {
		t.Fatalf("expected both plans sorted by end date, got %+v", plans)
	}
}

// TestCalendarBatchesMonths: tasks across two months come back from one call.
func TestCalendarBatchesMonths(t *testing.T) {
	store, life := newLife(t)
	ctx := context.Background()
	for _, item := range []struct{ month, date, id string }{{"2000-01", "2000-01-31", "task-a"}, {"2000-02", "2000-02-01", "task-b"}} {
		db, err := store.OpenLifeMonth(ctx, life, item.month)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := db.Exec(`INSERT INTO tasks(id,life_id,task_date,title,description,priority,done,created_at,updated_at) VALUES(?,?,?,'t','','normal',0,?,?)`, item.id, life, item.date, now, now); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
	tasks, err := New(store).Calendar(ctx, life, "2000-01-30", "2000-02-02")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].ID != "task-a" || tasks[1].ID != "task-b" {
		t.Fatalf("expected two tasks in date order, got %+v", tasks)
	}
}
