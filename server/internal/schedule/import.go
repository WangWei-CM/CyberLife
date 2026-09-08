package schedule

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type CSVMapping struct {
	Title              string `json:"title"`
	Weekday            string `json:"weekday"`
	SessionDate        string `json:"sessionDate"`
	StartTime          string `json:"startTime"`
	EndTime            string `json:"endTime"`
	Location           string `json:"location"`
	Note               string `json:"note"`
	EffectiveStartDate string `json:"effectiveStartDate"`
	EffectiveEndDate   string `json:"effectiveEndDate"`
}
type ImportPreview struct {
	Format          string     `json:"format"`
	DetectedMapping CSVMapping `json:"detectedMapping"`
	Columns         []string   `json:"columns"`
	Items           []Class    `json:"items"`
	Warnings        []string   `json:"warnings"`
}

func Preview(name string, data []byte, mapping CSVMapping) (ImportPreview, error) {
	if strings.HasSuffix(strings.ToLower(name), ".csv") {
		return previewCSV(data, mapping)
	}
	if strings.HasSuffix(strings.ToLower(name), ".ics") || strings.HasSuffix(strings.ToLower(name), ".ical") {
		return previewICS(data)
	}
	return ImportPreview{}, fmt.Errorf("仅支持 ICS 或 CSV 文件")
}
func previewCSV(data []byte, m CSVMapping) (ImportPreview, error) {
	r := csv.NewReader(bytes.NewReader(data))
	headers, err := r.Read()
	if err != nil {
		return ImportPreview{}, fmt.Errorf("CSV 表头无效")
	}
	p := ImportPreview{Format: "csv", Columns: headers}
	p.DetectedMapping = detect(headers, m)
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			p.Warnings = append(p.Warnings, "存在无法读取的行")
			continue
		}
		x, e := classFromCSV(headers, row, p.DetectedMapping)
		if e != nil {
			p.Warnings = append(p.Warnings, e.Error())
			continue
		}
		p.Items = append(p.Items, x)
	}
	if len(p.Items) == 0 {
		return p, fmt.Errorf("没有可导入的有效课程")
	}
	return p, nil
}
func detect(h []string, m CSVMapping) CSVMapping {
	if m.Title != "" {
		return m
	}
	find := func(keys ...string) string {
		for _, x := range h {
			lx := strings.ToLower(strings.TrimSpace(x))
			for _, k := range keys {
				if lx == k || strings.Contains(lx, k) {
					return x
				}
			}
		}
		return ""
	}
	return CSVMapping{Title: find("title", "name", "课程名", "课程名称"), Weekday: find("weekday", "day", "星期", "周几"), SessionDate: find("sessiondate", "date", "日期"), StartTime: find("start_time", "start", "开始"), EndTime: find("end_time", "end", "结束"), Location: find("location", "room", "教室", "地点"), Note: find("note", "备注")}
}
func value(h []string, row []string, col string) string {
	for i, x := range h {
		if x == col && i < len(row) {
			return strings.TrimSpace(row[i])
		}
	}
	return ""
}
func classFromCSV(h, row []string, m CSVMapping) (Class, error) {
	x := Class{Title: value(h, row, m.Title), SessionDate: value(h, row, m.SessionDate), StartTime: value(h, row, m.StartTime), EndTime: value(h, row, m.EndTime), Location: value(h, row, m.Location), Note: value(h, row, m.Note), EffectiveStartDate: value(h, row, m.EffectiveStartDate), EffectiveEndDate: value(h, row, m.EffectiveEndDate), Source: "csv"}
	if x.SessionDate == "" {
		wd := strings.TrimSpace(value(h, row, m.Weekday))
		n, err := strconv.Atoi(strings.TrimPrefix(wd, "周"))
		if err != nil {
			switch wd {
			case "一", "星期一":
				n = 1
			case "二", "星期二":
				n = 2
			case "三", "星期三":
				n = 3
			case "四", "星期四":
				n = 4
			case "五", "星期五":
				n = 5
			case "六", "星期六":
				n = 6
			case "日", "天", "星期日", "星期天":
				n = 7
			default:
				return x, fmt.Errorf("课程 %q 的星期无效", x.Title)
			}
		}
		x.Weekday = &n
	}
	if err := validateClass(x); err != nil {
		return x, err
	}
	return x, nil
}

func previewICS(data []byte) (ImportPreview, error) {
	p := ImportPreview{Format: "ics"}
	var cur map[string]string
	flush := func() {
		if cur == nil {
			return
		}
		x, e := icsClass(cur)
		if e != nil {
			p.Warnings = append(p.Warnings, e.Error())
		} else {
			p.Items = append(p.Items, expandWeekly(x, cur["RRULE"])...)
		}
		cur = nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "BEGIN:VEVENT" {
			cur = map[string]string{}
			continue
		}
		if line == "END:VEVENT" {
			flush()
			continue
		}
		if cur == nil {
			continue
		}
		if i := strings.Index(line, ":"); i > 0 {
			key := strings.ToUpper(strings.Split(line[:i], ";")[0])
			cur[key] = line[i+1:]
		}
	}
	flush()
	if len(p.Items) == 0 {
		return p, fmt.Errorf("ICS 中没有可导入的有效课程")
	}
	return p, nil
}

func expandWeekly(x Class, rule string) []Class {
	parts := regexp.MustCompile(`(?:^|;)BYDAY=([^;]+)`).FindStringSubmatch(strings.ToUpper(rule))
	if len(parts) < 2 {
		return []Class{x}
	}
	mapDay := map[string]int{"MO": 1, "TU": 2, "WE": 3, "TH": 4, "FR": 5, "SA": 6, "SU": 7}
	out := []Class{}
	for _, token := range strings.Split(parts[1], ",") {
		if n, ok := mapDay[strings.TrimSpace(token)]; ok {
			y := x
			y.Weekday = &n
			out = append(out, y)
		}
	}
	if len(out) == 0 {
		return []Class{x}
	}
	return out
}
func icsClass(v map[string]string) (Class, error) {
	title := strings.TrimSpace(v["SUMMARY"])
	start := v["DTSTART"]
	end := v["DTEND"]
	if title == "" || start == "" || end == "" {
		return Class{}, fmt.Errorf("ICS 事件缺少标题或起止时间")
	}
	parse := func(s string) (time.Time, error) {
		s = strings.TrimSuffix(s, "Z")
		for _, layout := range []string{"20060102T150405", "20060102T1504", "20060102"} {
			if t, e := time.ParseInLocation(layout, s, shanghai); e == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("ICS 时间无效: %s", s)
	}
	st, e := parse(start)
	if e != nil {
		return Class{}, e
	}
	et, e := parse(end)
	if e != nil {
		return Class{}, e
	}
	x := Class{Title: title, StartTime: st.Format("15:04"), EndTime: et.Format("15:04"), Location: v["LOCATION"], Note: v["DESCRIPTION"], Source: "ics"}
	if strings.TrimSpace(v["RRULE"]) != "" {
		if !strings.Contains(strings.ToUpper(v["RRULE"]), "FREQ=WEEKLY") {
			return x, fmt.Errorf("课程 %q 使用了不支持的重复规则", title)
		}
		wd := int(st.Weekday())
		if wd == 0 {
			wd = 7
		}
		x.Weekday = &wd
		if u := regexp.MustCompile(`UNTIL=([0-9]{8})`).FindStringSubmatch(v["RRULE"]); len(u) > 1 {
			x.EffectiveEndDate = u[1][:4] + "-" + u[1][4:6] + "-" + u[1][6:]
		}
	} else {
		x.SessionDate = st.Format("2006-01-02")
	}
	return x, nil
}
