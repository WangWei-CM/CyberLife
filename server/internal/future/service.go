package future

import (
	"context"
	"cyberlife/server/internal/storage"
	"database/sql"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	maxImageSize int64 = 5 << 20
	maxFileSize  int64 = 20 << 20
)

type Service struct{ store *storage.Store }

func New(store *storage.Store) *Service { return &Service{store} }

type PlanFile struct {
	ID           string `json:"id"`
	PlanID       string `json:"planId"`
	OriginalName string `json:"originalName"`
	MimeType     string `json:"mimeType"`
	ByteSize     int64  `json:"byteSize"`
	URL          string `json:"url"`
}
type Plan struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	StartDate    string     `json:"startDate"`
	EndDate      string     `json:"endDate"`
	Intro        string     `json:"intro"`
	Progress     float64    `json:"progress"`
	TimeProgress float64    `json:"timeProgress"`
	Secret       bool       `json:"secret"`
	PresetID     string     `json:"presetId"`
	CoverURL     string     `json:"coverUrl"`
	IconURL      string     `json:"iconUrl"`
	Files        []PlanFile `json:"files"`
	SortOrder    int        `json:"sortOrder"`
	coverPath    string
	iconPath     string
}
type Task struct {
	ID       string `json:"id"`
	Date     string `json:"date"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Done     bool   `json:"done"`
	PresetID string `json:"presetId"`
	Secret   bool   `json:"secret"`
}

type planScanner interface{ Scan(dest ...any) error }

const planColumns = "id,name,start_date,end_date,intro_md,COALESCE(secret,0),COALESCE(visibility_preset_id,''),COALESCE(cover_path,''),COALESCE(icon_path,''),COALESCE(sort_order,0)"

func scanPlan(row planScanner) (Plan, error) {
	var x Plan
	var secret int
	e := row.Scan(&x.ID, &x.Name, &x.StartDate, &x.EndDate, &x.Intro, &secret, &x.PresetID, &x.coverPath, &x.iconPath, &x.SortOrder)
	if e != nil {
		return Plan{}, e
	}
	x.Secret = secret == 1
	if x.coverPath != "" {
		x.CoverURL = "/api/v1/plans/" + x.ID + "/cover"
	}
	if x.iconPath != "" {
		x.IconURL = "/api/v1/plans/" + x.ID + "/icon"
	}
	x.Files = []PlanFile{}
	return x, nil
}

// hydrate fills progress, time progress and the file list from the plan's own month database.
func (s *Service) hydrate(ctx context.Context, db *sql.DB, x *Plan) error {
	x.Progress = s.progress(ctx, db, x.ID)
	x.TimeProgress = timeProgress(x.StartDate, x.EndDate)
	rows, e := db.QueryContext(ctx, "SELECT id,original_name,mime_type,byte_size FROM plan_files WHERE plan_id=? ORDER BY created_at", x.ID)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var f PlanFile
		if e = rows.Scan(&f.ID, &f.OriginalName, &f.MimeType, &f.ByteSize); e != nil {
			return e
		}
		f.PlanID = x.ID
		f.URL = "/api/v1/plans/" + x.ID + "/files/" + f.ID
		x.Files = append(x.Files, f)
	}
	return rows.Err()
}

// ListPlans unions the plans of every month database. Plans are written to the month database that is
// current at creation time, so listing only the present month would hide them after a month rolls over.
func (s *Service) ListPlans(ctx context.Context, life string) ([]Plan, error) {
	months, e := s.store.LifeMonths(ctx, life)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	out := []Plan{}
	for _, month := range months {
		db, e := s.store.OpenLifeMonth(ctx, life, month)
		if e != nil {
			return nil, e
		}
		rows, e := db.QueryContext(ctx, "SELECT "+planColumns+" FROM plans")
		if e != nil {
			db.Close()
			return nil, e
		}
		batch := []Plan{}
		for rows.Next() {
			x, e := scanPlan(rows)
			if e != nil {
				rows.Close()
				db.Close()
				return nil, e
			}
			if seen[x.ID] {
				continue
			}
			seen[x.ID] = true
			batch = append(batch, x)
		}
		rows.Close()
		for i := range batch {
			if e := s.hydrate(ctx, db, &batch[i]); e != nil {
				db.Close()
				return nil, e
			}
		}
		db.Close()
		out = append(out, batch...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].EndDate < out[j].EndDate
	})
	return out, nil
}

// findPlan locates the month database holding the plan; the caller closes the returned handle.
func (s *Service) findPlan(ctx context.Context, life, id string) (*sql.DB, Plan, error) {
	if id == "" {
		return nil, Plan{}, fmt.Errorf("规划不存在")
	}
	months, e := s.store.LifeMonths(ctx, life)
	if e != nil {
		return nil, Plan{}, e
	}
	for _, month := range months {
		db, e := s.store.OpenLifeMonth(ctx, life, month)
		if e != nil {
			return nil, Plan{}, e
		}
		x, e := scanPlan(db.QueryRowContext(ctx, "SELECT "+planColumns+" FROM plans WHERE id=?", id))
		if e == nil {
			if e := s.hydrate(ctx, db, &x); e != nil {
				db.Close()
				return nil, Plan{}, e
			}
			return db, x, nil
		}
		db.Close()
		if e != sql.ErrNoRows {
			return nil, Plan{}, e
		}
	}
	return nil, Plan{}, fmt.Errorf("规划不存在")
}

func validatePlan(name, start, end string) error {
	if name == "" || start == "" || end == "" {
		return fmt.Errorf("规划名称和起止日期不能为空")
	}
	if _, e := time.Parse("2006-01-02", start); e != nil {
		return fmt.Errorf("开始日期无效")
	}
	if _, e := time.Parse("2006-01-02", end); e != nil {
		return fmt.Errorf("截止日期无效")
	}
	if start > end {
		return fmt.Errorf("截止日期不能早于开始日期")
	}
	return nil
}

func (s *Service) CreatePlan(ctx context.Context, life, name, start, end, intro string) (Plan, error) {
	if e := validatePlan(name, start, end); e != nil {
		return Plan{}, e
	}
	existing, e := s.ListPlans(ctx, life)
	if e != nil {
		return Plan{}, e
	}
	nextOrder := 0
	for _, plan := range existing {
		if plan.SortOrder >= nextOrder {
			nextOrder = plan.SortOrder + 1
		}
	}
	db, e := s.store.OpenLifeMonth(ctx, life, storage.MonthKey(time.Now()))
	if e != nil {
		return Plan{}, e
	}
	defer db.Close()
	x := Plan{ID: uuid.NewString(), Name: name, StartDate: start, EndDate: end, Intro: intro, SortOrder: nextOrder, Files: []PlanFile{}}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, e = db.ExecContext(ctx, "INSERT INTO plans(id,life_id,name,start_date,end_date,intro_md,secret,commentable,sort_order,created_at,updated_at) VALUES(?,?,?,?,?,?,0,0,?,?,?)", x.ID, life, x.Name, x.StartDate, x.EndDate, x.Intro, x.SortOrder, now, now)
	x.TimeProgress = timeProgress(start, end)
	return x, e
}

// ReorderPlans persists the exact writer-defined ordering across plans stored in different month databases.
func (s *Service) ReorderPlans(ctx context.Context, life string, ids []string) error {
	plans, e := s.ListPlans(ctx, life)
	if e != nil {
		return e
	}
	if len(ids) != len(plans) {
		return fmt.Errorf("排序列表不完整")
	}
	known := make(map[string]struct{}, len(plans))
	for _, plan := range plans {
		known[plan.ID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := known[id]; !ok {
			return fmt.Errorf("规划不存在")
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("排序列表包含重复规划")
		}
		seen[id] = struct{}{}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for order, id := range ids {
		db, _, e := s.findPlan(ctx, life, id)
		if e != nil {
			return e
		}
		_, e = db.ExecContext(ctx, "UPDATE plans SET sort_order=?,updated_at=? WHERE id=?", order, now, id)
		db.Close()
		if e != nil {
			return e
		}
	}
	return nil
}

// UpdatePlan edits name, dates and the Markdown intro of an existing plan.
func (s *Service) UpdatePlan(ctx context.Context, life, id, name, start, end, intro string) (Plan, error) {
	if e := validatePlan(name, start, end); e != nil {
		return Plan{}, e
	}
	db, x, e := s.findPlan(ctx, life, id)
	if e != nil {
		return Plan{}, e
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, e = db.ExecContext(ctx, "UPDATE plans SET name=?,start_date=?,end_date=?,intro_md=?,updated_at=? WHERE id=?", name, start, end, intro, now, id); e != nil {
		return Plan{}, e
	}
	x.Name, x.StartDate, x.EndDate, x.Intro = name, start, end, intro
	x.TimeProgress = timeProgress(start, end)
	return x, nil
}

func (s *Service) SetProgress(ctx context.Context, life, id, date string, percent float64) (Plan, error) {
	if percent < 0 || percent > 100 {
		return Plan{}, fmt.Errorf("进度必须在 0 到 100 之间")
	}
	if _, e := time.Parse("2006-01-02", date); e != nil {
		return Plan{}, fmt.Errorf("标记日期无效")
	}
	db, x, e := s.findPlan(ctx, life, id)
	if e != nil {
		return Plan{}, e
	}
	defer db.Close()
	_, e = db.ExecContext(ctx, "INSERT INTO plan_progress(id,plan_id,date,percent,created_at) VALUES(?,?,?,?,?) ON CONFLICT(plan_id,date) DO UPDATE SET percent=excluded.percent", uuid.NewString(), id, date, percent, time.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return Plan{}, e
	}
	x.Progress = s.progress(ctx, db, id)
	return x, nil
}

// SetImage stores the cover or icon image of a plan and replaces the previous one.
func (s *Service) SetImage(ctx context.Context, life, id, kind, name, declaredType string, size int64, source io.Reader) (Plan, error) {
	if kind != "cover" && kind != "icon" {
		return Plan{}, fmt.Errorf("图片类型无效")
	}
	if size < 1 || size > maxImageSize {
		return Plan{}, fmt.Errorf("图片大小必须在 1B 至 5MB 之间")
	}
	if !strings.HasPrefix(declaredType, "image/") {
		return Plan{}, fmt.Errorf("仅支持图片文件")
	}
	db, x, e := s.findPlan(ctx, life, id)
	if e != nil {
		return Plan{}, e
	}
	defer db.Close()
	stored, e := s.storeFile(life, x.ID+"-"+kind+"-"+uuid.NewString()+extensionFor(name, declaredType), size, source, maxImageSize)
	if e != nil {
		return Plan{}, e
	}
	column, previous := "cover_path", x.coverPath
	if kind == "icon" {
		column, previous = "icon_path", x.iconPath
	}
	if _, e = db.ExecContext(ctx, "UPDATE plans SET "+column+"=?,updated_at=? WHERE id=?", stored, time.Now().UTC().Format(time.RFC3339Nano), id); e != nil {
		_ = os.Remove(filepath.Join(s.store.PlanUploadDir(life), stored))
		return Plan{}, e
	}
	if previous != "" {
		_ = os.Remove(filepath.Join(s.store.PlanUploadDir(life), previous))
	}
	if kind == "icon" {
		x.iconPath = stored
		x.IconURL = "/api/v1/plans/" + x.ID + "/icon"
	} else {
		x.coverPath = stored
		x.CoverURL = "/api/v1/plans/" + x.ID + "/cover"
	}
	return x, nil
}

// ImageForRead returns the plan (for the access check), the file path and the content type.
func (s *Service) ImageForRead(ctx context.Context, life, id, kind string) (Plan, string, string, error) {
	db, x, e := s.findPlan(ctx, life, id)
	if e != nil {
		return Plan{}, "", "", e
	}
	db.Close()
	stored := x.coverPath
	if kind == "icon" {
		stored = x.iconPath
	}
	if stored == "" {
		return x, "", "", fmt.Errorf("图片不存在")
	}
	contentType := mime.TypeByExtension(filepath.Ext(stored))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return x, filepath.Join(s.store.PlanUploadDir(life), stored), contentType, nil
}

// AddFile attaches a document to a plan (same 20MB limit as diary attachments).
func (s *Service) AddFile(ctx context.Context, life, id, name, declaredType string, size int64, source io.Reader) (PlanFile, error) {
	if size < 1 || size > maxFileSize {
		return PlanFile{}, fmt.Errorf("文件大小必须在 1B 至 20MB 之间")
	}
	if name == "" || strings.ContainsAny(name, "\\/\x00") {
		return PlanFile{}, fmt.Errorf("文件名无效")
	}
	db, x, e := s.findPlan(ctx, life, id)
	if e != nil {
		return PlanFile{}, e
	}
	defer db.Close()
	f := PlanFile{ID: uuid.NewString(), PlanID: x.ID, OriginalName: name, MimeType: declaredType, ByteSize: size}
	if f.MimeType == "" {
		f.MimeType = "application/octet-stream"
	}
	stored, e := s.storeFile(life, f.ID+extensionFor(name, declaredType), size, source, maxFileSize)
	if e != nil {
		return PlanFile{}, e
	}
	if _, e = db.ExecContext(ctx, "INSERT INTO plan_files(id,plan_id,original_name,stored_name,mime_type,byte_size,created_at) VALUES(?,?,?,?,?,?,?)", f.ID, x.ID, f.OriginalName, stored, f.MimeType, f.ByteSize, time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
		_ = os.Remove(filepath.Join(s.store.PlanUploadDir(life), stored))
		return PlanFile{}, e
	}
	f.URL = "/api/v1/plans/" + x.ID + "/files/" + f.ID
	return f, nil
}

func (s *Service) DeleteFile(ctx context.Context, life, planID, fileID string) error {
	db, _, e := s.findPlan(ctx, life, planID)
	if e != nil {
		return e
	}
	defer db.Close()
	var stored string
	if e = db.QueryRowContext(ctx, "SELECT stored_name FROM plan_files WHERE id=? AND plan_id=?", fileID, planID).Scan(&stored); e != nil {
		return fmt.Errorf("文件不存在")
	}
	if _, e = db.ExecContext(ctx, "DELETE FROM plan_files WHERE id=?", fileID); e != nil {
		return e
	}
	if e = os.Remove(filepath.Join(s.store.PlanUploadDir(life), stored)); e != nil && !os.IsNotExist(e) {
		return e
	}
	return nil
}

// FileForRead returns the file record, its plan (for the access check) and the path on disk.
func (s *Service) FileForRead(ctx context.Context, life, planID, fileID string) (PlanFile, Plan, string, error) {
	db, x, e := s.findPlan(ctx, life, planID)
	if e != nil {
		return PlanFile{}, Plan{}, "", e
	}
	defer db.Close()
	var f PlanFile
	var stored string
	if e = db.QueryRowContext(ctx, "SELECT id,original_name,stored_name,mime_type,byte_size FROM plan_files WHERE id=? AND plan_id=?", fileID, planID).Scan(&f.ID, &f.OriginalName, &stored, &f.MimeType, &f.ByteSize); e != nil {
		return PlanFile{}, x, "", fmt.Errorf("文件不存在")
	}
	f.PlanID = x.ID
	f.URL = "/api/v1/plans/" + x.ID + "/files/" + f.ID
	return f, x, filepath.Join(s.store.PlanUploadDir(life), stored), nil
}

// storeFile writes an upload atomically into the plan upload directory and returns the stored name.
func (s *Service) storeFile(life, storedName string, size int64, source io.Reader, limit int64) (string, error) {
	if storedName == "" || strings.ContainsAny(storedName, "\\/\x00") {
		return "", fmt.Errorf("文件名无效")
	}
	dir := s.store.PlanUploadDir(life)
	if e := os.MkdirAll(dir, 0o750); e != nil {
		return "", e
	}
	temp := filepath.Join(dir, storedName+".part")
	target := filepath.Join(dir, storedName)
	out, e := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if e != nil {
		return "", e
	}
	written, copyErr := io.Copy(out, io.LimitReader(source, limit+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || written != size {
		_ = os.Remove(temp)
		return "", fmt.Errorf("保存文件失败")
	}
	if e = os.Rename(temp, target); e != nil {
		return "", e
	}
	return storedName, nil
}

func extensionFor(name, declaredType string) string {
	if ext := strings.ToLower(filepath.Ext(name)); ext != "" && len(ext) <= 8 {
		return ext
	}
	if exts, _ := mime.ExtensionsByType(declaredType); len(exts) > 0 {
		return exts[0]
	}
	return ""
}

// Calendar returns the tasks in [from, to] by opening each month database once. Callers apply the
// ACL to the returned preset/secret fields.
func (s *Service) Calendar(ctx context.Context, life, from, to string) ([]Task, error) {
	start, e := time.ParseInLocation("2006-01-02", from, location())
	if e != nil {
		return nil, fmt.Errorf("起始日期无效")
	}
	end, e := time.ParseInLocation("2006-01-02", to, location())
	if e != nil || end.Before(start) {
		return nil, fmt.Errorf("截止日期无效")
	}
	if end.Sub(start) > 366*24*time.Hour {
		return nil, fmt.Errorf("单次日历范围最多 366 天")
	}
	out := []Task{}
	for month := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, location()); !month.After(end); month = month.AddDate(0, 1, 0) {
		low := start
		if month.After(low) {
			low = month
		}
		high := month.AddDate(0, 1, -1)
		if high.After(end) {
			high = end
		}
		db, e := s.store.OpenLifeMonth(ctx, life, storage.MonthKey(month))
		if e != nil {
			return nil, e
		}
		rows, e := db.QueryContext(ctx, "SELECT id,task_date,title,priority,done,COALESCE(visibility_preset_id,''),COALESCE(secret,0) FROM tasks WHERE task_date BETWEEN ? AND ? ORDER BY task_date,created_at", low.Format("2006-01-02"), high.Format("2006-01-02"))
		if e != nil {
			db.Close()
			return nil, e
		}
		for rows.Next() {
			var x Task
			var done, secret int
			if e = rows.Scan(&x.ID, &x.Date, &x.Title, &x.Priority, &done, &x.PresetID, &secret); e != nil {
				rows.Close()
				db.Close()
				return nil, e
			}
			x.Done = done == 1
			x.Secret = secret == 1
			out = append(out, x)
		}
		rows.Close()
		db.Close()
	}
	return out, nil
}

func (s *Service) progress(ctx context.Context, db *sql.DB, id string) float64 {
	var v sql.NullFloat64
	_ = db.QueryRowContext(ctx, "SELECT percent FROM plan_progress WHERE plan_id=? ORDER BY date DESC LIMIT 1", id).Scan(&v)
	return v.Float64
}
func timeProgress(start, end string) float64 {
	a, e := time.Parse("2006-01-02", start)
	if e != nil {
		return 0
	}
	b, e := time.Parse("2006-01-02", end)
	if e != nil || !b.After(a) {
		return 100
	}
	v := time.Now().In(location())
	if v.Before(a) {
		return 0
	}
	if v.After(b) {
		return 100
	}
	return float64(v.Sub(a)) / float64(b.Sub(a)) * 100
}
func location() *time.Location {
	v, e := time.LoadLocation("Asia/Shanghai")
	if e != nil {
		return time.FixedZone("CST", 28800)
	}
	return v
}
