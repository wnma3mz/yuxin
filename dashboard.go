package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

//go:embed data/holidays-2026.json
var bundledHoliday2026 []byte

var (
	holiday2026Once     sync.Once
	holiday2026Calendar HolidayCalendar
	holiday2026Error    error
)

type SalarySnapshot struct {
	EarnedToday      float64
	ExpectedToday    float64
	DailyRate        float64
	HourlyRate       float64
	Progress         float64
	ElapsedSeconds   int
	RemainingSeconds int
	Status           string
}

type RetirementSnapshot struct {
	RetirementMonth time.Time
	RemainingDays   int
	DelayedMonths   int
	StatutoryAge    string
	IsEstimate      bool
	Progress        float64
}

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

type DashboardSnapshot struct {
	Now                  time.Time
	Salary               SalarySnapshot
	Retirement           RetirementSnapshot
	RetirementEnabled    bool
	AssetsEnabled        bool
	TotalAssets          float64
	LiveBalance          float64
	SpendableAssets      float64
	DailyUntilRetirement float64
	RemainingWorkdays    int
	RemainingSalary      float64
	TodayCoversDays      float64
	Holiday              *HolidaySnapshot
	HolidayDataAvailable bool
}

func CalculateSalary(now time.Time, config Config) SalarySnapshot {
	calendar, _ := LoadHolidayCalendar(now.Year())
	if !isWorkingDate(now, config, calendar) {
		return SalarySnapshot{HourlyRate: hourlyRateForRestDay(config), Status: "rest-day"}
	}

	spans := workSpans(config)
	totalSeconds := 0
	elapsedSeconds := 0
	nowSecond := now.Hour()*3600 + now.Minute()*60 + now.Second()
	for _, span := range spans {
		totalSeconds += span.end - span.start
		elapsedSeconds += clamp(nowSecond-span.start, 0, span.end-span.start)
	}

	rate := dailyRate(config, totalSeconds)
	hourly := 0.0
	if totalSeconds > 0 {
		hourly = rate * 3600 / float64(totalSeconds)
	}
	progress := 0.0
	if totalSeconds > 0 {
		progress = float64(elapsedSeconds) / float64(totalSeconds)
	}
	status := "lunch-break"
	if nowSecond < spans[0].start {
		status = "before-work"
	} else if nowSecond >= spans[len(spans)-1].end {
		status = "after-work"
	} else {
		for _, span := range spans {
			if nowSecond >= span.start && nowSecond < span.end {
				status = "working"
				break
			}
		}
	}
	return SalarySnapshot{
		EarnedToday:      rate * progress,
		ExpectedToday:    rate,
		DailyRate:        rate,
		HourlyRate:       hourly,
		Progress:         progress,
		ElapsedSeconds:   elapsedSeconds,
		RemainingSeconds: max(0, spans[len(spans)-1].end-nowSecond),
		Status:           status,
	}
}

type secondSpan struct{ start, end int }

func workSpans(config Config) []secondSpan {
	if config.LunchEnabled {
		return []secondSpan{{config.StartSecond, config.LunchStart}, {config.LunchEnd, config.EndSecond}}
	}
	return []secondSpan{{config.StartSecond, config.EndSecond}}
}

func effectiveWorkSeconds(config Config) int {
	total := config.EndSecond - config.StartSecond
	if config.LunchEnabled {
		total -= config.LunchEnd - config.LunchStart
	}
	return max(0, total)
}

func dailyRate(config Config, workSeconds int) float64 {
	switch config.SalaryMode {
	case "monthly":
		return config.SalaryAmount / config.MonthlyWorkdays
	case "daily":
		return config.SalaryAmount
	case "hourly":
		return config.SalaryAmount * float64(workSeconds) / 3600
	default:
		return 0
	}
}

func hourlyRateForRestDay(config Config) float64 {
	if config.SalaryMode == "hourly" {
		return config.SalaryAmount
	}
	return 0
}

func CalculateRetirement(config Config, today time.Time) (RetirementSnapshot, error) {
	baseAge, delay, err := retirementDelay(config)
	if err != nil {
		return RetirementSnapshot{}, err
	}
	birth := normalizedDate(config.BirthDate)
	retirement := addMonths(time.Date(birth.Year()+baseAge, birth.Month(), 1, 0, 0, 0, 0, time.UTC), delay)
	ageMonths := baseAge*12 + delay
	age := fmt.Sprintf("%d 岁", ageMonths/12)
	if ageMonths%12 != 0 {
		age += fmt.Sprintf(" %d 个月", ageMonths%12)
	}
	today = normalizedDate(today)
	remaining := max(0, daysBetween(today, retirement))
	totalDays := max(1, daysBetween(birth, retirement))
	progress := math.Max(0, math.Min(1, float64(daysBetween(birth, today))/float64(totalDays)))
	return RetirementSnapshot{
		RetirementMonth: retirement,
		RemainingDays:   remaining,
		DelayedMonths:   delay,
		StatutoryAge:    age,
		Progress:        progress,
	}, nil
}

func CalculateDefaultRetirement(config Config, today time.Time) RetirementSnapshot {
	start := normalizedDate(config.RetirementStart)
	retirement := time.Date(start.Year()+config.RetirementYears, start.Month(), 1, 0, 0, 0, 0, time.UTC)
	progressStart := start
	if !config.ProgressBirthDate.IsZero() {
		progressStart = normalizedDate(config.ProgressBirthDate)
	}
	today = normalizedDate(today)
	totalDays := max(1, daysBetween(progressStart, retirement))
	progress := math.Max(0, math.Min(1, float64(daysBetween(progressStart, today))/float64(totalDays)))
	return RetirementSnapshot{
		RetirementMonth: retirement,
		RemainingDays:   max(0, daysBetween(today, retirement)),
		StatutoryAge:    fmt.Sprintf("距离现在 %d 年", config.RetirementYears),
		IsEstimate:      true,
		Progress:        progress,
	}
}

func retirementDelay(config Config) (baseAge, delay int, err error) {
	var firstYear int
	var firstMonth time.Month
	var step, cap int
	switch {
	case config.Sex == "male":
		baseAge, firstYear, firstMonth, step, cap = 60, 1965, time.January, 4, 36
	case config.Sex == "female" && config.FemaleTrack == "55":
		baseAge, firstYear, firstMonth, step, cap = 55, 1970, time.January, 4, 36
	case config.Sex == "female" && config.FemaleTrack == "50":
		baseAge, firstYear, firstMonth, step, cap = 50, 1975, time.January, 2, 60
	default:
		return 0, 0, fmt.Errorf("女性退休计算需要选择原法定退休年龄 50 岁或 55 岁")
	}
	birthIndex := config.BirthDate.Year()*12 + int(config.BirthDate.Month()) - 1
	startIndex := firstYear*12 + int(firstMonth) - 1
	if birthIndex >= startIndex {
		delay = min((birthIndex-startIndex)/step+1, cap)
	}
	return baseAge, delay, nil
}

func CountWorkdays(start, end time.Time, workdays map[time.Weekday]bool) int {
	start, end = normalizedDate(start), normalizedDate(end)
	count := 0
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		if workdays[day.Weekday()] {
			count++
		}
	}
	return count
}

func CountConfiguredWorkdays(start, end time.Time, config Config) int {
	start, end = normalizedDate(start), normalizedDate(end)
	count := CountWorkdays(start, end, config.Workdays)
	for year := start.Year(); year <= end.Year(); year++ {
		calendar, _ := LoadHolidayCalendar(year)
		if calendar == nil {
			continue
		}
		overrides := make(map[string]bool)
		for _, period := range calendar.Periods {
			for day := period.Start; !day.After(period.End); day = day.AddDate(0, 0, 1) {
				overrides[day.Format("2006-01-02")] = false
			}
		}
		for day := range calendar.Workdays {
			overrides[day] = true
		}
		for value, actualWorkday := range overrides {
			day, err := time.Parse("2006-01-02", value)
			if err != nil || day.Before(start) || !day.Before(end) {
				continue
			}
			scheduledWorkday := config.Workdays[day.Weekday()]
			if actualWorkday && !scheduledWorkday {
				count++
			} else if !actualWorkday && scheduledWorkday {
				count--
			}
		}
	}
	return count
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
		if item.Name == "" || start.Year() != expectedYear || end.Before(start) {
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
	if year != 2026 {
		return nil, nil
	}
	holiday2026Once.Do(func() {
		holiday2026Calendar, holiday2026Error = ParseHolidayCalendar(bundledHoliday2026, year)
	})
	if holiday2026Error != nil {
		return nil, holiday2026Error
	}
	return &holiday2026Calendar, nil
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

func CalculateDashboard(now time.Time, config Config) (DashboardSnapshot, error) {
	salary := CalculateSalary(now, config)
	var retirement RetirementSnapshot
	var err error
	if config.ProfileEnabled {
		retirement, err = CalculateRetirement(config, now)
		if err != nil {
			return DashboardSnapshot{}, err
		}
	} else if config.RetirementYears > 0 {
		retirement = CalculateDefaultRetirement(config, now)
	}
	holiday, err := NextHoliday(now)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	calendar, err := LoadHolidayCalendar(now.Year())
	if err != nil {
		return DashboardSnapshot{}, err
	}
	totalAssets := 0.0
	liveBalance := 0.0
	spendable := 0.0
	if config.AssetsEnabled {
		totalAssets = config.Assets
		liveBalance = totalAssets + salary.EarnedToday
		spendable = math.Max(0, liveBalance-config.Reserve)
	}
	remainingWorkdays := 0
	remainingSalary := 0.0
	dailyBudget := 0.0
	covers := 0.0
	retirementEnabled := !retirement.RetirementMonth.IsZero()
	if retirementEnabled {
		remainingWorkdays = CountConfiguredWorkdays(normalizedDate(now).AddDate(0, 0, 1), retirement.RetirementMonth, config)
		remainingSalary = dailyRate(config, effectiveWorkSeconds(config)) * float64(remainingWorkdays)
	}
	if config.AssetsEnabled && retirementEnabled && retirement.RemainingDays > 0 {
		dailyBudget = spendable / float64(retirement.RemainingDays)
		if dailyBudget > 0 {
			covers = salary.ExpectedToday / dailyBudget
		}
	}
	return DashboardSnapshot{
		Now: now, Salary: salary, Retirement: retirement,
		RetirementEnabled: retirementEnabled, AssetsEnabled: config.AssetsEnabled,
		TotalAssets: totalAssets, LiveBalance: liveBalance, SpendableAssets: spendable,
		DailyUntilRetirement: dailyBudget, RemainingWorkdays: remainingWorkdays,
		RemainingSalary: remainingSalary, TodayCoversDays: covers,
		Holiday: holiday, HolidayDataAvailable: calendar != nil,
	}, nil
}

func normalizedDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func daysBetween(start, end time.Time) int {
	return int(normalizedDate(end).Sub(normalizedDate(start)).Hours() / 24)
}

func addMonths(value time.Time, months int) time.Time {
	index := value.Year()*12 + int(value.Month()) - 1 + months
	return time.Date(index/12, time.Month(index%12+1), 1, 0, 0, 0, 0, time.UTC)
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
