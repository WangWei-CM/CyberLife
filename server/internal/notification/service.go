package notification

import (
	"context"
	"cyberlife/server/internal/storage"
	"fmt"
	"github.com/google/uuid"
	"sort"
	"time"
)

type Service struct{ store *storage.Store }

func New(store *storage.Store) *Service { return &Service{store} }

// Message is one inbox entry. RefID/RefDate let the client jump to the referenced content:
// comment → the day of the diary or task, plan_due → the plan, key_expired → the key.
type Message struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	RefID     string `json:"refID"`
	RefDate   string `json:"refDate"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
	Read      bool   `json:"read"`
}

// listMonths bounds how far back the inbox is merged; older messages stay in their month databases.
const listMonths = 12

// List merges the inbox rows of the most recent month databases, newest first.
func (s *Service) List(ctx context.Context, life, recipient string) ([]Message, error) {
	months, e := s.store.LifeMonths(ctx, life)
	if e != nil {
		return nil, e
	}
	if len(months) > listMonths {
		months = months[:listMonths]
	}
	out := []Message{}
	for _, month := range months {
		db, e := s.store.OpenLifeMonth(ctx, life, month)
		if e != nil {
			return nil, e
		}
		rows, e := db.QueryContext(ctx, "SELECT id,type,COALESCE(ref_id,''),COALESCE(ref_date,''),text,created_at,read_at IS NOT NULL FROM inbox_messages WHERE life_id=? AND recipient_id=? ORDER BY created_at DESC", life, recipient)
		if e != nil {
			db.Close()
			return nil, e
		}
		for rows.Next() {
			var x Message
			var read int
			if e = rows.Scan(&x.ID, &x.Type, &x.RefID, &x.RefDate, &x.Text, &x.CreatedAt, &read); e != nil {
				rows.Close()
				db.Close()
				return nil, e
			}
			x.Read = read == 1
			out = append(out, x)
		}
		rows.Close()
		db.Close()
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

func (s *Service) MarkRead(ctx context.Context, life, id string) error {
	months, e := s.store.LifeMonths(ctx, life)
	if e != nil {
		return e
	}
	for _, month := range months {
		db, e := s.store.OpenLifeMonth(ctx, life, month)
		if e != nil {
			return e
		}
		r, e := db.ExecContext(ctx, "UPDATE inbox_messages SET read_at=COALESCE(read_at,?) WHERE id=? AND life_id=?", time.Now().UTC().Format(time.RFC3339Nano), id, life)
		db.Close()
		if e != nil {
			return e
		}
		if n, _ := r.RowsAffected(); n == 1 {
			return nil
		}
	}
	return fmt.Errorf("通知不存在")
}

// Enqueue appends a message to the current month's inbox.
func (s *Service) Enqueue(ctx context.Context, life, recipient, typ, ref, refDate, text string) error {
	db, e := s.store.OpenLifeMonth(ctx, life, storage.MonthKey(time.Now()))
	if e != nil {
		return e
	}
	defer db.Close()
	_, e = db.ExecContext(ctx, "INSERT INTO inbox_messages(id,life_id,recipient_type,recipient_id,type,ref_id,ref_date,text,created_at) VALUES(?,?,?,?,?,?,?,?,?)", uuid.NewString(), life, "actor", recipient, typ, ref, refDate, text, time.Now().UTC().Format(time.RFC3339Nano))
	return e
}

// Exists reports whether the recipient already has a message of this type for the reference.
func (s *Service) Exists(ctx context.Context, life, recipient, typ, ref string) (bool, error) {
	months, e := s.store.LifeMonths(ctx, life)
	if e != nil {
		return false, e
	}
	if len(months) > listMonths {
		months = months[:listMonths]
	}
	for _, month := range months {
		db, e := s.store.OpenLifeMonth(ctx, life, month)
		if e != nil {
			return false, e
		}
		var count int
		e = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM inbox_messages WHERE life_id=? AND recipient_id=? AND type=? AND ref_id=?", life, recipient, typ, ref).Scan(&count)
		db.Close()
		if e != nil {
			return false, e
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

// EnsureOnce enqueues a message unless the same type/reference was already delivered; sweeps that
// run on every inbox read use it to stay idempotent.
func (s *Service) EnsureOnce(ctx context.Context, life, recipient, typ, ref, refDate, text string) error {
	exists, e := s.Exists(ctx, life, recipient, typ, ref)
	if e != nil || exists {
		return e
	}
	return s.Enqueue(ctx, life, recipient, typ, ref, refDate, text)
}

// WriterOf returns the writer id that owns the life.
func (s *Service) WriterOf(ctx context.Context, life string) (string, error) {
	var owner string
	e := s.store.Global().QueryRowContext(ctx, "SELECT owner_id FROM lives WHERE id=?", life).Scan(&owner)
	return owner, e
}
