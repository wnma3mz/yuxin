package app

import (
	"math"
	"time"
)

const (
	averageDaysPerMonth = 30.436875
	averageDaysPerYear  = 365.2425
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

type DashboardSnapshot struct {
	Now                  time.Time
	DemoMode             bool
	Salary               SalarySnapshot
	Retirement           RetirementSnapshot
	RetirementEnabled    bool
	AssetsEnabled        bool
	TotalAssets          float64
	LiveBalance          float64
	SpendableAssets      float64
	DailyUntilRetirement float64
	SavingsTarget        float64
	SavingsGap           float64
	SavingsProgress      float64
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
	remainingSeconds := max(0, spans[len(spans)-1].end-nowSecond)
	if status == "before-work" {
		remainingSeconds = max(0, spans[0].start-nowSecond)
	}
	return SalarySnapshot{
		EarnedToday:      rate * progress,
		ExpectedToday:    rate,
		DailyRate:        rate,
		HourlyRate:       hourly,
		Progress:         progress,
		ElapsedSeconds:   elapsedSeconds,
		RemainingSeconds: remainingSeconds,
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

// DemoDashboard returns a fixed synthetic dashboard for privacy-safe screenshots.
// It never reads or derives values from the user's configuration.
func DemoDashboard() (DashboardSnapshot, Config, error) {
	config := defaultConfig()
	config.SalaryAmount = 16800
	config.MonthlyWorkdays = 21.75
	config.RetirementYears = 30
	config.ProfileEnabled = false
	config.RetirementStart = mustDate("2026-07-16")
	config.ProgressBirthDate = mustDate("1996-07-16")
	config.RetirementMode = "countdown"
	config.AssetsEnabled = true
	config.Assets = 258000
	config.TargetMonthlySpend = 3000
	config.AssetItems = []AssetItem{
		{Name: "演示存款", Kind: "deposit", Balance: 258000},
	}
	config.Reserve = 0
	now := time.Date(2026, time.July, 16, 15, 24, 0, 0, time.Local)
	snapshot, err := CalculateDashboard(now, config)
	if err != nil {
		return DashboardSnapshot{}, Config{}, err
	}
	snapshot.DemoMode = true
	return snapshot, config, nil
}

func CountWorkdays(start, end time.Time, workdays map[time.Weekday]bool) int {
	start, end = normalizedDate(start), normalizedDate(end)
	days := daysBetween(start, end)
	if days <= 0 {
		return 0
	}
	workdaysPerWeek := 0
	for _, enabled := range workdays {
		if enabled {
			workdaysPerWeek++
		}
	}
	count := days / 7 * workdaysPerWeek
	for offset := 0; offset < days%7; offset++ {
		if workdays[start.AddDate(0, 0, offset).Weekday()] {
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
	savingsTarget := 0.0
	savingsGap := 0.0
	savingsProgress := 0.0
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
		if config.TargetMonthlySpend > 0 {
			savingsTarget = config.TargetMonthlySpend * float64(retirement.RemainingDays) / averageDaysPerMonth
			if savingsTarget > 0 {
				savingsGap = math.Max(0, savingsTarget-spendable)
				savingsProgress = math.Max(0, math.Min(1, spendable/savingsTarget))
			}
		}
	}
	return DashboardSnapshot{
		Now: now, Salary: salary, Retirement: retirement,
		RetirementEnabled: retirementEnabled, AssetsEnabled: config.AssetsEnabled,
		TotalAssets: totalAssets, LiveBalance: liveBalance, SpendableAssets: spendable,
		DailyUntilRetirement: dailyBudget, RemainingWorkdays: remainingWorkdays,
		SavingsTarget: savingsTarget, SavingsGap: savingsGap, SavingsProgress: savingsProgress,
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

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
