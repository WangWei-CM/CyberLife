package now

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

// TestDiaryLayers: the public and secret diaries of one day are independent rows.
func TestDiaryLayers(t *testing.T) {
	store, life := newLife(t)
	ctx := context.Background()
	service := New(store)
	public, err := service.SaveDiary(ctx, life, "公开层", false)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := service.SaveDiary(ctx, life, "绝密层", true)
	if err != nil {
		t.Fatal(err)
	}
	if public.ID == secret.ID || !secret.Secret || public.Secret {
		t.Fatalf("layers not independent: %+v %+v", public, secret)
	}
	diary, _, _, _, err := service.Today(ctx, life)
	if err != nil {
		t.Fatal(err)
	}
	if diary.ID != public.ID || diary.Content != "公开层" {
		t.Fatalf("today should expose the public layer, got %+v", diary)
	}
	layer, err := service.SecretDiary(ctx, life)
	if err != nil {
		t.Fatal(err)
	}
	if layer.ID != secret.ID || layer.Content != "绝密层" {
		t.Fatalf("secret layer mismatch: %+v", layer)
	}
	again, err := service.SaveDiary(ctx, life, "公开层 v2", false)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != public.ID {
		t.Fatal("saving the public layer again must update the same row")
	}
}

// TestTagsCopyForward: tags defined in an earlier month are available in the current month.
func TestTagsCopyForward(t *testing.T) {
	store, life := newLife(t)
	ctx := context.Background()
	old, err := store.OpenLifeMonth(ctx, life, "2000-01")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(`INSERT INTO mood_tags(id,name,emoji,value,sort_order,created_at) VALUES('tag-1','开心','😄',90,1,'2000-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	old.Close()
	service := New(store)
	tags, err := service.Tags(ctx, life)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].ID != "tag-1" {
		t.Fatalf("expected the tag to be copied forward, got %+v", tags)
	}
	record, err := service.AddMood(ctx, life, "", []string{"tag-1"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if record.Value != 90 {
		t.Fatalf("unexpected mood value %v", record.Value)
	}
}

// TestTasksOnOtherDates: tasks can be created on and toggled in another month.
func TestTasksOnOtherDates(t *testing.T) {
	store, life := newLife(t)
	ctx := context.Background()
	service := New(store)
	task, err := service.AddTask(ctx, life, "远期任务", "", "high", "2000-03-15")
	if err != nil {
		t.Fatal(err)
	}
	if task.TaskDate != "2000-03-15" {
		t.Fatalf("unexpected task date %q", task.TaskDate)
	}
	done, err := service.SetTaskDone(ctx, life, task.ID, true, "2000-03-15")
	if err != nil {
		t.Fatal(err)
	}
	if !done.Done || done.Priority != "high" {
		t.Fatalf("unexpected task after toggle %+v", done)
	}
	if _, err := service.SetTaskDone(ctx, life, task.ID, true, ""); err == nil {
		t.Fatal("task should not be found in the current month")
	}
}

func TestTaskReadsFutureTaskDetailByDate(t *testing.T) {
	store, life := newLife(t)
	ctx := context.Background()
	service := New(store)
	created, err := service.AddTask(ctx, life, "远期详情", "完整 **描述**", "high", "2030-03-15")
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.Task(ctx, life, created.ID, "2030-03-15")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != created.Title || got.Description != "完整 **描述**" || got.Priority != "high" {
		t.Fatalf("unexpected task detail: %+v", got)
	}
	if _, err := service.Task(ctx, life, created.ID, ""); err == nil {
		t.Fatal("expected a required-date error")
	}
	if _, err := service.Task(ctx, life, "missing", "2030-03-15"); err == nil {
		t.Fatal("expected a missing-task error")
	}
	updated, err := service.UpdateTaskDetail(ctx, life, created.ID, "2030-03-15", "已更新", "更新后的详情", "low")
	if err != nil || updated.Title != "已更新" || updated.Priority != "low" {
		t.Fatalf("unexpected update: task=%+v err=%v", updated, err)
	}
	if err := service.DeleteTaskForDate(ctx, life, created.ID, "2030-03-15"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Task(ctx, life, created.ID, "2030-03-15"); err == nil {
		t.Fatal("expected deleted task to be missing")
	}
}
