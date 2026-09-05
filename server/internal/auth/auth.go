package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

// Actor is the authenticated identity supplied to application services.
// Nickname and LifeCreatedAt are display-only fields filled for writers and readers so the
// front end can greet by name and anchor the life timeline at the registration day.
type Actor struct {
	SessionID     string `json:"session_id"`
	Type          string `json:"type"`
	ID            string `json:"id"`
	LifeID        string `json:"life_id"`
	Nickname      string `json:"nickname"`
	LifeCreatedAt string `json:"life_created_at"`
}

// enrich fills Nickname and LifeCreatedAt; failures leave the fields empty on purpose.
func (s *Service) enrich(ctx context.Context, actor *Actor) {
	switch actor.Type {
	case "writer":
		_ = s.db.QueryRowContext(ctx, "SELECT nickname FROM writers WHERE id=?", actor.ID).Scan(&actor.Nickname)
	case "reader":
		_ = s.db.QueryRowContext(ctx, "SELECT nickname FROM reader_keys WHERE id=?", actor.ID).Scan(&actor.Nickname)
	}
	if actor.LifeID != "" {
		_ = s.db.QueryRowContext(ctx, "SELECT created_at FROM lives WHERE id=?", actor.LifeID).Scan(&actor.LifeCreatedAt)
	}
}

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) EnsureAdmin(ctx context.Context, password string) error {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := hashSecret(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, "INSERT INTO admins(id,password_hash,created_at,updated_at) VALUES(?,?,?,?)", uuid.NewString(), hash, now, now)
	return err
}

func (s *Service) AdminLogin(ctx context.Context, password, userAgent string) (string, Actor, error) {
	var id, hash string
	if err := s.db.QueryRowContext(ctx, "SELECT id,password_hash FROM admins LIMIT 1").Scan(&id, &hash); err != nil {
		return "", Actor{}, invalidCredentials(err)
	}
	if !verifySecret(hash, password) {
		return "", Actor{}, invalidCredentials(nil)
	}
	return s.createSession(ctx, Actor{Type: "admin", ID: id}, 1, userAgent)
}

func (s *Service) KeyLogin(ctx context.Context, key, userAgent string) (string, Actor, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", Actor{}, invalidCredentials(nil)
	}
	// The pool is limited to a single connection (storage.Open sets MaxOpenConns(1)), so every
	// candidate row must be read and the cursor closed before any nested query or insert runs.
	// Keeping the cursor open while calling createSession deadlocked the login until the client
	// gave up, which made every writer/reader key login fail.
	type writerCandidate struct {
		id, hash string
		version  int
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, master_key_hash, key_version FROM writers WHERE revoked_at IS NULL`)
	if err != nil {
		return "", Actor{}, err
	}
	var writers []writerCandidate
	for rows.Next() {
		var candidate writerCandidate
		if err := rows.Scan(&candidate.id, &candidate.hash, &candidate.version); err != nil {
			rows.Close()
			return "", Actor{}, err
		}
		writers = append(writers, candidate)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", Actor{}, err
	}
	rows.Close()
	for _, candidate := range writers {
		if verifySecret(candidate.hash, key) {
			var lifeID string
			if err := s.db.QueryRowContext(ctx, "SELECT id FROM lives WHERE owner_id=?", candidate.id).Scan(&lifeID); err != nil {
				return "", Actor{}, err
			}
			return s.createSession(ctx, Actor{Type: "writer", ID: candidate.id, LifeID: lifeID}, candidate.version, userAgent)
		}
	}

	type readerCandidate struct {
		id, lifeID, hash string
		version          int
		expiresAt        sql.NullString
	}
	rows, err = s.db.QueryContext(ctx, `SELECT id, life_id, key_hash, key_version, expires_at FROM reader_keys WHERE revoked_at IS NULL`)
	if err != nil {
		return "", Actor{}, err
	}
	var readers []readerCandidate
	for rows.Next() {
		var candidate readerCandidate
		if err := rows.Scan(&candidate.id, &candidate.lifeID, &candidate.hash, &candidate.version, &candidate.expiresAt); err != nil {
			rows.Close()
			return "", Actor{}, err
		}
		readers = append(readers, candidate)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", Actor{}, err
	}
	rows.Close()
	now := time.Now().UTC()
	for _, candidate := range readers {
		if candidate.expiresAt.Valid {
			expiry, parseErr := time.Parse(time.RFC3339Nano, candidate.expiresAt.String)
			if parseErr != nil || !now.Before(expiry) {
				continue
			}
		}
		if verifySecret(candidate.hash, key) {
			return s.createSession(ctx, Actor{Type: "reader", ID: candidate.id, LifeID: candidate.lifeID}, candidate.version, userAgent)
		}
	}
	return "", Actor{}, invalidCredentials(nil)
}

func (s *Service) Authenticate(ctx context.Context, token string) (Actor, error) {
	if token == "" {
		return Actor{}, invalidCredentials(nil)
	}
	var a Actor
	var version int
	var revoked sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, actor_type, actor_id, COALESCE(life_id,''), key_version, revoked_at FROM sessions WHERE token_hash=?`, tokenHash(token)).Scan(&a.SessionID, &a.Type, &a.ID, &a.LifeID, &version, &revoked)
	if err != nil || revoked.Valid {
		return Actor{}, invalidCredentials(err)
	}
	if err := s.validateActor(ctx, a, version); err != nil {
		return Actor{}, invalidCredentials(err)
	}
	_, _ = s.db.ExecContext(ctx, "UPDATE sessions SET last_active_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), a.SessionID)
	s.enrich(ctx, &a)
	return a, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE sessions SET revoked_at=? WHERE token_hash=? AND revoked_at IS NULL", time.Now().UTC().Format(time.RFC3339Nano), tokenHash(token))
	return err
}

func (s *Service) createSession(ctx context.Context, actor Actor, version int, userAgent string) (string, Actor, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", Actor{}, err
	}
	actor.SessionID = uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions(id,token_hash,actor_type,actor_id,life_id,key_version,created_at,last_active_at,user_agent) VALUES(?,?,?,?,?,?,?,?,?)`, actor.SessionID, tokenHash(token), actor.Type, actor.ID, nullIfEmpty(actor.LifeID), version, now, now, userAgent)
	if err != nil {
		return "", Actor{}, err
	}
	s.enrich(ctx, &actor)
	return token, actor, nil
}

func (s *Service) validateActor(ctx context.Context, actor Actor, version int) error {
	switch actor.Type {
	case "admin":
		var count int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins WHERE id=?", actor.ID).Scan(&count); err != nil || count != 1 {
			return fmt.Errorf("administrator is unavailable")
		}
		return nil
	case "writer":
		var actual int
		var revoked sql.NullString
		err := s.db.QueryRowContext(ctx, "SELECT key_version,revoked_at FROM writers WHERE id=?", actor.ID).Scan(&actual, &revoked)
		if err != nil || revoked.Valid || actual != version {
			return fmt.Errorf("writer is revoked")
		}
		return nil
	case "reader":
		var actual int
		var revoked, expires sql.NullString
		err := s.db.QueryRowContext(ctx, "SELECT key_version,revoked_at,expires_at FROM reader_keys WHERE id=?", actor.ID).Scan(&actual, &revoked, &expires)
		if err != nil || revoked.Valid || actual != version {
			return fmt.Errorf("reader is revoked")
		}
		if expires.Valid {
			expiry, parseErr := time.Parse(time.RFC3339Nano, expires.String)
			if parseErr != nil || !time.Now().UTC().Before(expiry) {
				return fmt.Errorf("reader key expired")
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported actor type")
	}
}

func randomToken(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
func tokenHash(token string) string {
	value := sha256.Sum256([]byte(token))
	return base64.RawStdEncoding.EncodeToString(value[:])
}
func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func invalidCredentials(cause error) error {
	if cause != nil && cause != sql.ErrNoRows {
		return fmt.Errorf("invalid credentials: %w", cause)
	}
	return fmt.Errorf("invalid credentials")
}

// Argon2id encoded hash: argon2id$v=19$m=65536,t=3,p=2$salt$hash.
func hashSecret(secret string) (string, error) {
	if len(secret) < 8 {
		return "", fmt.Errorf("secret must have at least 8 characters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	derived := argon2.IDKey([]byte(secret), salt, 3, 64*1024, 2, 32)
	return fmt.Sprintf("argon2id$v=19$m=65536,t=3,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived)), nil
}
func verifySecret(encoded, secret string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" {
		return false
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(secret), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
