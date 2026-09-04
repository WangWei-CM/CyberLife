package notification

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

type Message struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	RefID     string `json:"refID"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
	Read      bool   `json:"read"`
}

func (s *Service) db(ctx context.Context, life string) (*sql.DB, error) {
	now := time.Now()
	if e := s.store.EnsureLifeMonth(ctx, life, now); e != nil {
		return nil, e
	}
	return sql.Open("sqlite", "file:"+s.store.LifeDBPath(life, now.In(time.FixedZone("CST", 28800)).Format("2006-01"))+"?_pragma=foreign_keys(1)")
}
func (s *Service) List(ctx context.Context, life, recipient string) ([]Message, error) {
	db, e := s.db(ctx, life)
	if e != nil {
		return nil, e
	}
	defer db.Close()
	rows, e := db.QueryContext(ctx, "SELECT id,type,COALESCE(ref_id,''),text,created_at,read_at IS NOT NULL FROM inbox_messages WHERE life_id=? AND recipient_id=? ORDER BY created_at DESC", life, recipient)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var x Message
		var read int
		if e = rows.Scan(&x.ID, &x.Type, &x.RefID, &x.Text, &x.CreatedAt, &read); e != nil {
			return nil, e
		}
		x.Read = read == 1
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) MarkRead(ctx context.Context, life, id string) error {
	db, e := s.db(ctx, life)
	if e != nil {
		return e
	}
	defer db.Close()
	r, e := db.ExecContext(ctx, "UPDATE inbox_messages SET read_at=? WHERE id=? AND life_id=?", time.Now().UTC().Format(time.RFC3339Nano), id, life)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return fmt.Errorf("通知不存在")
	}
	return nil
}
func (s *Service) Enqueue(ctx context.Context, life, recipient, typ, ref, text string) error {
	db, e := s.db(ctx, life)
	if e != nil {
		return e
	}
	defer db.Close()
	_, e = db.ExecContext(ctx, "INSERT INTO inbox_messages(id,life_id,recipient_type,recipient_id,type,ref_id,text,created_at) VALUES(?,?,?,?,?,?,?,?)", uuid.NewString(), life, "actor", recipient, typ, ref, text, time.Now().UTC().Format(time.RFC3339Nano))
	return e
}
