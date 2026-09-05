package future

import (
	"context"
	"cyberlife/server/internal/storage"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"sort"
	"time"
)

type Service struct{ store *storage.Store }

func New(store *storage.Store) *Service { return &Service{store} }

type Plan struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	StartDate    string  `json:"startDate"`
	EndDate      string  `json:"endDate"`
	Intro        string  `json:"intro"`
	Progress     float64 `json:"progress"`
	TimeProgress float64 `json:"timeProgress"`
}
type Task struct {
	ID       string `json:"id"`
	Date     string `json:"date"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Done     bool   `json:"done"`
}

func (s *Service) db(ctx context.Context, life string, t time.Time) (*sql.DB, error) {
	if e := s.store.EnsureLifeMonth(ctx, life, t); e != nil {
		return nil, e
	}
	return sql.Open("sqlite", "file:"+s.store.LifeDBPath(life, t.In(location()).Format("2006-01"))+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
}
// months lists the month databases registered for a life (newest first), always including the
// current month so a fresh life works before any month database exists.
func (s *Service) months(ctx context.Context, life string) ([]string, error) {
	rows, e := s.store.Global().QueryContext(ctx, "SELECT month_key FROM life_months WHERE life_id=? ORDER BY month_key DESC", life)
	if e != nil {
		return nil, e
	}
	out := []string{}
	for rows.Next() {
		var key string
		if e = rows.Scan(&key); e != nil {
			rows.Close()
			return nil, e
		}
		out = append(out, key)
	}
	rows.Close()
	current := time.Now().In(location()).Format("2006-01")
	for _, key := range out {
		if key == current {
			return out, nil
		}
	}
	return append([]string{current}, out...), nil
}

func (s *Service) monthDB(ctx context.Context, life, month string) (*sql.DB, error) {
	t, e := time.ParseInLocation("2006-01", month, location())
	if e != nil {
		return nil, fmt.Errorf("月份无效")
	}
	return s.db(ctx, life, t)
}

type planScanner interface{ Scan(dest ...any) error }

func scanPlan(row planScanner) (Plan, error) {
	var x Plan
	e := row.Scan(&x.ID, &x.Name, &x.StartDate, &x.EndDate, &x.Intro)
	return x, e
}

// ListPlans unions the plans of every month database. Plans are written to the month database that is
// current at creation time, so listing only the present month would hide them after a month rolls over.
func (s *Service) ListPlans(ctx context.Context, life string) ([]Plan, error) {
	months, e := s.months(ctx, life)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	out := []Plan{}
	for _, month := range months {
		db, e := s.monthDB(ctx, life, month)
		if e != nil {
			return nil, e
		}
		rows, e := db.QueryContext(ctx, "SELECT id,name,start_date,end_date,intro_md FROM plans")
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
			batch[i].Progress = s.progress(ctx, db, batch[i].ID)
			batch[i].TimeProgress = timeProgress(batch[i].StartDate, batch[i].EndDate)
		}
		db.Close()
		out = append(out, batch...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].EndDate < out[j].EndDate })
	return out, nil
}

// findPlan locates the month database holding the plan; the caller closes the returned handle.
func (s *Service) findPlan(ctx context.Context, life, id string) (*sql.DB, Plan, error) {
	months, e := s.months(ctx, life)
	if e != nil {
		return nil, Plan{}, e
	}
	for _, month := range months {
		db, e := s.monthDB(ctx, life, month)
		if e != nil {
			return nil, Plan{}, e
		}
		x, e := scanPlan(db.QueryRowContext(ctx, "SELECT id,name,start_date,end_date,intro_md FROM plans WHERE id=?", id))
		if e == nil {
			x.Progress = s.progress(ctx, db, x.ID)
			x.TimeProgress = timeProgress(x.StartDate, x.EndDate)
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
	db, e := s.db(ctx, life, time.Now())
	if e != nil {
		return Plan{}, e
	}
	defer db.Close()
	x := Plan{ID: uuid.NewString(), Name: name, StartDate: start, EndDate: end, Intro: intro}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, e = db.ExecContext(ctx, "INSERT INTO plans(id,life_id,name,start_date,end_date,intro_md,secret,commentable,created_at,updated_at) VALUES(?,?,?,?,?,?,0,0,?,?)", x.ID, life, x.Name, x.StartDate, x.EndDate, x.Intro, now, now)
	return x, e
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
func (s *Service) Calendar(ctx context.Context, life, from, to string) ([]Task, error) {
	start, e := time.Parse("2006-01-02", from)
	if e != nil {
		return nil, fmt.Errorf("起始日期无效")
	}
	end, e := time.Parse("2006-01-02", to)
	if e != nil || end.Before(start) {
		return nil, fmt.Errorf("截止日期无效")
	}
	if end.Sub(start) > 366*24*time.Hour {
		return nil, fmt.Errorf("单次日历范围最多 366 天")
	}
	out := []Task{}
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		db, e := s.db(ctx, life, date)
		if e != nil {
			return nil, e
		}
		rows, e := db.QueryContext(ctx, "SELECT id,task_date,title,priority,done FROM tasks WHERE task_date=? ORDER BY created_at", date.Format("2006-01-02"))
		if e == nil {
			for rows.Next() {
				var x Task
				var done int
				if e = rows.Scan(&x.ID, &x.Date, &x.Title, &x.Priority, &done); e != nil {
					rows.Close()
					db.Close()
					return nil, e
				}
				x.Done = done == 1
				out = append(out, x)
			}
			rows.Close()
		}
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
