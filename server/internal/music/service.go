package music

import (
	"bytes"
	"context"
	"cyberlife/server/internal/storage"
	"database/sql"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxTrackSize = 50 << 20

type Service struct {
	store *storage.Store
}

func New(store *storage.Store) *Service { return &Service{store: store} }

type Playlist struct {
	ID             string  `json:"id"`
	Page           string  `json:"page"`
	Name           string  `json:"name"`
	Mode           string  `json:"mode"`
	Volume         int     `json:"volume"`
	DefaultEnabled bool    `json:"defaultEnabled"`
	Tracks         []Track `json:"tracks"`
	UpdatedAt      string  `json:"updatedAt"`
}

type Track struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	MimeType  string `json:"mimeType"`
	ByteSize  int64  `json:"byteSize"`
	SortOrder int    `json:"sortOrder"`
	URL       string `json:"url"`
}

func validPage(page string) bool {
	return page == "now" || page == "past" || page == "future"
}

func validMode(mode string) bool {
	return mode == "list" || mode == "random" || mode == "single"
}

func (s *Service) List(ctx context.Context, lifeID string) ([]Playlist, error) {
	items := make([]Playlist, 0, 3)
	for _, page := range []string{"now", "past", "future"} {
		item, found, err := s.playlist(ctx, lifeID, page)
		if err != nil {
			return nil, err
		}
		if !found {
			item = Playlist{Page: page, Name: page, Mode: "list", Volume: 70, DefaultEnabled: true, Tracks: []Track{}}
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) Replace(ctx context.Context, lifeID, page, name, mode string, volume int, defaultEnabled *bool) (Playlist, error) {
	if !validPage(page) || strings.TrimSpace(name) == "" || !validMode(mode) || volume < 0 || volume > 100 {
		return Playlist{}, fmt.Errorf("歌单参数无效")
	}
	item, found, err := s.playlist(ctx, lifeID, page)
	if err != nil {
		return Playlist{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if !found {
		enabled := true
		if defaultEnabled != nil {
			enabled = *defaultEnabled
		}
		item.ID = uuid.NewString()
		_, err = s.store.Global().ExecContext(ctx, "INSERT INTO music_playlists(id,life_id,page,name,mode,volume,default_enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)", item.ID, lifeID, page, strings.TrimSpace(name), mode, volume, enabled, now, now)
	} else if defaultEnabled == nil {
		_, err = s.store.Global().ExecContext(ctx, "UPDATE music_playlists SET name=?,mode=?,volume=?,updated_at=? WHERE id=? AND life_id=?", strings.TrimSpace(name), mode, volume, now, item.ID, lifeID)
	} else {
		_, err = s.store.Global().ExecContext(ctx, "UPDATE music_playlists SET name=?,mode=?,volume=?,default_enabled=?,updated_at=? WHERE id=? AND life_id=?", strings.TrimSpace(name), mode, volume, *defaultEnabled, now, item.ID, lifeID)
	}
	if err != nil {
		return Playlist{}, err
	}
	item, _, err = s.playlist(ctx, lifeID, page)
	return item, err
}

func (s *Service) AddTrack(ctx context.Context, lifeID, page, name, declaredType string, size int64, source io.Reader) (Track, error) {
	if !validPage(page) || size < 1 || size > maxTrackSize || strings.ContainsAny(name, "\\/\x00") || name == "" {
		return Track{}, fmt.Errorf("音频文件无效或超过 50MB")
	}
	if !allowedAudioType(declaredType) {
		return Track{}, fmt.Errorf("仅支持常见音频格式")
	}
	prefix := make([]byte, 512)
	n, err := io.ReadFull(source, prefix)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return Track{}, fmt.Errorf("读取音频失败")
	}
	prefix = prefix[:n]
	if !matchesAudioSignature(declaredType, prefix) {
		return Track{}, fmt.Errorf("文件内容不是受支持的音频")
	}
	playlist, err := s.ensurePlaylist(ctx, lifeID, page)
	if err != nil {
		return Track{}, err
	}

	track := Track{ID: uuid.NewString(), Title: strings.TrimSuffix(name, filepath.Ext(name)), MimeType: declaredType, ByteSize: size}
	if track.Title == "" {
		track.Title = name
	}
	storedName := track.ID + normalizedExtension(name, declaredType)
	dir := s.store.MusicUploadDir(lifeID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return Track{}, err
	}
	temp := filepath.Join(dir, storedName+".part")
	target := filepath.Join(dir, storedName)
	out, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Track{}, err
	}
	written, copyErr := io.Copy(out, io.LimitReader(io.MultiReader(bytes.NewReader(prefix), source), maxTrackSize+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || written != size {
		_ = os.Remove(temp)
		return Track{}, fmt.Errorf("保存音频失败")
	}
	if err = os.Rename(temp, target); err != nil {
		return Track{}, err
	}
	if err = s.store.Global().QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order),0)+1 FROM music_tracks WHERE playlist_id=?", playlist.ID).Scan(&track.SortOrder); err == nil {
		_, err = s.store.Global().ExecContext(ctx, "INSERT INTO music_tracks(id,playlist_id,stored_name,original_name,mime_type,byte_size,sort_order,created_at) VALUES(?,?,?,?,?,?,?,?)", track.ID, playlist.ID, storedName, name, declaredType, size, track.SortOrder, time.Now().UTC().Format(time.RFC3339Nano))
	}
	if err != nil {
		_ = os.Remove(target)
		return Track{}, err
	}
	track.URL = "/api/v1/music/tracks/" + track.ID
	return track, nil
}

func (s *Service) DeletePlaylist(ctx context.Context, lifeID, page string) error {
	if !validPage(page) {
		return fmt.Errorf("歌单不存在")
	}
	rows, err := s.store.Global().QueryContext(ctx, "SELECT t.stored_name FROM music_tracks t JOIN music_playlists p ON p.id=t.playlist_id WHERE p.life_id=? AND p.page=?", lifeID, page)
	if err != nil {
		return err
	}
	defer rows.Close()
	storedNames := []string{}
	for rows.Next() {
		var storedName string
		if err := rows.Scan(&storedName); err != nil {
			return err
		}
		storedNames = append(storedNames, storedName)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	result, err := s.store.Global().ExecContext(ctx, "DELETE FROM music_playlists WHERE life_id=? AND page=?", lifeID, page)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("歌单不存在")
	}
	for _, storedName := range storedNames {
		if err := os.Remove(filepath.Join(s.store.MusicUploadDir(lifeID), storedName)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除音频文件: %w", err)
		}
	}
	return nil
}

func (s *Service) DeleteTrack(ctx context.Context, lifeID, trackID string) error {
	var storedName string
	err := s.store.Global().QueryRowContext(ctx, "SELECT t.stored_name FROM music_tracks t JOIN music_playlists p ON p.id=t.playlist_id WHERE t.id=? AND p.life_id=?", trackID, lifeID).Scan(&storedName)
	if err == sql.ErrNoRows {
		return fmt.Errorf("音频不存在")
	}
	if err != nil {
		return err
	}
	result, err := s.store.Global().ExecContext(ctx, "DELETE FROM music_tracks WHERE id=?", trackID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("音频不存在")
	}
	if err := os.Remove(filepath.Join(s.store.MusicUploadDir(lifeID), storedName)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除音频文件: %w", err)
	}
	return nil
}

func (s *Service) TrackForRead(ctx context.Context, lifeID, trackID string) (Track, string, error) {
	var storedName string
	track := Track{ID: trackID, URL: "/api/v1/music/tracks/" + trackID}
	err := s.store.Global().QueryRowContext(ctx, "SELECT t.original_name,t.mime_type,t.byte_size,t.sort_order,t.stored_name FROM music_tracks t JOIN music_playlists p ON p.id=t.playlist_id WHERE t.id=? AND p.life_id=?", trackID, lifeID).Scan(&track.Title, &track.MimeType, &track.ByteSize, &track.SortOrder, &storedName)
	if err == sql.ErrNoRows {
		return Track{}, "", fmt.Errorf("音频不存在")
	}
	if err != nil {
		return Track{}, "", err
	}
	return track, filepath.Join(s.store.MusicUploadDir(lifeID), storedName), nil
}

func (s *Service) ensurePlaylist(ctx context.Context, lifeID, page string) (Playlist, error) {
	item, found, err := s.playlist(ctx, lifeID, page)
	if err != nil || found {
		return item, err
	}
	enabled := true
	return s.Replace(ctx, lifeID, page, page, "list", 70, &enabled)
}

func (s *Service) playlist(ctx context.Context, lifeID, page string) (Playlist, bool, error) {
	item := Playlist{Page: page, Tracks: []Track{}}
	var defaultEnabled int
	err := s.store.Global().QueryRowContext(ctx, "SELECT id,name,mode,volume,default_enabled,updated_at FROM music_playlists WHERE life_id=? AND page=?", lifeID, page).Scan(&item.ID, &item.Name, &item.Mode, &item.Volume, &defaultEnabled, &item.UpdatedAt)
	item.DefaultEnabled = defaultEnabled == 1
	if err == sql.ErrNoRows {
		return item, false, nil
	}
	if err != nil {
		return item, false, err
	}
	rows, err := s.store.Global().QueryContext(ctx, "SELECT id,original_name,mime_type,byte_size,sort_order FROM music_tracks WHERE playlist_id=? ORDER BY sort_order,id", item.ID)
	if err != nil {
		return item, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var track Track
		if err := rows.Scan(&track.ID, &track.Title, &track.MimeType, &track.ByteSize, &track.SortOrder); err != nil {
			return item, false, err
		}
		track.URL = "/api/v1/music/tracks/" + track.ID
		item.Tracks = append(item.Tracks, track)
	}
	return item, true, rows.Err()
}

func allowedAudioType(value string) bool {
	value, _, _ = mime.ParseMediaType(value)
	switch value {
	case "audio/mpeg", "audio/ogg", "audio/wav", "audio/x-wav", "audio/webm", "audio/mp4", "audio/aac", "audio/flac":
		return true
	default:
		return false
	}
}

func matchesAudioSignature(declared string, prefix []byte) bool {
	declared, _, _ = mime.ParseMediaType(declared)
	has := func(signature string) bool {
		return len(prefix) >= len(signature) && string(prefix[:len(signature)]) == signature
	}
	switch declared {
	case "audio/mpeg":
		return has("ID3") || (len(prefix) >= 2 && prefix[0] == 0xff && prefix[1]&0xe0 == 0xe0)
	case "audio/ogg":
		return has("OggS")
	case "audio/wav", "audio/x-wav":
		return has("RIFF") && len(prefix) >= 12 && string(prefix[8:12]) == "WAVE"
	case "audio/webm":
		return len(prefix) >= 4 && prefix[0] == 0x1a && prefix[1] == 0x45 && prefix[2] == 0xdf && prefix[3] == 0xa3
	case "audio/mp4":
		return len(prefix) >= 8 && string(prefix[4:8]) == "ftyp"
	case "audio/aac":
		return len(prefix) >= 2 && prefix[0] == 0xff && prefix[1]&0xf6 == 0xf0
	case "audio/flac":
		return has("fLaC")
	default:
		return false
	}
}

func normalizedExtension(name, mediaType string) string {
	extension := strings.ToLower(filepath.Ext(name))
	if extension != "" && len(extension) <= 10 {
		return extension
	}
	switch mediaType {
	case "audio/mpeg":
		return ".mp3"
	case "audio/ogg":
		return ".ogg"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "audio/webm":
		return ".webm"
	case "audio/mp4":
		return ".m4a"
	default:
		return ".audio"
	}
}
