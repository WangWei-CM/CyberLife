package now

import (
	"context"
	"cyberlife/server/internal/storage"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Service struct{ store *storage.Store }

func New(store *storage.Store) *Service { return &Service{store} }

type MoodTag struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Emoji     string `json:"emoji"`
	Value     int    `json:"value"`
	SortOrder int    `json:"sortOrder"`
}
type MoodRecord struct {
	ID           string    `json:"id"`
	RecordedAt   string    `json:"recordedAt"`
	RecordedDate string    `json:"recordedDate"`
	Note         string    `json:"note"`
	Value        float64   `json:"value"`
	Tags         []MoodTag `json:"tags"`
	Secret       bool      `json:"secret"`
}
type BodyRecord struct {
	ID           string `json:"id"`
	RecordedAt   string `json:"recordedAt"`
	RecordedDate string `json:"recordedDate"`
	Note         string `json:"note"`
	Score        int    `json:"score"`
	Secret       bool   `json:"secret"`
}
type Diary struct {
	ID          string `json:"id"`
	EntryDate   string `json:"entryDate"`
	Content     string `json:"content"`
	PresetID    string `json:"presetId"`
	Secret      bool   `json:"secret"`
	Commentable bool   `json:"commentable"`
}
type Task struct {
	ID          string `json:"id"`
	TaskDate    string `json:"taskDate"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Done        bool   `json:"done"`
	PresetID    string `json:"presetId"`
	Secret      bool   `json:"secret"`
	Commentable bool   `json:"commentable"`
}
type Draft struct {
	EntryDate string `json:"entryDate"`
	Content   string `json:"content"`
	UpdatedAt string `json:"updatedAt"`
}
type Attachment struct {
	ID           string `json:"id"`
	OriginalName string `json:"originalName"`
	MimeType     string `json:"mimeType"`
	ByteSize     int64  `json:"byteSize"`
	PresetID     string `json:"presetId"`
	Secret       bool   `json:"secret"`
}

func (s *Service) db(ctx context.Context, life string, t time.Time) (*sql.DB, error) {
	if err := s.store.EnsureLifeMonth(ctx, life, t); err != nil {
		return nil, err
	}
	return sql.Open("sqlite", "file:"+s.store.LifeDBPath(life, t.In(time.FixedZone("CST", 28800)).Format("2006-01"))+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
}
func date(t time.Time) string { return t.In(time.FixedZone("CST", 28800)).Format("2006-01-02") }
type tagRow struct {
	id, name, emoji, createdAt string
	value, sortOrder           int
}

// ensureTags copies the mood tag set forward into a month database that has none yet. Tags live in
// month databases, so without this every new month would start with an empty tag grid.
func (s *Service) ensureTags(ctx context.Context, life string, db *sql.DB) error {
	var count int
	if e := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mood_tags").Scan(&count); e != nil || count > 0 {
		return e
	}
	months, e := s.store.LifeMonths(ctx, life)
	if e != nil {
		return e
	}
	current := storage.MonthKey(time.Now())
	for _, month := range months {
		if month == current {
			continue
		}
		source, e := s.store.OpenLifeMonth(ctx, life, month)
		if e != nil {
			return e
		}
		rows, e := source.QueryContext(ctx, "SELECT id,name,emoji,value,sort_order,created_at FROM mood_tags ORDER BY sort_order,id")
		if e != nil {
			source.Close()
			return e
		}
		copied := []tagRow{}
		for rows.Next() {
			var r tagRow
			if e = rows.Scan(&r.id, &r.name, &r.emoji, &r.value, &r.sortOrder, &r.createdAt); e != nil {
				rows.Close()
				source.Close()
				return e
			}
			copied = append(copied, r)
		}
		rows.Close()
		source.Close()
		if len(copied) == 0 {
			continue
		}
		for _, r := range copied {
			if _, e = db.ExecContext(ctx, "INSERT OR IGNORE INTO mood_tags(id,name,emoji,value,sort_order,created_at) VALUES(?,?,?,?,?,?)", r.id, r.name, r.emoji, r.value, r.sortOrder, r.createdAt); e != nil {
				return e
			}
		}
		return nil
	}
	return nil
}
func (s *Service) Tags(ctx context.Context, life string) ([]MoodTag, error) {
	db, e := s.db(ctx, life, time.Now())
	if e != nil {
		return nil, e
	}
	defer db.Close()
	if e = s.ensureTags(ctx, life, db); e != nil {
		return nil, e
	}
	r, e := db.QueryContext(ctx, "SELECT id,name,emoji,value,sort_order FROM mood_tags ORDER BY sort_order,id")
	if e != nil {
		return nil, e
	}
	defer r.Close()
	out := []MoodTag{}
	for r.Next() {
		var x MoodTag
		if e = r.Scan(&x.ID, &x.Name, &x.Emoji, &x.Value, &x.SortOrder); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, r.Err()
}
func (s *Service) AddTag(ctx context.Context, life, name, emoji string, value int) (MoodTag, error) {
	if name == "" || emoji == "" || value < 1 {
		return MoodTag{}, fmt.Errorf("标签名称、图标和正整数数值均为必填")
	}
	db, e := s.db(ctx, life, time.Now())
	if e != nil {
		return MoodTag{}, e
	}
	defer db.Close()
	x := MoodTag{ID: uuid.NewString(), Name: name, Emoji: emoji, Value: value}
	e = db.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order),0)+1 FROM mood_tags").Scan(&x.SortOrder)
	if e != nil {
		return x, e
	}
	_, e = db.ExecContext(ctx, "INSERT INTO mood_tags(id,name,emoji,value,sort_order,created_at) VALUES(?,?,?,?,?,?)", x.ID, x.Name, x.Emoji, x.Value, x.SortOrder, time.Now().UTC().Format(time.RFC3339Nano))
	return x, e
}
func (s *Service) AddMood(ctx context.Context, life, note string, tagIDs []string, secret bool) (MoodRecord, error) {
	if len(tagIDs) == 0 {
		return MoodRecord{}, fmt.Errorf("至少选择一个心情标签")
	}
	now := time.Now().UTC()
	db, e := s.db(ctx, life, now)
	if e != nil {
		return MoodRecord{}, e
	}
	defer db.Close()
	if e = s.ensureTags(ctx, life, db); e != nil {
		return MoodRecord{}, e
	}
	tags := make([]MoodTag, 0, len(tagIDs))
	sum := 0
	for _, id := range tagIDs {
		var tag MoodTag
		if e = db.QueryRowContext(ctx, "SELECT id,name,emoji,value,sort_order FROM mood_tags WHERE id=?", id).Scan(&tag.ID, &tag.Name, &tag.Emoji, &tag.Value, &tag.SortOrder); e != nil {
			return MoodRecord{}, fmt.Errorf("无效心情标签")
		}
		tags = append(tags, tag)
		sum += tag.Value
	}
	raw, _ := json.Marshal(tags)
	x := MoodRecord{ID: uuid.NewString(), RecordedAt: now.Format(time.RFC3339Nano), RecordedDate: date(now), Note: note, Value: float64(sum) / float64(len(tags)), Tags: tags, Secret: secret}
	_, e = db.ExecContext(ctx, "INSERT INTO mood_records(id,life_id,recorded_at,recorded_date,value,note,tags_json,secret,created_at) VALUES(?,?,?,?,?,?,?,?,?)", x.ID, life, x.RecordedAt, x.RecordedDate, x.Value, x.Note, string(raw), secret, x.RecordedAt)
	return x, e
}
func (s *Service) AddBody(ctx context.Context, life string, score int, note string, secret bool) (BodyRecord, error) {
	if score < 0 || score > 100 {
		return BodyRecord{}, fmt.Errorf("身体评分必须在 0 至 100 之间")
	}
	now := time.Now().UTC()
	x := BodyRecord{ID: uuid.NewString(), RecordedAt: now.Format(time.RFC3339Nano), RecordedDate: date(now), Score: score, Note: note, Secret: secret}
	db, e := s.db(ctx, life, now)
	if e != nil {
		return x, e
	}
	defer db.Close()
	_, e = db.ExecContext(ctx, "INSERT INTO body_records(id,life_id,recorded_at,recorded_date,score,note,secret,created_at) VALUES(?,?,?,?,?,?,?,?)", x.ID, life, x.RecordedAt, x.RecordedDate, x.Score, x.Note, secret, x.RecordedAt)
	return x, e
}
func (s *Service) SaveAttachment(ctx context.Context, life, name, mime string, size int64, source io.Reader) (Attachment, error) {
	if size < 1 || size > 20<<20 {
		return Attachment{}, fmt.Errorf("附件大小必须在 1B 至 20MB 之间")
	}
	if strings.ContainsAny(name, "\\/\x00") || name == "" {
		return Attachment{}, fmt.Errorf("附件名称无效")
	}
	now := time.Now().UTC()
	month := now.In(time.FixedZone("CST", 28800)).Format("2006-01")
	dir := s.store.UploadDir(life, month)
	if e := os.MkdirAll(dir, 0o750); e != nil {
		return Attachment{}, e
	}
	x := Attachment{ID: uuid.NewString(), OriginalName: name, MimeType: mime, ByteSize: size}
	stored := x.ID + filepath.Ext(name)
	temp := filepath.Join(dir, stored+".part")
	target := filepath.Join(dir, stored)
	out, e := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if e != nil {
		return x, e
	}
	written, e := io.Copy(out, io.LimitReader(source, (20<<20)+1))
	closeErr := out.Close()
	if e != nil || closeErr != nil || written != size {
		_ = os.Remove(temp)
		return x, fmt.Errorf("保存附件失败")
	}
	if e = os.Rename(temp, target); e != nil {
		return x, e
	}
	db, e := s.db(ctx, life, now)
	if e != nil {
		_ = os.Remove(target)
		return x, e
	}
	defer db.Close()
	_, e = db.ExecContext(ctx, "INSERT INTO content_attachments(id,entry_date,original_name,stored_name,mime_type,byte_size,created_at) VALUES(?,?,?,?,?,?,?)", x.ID, date(now), x.OriginalName, stored, x.MimeType, x.ByteSize, now.Format(time.RFC3339Nano))
	if e != nil {
		_ = os.Remove(target)
	}
	return x, e
}
// AttachmentForRead walks the month databases (newest first) because attachments are stored in the
// month they were uploaded; the file lives in that month's upload directory.
func (s *Service) AttachmentForRead(ctx context.Context, life, id string) (Attachment, string, string, error) {
	months, e := s.store.LifeMonths(ctx, life)
	if e != nil {
		return Attachment{}, "", "", e
	}
	for _, month := range months {
		db, e := s.store.OpenLifeMonth(ctx, life, month)
		if e != nil {
			return Attachment{}, "", "", e
		}
		var entryDate, stored string
		var secret int
		x := Attachment{ID: id}
		e = db.QueryRowContext(ctx, "SELECT entry_date,original_name,stored_name,mime_type,byte_size,COALESCE(visibility_preset_id,''),COALESCE(secret,0) FROM content_attachments WHERE id=?", id).Scan(&entryDate, &x.OriginalName, &stored, &x.MimeType, &x.ByteSize, &x.PresetID, &secret)
		db.Close()
		if e == sql.ErrNoRows {
			continue
		}
		if e != nil {
			return x, "", "", e
		}
		x.Secret = secret == 1
		return x, entryDate, filepath.Join(s.store.UploadDir(life, month), stored), nil
	}
	return Attachment{}, "", "", fmt.Errorf("附件不存在")
}
func (s *Service) SetAttachmentAccess(ctx context.Context, life, id, presetID string, secret bool) (Attachment, error) {
	months, e := s.store.LifeMonths(ctx, life)
	if e != nil {
		return Attachment{}, e
	}
	for _, month := range months {
		db, e := s.store.OpenLifeMonth(ctx, life, month)
		if e != nil {
			return Attachment{}, e
		}
		r, e := db.ExecContext(ctx, "UPDATE content_attachments SET visibility_preset_id=?,secret=? WHERE id=?", nullString(presetID), secret, id)
		db.Close()
		if e != nil {
			return Attachment{}, e
		}
		if n, _ := r.RowsAffected(); n == 1 {
			return Attachment{ID: id, PresetID: presetID, Secret: secret}, nil
		}
	}
	return Attachment{}, fmt.Errorf("附件不存在")
}
// SaveDraft stores today's draft. The secret layer keeps its own draft row; diary_drafts is keyed by
// entry_date alone, so the secret draft is stored under "<date>#secret".
func (s *Service) SaveDraft(ctx context.Context, life, content string, secret bool) (Draft, error) {
	now := time.Now().UTC()
	x := Draft{EntryDate: date(now), Content: content, UpdatedAt: now.Format(time.RFC3339Nano)}
	db, e := s.db(ctx, life, now)
	if e != nil {
		return x, e
	}
	defer db.Close()
	key := x.EntryDate
	if secret {
		key += "#secret"
	}
	_, e = db.ExecContext(ctx, "INSERT INTO diary_drafts(entry_date,content_md,updated_at) VALUES(?,?,?) ON CONFLICT(entry_date) DO UPDATE SET content_md=excluded.content_md,updated_at=excluded.updated_at", key, x.Content, x.UpdatedAt)
	return x, e
}

// SaveDiary upserts today's diary for one layer: the public entry (secret=false) or the writer-only
// secret entry (secret=true). Both layers may coexist on the same day.
func (s *Service) SaveDiary(ctx context.Context, life, content string, secret bool) (Diary, error) {
	now := time.Now().UTC()
	x := Diary{EntryDate: date(now), Content: content, Secret: secret}
	db, e := s.db(ctx, life, now)
	if e != nil {
		return x, e
	}
	defer db.Close()
	var commentable int
	e = db.QueryRowContext(ctx, "SELECT id,COALESCE(visibility_preset_id,''),COALESCE(commentable,0) FROM diary_entries WHERE entry_date=? AND secret=? LIMIT 1", x.EntryDate, secret).Scan(&x.ID, &x.PresetID, &commentable)
	if e == sql.ErrNoRows {
		x.ID = uuid.NewString()
		_, e = db.ExecContext(ctx, "INSERT INTO diary_entries(id,life_id,entry_date,content_md,secret,created_at,updated_at) VALUES(?,?,?,?,?,?,?)", x.ID, life, x.EntryDate, x.Content, secret, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
		return x, e
	}
	if e != nil {
		return x, e
	}
	x.Commentable = commentable == 1
	_, e = db.ExecContext(ctx, "UPDATE diary_entries SET content_md=?,updated_at=? WHERE id=?", x.Content, now.Format(time.RFC3339Nano), x.ID)
	return x, e
}

// SecretDiary returns today's secret-layer diary; ID is empty when the layer has not been written.
func (s *Service) SecretDiary(ctx context.Context, life string) (Diary, error) {
	now := time.Now().UTC()
	db, e := s.db(ctx, life, now)
	if e != nil {
		return Diary{}, e
	}
	defer db.Close()
	d := Diary{EntryDate: date(now), Secret: true}
	var commentable int
	e = db.QueryRowContext(ctx, "SELECT id,content_md,COALESCE(visibility_preset_id,''),COALESCE(commentable,0) FROM diary_entries WHERE entry_date=? AND secret=1 LIMIT 1", d.EntryDate).Scan(&d.ID, &d.Content, &d.PresetID, &commentable)
	if e == sql.ErrNoRows {
		return d, nil
	}
	d.Commentable = commentable == 1
	return d, e
}
func (s *Service) SetDiaryAccess(ctx context.Context, life, presetID string, secret, commentable bool) (Diary, error) {
	now := time.Now().UTC()
	db, e := s.db(ctx, life, now)
	if e != nil {
		return Diary{}, e
	}
	defer db.Close()
	// Access settings apply to the public layer; the secret layer is always writer-only.
	_, e = db.ExecContext(ctx, "UPDATE diary_entries SET visibility_preset_id=?,secret=?,commentable=?,updated_at=? WHERE life_id=? AND entry_date=? AND secret=0", nullString(presetID), secret, commentable, now.Format(time.RFC3339Nano), life, date(now))
	if e != nil {
		return Diary{}, e
	}
	d, _, _, _, e := s.Today(ctx, life)
	return d, e
}
// SetTaskAccess walks the month databases because the task may belong to an earlier month.
func (s *Service) SetTaskAccess(ctx context.Context, life, id, presetID string, secret, commentable bool) (Task, error) {
	months, e := s.store.LifeMonths(ctx, life)
	if e != nil {
		return Task{}, e
	}
	for _, month := range months {
		db, e := s.store.OpenLifeMonth(ctx, life, month)
		if e != nil {
			return Task{}, e
		}
		r, e := db.ExecContext(ctx, "UPDATE tasks SET visibility_preset_id=?,secret=?,commentable=?,updated_at=? WHERE id=? AND life_id=?", nullString(presetID), secret, commentable, time.Now().UTC().Format(time.RFC3339Nano), id, life)
		if e != nil {
			db.Close()
			return Task{}, e
		}
		if n, _ := r.RowsAffected(); n == 1 {
			x, e := readTask(ctx, db, id)
			db.Close()
			return x, e
		}
		db.Close()
	}
	return Task{}, fmt.Errorf("任务不存在")
}
func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
// taskMoment resolves an optional YYYY-MM-DD task date (Beijing time) to the instant used to pick the
// month database; an empty date means today.
func taskMoment(taskDate string) (time.Time, error) {
	if taskDate == "" {
		return time.Now().UTC(), nil
	}
	parsed, e := time.ParseInLocation("2006-01-02", taskDate, time.FixedZone("CST", 28800))
	if e != nil {
		return time.Time{}, fmt.Errorf("任务日期无效")
	}
	return parsed, nil
}

// AddTask creates a task on the given date (any day, past or future); an empty date means today.
func (s *Service) AddTask(ctx context.Context, life, title, description, priority, taskDate string) (Task, error) {
	if title == "" {
		return Task{}, fmt.Errorf("任务标题不能为空")
	}
	if priority != "low" && priority != "normal" && priority != "high" {
		priority = "normal"
	}
	at, e := taskMoment(taskDate)
	if e != nil {
		return Task{}, e
	}
	now := time.Now().UTC()
	x := Task{ID: uuid.NewString(), TaskDate: date(at), Title: title, Description: description, Priority: priority}
	db, e := s.db(ctx, life, at)
	if e != nil {
		return x, e
	}
	defer db.Close()
	_, e = db.ExecContext(ctx, "INSERT INTO tasks(id,life_id,task_date,title,description,priority,done,created_at,updated_at) VALUES(?,?,?,?,?,?,0,?,?)", x.ID, life, x.TaskDate, x.Title, x.Description, x.Priority, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	return x, e
}

// SetTaskDone toggles completion. Tasks live in the month database of their date, so callers pass the
// task date to locate it; an empty date falls back to the current month.
func (s *Service) SetTaskDone(ctx context.Context, life, id string, done bool, taskDate string) (Task, error) {
	at, e := taskMoment(taskDate)
	if e != nil {
		return Task{}, e
	}
	db, e := s.db(ctx, life, at)
	if e != nil {
		return Task{}, e
	}
	defer db.Close()
	result, e := db.ExecContext(ctx, "UPDATE tasks SET done=?,updated_at=? WHERE id=? AND life_id=?", done, time.Now().UTC().Format(time.RFC3339Nano), id, life)
	if e != nil {
		return Task{}, e
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return Task{}, fmt.Errorf("任务不存在")
	}
	return readTask(ctx, db, id)
}

func readTask(ctx context.Context, db *sql.DB, id string) (Task, error) {
	var x Task
	var done, secret, commentable int
	e := db.QueryRowContext(ctx, "SELECT id,task_date,title,description,priority,done,COALESCE(visibility_preset_id,''),COALESCE(secret,0),COALESCE(commentable,0) FROM tasks WHERE id=?", id).Scan(&x.ID, &x.TaskDate, &x.Title, &x.Description, &x.Priority, &done, &x.PresetID, &secret, &commentable)
	if e != nil {
		return Task{}, fmt.Errorf("任务不存在")
	}
	x.Done = done == 1
	x.Secret = secret == 1
	x.Commentable = commentable == 1
	return x, nil
}
func (s *Service) Today(ctx context.Context, life string) (Diary, []MoodRecord, []BodyRecord, []Task, error) {
	now := time.Now().UTC()
	db, e := s.db(ctx, life, now)
	if e != nil {
		return Diary{}, nil, nil, nil, e
	}
	defer db.Close()
	d := Diary{EntryDate: date(now)}
	var diarySecret, diaryCommentable int
	_ = db.QueryRowContext(ctx, "SELECT id,content_md,COALESCE(visibility_preset_id,''),secret,COALESCE(commentable,0) FROM diary_entries WHERE entry_date=? AND secret=0 LIMIT 1", d.EntryDate).Scan(&d.ID, &d.Content, &d.PresetID, &diarySecret, &diaryCommentable)
	d.Secret = diarySecret == 1
	d.Commentable = diaryCommentable == 1
	moods := []MoodRecord{}
	r, e := db.QueryContext(ctx, "SELECT id,recorded_at,recorded_date,value,note,tags_json,COALESCE(secret,0) FROM mood_records WHERE recorded_date=? ORDER BY recorded_at", d.EntryDate)
	if e != nil {
		return d, nil, nil, nil, e
	}
	for r.Next() {
		var x MoodRecord
		var raw string
		var secret int
		if e = r.Scan(&x.ID, &x.RecordedAt, &x.RecordedDate, &x.Value, &x.Note, &raw, &secret); e != nil {
			r.Close()
			return d, nil, nil, nil, e
		}
		_ = json.Unmarshal([]byte(raw), &x.Tags)
		x.Secret = secret == 1
		moods = append(moods, x)
	}
	r.Close()
	bodies := []BodyRecord{}
	r, e = db.QueryContext(ctx, "SELECT id,recorded_at,recorded_date,score,note,COALESCE(secret,0) FROM body_records WHERE recorded_date=? ORDER BY recorded_at", d.EntryDate)
	if e != nil {
		return d, moods, nil, nil, e
	}
	for r.Next() {
		var x BodyRecord
		var secret int
		if e = r.Scan(&x.ID, &x.RecordedAt, &x.RecordedDate, &x.Score, &x.Note, &secret); e != nil {
			r.Close()
			return d, moods, nil, nil, e
		}
		x.Secret = secret == 1
		bodies = append(bodies, x)
	}
	r.Close()
	tasks := []Task{}
	r, e = db.QueryContext(ctx, "SELECT id,task_date,title,description,priority,done,COALESCE(visibility_preset_id,''),COALESCE(secret,0),COALESCE(commentable,0) FROM tasks WHERE task_date=? ORDER BY done,created_at", d.EntryDate)
	if e != nil {
		return d, moods, bodies, nil, e
	}
	for r.Next() {
		var x Task
		var done, secret, commentable int
		if e = r.Scan(&x.ID, &x.TaskDate, &x.Title, &x.Description, &x.Priority, &done, &x.PresetID, &secret, &commentable); e != nil {
			return d, moods, bodies, nil, e
		}
		x.Done = done == 1
		x.Secret = secret == 1
		x.Commentable = commentable == 1
		tasks = append(tasks, x)
	}
	return d, moods, bodies, tasks, r.Close()
}
