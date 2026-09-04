package backup

import (
	"context"
	"crypto/sha256"
	"cyberlife/server/internal/storage"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Service struct {
	store *storage.Store
	root  string
}
type ManifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type Manifest struct {
	Version   int            `json:"version"`
	LifeID    string         `json:"lifeId"`
	CreatedAt string         `json:"createdAt"`
	Files     []ManifestFile `json:"files"`
}

func New(store *storage.Store, root string) *Service { return &Service{store: store, root: root} }

// ExportLife builds a manifest in a fixed server-side directory. Call from a background job in production.
func (s *Service) ExportLife(ctx context.Context, lifeID string) (string, error) {
	target := filepath.Join(s.root, lifeID, time.Now().UTC().Format("20060102T150405Z"))
	if e := os.MkdirAll(target, 0o750); e != nil {
		return "", e
	}
	manifest := Manifest{Version: 1, LifeID: lifeID, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Files: []ManifestFile{}}
	lifeDir := filepath.Dir(s.store.LifeDBPath(lifeID, "placeholder"))
	if e := copyTree(lifeDir, filepath.Join(target, "lives", lifeID), &manifest); e != nil {
		return "", e
	}
	payload, e := json.MarshalIndent(manifest, "", "  ")
	if e != nil {
		return "", e
	}
	if e = os.WriteFile(filepath.Join(target, "manifest.json"), payload, 0o600); e != nil {
		return "", e
	}
	return target, nil
}
func copyTree(source, target string, manifest *Manifest) error {
	entries, e := os.ReadDir(source)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		from := filepath.Join(source, entry.Name())
		to := filepath.Join(target, entry.Name())
		if entry.IsDir() {
			if e = copyTree(from, to, manifest); e != nil {
				return e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(to), 0o750); e != nil {
			return e
		}
		in, e := os.Open(from)
		if e != nil {
			return e
		}
		out, e := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if e != nil {
			in.Close()
			return e
		}
		hash := sha256.New()
		size, e := io.Copy(io.MultiWriter(out, hash), in)
		closeIn, closeOut := in.Close(), out.Close()
		if e != nil || closeIn != nil || closeOut != nil {
			return fmt.Errorf("copy backup file: %w", e)
		}
		manifest.Files = append(manifest.Files, ManifestFile{Path: entry.Name(), Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))})
	}
	return nil
}
func NewJobID() string { return uuid.NewString() }
