package schedule

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"cyberlife/server/internal/storage"
	"github.com/google/uuid"
)

var shanghai, _ = time.LoadLocation("Asia/Shanghai")

type Service struct{ store *storage.Store }

func New(store *storage.Store) *Service { return &Service{store: store} }

type Class struct {
	ID                 string `json:"id"`
	LifeID             string `json:"lifeId"`
	Title              string `json:"title"`
	Weekday            *int   `json:"weekday,omitempty"`
	SessionDate        string `json:"sessionDate,omitempty"`
	StartTime          string `json:"startTime"`
	EndTime            string `json:"endTime"`
	EffectiveStartDate string `json:"effectiveStartDate,omitempty"`
	EffectiveEndDate   string `json:"effectiveEndDate,omitempty"`
	Location           string `json:"location"`
	Note               string `json:"note"`
	Source             string `json:"source"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}
type Occurrence struct {
	ClassID  string `json:"classId"`
	Title    string `json:"title"`
	StartsAt string `json:"startsAt"`
	EndsAt   string `json:"endsAt"`
	Location string `json:"location"`
	Note     string `json:"note"`
}
type Agenda struct {
	GeneratedAt string       `json:"generatedAt"`
	Current     *Occurrence  `json:"current"`
	Next        []Occurrence `json:"next"`
}

func validateClass(x Class) error {
	if strings.TrimSpace(x.Title) == "" {
		return fmt.Errorf("课程名称不能为空")
	}
	if _, err := time.Parse("15:04", x.StartTime); err != nil {
		return fmt.Errorf("开始时间无效")
	}
	if _, err := time.Parse("15:04", x.EndTime); err != nil {
		return fmt.Errorf("结束时间无效")
	}
	if x.StartTime >= x.EndTime {
		return fmt.Errorf("结束时间必须晚于开始时间")
	}
	if (x.Weekday == nil) == (strings.TrimSpace(x.SessionDate) == "") {
		return fmt.Errorf("必须填写星期或具体日期之一")
	}
	if x.Weekday != nil && (*x.Weekday < 1 || *x.Weekday > 7) {
		return fmt.Errorf("星期必须在 1 到 7 之间")
	}
	if x.SessionDate != "" {
		if _, err := time.ParseInLocation("2006-01-02", x.SessionDate, shanghai); err != nil {
			return fmt.Errorf("课程日期无效")
		}
	}
	if x.EffectiveStartDate != "" {
		if _, err := time.ParseInLocation("2006-01-02", x.EffectiveStartDate, shanghai); err != nil {
			return fmt.Errorf("生效开始日期无效")
		}
	}
	if x.EffectiveEndDate != "" {
		if _, err := time.ParseInLocation("2006-01-02", x.EffectiveEndDate, shanghai); err != nil {
			return fmt.Errorf("生效结束日期无效")
		}
	}
	if x.EffectiveStartDate != "" && x.EffectiveEndDate != "" && x.EffectiveStartDate > x.EffectiveEndDate {
		return fmt.Errorf("生效日期范围无效")
	}
	return nil
}

func (s *Service) List(ctx context.Context, life string) ([]Class, error) {
	rows, err := s.store.Global().QueryContext(ctx, `SELECT id,life_id,title,weekday,COALESCE(session_date,''),start_time,end_time,COALESCE(effective_start_date,''),COALESCE(effective_end_date,''),location,note,source,created_at,updated_at FROM schedule_classes WHERE life_id=? ORDER BY COALESCE(session_date,''),weekday,start_time,id`, life)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Class{}
	for rows.Next() {
		var x Class
		var wd sql.NullInt64
		if err := rows.Scan(&x.ID, &x.LifeID, &x.Title, &wd, &x.SessionDate, &x.StartTime, &x.EndTime, &x.EffectiveStartDate, &x.EffectiveEndDate, &x.Location, &x.Note, &x.Source, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		if wd.Valid {
			v := int(wd.Int64)
			x.Weekday = &v
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) Create(ctx context.Context, life string, x Class) (Class, error) {
	x.ID = uuid.NewString()
	x.LifeID = life
	if err := validateClass(x); err != nil {
		return x, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	x.CreatedAt = now
	x.UpdatedAt = now
	_, err := s.store.Global().ExecContext(ctx, `INSERT INTO schedule_classes(id,life_id,title,weekday,session_date,start_time,end_time,effective_start_date,effective_end_date,location,note,source,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, x.ID, life, x.Title, x.Weekday, nullString(x.SessionDate), x.StartTime, x.EndTime, nullString(x.EffectiveStartDate), nullString(x.EffectiveEndDate), x.Location, x.Note, sourceOrManual(x.Source), now, now)
	return x, err
}
func (s *Service) Update(ctx context.Context, life, id string, x Class) (Class, error) {
	x.ID = id
	x.LifeID = life
	if err := validateClass(x); err != nil {
		return x, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.store.Global().ExecContext(ctx, `UPDATE schedule_classes SET title=?,weekday=?,session_date=?,start_time=?,end_time=?,effective_start_date=?,effective_end_date=?,location=?,note=?,updated_at=? WHERE id=? AND life_id=?`, x.Title, x.Weekday, nullString(x.SessionDate), x.StartTime, x.EndTime, nullString(x.EffectiveStartDate), nullString(x.EffectiveEndDate), x.Location, x.Note, now, id, life)
	if err != nil {
		return x, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return x, fmt.Errorf("课程不存在")
	}
	x.UpdatedAt = now
	return x, nil
}
func (s *Service) Delete(ctx context.Context, life, id string) error {
	res, err := s.store.Global().ExecContext(ctx, "DELETE FROM schedule_classes WHERE id=? AND life_id=?", id, life)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fmt.Errorf("课程不存在")
	}
	return nil
}
func (s *Service) Import(ctx context.Context, life, mode string, items []Class) error {
	if len(items) == 0 {
		return fmt.Errorf("没有可导入的有效课程")
	}
	if mode != "replace" && mode != "merge" {
		return fmt.Errorf("导入模式无效")
	}
	tx, e := s.store.Global().BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if mode == "replace" {
		if _, e = tx.ExecContext(ctx, "DELETE FROM schedule_classes WHERE life_id=?", life); e != nil {
			return e
		}
	}
	existing := map[string]bool{}
	if mode == "merge" {
		rows, e := tx.QueryContext(ctx, "SELECT title,weekday,COALESCE(session_date,''),start_time,end_time FROM schedule_classes WHERE life_id=?", life)
		if e != nil {
			return e
		}
		for rows.Next() {
			var t, sd, st, et string
			var wd sql.NullInt64
			_ = rows.Scan(&t, &wd, &sd, &st, &et)
			existing[dedupeKey(t, wd, sd, st, et)] = true
		}
		rows.Close()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, x := range items {
		key := dedupeKey(x.Title, func() sql.NullInt64 {
			if x.Weekday == nil {
				return sql.NullInt64{}
			}
			return sql.NullInt64{Int64: int64(*x.Weekday), Valid: true}
		}(), x.SessionDate, x.StartTime, x.EndTime)
		if mode == "merge" && existing[key] {
			continue
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO schedule_classes(id,life_id,title,weekday,session_date,start_time,end_time,effective_start_date,effective_end_date,location,note,source,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, uuid.NewString(), life, x.Title, x.Weekday, nullString(x.SessionDate), x.StartTime, x.EndTime, nullString(x.EffectiveStartDate), nullString(x.EffectiveEndDate), x.Location, x.Note, sourceOrManual(x.Source), now, now); e != nil {
			return e
		}
		existing[key] = true
	}
	return tx.Commit()
}
func dedupeKey(t string, wd sql.NullInt64, sd, st, et string) string {
	return strings.ToLower(strings.TrimSpace(t)) + "|" + fmt.Sprint(wd.Int64) + "|" + sd + "|" + st + "|" + et
}
func sourceOrManual(s string) string {
	if s == "ics" || s == "csv" {
		return s
	}
	return "manual"
}
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Service) Agenda(ctx context.Context, life string, at time.Time) (Agenda, error) {
	local := at.In(shanghai)
	classes, err := s.List(ctx, life)
	if err != nil {
		return Agenda{}, err
	}
	all := []Occurrence{}
	current := (*Occurrence)(nil)
	for d := 0; d < 15; d++ {
		day := local.AddDate(0, 0, d)
		wd := int(day.Weekday())
		if wd == 0 {
			wd = 7
		}
		date := day.Format("2006-01-02")
		for _, c := range classes {
			match := (c.SessionDate == date) || (c.SessionDate == "" && c.Weekday != nil && *c.Weekday == wd && (c.EffectiveStartDate == "" || date >= c.EffectiveStartDate) && (c.EffectiveEndDate == "" || date <= c.EffectiveEndDate))
			if !match {
				continue
			}
			st, _ := time.ParseInLocation("15:04", c.StartTime, shanghai)
			et, _ := time.ParseInLocation("15:04", c.EndTime, shanghai)
			start := time.Date(day.Year(), day.Month(), day.Day(), st.Hour(), st.Minute(), 0, 0, shanghai)
			end := time.Date(day.Year(), day.Month(), day.Day(), et.Hour(), et.Minute(), 0, 0, shanghai)
			o := Occurrence{ClassID: c.ID, Title: c.Title, StartsAt: start.Format(time.RFC3339), EndsAt: end.Format(time.RFC3339), Location: c.Location, Note: c.Note}
			if !end.After(local) {
				continue
			}
			if !start.After(local) && local.Before(end) {
				copy := o
				current = &copy
			} else {
				all = append(all, o)
			}
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].StartsAt < all[j].StartsAt })
	if len(all) > 4 {
		all = all[:4]
	}
	return Agenda{GeneratedAt: local.Format(time.RFC3339), Current: current, Next: all}, nil
}
