package interaction

import (
	"context"
	"cyberlife/server/internal/storage"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"time"
)

type Service struct{ store *storage.Store }

func New(store *storage.Store) *Service { return &Service{store} }

type Comment struct {
	ID          string `json:"id"`
	TargetType  string `json:"targetType"`
	TargetID    string `json:"targetId"`
	AuthorKeyID string `json:"authorKeyId"`
	Content     string `json:"content"`
	CreatedAt   string `json:"createdAt"`
}
type Milestone struct {
	ID          string `json:"id"`
	TargetType  string `json:"targetType"`
	TargetID    string `json:"targetId"`
	Description string `json:"description"`
	Detail      string `json:"detail"`
	PresetID    string `json:"presetId"`
	Secret      bool   `json:"secret"`
}
type TargetAccess struct {
	TargetType  string
	TargetID    string
	Date        string
	PresetID    string
	Secret      bool
	Commentable bool
}

// findTarget locates the month database holding the diary or task. Comments and milestones are stored
// next to their target, so a comment on last month's diary lands in last month's database and is found
// again when that day is opened on the past page. The caller closes the returned handle.
func (s *Service) findTarget(ctx context.Context, life, targetType, targetID string) (*sql.DB, TargetAccess, error) {
	if targetType != "diary" && targetType != "task" {
		return nil, TargetAccess{}, fmt.Errorf("目标类型无效")
	}
	if targetID == "" {
		return nil, TargetAccess{}, fmt.Errorf("目标不存在")
	}
	months, e := s.store.LifeMonths(ctx, life)
	if e != nil {
		return nil, TargetAccess{}, e
	}
	for _, month := range months {
		db, e := s.store.OpenLifeMonth(ctx, life, month)
		if e != nil {
			return nil, TargetAccess{}, e
		}
		x := TargetAccess{TargetType: targetType, TargetID: targetID}
		var secret, commentable int
		if targetType == "diary" {
			e = db.QueryRowContext(ctx, "SELECT entry_date,COALESCE(visibility_preset_id,''),COALESCE(secret,0),COALESCE(commentable,0) FROM diary_entries WHERE id=? AND life_id=?", targetID, life).Scan(&x.Date, &x.PresetID, &secret, &commentable)
		} else {
			e = db.QueryRowContext(ctx, "SELECT task_date,COALESCE(visibility_preset_id,''),COALESCE(secret,0),COALESCE(commentable,0) FROM tasks WHERE id=? AND life_id=?", targetID, life).Scan(&x.Date, &x.PresetID, &secret, &commentable)
		}
		if e == nil {
			x.Secret = secret == 1
			x.Commentable = commentable == 1
			return db, x, nil
		}
		db.Close()
		if e != sql.ErrNoRows {
			return nil, TargetAccess{}, e
		}
	}
	return nil, TargetAccess{}, fmt.Errorf("目标不存在")
}

func (s *Service) TargetAccess(ctx context.Context, life, targetType, targetID string) (TargetAccess, error) {
	db, x, e := s.findTarget(ctx, life, targetType, targetID)
	if e != nil {
		return TargetAccess{}, e
	}
	db.Close()
	return x, nil
}

// AddComment stores the comment in the target's month database and returns the target so callers can
// notify the writer with the target date.
func (s *Service) AddComment(ctx context.Context, life, actorID, targetType, targetID, content string) (Comment, TargetAccess, error) {
	if content == "" {
		return Comment{}, TargetAccess{}, fmt.Errorf("评论内容不能为空")
	}
	db, target, e := s.findTarget(ctx, life, targetType, targetID)
	if e != nil {
		return Comment{}, TargetAccess{}, e
	}
	defer db.Close()
	if !target.Commentable {
		return Comment{}, target, fmt.Errorf("目标不存在或未开启评论")
	}
	x := Comment{ID: uuid.NewString(), TargetType: targetType, TargetID: targetID, AuthorKeyID: actorID, Content: content, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	_, e = db.ExecContext(ctx, "INSERT INTO comments(id,target_type,target_id,author_key_id,content,created_at) VALUES(?,?,?,?,?,?)", x.ID, x.TargetType, x.TargetID, x.AuthorKeyID, x.Content, x.CreatedAt)
	return x, target, e
}

func (s *Service) ListComments(ctx context.Context, life, targetType, targetID string) ([]Comment, error) {
	db, _, e := s.findTarget(ctx, life, targetType, targetID)
	if e != nil {
		return nil, e
	}
	defer db.Close()
	rows, e := db.QueryContext(ctx, "SELECT id,target_type,target_id,author_key_id,content,created_at FROM comments WHERE target_type=? AND target_id=? AND deleted_at IS NULL ORDER BY created_at", targetType, targetID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		var x Comment
		if e = rows.Scan(&x.ID, &x.TargetType, &x.TargetID, &x.AuthorKeyID, &x.Content, &x.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) ListMilestones(ctx context.Context, life, targetType, targetID string) ([]Milestone, error) {
	db, _, e := s.findTarget(ctx, life, targetType, targetID)
	if e != nil {
		return nil, e
	}
	defer db.Close()
	rows, e := db.QueryContext(ctx, "SELECT id,target_type,target_id,description,detail,COALESCE(visibility_preset_id,''),COALESCE(secret,0) FROM milestones WHERE target_type=? AND target_id=? ORDER BY created_at", targetType, targetID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Milestone{}
	for rows.Next() {
		var x Milestone
		var secret int
		if e = rows.Scan(&x.ID, &x.TargetType, &x.TargetID, &x.Description, &x.Detail, &x.PresetID, &secret); e != nil {
			return nil, e
		}
		x.Secret = secret == 1
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) AddMilestone(ctx context.Context, life, targetType, targetID, description, detail, presetID string, secret bool) (Milestone, error) {
	if description == "" {
		return Milestone{}, fmt.Errorf("里程碑参数无效")
	}
	db, _, e := s.findTarget(ctx, life, targetType, targetID)
	if e != nil {
		return Milestone{}, e
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	x := Milestone{ID: uuid.NewString(), TargetType: targetType, TargetID: targetID, Description: description, Detail: detail, PresetID: presetID, Secret: secret}
	_, e = db.ExecContext(ctx, "INSERT INTO milestones(id,target_type,target_id,description,detail,visibility_preset_id,secret,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)", x.ID, x.TargetType, x.TargetID, x.Description, x.Detail, nullString(presetID), x.Secret, now, now)
	return x, e
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
