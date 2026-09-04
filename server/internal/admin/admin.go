package admin

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"cyberlife/server/internal/storage"
)

type Service struct {
	db    *sql.DB
	store *storage.Store
}

func New(db *sql.DB, store *storage.Store) *Service { return &Service{db: db, store: store} }

type Writer struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	LifeID    string `json:"life_id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}
type ReaderKey struct {
	ID              string  `json:"id"`
	Nickname        string  `json:"nickname"`
	AnchorLocalDate string  `json:"anchor_local_date"`
	ExpiresAt       *string `json:"expires_at"`
	RevokedAt       *string `json:"revoked_at"`
	Note            string  `json:"note"`
	CreatedAt       string  `json:"created_at"`
}

func (s *Service) CreateWriter(ctx context.Context, nickname string) (Writer, string, error) {
	if nickname == "" {
		return Writer{}, "", fmt.Errorf("nickname is required")
	}
	key, err := randomKey()
	if err != nil {
		return Writer{}, "", err
	}
	writerID, lifeID := uuid.NewString(), uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Writer{}, "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO writers(id,nickname,master_key_hash,key_version,created_at) VALUES(?,?,?,?,?)`, writerID, nickname, hashKey(key), 1, now); err != nil {
		return Writer{}, "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO lives(id,owner_id,status,last_active_at,created_at) VALUES(?,?,?,?,?)`, lifeID, writerID, "active", now, now); err != nil {
		return Writer{}, "", err
	}
	if err = tx.Commit(); err != nil {
		return Writer{}, "", err
	}
	if err = s.store.EnsureLifeMonth(ctx, lifeID, time.Now()); err != nil {
		return Writer{}, "", fmt.Errorf("create initial life month: %w", err)
	}
	return Writer{ID: writerID, Nickname: nickname, LifeID: lifeID, Status: "active", CreatedAt: now}, key, nil
}

func (s *Service) ListWriters(ctx context.Context) ([]Writer, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT w.id,w.nickname,l.id,l.status,w.created_at FROM writers w JOIN lives l ON l.owner_id=w.id ORDER BY w.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Writer{}
	for rows.Next() {
		var item Writer
		if err := rows.Scan(&item.ID, &item.Nickname, &item.LifeID, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreateReaderKey(ctx context.Context, lifeID, nickname, note string, expiresAt *time.Time) (ReaderKey, string, error) {
	if nickname == "" {
		return ReaderKey{}, "", fmt.Errorf("nickname is required")
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM lives WHERE id=?", lifeID).Scan(&exists); err != nil || exists != 1 {
		return ReaderKey{}, "", fmt.Errorf("life not found")
	}
	key, err := randomKey()
	if err != nil {
		return ReaderKey{}, "", err
	}
	id := uuid.NewString()
	now := time.Now().UTC()
	anchorDate := now.In(shanghai()).Format("2006-01-02")
	var expiryValue any
	var expiryResult *string
	if expiresAt != nil {
		value := expiresAt.UTC().Format(time.RFC3339Nano)
		expiryValue = value
		expiryResult = &value
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO reader_keys(id,life_id,key_hash,nickname,anchor_at_utc,anchor_local_date,expires_at,note,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, id, lifeID, hashKey(key), nickname, now.Format(time.RFC3339Nano), anchorDate, expiryValue, note, now.Format(time.RFC3339Nano))
	if err != nil {
		return ReaderKey{}, "", err
	}
	return ReaderKey{ID: id, Nickname: nickname, AnchorLocalDate: anchorDate, ExpiresAt: expiryResult, Note: note, CreatedAt: now.Format(time.RFC3339Nano)}, key, nil
}

func (s *Service) ListReaderKeys(ctx context.Context, lifeID string) ([]ReaderKey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,nickname,anchor_local_date,expires_at,revoked_at,COALESCE(note,''),created_at FROM reader_keys WHERE life_id=? ORDER BY created_at DESC`, lifeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ReaderKey{}
	for rows.Next() {
		var item ReaderKey
		var expires, revoked sql.NullString
		if err := rows.Scan(&item.ID, &item.Nickname, &item.AnchorLocalDate, &expires, &revoked, &item.Note, &item.CreatedAt); err != nil {
			return nil, err
		}
		if expires.Valid {
			item.ExpiresAt = &expires.String
		}
		if revoked.Valid {
			item.RevokedAt = &revoked.String
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) RevokeReaderKey(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE reader_keys SET revoked_at=?,key_version=key_version+1 WHERE id=? AND revoked_at IS NULL", time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return fmt.Errorf("reader key not found or already revoked")
	}
	return nil
}

func randomKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "cl_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}
func hashKey(key string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	hash := argon2.IDKey([]byte(key), salt, 3, 64*1024, 2, 32)
	return fmt.Sprintf("argon2id$v=19$m=65536,t=3,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash))
}
func shanghai() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return location
}
