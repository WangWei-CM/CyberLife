package acl

import (
	"context"
	"cyberlife/server/internal/auth"
	"database/sql"
	"time"
)

type Resource struct {
	LifeID, Date, PresetID string
	Secret                 bool
}
type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }

// CanRead is the only content visibility decision point. Writer is always allowed;
// all reader paths must satisfy secret -> anchor -> preset in that order.
func (s *Service) CanRead(ctx context.Context, actor auth.Actor, resource Resource) (bool, error) {
	if actor.Type == "writer" {
		return actor.LifeID == resource.LifeID, nil
	}
	if actor.Type != "reader" || actor.LifeID != resource.LifeID || resource.Secret {
		return false, nil
	}
	var anchor string
	if err := s.db.QueryRowContext(ctx, "SELECT anchor_local_date FROM reader_keys WHERE id=? AND life_id=? AND revoked_at IS NULL", actor.ID, resource.LifeID).Scan(&anchor); err != nil {
		return false, nil
	}
	if resource.Date < anchor {
		return false, nil
	}
	if resource.PresetID == "" {
		return true, nil
	}
	var allowed int
	err := s.db.QueryRowContext(ctx, "SELECT allowed FROM preset_key_rules WHERE preset_id=? AND reader_key_id=?", resource.PresetID, actor.ID).Scan(&allowed)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return allowed == 1, nil
}
func Today() string {
	location, _ := time.LoadLocation("Asia/Shanghai")
	if location == nil {
		location = time.FixedZone("CST", 28800)
	}
	return time.Now().In(location).Format("2006-01-02")
}
