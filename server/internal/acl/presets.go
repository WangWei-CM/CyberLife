package acl

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"time"
)

type Rule struct {
	ReaderKeyID string `json:"readerKeyId"`
	Allowed     bool   `json:"allowed"`
}
type Preset struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Rules []Rule `json:"rules"`
}

func (s *Service) ListPresets(ctx context.Context, lifeID string) ([]Preset, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,name FROM permission_presets WHERE life_id=? ORDER BY name", lifeID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Preset{}
	for rows.Next() {
		var x Preset
		if e = rows.Scan(&x.ID, &x.Name); e != nil {
			return nil, e
		}
		rules, e := s.rules(ctx, x.ID)
		if e != nil {
			return nil, e
		}
		x.Rules = rules
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) CreatePreset(ctx context.Context, lifeID, name string, rules []Rule) (Preset, error) {
	if name == "" {
		return Preset{}, fmt.Errorf("预设名称不能为空")
	}
	x := Preset{ID: uuid.NewString(), Name: name, Rules: rules}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return x, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "INSERT INTO permission_presets(id,life_id,name,created_at,updated_at) VALUES(?,?,?,?,?)", x.ID, lifeID, x.Name, now, now); e != nil {
		return x, e
	}
	for _, r := range rules {
		var keyLife string
		if e = tx.QueryRowContext(ctx, "SELECT life_id FROM reader_keys WHERE id=?", r.ReaderKeyID).Scan(&keyLife); e != nil || keyLife != lifeID {
			return x, fmt.Errorf("阅读密钥不属于当前人生")
		}
		allowed := 0
		if r.Allowed {
			allowed = 1
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO preset_key_rules(preset_id,reader_key_id,allowed,updated_at) VALUES(?,?,?,?)", x.ID, r.ReaderKeyID, allowed, now); e != nil {
			return x, e
		}
	}
	return x, tx.Commit()
}
func (s *Service) ReplacePresetRules(ctx context.Context, lifeID, presetID string, rules []Rule) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var owner string
	if e = tx.QueryRowContext(ctx, "SELECT life_id FROM permission_presets WHERE id=?", presetID).Scan(&owner); e != nil || owner != lifeID {
		return fmt.Errorf("权限预设不存在")
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM preset_key_rules WHERE preset_id=?", presetID); e != nil {
		return e
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, r := range rules {
		var keyLife string
		if e = tx.QueryRowContext(ctx, "SELECT life_id FROM reader_keys WHERE id=?", r.ReaderKeyID).Scan(&keyLife); e != nil || keyLife != lifeID {
			return fmt.Errorf("阅读密钥不属于当前人生")
		}
		allowed := 0
		if r.Allowed {
			allowed = 1
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO preset_key_rules(preset_id,reader_key_id,allowed,updated_at) VALUES(?,?,?,?)", presetID, r.ReaderKeyID, allowed, now); e != nil {
			return e
		}
	}
	_, e = tx.ExecContext(ctx, "UPDATE permission_presets SET updated_at=? WHERE id=?", now, presetID)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) rules(ctx context.Context, presetID string) ([]Rule, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT reader_key_id,allowed FROM preset_key_rules WHERE preset_id=?", presetID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var x Rule
		var allowed int
		if e = rows.Scan(&x.ReaderKeyID, &allowed); e != nil {
			return nil, e
		}
		x.Allowed = allowed == 1
		out = append(out, x)
	}
	return out, rows.Err()
}
