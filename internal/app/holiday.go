package app

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"sort"
	"sync"
	"time"
)

//go:embed data/holidays-*.json
var bundledHolidayFiles embed.FS

type holidayCacheEntry struct {
	calendar *HolidayCalendar
	err      error
}

var holidayCache sync.Map

type HolidayPeriod struct {
	Name  string
	Start time.Time
	End   time.Time
}

type HolidayCalendar struct {
	Year     int
	Periods  []HolidayPeriod
	Workdays map[string]bool
	Source   string
}

type HolidaySnapshot struct {
	Name              string
	Start             time.Time
	End               time.Time
	DaysUntil         int
	PreviousName      string
	PreviousEnd       time.Time
	DaysSincePrevious int
	IntervalProgress  float64
	HasPrevious       bool
}

func (snapshot HolidaySnapshot) IsActive() bool {
	return snapshot.DaysUntil == 0
}

func isWorkingDate(day time.Time, config Config, calendar *HolidayCalendar) bool {
	day = normalizedDate(day)
	if calendar != nil {
		key := day.Format("2006-01-02")
		if calendar.Workdays[key] {
			return true
		}
		for _, period := range calendar.Periods {
			if !day.Before(period.Start) && !day.After(period.End) {
				return false
			}
		}
	}
	return config.Workdays[day.Weekday()]
}

type holidayJSON struct {
	Year    int    `json:"year"`
	Source  string `json:"source"`
	Periods []struct {
		Name  string `json:"name"`
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"periods"`
	Workdays []string `json:"workdays"`
}

func ParseHolidayCalendar(data []byte, expectedYear int) (HolidayCalendar, error) {
	var raw holidayJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return HolidayCalendar{}, fmt.Errorf("解析节假日数据: %w", err)
	}
	if raw.Year != expectedYear {
		return HolidayCalendar{}, fmt.Errorf("节假日年份为 %d，预期 %d", raw.Year, expectedYear)
	}
	calendar := HolidayCalendar{Year: raw.Year, Source: raw.Source, Workdays: make(map[string]bool)}
	for _, item := range raw.Periods {
		start, err := time.Parse("2006-01-02", item.Start)
		if err != nil {
			return HolidayCalendar{}, fmt.Errorf("节假日 %q 开始日期: %w", item.Name, err)
		}
		end, err := time.Parse("2006-01-02", item.End)
		if err != nil {
			return HolidayCalendar{}, fmt.Errorf("节假日 %q 结束日期: %w", item.Name, err)
		}
		if item.Name == "" || start.Year() != expectedYear || end.Year() != expectedYear || end.Before(start) {
			return HolidayCalendar{}, fmt.Errorf("节假日 %q 日期范围无效", item.Name)
		}
		calendar.Periods = append(calendar.Periods, HolidayPeriod{item.Name, start, end})
	}
	for _, value := range raw.Workdays {
		day, err := time.Parse("2006-01-02", value)
		if err != nil || day.Year() != expectedYear {
			return HolidayCalendar{}, fmt.Errorf("调休工作日 %q 无效", value)
		}
		calendar.Workdays[value] = true
	}
	sort.Slice(calendar.Periods, func(i, j int) bool { return calendar.Periods[i].Start.Before(calendar.Periods[j].Start) })
	return calendar, nil
}

func LoadHolidayCalendar(year int) (*HolidayCalendar, error) {
	if cached, ok := holidayCache.Load(year); ok {
		entry := cached.(holidayCacheEntry)
		return entry.calendar, entry.err
	}
	data, err := bundledHolidayFiles.ReadFile(fmt.Sprintf("data/holidays-%d.json", year))
	if errors.Is(err, fs.ErrNotExist) {
		entry := holidayCacheEntry{}
		holidayCache.LoadOrStore(year, entry)
		return nil, nil
	}
	if err != nil {
		entry := holidayCacheEntry{err: fmt.Errorf("读取 %d 年节假日数据：%w", year, err)}
		actual, _ := holidayCache.LoadOrStore(year, entry)
		stored := actual.(holidayCacheEntry)
		return stored.calendar, stored.err
	}
	calendar, err := ParseHolidayCalendar(data, year)
	entry := holidayCacheEntry{err: err}
	if err == nil {
		entry.calendar = &calendar
	}
	actual, _ := holidayCache.LoadOrStore(year, entry)
	stored := actual.(holidayCacheEntry)
	return stored.calendar, stored.err
}

func NextHoliday(today time.Time) (*HolidaySnapshot, error) {
	today = normalizedDate(today)
	periods := make([]HolidayPeriod, 0)
	for year := today.Year() - 1; year <= today.Year()+1; year++ {
		calendar, err := LoadHolidayCalendar(year)
		if err != nil {
			return nil, err
		}
		if calendar != nil {
			periods = append(periods, calendar.Periods...)
		}
	}
	sort.Slice(periods, func(i, j int) bool { return periods[i].Start.Before(periods[j].Start) })
	var previous *HolidayPeriod
	for i := range periods {
		period := periods[i]
		if period.End.Before(today) {
			copy := period
			previous = &copy
			continue
		}
		daysUntil := max(0, daysBetween(today, period.Start))
		snapshot := &HolidaySnapshot{Name: period.Name, Start: period.Start, End: period.End, DaysUntil: daysUntil}
		if previous != nil {
			snapshot.HasPrevious = true
			snapshot.PreviousName = previous.Name
			snapshot.PreviousEnd = previous.End
			snapshot.DaysSincePrevious = daysBetween(previous.End, today)
			intervalDays := daysBetween(previous.End, period.Start)
			if intervalDays > 0 {
				snapshot.IntervalProgress = math.Max(0, math.Min(1, float64(snapshot.DaysSincePrevious)/float64(intervalDays)))
			}
		}
		return snapshot, nil
	}
	return nil, nil
}
