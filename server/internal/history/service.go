package history

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cyberlife/server/internal/acl"
	"cyberlife/server/internal/auth"
	nowservice "cyberlife/server/internal/now"
	"cyberlife/server/internal/storage"
)

type Service struct {
	store *storage.Store
	acl   *acl.Service
}

func New(store *storage.Store, aclService *acl.Service) *Service {
	return &Service{store: store, acl: aclService}
}

type Point struct {
	Date string   `json:"date"`
	Mood *float64 `json:"mood"`
	Body *float64 `json:"body"`
}
type Day struct {
	Date           string            `json:"date"`
	Diary          nowservice.Diary  `json:"diary"`
	SecretDiary    *nowservice.Diary `json:"secretDiary,omitempty"`
	Tasks          []nowservice.Task `json:"tasks"`
	MoodCount      int               `json:"moodCount"`
	BodyCount      int               `json:"bodyCount"`
	MilestoneCount int               `json:"milestoneCount"`
}
type Range struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Days   []Day   `json:"days"`
	Points []Point `json:"points"`
}

func (s *Service) Range(ctx context.Context, actor auth.Actor, from, to time.Time) (Range, error) {
	if from.After(to) {
		return Range{}, fmt.Errorf("起始日期不能晚于结束日期")
	}
	location := shanghai()
	from = dayStart(from, location)
	to = dayStart(to, location)
	today := dayStart(time.Now(), location)
	if to.After(today) {
		return Range{}, fmt.Errorf("历史范围不能超过今天")
	}
	if to.Sub(from) > 366*24*time.Hour {
		return Range{}, fmt.Errorf("单次查询最多 366 天")
	}
	out := Range{From: from.Format("2006-01-02"), To: to.Format("2006-01-02"), Days: []Day{}, Points: []Point{}}
	for date := from; !date.After(to); date = date.AddDate(0, 0, 1) {
		day, e := s.day(ctx, actor, date)
		if e != nil {
			return out, e
		}
		out.Days = append(out.Days, day)
		out.Points = append(out.Points, Point{Date: day.Date})
	}
	for i := range out.Days {
		point := &out.Points[i]
		day := out.Days[i]
		if day.MoodCount > 0 {
			value := s.averageMood(ctx, actor, from.AddDate(0, 0, i))
			point.Mood = value
		}
		if day.BodyCount > 0 {
			value := s.averageBody(ctx, actor, from.AddDate(0, 0, i))
			point.Body = value
		}
	}
	return out, nil
}
func (s *Service) day(ctx context.Context, actor auth.Actor, date time.Time) (Day, error) {
	db, e := s.db(ctx, actor.LifeID, date)
	if e != nil {
		return Day{}, e
	}
	defer db.Close()
	value := Day{Date: date.Format("2006-01-02"), Tasks: []nowservice.Task{}}
	var diary nowservice.Diary
	var secret, commentable int
	e = db.QueryRowContext(ctx, "SELECT id,entry_date,content_md,COALESCE(visibility_preset_id,''),secret,COALESCE(commentable,0) FROM diary_entries WHERE entry_date=? AND secret=0 LIMIT 1", value.Date).Scan(&diary.ID, &diary.EntryDate, &diary.Content, &diary.PresetID, &secret, &commentable)
	if e == nil {
		diary.Secret = secret == 1
		diary.Commentable = commentable == 1
		ok, err := s.acl.CanRead(ctx, actor, acl.Resource{LifeID: actor.LifeID, Date: value.Date, PresetID: diary.PresetID, Secret: diary.Secret})
		if err != nil {
			return value, err
		}
		if ok {
			value.Diary = diary
		}
	}
	// The secret layer is a separate entry on the same day; CanRead only passes it for the writer.
	var secretDiary nowservice.Diary
	var secretCommentable int
	if scanErr := db.QueryRowContext(ctx, "SELECT id,entry_date,content_md,COALESCE(visibility_preset_id,''),COALESCE(commentable,0) FROM diary_entries WHERE entry_date=? AND secret=1 LIMIT 1", value.Date).Scan(&secretDiary.ID, &secretDiary.EntryDate, &secretDiary.Content, &secretDiary.PresetID, &secretCommentable); scanErr == nil {
		secretDiary.Secret = true
		secretDiary.Commentable = secretCommentable == 1
		ok, err := s.acl.CanRead(ctx, actor, acl.Resource{LifeID: actor.LifeID, Date: value.Date, PresetID: secretDiary.PresetID, Secret: true})
		if err != nil {
			return value, err
		}
		if ok {
			layer := secretDiary
			value.SecretDiary = &layer
		}
	}
	rows, e := db.QueryContext(ctx, "SELECT id,task_date,title,description,priority,done,COALESCE(visibility_preset_id,''),COALESCE(secret,0),COALESCE(commentable,0) FROM tasks WHERE task_date=? ORDER BY done,created_at", value.Date)
	if e != nil {
		return value, e
	}
	for rows.Next() {
		var task nowservice.Task
		var doneInt, secretInt, commentableInt int
		if e = rows.Scan(&task.ID, &task.TaskDate, &task.Title, &task.Description, &task.Priority, &doneInt, &task.PresetID, &secretInt, &commentableInt); e != nil {
			rows.Close()
			return value, e
		}
		task.Done = doneInt == 1
		task.Secret = secretInt == 1
		task.Commentable = commentableInt == 1
		ok, err := s.acl.CanRead(ctx, actor, acl.Resource{LifeID: actor.LifeID, Date: value.Date, PresetID: task.PresetID, Secret: task.Secret})
		if err != nil {
			rows.Close()
			return value, err
		}
		if ok {
			value.Tasks = append(value.Tasks, task)
		}
	}
	rows.Close()
	value.MoodCount, e = s.visibleMoodCount(ctx, db, actor, value.Date)
	if e != nil {
		return value, e
	}
	value.BodyCount, e = s.visibleBodyCount(ctx, db, actor, value.Date)
	if e != nil {
		return value, e
	}
	value.MilestoneCount, e = s.visibleMilestoneCount(ctx, db, actor, value.Date)
	return value, e
}
func (s *Service) visibleMoodCount(ctx context.Context, db *sql.DB, actor auth.Actor, date string) (int, error) {
	rows, e := db.QueryContext(ctx, "SELECT COALESCE(secret,0) FROM mood_records WHERE recorded_date=?", date)
	if e != nil {
		return 0, e
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var secret int
		if e = rows.Scan(&secret); e != nil {
			return 0, e
		}
		ok, err := s.acl.CanRead(ctx, actor, acl.Resource{LifeID: actor.LifeID, Date: date, Secret: secret == 1})
		if err != nil {
			return 0, err
		}
		if ok {
			count++
		}
	}
	return count, rows.Err()
}
func (s *Service) visibleBodyCount(ctx context.Context, db *sql.DB, actor auth.Actor, date string) (int, error) {
	rows, e := db.QueryContext(ctx, "SELECT COALESCE(secret,0) FROM body_records WHERE recorded_date=?", date)
	if e != nil {
		return 0, e
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var secret int
		if e = rows.Scan(&secret); e != nil {
			return 0, e
		}
		ok, err := s.acl.CanRead(ctx, actor, acl.Resource{LifeID: actor.LifeID, Date: date, Secret: secret == 1})
		if err != nil {
			return 0, err
		}
		if ok {
			count++
		}
	}
	return count, rows.Err()
}
func (s *Service) visibleMilestoneCount(ctx context.Context, db *sql.DB, actor auth.Actor, date string) (int, error) {
	// visibility_preset_id is NULL when no preset is chosen; scan it through COALESCE like every other query does.
	rows, e := db.QueryContext(ctx, "SELECT COALESCE(m.visibility_preset_id,''),COALESCE(m.secret,0),COALESCE(d.visibility_preset_id,''),COALESCE(d.secret,0) FROM milestones m JOIN diary_entries d ON m.target_type='diary' AND m.target_id=d.id WHERE d.entry_date=?", date)
	if e != nil {
		return 0, e
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var preset, milestonePreset string
		var secret, targetSecret int
		if e = rows.Scan(&milestonePreset, &secret, &preset, &targetSecret); e != nil {
			return 0, e
		}
		ok, err := s.acl.CanRead(ctx, actor, acl.Resource{LifeID: actor.LifeID, Date: date, PresetID: preset, Secret: targetSecret == 1})
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		ok, err = s.acl.CanRead(ctx, actor, acl.Resource{LifeID: actor.LifeID, Date: date, PresetID: milestonePreset, Secret: secret == 1})
		if err != nil {
			return 0, err
		}
		if ok {
			count++
		}
	}
	return count, rows.Err()
}
func (s *Service) averageMood(ctx context.Context, actor auth.Actor, date time.Time) *float64 {
	db, e := s.db(ctx, actor.LifeID, date)
	if e != nil {
		return nil
	}
	defer db.Close()
	rows, e := db.QueryContext(ctx, "SELECT value,COALESCE(secret,0) FROM mood_records WHERE recorded_date=?", date.Format("2006-01-02"))
	if e != nil {
		return nil
	}
	defer rows.Close()
	sum := 0.0
	count := 0
	for rows.Next() {
		var value float64
		var secret int
		if rows.Scan(&value, &secret) != nil {
			continue
		}
		ok, _ := s.acl.CanRead(ctx, actor, acl.Resource{LifeID: actor.LifeID, Date: date.Format("2006-01-02"), Secret: secret == 1})
		if ok {
			sum += value
			count++
		}
	}
	if count == 0 {
		return nil
	}
	value := sum / float64(count)
	return &value
}
func (s *Service) averageBody(ctx context.Context, actor auth.Actor, date time.Time) *float64 {
	db, e := s.db(ctx, actor.LifeID, date)
	if e != nil {
		return nil
	}
	defer db.Close()
	rows, e := db.QueryContext(ctx, "SELECT score,COALESCE(secret,0) FROM body_records WHERE recorded_date=?", date.Format("2006-01-02"))
	if e != nil {
		return nil
	}
	defer rows.Close()
	sum := 0.0
	count := 0
	for rows.Next() {
		var score float64
		var secret int
		if rows.Scan(&score, &secret) != nil {
			continue
		}
		ok, _ := s.acl.CanRead(ctx, actor, acl.Resource{LifeID: actor.LifeID, Date: date.Format("2006-01-02"), Secret: secret == 1})
		if ok {
			sum += score
			count++
		}
	}
	if count == 0 {
		return nil
	}
	value := sum / float64(count)
	return &value
}
func (s *Service) db(ctx context.Context, life string, date time.Time) (*sql.DB, error) {
	if e := s.store.EnsureLifeMonth(ctx, life, date); e != nil {
		return nil, e
	}
	return sql.Open("sqlite", "file:"+s.store.LifeDBPath(life, date.In(shanghai()).Format("2006-01"))+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
}
func shanghai() *time.Location {
	location, e := time.LoadLocation("Asia/Shanghai")
	if e != nil {
		return time.FixedZone("CST", 28800)
	}
	return location
}
func dayStart(value time.Time, location *time.Location) time.Time {
	v := value.In(location)
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, location)
}
