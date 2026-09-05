package history

import (
	"context"
	"testing"
	"time"

	"cyberlife/server/internal/acl"
	"cyberlife/server/internal/auth"
	nowservice "cyberlife/server/internal/now"
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

// TestRangeAcrossMonths: one call covers two month databases and aggregates diaries, tasks,
// mood averages and the secret layer for the writer.
func TestRangeAcrossMonths(t *testing.T) {
	store, life := newLife(t)
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	january, err := store.OpenLifeMonth(ctx, life, "2000-01")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := january.Exec(`INSERT INTO diary_entries(id,life_id,entry_date,content_md,secret,created_at,updated_at) VALUES('d-public',?,'2000-01-31','公开',0,?,?),('d-secret',?,'2000-01-31','绝密',1,?,?)`, life, now, now, life, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := january.Exec(`INSERT INTO mood_records(id,life_id,recorded_at,recorded_date,value,note,tags_json,created_at) VALUES('m-1',?,?,'2000-01-31',80,'','[]',?),('m-2',?,?,'2000-01-31',60,'','[]',?)`, life, now, now, life, now, now); err != nil {
		t.Fatal(err)
	}
	january.Close()
	february, err := store.OpenLifeMonth(ctx, life, "2000-02")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := february.Exec(`INSERT INTO tasks(id,life_id,task_date,title,description,priority,done,created_at,updated_at) VALUES('t-1',?,'2000-02-01','任务','','normal',0,?,?)`, life, now, now); err != nil {
		t.Fatal(err)
	}
	february.Close()

	writer := auth.Actor{Type: "writer", ID: "writer-1", LifeID: life}
	service := New(store, acl.New(store.Global()))
	from := time.Date(2000, 1, 30, 0, 0, 0, 0, time.UTC)
	to := time.Date(2000, 2, 2, 0, 0, 0, 0, time.UTC)
	result, err := service.Range(ctx, writer, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Days) != 4 {
		t.Fatalf("expected 4 days, got %d", len(result.Days))
	}
	day := result.Days[1]
	if day.Date != "2000-01-31" || day.Diary.ID != "d-public" || day.SecretDiary == nil || day.SecretDiary.ID != "d-secret" {
		t.Fatalf("unexpected day %+v", day)
	}
	if day.MoodCount != 2 || result.Points[1].Mood == nil || *result.Points[1].Mood != 70 {
		t.Fatalf("unexpected mood aggregation %+v %+v", day, result.Points[1])
	}
	if len(result.Days[2].Tasks) != 1 || result.Days[2].Tasks[0].ID != "t-1" {
		t.Fatalf("expected the February task, got %+v", result.Days[2])
	}
	var _ nowservice.Task = result.Days[2].Tasks[0]
}
