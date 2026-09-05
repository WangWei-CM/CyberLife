package history

import (
	"context"
	"fmt"
	"strconv"
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

type accumulator struct {
	moodSum   float64
	moodCount int
	bodySum   float64
	bodyCount int
}

// decision answers "can this actor read a resource on this date" with memoisation per request.
type decision func(date, preset string, secret bool) (bool, error)

// Range reads a date span by opening each month database once and querying whole date ranges,
// instead of opening a database per day. A year on the timeline is therefore twelve database opens.
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

	decisions := map[string]bool{}
	canRead := func(date, preset string, secret bool) (bool, error) {
		key := date + "|" + preset + "|" + strconv.FormatBool(secret)
		if value, ok := decisions[key]; ok {
			return value, nil
		}
		value, err := s.acl.CanRead(ctx, actor, acl.Resource{LifeID: actor.LifeID, Date: date, PresetID: preset, Secret: secret})
		if err != nil {
			return false, err
		}
		decisions[key] = value
		return value, nil
	}

	days := map[string]*Day{}
	sums := map[string]*accumulator{}
	order := []string{}
	for date := from; !date.After(to); date = date.AddDate(0, 0, 1) {
		key := date.Format("2006-01-02")
		days[key] = &Day{Date: key, Tasks: []nowservice.Task{}}
		sums[key] = &accumulator{}
		order = append(order, key)
	}
	for month := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, location); !month.After(to); month = month.AddDate(0, 1, 0) {
		low := from
		if month.After(low) {
			low = month
		}
		high := month.AddDate(0, 1, -1)
		if high.After(to) {
			high = to
		}
		if e := s.collectMonth(ctx, actor.LifeID, storage.MonthKey(month), low.Format("2006-01-02"), high.Format("2006-01-02"), days, sums, canRead); e != nil {
			return out, e
		}
	}
	for _, key := range order {
		out.Days = append(out.Days, *days[key])
		point := Point{Date: key}
		if sum := sums[key]; sum.moodCount > 0 {
			value := sum.moodSum / float64(sum.moodCount)
			point.Mood = &value
		}
		if sum := sums[key]; sum.bodyCount > 0 {
			value := sum.bodySum / float64(sum.bodyCount)
			point.Body = &value
		}
		out.Points = append(out.Points, point)
	}
	return out, nil
}

// collectMonth fills days/sums for the [low, high] slice of one month database.
func (s *Service) collectMonth(ctx context.Context, life, monthKey, low, high string, days map[string]*Day, sums map[string]*accumulator, canRead decision) error {
	db, e := s.store.OpenLifeMonth(ctx, life, monthKey)
	if e != nil {
		return e
	}
	defer db.Close()

	// Diaries: both layers; the secret layer only survives the ACL check for the writer.
	rows, e := db.QueryContext(ctx, "SELECT id,entry_date,content_md,COALESCE(visibility_preset_id,''),COALESCE(secret,0),COALESCE(commentable,0) FROM diary_entries WHERE entry_date BETWEEN ? AND ? ORDER BY entry_date", low, high)
	if e != nil {
		return e
	}
	diaries := []nowservice.Diary{}
	for rows.Next() {
		var d nowservice.Diary
		var secret, commentable int
		if e = rows.Scan(&d.ID, &d.EntryDate, &d.Content, &d.PresetID, &secret, &commentable); e != nil {
			rows.Close()
			return e
		}
		d.Secret = secret == 1
		d.Commentable = commentable == 1
		diaries = append(diaries, d)
	}
	rows.Close()
	for _, d := range diaries {
		day := days[d.EntryDate]
		if day == nil {
			continue
		}
		ok, err := canRead(d.EntryDate, d.PresetID, d.Secret)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if d.Secret {
			layer := d
			day.SecretDiary = &layer
		} else {
			day.Diary = d
		}
	}

	// Tasks.
	rows, e = db.QueryContext(ctx, "SELECT id,task_date,title,description,priority,done,COALESCE(visibility_preset_id,''),COALESCE(secret,0),COALESCE(commentable,0) FROM tasks WHERE task_date BETWEEN ? AND ? ORDER BY task_date,done,created_at", low, high)
	if e != nil {
		return e
	}
	tasks := []nowservice.Task{}
	for rows.Next() {
		var t nowservice.Task
		var done, secret, commentable int
		if e = rows.Scan(&t.ID, &t.TaskDate, &t.Title, &t.Description, &t.Priority, &done, &t.PresetID, &secret, &commentable); e != nil {
			rows.Close()
			return e
		}
		t.Done = done == 1
		t.Secret = secret == 1
		t.Commentable = commentable == 1
		tasks = append(tasks, t)
	}
	rows.Close()
	for _, t := range tasks {
		day := days[t.TaskDate]
		if day == nil {
			continue
		}
		ok, err := canRead(t.TaskDate, t.PresetID, t.Secret)
		if err != nil {
			return err
		}
		if ok {
			day.Tasks = append(day.Tasks, t)
		}
	}

	// Mood and body samples: averages per day of the records the actor may read.
	type sample struct {
		date   string
		value  float64
		secret bool
	}
	readSamples := func(query string) ([]sample, error) {
		rows, e := db.QueryContext(ctx, query, low, high)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		samples := []sample{}
		for rows.Next() {
			var x sample
			var secret int
			if e = rows.Scan(&x.date, &x.value, &secret); e != nil {
				return nil, e
			}
			x.secret = secret == 1
			samples = append(samples, x)
		}
		return samples, rows.Err()
	}
	moods, e := readSamples("SELECT recorded_date,value,COALESCE(secret,0) FROM mood_records WHERE recorded_date BETWEEN ? AND ?")
	if e != nil {
		return e
	}
	for _, m := range moods {
		day, sum := days[m.date], sums[m.date]
		if day == nil || sum == nil {
			continue
		}
		ok, err := canRead(m.date, "", m.secret)
		if err != nil {
			return err
		}
		if ok {
			sum.moodSum += m.value
			sum.moodCount++
			day.MoodCount++
		}
	}
	bodies, e := readSamples("SELECT recorded_date,CAST(score AS REAL),COALESCE(secret,0) FROM body_records WHERE recorded_date BETWEEN ? AND ?")
	if e != nil {
		return e
	}
	for _, b := range bodies {
		day, sum := days[b.date], sums[b.date]
		if day == nil || sum == nil {
			continue
		}
		ok, err := canRead(b.date, "", b.secret)
		if err != nil {
			return err
		}
		if ok {
			sum.bodySum += b.value
			sum.bodyCount++
			day.BodyCount++
		}
	}

	// Milestones on diaries: both the diary and the milestone must be readable.
	rows, e = db.QueryContext(ctx, "SELECT d.entry_date,COALESCE(m.visibility_preset_id,''),COALESCE(m.secret,0),COALESCE(d.visibility_preset_id,''),COALESCE(d.secret,0) FROM milestones m JOIN diary_entries d ON m.target_type='diary' AND m.target_id=d.id WHERE d.entry_date BETWEEN ? AND ?", low, high)
	if e != nil {
		return e
	}
	type medal struct {
		date, preset, targetPreset string
		secret, targetSecret       bool
	}
	medals := []medal{}
	for rows.Next() {
		var x medal
		var secret, targetSecret int
		if e = rows.Scan(&x.date, &x.preset, &secret, &x.targetPreset, &targetSecret); e != nil {
			rows.Close()
			return e
		}
		x.secret = secret == 1
		x.targetSecret = targetSecret == 1
		medals = append(medals, x)
	}
	rows.Close()
	for _, m := range medals {
		day := days[m.date]
		if day == nil {
			continue
		}
		ok, err := canRead(m.date, m.targetPreset, m.targetSecret)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		ok, err = canRead(m.date, m.preset, m.secret)
		if err != nil {
			return err
		}
		if ok {
			day.MilestoneCount++
		}
	}
	return nil
}

func dayStart(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}
func shanghai() *time.Location {
	location, e := time.LoadLocation("Asia/Shanghai")
	if e != nil {
		return time.FixedZone("CST", 28800)
	}
	return location
}
