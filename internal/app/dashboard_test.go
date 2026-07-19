package app

import (
	"math"
	"testing"
	"time"
)

func testDate(value string) time.Time {
	parsed, _ := time.Parse("2006-01-02 15:04:05", value)
	return parsed
}

func TestSalaryUsesEffectiveWorkTime(t *testing.T) {
	config := defaultConfig()
	config.SalaryAmount = 22000
	result := CalculateSalary(testDate("2026-07-16 11:00:00"), config)
	if result.Status != "working" || result.EarnedToday != 250 || result.ExpectedToday != 1000 || result.Progress != .25 {
		t.Fatalf("unexpected salary snapshot: %+v", result)
	}
	lunch := CalculateSalary(testDate("2026-07-16 12:30:00"), config)
	if lunch.Status != "lunch-break" || lunch.EarnedToday != 375 {
		t.Fatalf("lunch should freeze earnings: %+v", lunch)
	}
	before := CalculateSalary(testDate("2026-07-16 08:00:00"), config)
	if before.Status != "before-work" || before.RemainingSeconds != 3600 {
		t.Fatalf("before-work countdown = %+v", before)
	}
	after := CalculateSalary(testDate("2026-07-16 19:00:00"), config)
	if after.Status != "after-work" || after.RemainingSeconds != 0 {
		t.Fatalf("after-work countdown = %+v", after)
	}
}

func TestSalaryModesAndHolidayState(t *testing.T) {
	config := testFullConfig()
	if got := dailyRate(config, 8*3600); got != config.SalaryAmount/config.MonthlyWorkdays {
		t.Fatalf("monthly daily rate = %v", got)
	}
	config.SalaryMode = "daily"
	if got := dailyRate(config, 8*3600); got != config.SalaryAmount {
		t.Fatalf("daily rate = %v", got)
	}
	config.SalaryMode = "hourly"
	if got := dailyRate(config, 8*3600); got != config.SalaryAmount*8 {
		t.Fatalf("hourly daily rate = %v", got)
	}
	config.SalaryMode = "annual"
	if got := dailyRate(config, 8*3600); got != config.SalaryAmount/12/config.MonthlyWorkdays {
		t.Fatalf("annual daily rate = %v", got)
	}
	config.SalaryMode = "invalid"
	if got := dailyRate(config, 8*3600); got != 0 {
		t.Fatalf("invalid daily rate = %v", got)
	}
	if !(HolidaySnapshot{DaysUntil: 0}).IsActive() || (HolidaySnapshot{DaysUntil: 1}).IsActive() {
		t.Fatal("holiday active state is incorrect")
	}
}

func TestHolidayAndMakeupDayAffectSalary(t *testing.T) {
	config := defaultConfig()
	holiday := CalculateSalary(testDate("2026-10-01 11:00:00"), config)
	if holiday.Status != "rest-day" || holiday.EarnedToday != 0 {
		t.Fatalf("holiday salary = %+v", holiday)
	}
	makeup := CalculateSalary(testDate("2026-10-10 11:00:00"), config)
	if makeup.Status != "working" || makeup.EarnedToday <= 0 {
		t.Fatalf("makeup workday salary = %+v", makeup)
	}
}

func TestRetirementTracks(t *testing.T) {
	tests := []struct {
		name, birth, sex, track, wantMonth, wantAge string
		wantDelay                                   int
	}{
		{"male first", "1965-01-20", "male", "", "2025-02-01", "60 岁 1 个月", 1},
		{"male cap", "1990-01-01", "male", "", "2053-01-01", "63 岁", 36},
		{"female 55", "1970-01-01", "female", "55", "2025-02-01", "55 岁 1 个月", 1},
		{"female default", "1970-01-01", "female", "", "2025-02-01", "55 岁 1 个月", 1},
		{"female 50", "1975-02-01", "female", "50", "2025-03-01", "50 岁 1 个月", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := defaultConfig()
			config.BirthDate, _ = time.Parse("2006-01-02", tt.birth)
			config.Sex, config.FemaleTrack = tt.sex, tt.track
			result, err := CalculateRetirement(config, testDate("2024-01-01 00:00:00"))
			if err != nil {
				t.Fatal(err)
			}
			if result.RetirementMonth.Format("2006-01-02") != tt.wantMonth || result.StatutoryAge != tt.wantAge || result.DelayedMonths != tt.wantDelay {
				t.Fatalf("unexpected retirement: %+v", result)
			}
		})
	}
}

func TestRetirementProgressStartsAtAge18(t *testing.T) {
	config := defaultConfig()
	config.BirthDate, _ = time.Parse("2006-01-02", "1990-01-01")
	now := testDate("2026-01-01 00:00:00")
	result, err := CalculateRetirement(config, now)
	if err != nil {
		t.Fatal(err)
	}
	start := testDate("2008-01-01 00:00:00")
	want := float64(daysBetween(start, now)) / float64(daysBetween(start, result.RetirementMonth))
	if math.Abs(result.Progress-want) > 1e-12 {
		t.Fatalf("retirement progress = %v, want %v", result.Progress, want)
	}
}

func TestDemoDashboardUsesFixedSyntheticData(t *testing.T) {
	snapshot, config, err := DemoDashboard()
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.DemoMode || config.SalaryAmount != 16800 || config.Assets != 258000 || config.TargetMonthlySpend != 3000 || config.ProfileEnabled || config.RetirementYears != 30 {
		t.Fatalf("unexpected demo data: %+v, %+v", snapshot, config)
	}
	if len(config.AssetItems) != 1 || config.AssetItems[0].Name != "演示存款" {
		t.Fatalf("unexpected demo deposit: %#v", config.AssetItems)
	}
	if snapshot.Now.Format("2006-01-02 15:04") != "2026-07-16 15:24" {
		t.Fatalf("demo time = %s", snapshot.Now)
	}
	if snapshot.Retirement.Progress < .28 || snapshot.Retirement.Progress > .29 {
		t.Fatalf("demo retirement progress = %v", snapshot.Retirement.Progress)
	}
}

func TestBundledHolidayProgress(t *testing.T) {
	holiday, err := NextHoliday(testDate("2026-07-16 11:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	if holiday == nil || holiday.Name != "中秋节" || holiday.DaysUntil != 71 || holiday.PreviousName != "端午节" || holiday.DaysSincePrevious != 25 {
		t.Fatalf("unexpected holiday: %+v", holiday)
	}
	if math.Abs(holiday.IntervalProgress-25.0/96.0) > 1e-12 {
		t.Fatalf("unexpected interval progress: %v", holiday.IntervalProgress)
	}
}

func TestParseHolidayCalendarRejectsCrossYearPeriod(t *testing.T) {
	data := []byte(`{"year":2026,"source":"test","periods":[{"name":"跨年","start":"2026-12-31","end":"2027-01-01"}],"workdays":[]}`)
	if _, err := ParseHolidayCalendar(data, 2026); err == nil {
		t.Fatal("cross-year holiday period was accepted")
	}
}

func TestCountWorkdaysMatchesDailyReference(t *testing.T) {
	workdays := map[time.Weekday]bool{
		time.Monday: true, time.Tuesday: true, time.Wednesday: true,
		time.Thursday: true, time.Friday: true, time.Saturday: false,
	}
	for _, dates := range [][2]string{
		{"2026-07-13 00:00:00", "2026-07-13 00:00:00"},
		{"2026-07-13 00:00:00", "2026-07-14 00:00:00"},
		{"2026-07-16 00:00:00", "2026-07-27 00:00:00"},
		{"2026-07-18 00:00:00", "2026-07-21 00:00:00"},
		{"2026-07-21 00:00:00", "2026-07-18 00:00:00"},
		{"2026-01-01 00:00:00", "2066-01-01 00:00:00"},
	} {
		start, end := testDate(dates[0]), testDate(dates[1])
		want := 0
		for day := normalizedDate(start); day.Before(normalizedDate(end)); day = day.AddDate(0, 0, 1) {
			if workdays[day.Weekday()] {
				want++
			}
		}
		if got := CountWorkdays(start, end, workdays); got != want {
			t.Fatalf("CountWorkdays(%s, %s) = %d, want %d", dates[0], dates[1], got, want)
		}
	}
}

func BenchmarkCalculateDashboard(b *testing.B) {
	config := testFullConfig()
	now := testDate("2026-07-16 15:00:00")
	for i := 0; i < b.N; i++ {
		if _, err := CalculateDashboard(now, config); err != nil {
			b.Fatal(err)
		}
	}
}

func TestDashboardCombinesAssetsAndRemainingWork(t *testing.T) {
	config := testFullConfig()
	config.SalaryAmount = 22000
	config.Assets = 320000
	config.Reserve = 20000
	config.BirthDate, _ = time.Parse("2006-01-02", "1990-01-01")
	result, err := CalculateDashboard(testDate("2026-07-16 11:00:00"), config)
	if err != nil {
		t.Fatal(err)
	}
	if result.LiveBalance != 320250 || result.SpendableAssets != 300250 || result.DailyUntilRetirement <= 0 || result.RemainingWorkdays <= 0 || result.RemainingSalary <= 0 {
		t.Fatalf("unexpected dashboard: %+v", result)
	}
}

func TestDashboardAccumulatesSalaryAcrossWorkdays(t *testing.T) {
	config := testFullConfig()
	config.SalaryMode = "daily"
	config.SalaryAmount = 100
	config.Assets = 100000
	config.BalanceStartDate = testDate("2026-07-13 00:00:00")
	result, err := CalculateDashboard(testDate("2026-07-16 11:00:00"), config)
	if err != nil {
		t.Fatal(err)
	}
	// Monday through Wednesday are complete, and Thursday is 25% complete.
	if result.LiveBalance != 100325 {
		t.Fatalf("live balance = %.2f, want 100325", result.LiveBalance)
	}
}

func TestDashboardAccumulationHonorsHolidaysAndMakeupDays(t *testing.T) {
	config := testFullConfig()
	config.SalaryMode = "daily"
	config.SalaryAmount = 100
	config.Assets = 100000
	config.BalanceStartDate = testDate("2026-10-01 00:00:00")
	result, err := CalculateDashboard(testDate("2026-10-12 11:00:00"), config)
	if err != nil {
		t.Fatal(err)
	}
	// October 1-7 are holidays; October 8-9 and the October 10 makeup day are complete.
	if result.LiveBalance != 100325 {
		t.Fatalf("holiday-aware live balance = %.2f, want 100325", result.LiveBalance)
	}
}

func TestDashboardCalculatesSavingsTargetToRetirement(t *testing.T) {
	config := testFullConfig()
	config.Assets = 100000
	config.AssetItems = []AssetItem{{Name: "存款", Kind: "deposit", Balance: 100000}}
	config.TargetMonthlySpend = 3000
	result, err := CalculateDashboard(testDate("2026-07-16 00:00:00"), config)
	if err != nil {
		t.Fatal(err)
	}
	wantTarget := 3000 * float64(result.Retirement.RemainingDays) / averageDaysPerMonth
	if math.Abs(result.SavingsTarget-wantTarget) > 1e-9 || math.Abs(result.SavingsGap-(wantTarget-result.SpendableAssets)) > 1e-9 {
		t.Fatalf("savings target = target %.2f, gap %.2f, snapshot %+v", result.SavingsTarget, result.SavingsGap, result)
	}
	if result.SavingsProgress <= 0 || result.SavingsProgress >= 1 {
		t.Fatalf("savings progress = %v", result.SavingsProgress)
	}

	config.Assets = wantTarget + 1
	result, err = CalculateDashboard(testDate("2026-07-16 00:00:00"), config)
	if err != nil || result.SavingsGap != 0 || result.SavingsProgress != 1 {
		t.Fatalf("completed target = %+v, %v", result, err)
	}
}

func TestDashboardCalculatesWishTargetFromRealtimeSalary(t *testing.T) {
	config := testFullConfig()
	config.AssetsEnabled = false
	config.Assets = 0
	config.AssetItems = nil
	config.TargetMonthlySpend = 0
	config.WishName = "心仪的相机"
	config.WishAmount = 120000
	config.WishStartDate = testDate("2026-07-16 00:00:00")
	result, err := CalculateDashboard(testDate("2026-07-16 11:00:00"), config)
	if err != nil {
		t.Fatal(err)
	}
	if result.AssetsEnabled || result.WishTarget != 120000 || result.WishEarned != result.Salary.EarnedToday || math.Abs(result.WishGap-(120000-result.WishEarned)) > 1e-9 {
		t.Fatalf("wish target = target %.2f, earned %.2f, gap %.2f, salary %+v", result.WishTarget, result.WishEarned, result.WishGap, result.Salary)
	}
	afterOneSecond, err := CalculateDashboard(testDate("2026-07-16 11:00:01"), config)
	if err != nil || math.Abs(afterOneSecond.WishEarned-result.WishEarned-result.Salary.HourlyRate/3600) > 1e-9 {
		t.Fatalf("wish did not advance with realtime salary: before %.6f, after %.6f, error %v", result.WishEarned, afterOneSecond.WishEarned, err)
	}

	config.WishAmount = 50
	result, err = CalculateDashboard(testDate("2026-07-16 11:00:00"), config)
	if err != nil || result.WishGap != 0 || result.WishProgress != 1 {
		t.Fatalf("completed wish target = %+v, %v", result, err)
	}
}

func TestHourlyRemainingSalaryUsesEffectiveHours(t *testing.T) {
	config := testFullConfig()
	config.SalaryMode = "hourly"
	config.SalaryAmount = 100
	config.Workdays = map[time.Weekday]bool{time.Tuesday: true}
	config.StartSecond = 10 * 3600
	config.EndSecond = 18 * 3600
	result, err := CalculateDashboard(testDate("2026-07-13 11:00:00"), config)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.RemainingSalary / float64(result.RemainingWorkdays); got != 700 {
		t.Fatalf("daily hourly salary = %v, want 700", got)
	}
}

func TestDefaultRetirementUsesConfiguredProgressStart(t *testing.T) {
	config := testFullConfig()
	config.ProfileEnabled = false
	config.RetirementStart, _ = time.Parse("2006-01-02", "2026-07-16")
	config.ProgressBirthDate, _ = time.Parse("2006-01-02", "1996-07-16")
	result := CalculateDefaultRetirement(config, testDate("2031-07-16 00:00:00"))
	if !result.IsEstimate || result.RetirementMonth.Format("2006-01-02") != "2056-07-01" {
		t.Fatalf("unexpected default retirement: %+v", result)
	}
	if result.Progress < .4 || result.Progress > .45 {
		t.Fatalf("expected age-18 retirement progress, got %v", result.Progress)
	}
}

func TestManualRetirementStartsProgressWhenConfigured(t *testing.T) {
	start := testDate("2026-07-19 00:00:00")
	config := testFullConfig()
	config.ProfileEnabled = false
	config.RetirementYears = 10
	config.RetirementStart = start
	config.ProgressBirthDate = time.Time{}

	initial := CalculateDefaultRetirement(config, start)
	if initial.Progress != 0 {
		t.Fatalf("initial manual retirement progress = %.4f, want 0", initial.Progress)
	}
	midway := CalculateDefaultRetirement(config, start.AddDate(5, 0, 0))
	if midway.Progress < .49 || midway.Progress > .51 {
		t.Fatalf("midway manual retirement progress = %.4f, want about 0.5", midway.Progress)
	}
}
