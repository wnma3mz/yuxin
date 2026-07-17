package app

import (
	"fmt"
	"math"
	"time"
)

type RetirementSnapshot struct {
	RetirementMonth time.Time
	RemainingDays   int
	DelayedMonths   int
	StatutoryAge    string
	IsEstimate      bool
	Progress        float64
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
	progressStart := birth.AddDate(18, 0, 0)
	totalDays := max(1, daysBetween(progressStart, retirement))
	progress := math.Max(0, math.Min(1, float64(daysBetween(progressStart, today))/float64(totalDays)))
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
		progressStart = normalizedDate(config.ProgressBirthDate).AddDate(18, 0, 0)
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
	case config.Sex == "female" && (config.FemaleTrack == "" || config.FemaleTrack == "55"):
		baseAge, firstYear, firstMonth, step, cap = 55, 1970, time.January, 4, 36
	case config.Sex == "female" && config.FemaleTrack == "50":
		baseAge, firstYear, firstMonth, step, cap = 50, 1975, time.January, 2, 60
	default:
		return 0, 0, fmt.Errorf("性别必须是男或女")
	}
	birthIndex := config.BirthDate.Year()*12 + int(config.BirthDate.Month()) - 1
	startIndex := firstYear*12 + int(firstMonth) - 1
	if birthIndex >= startIndex {
		delay = min((birthIndex-startIndex)/step+1, cap)
	}
	return baseAge, delay, nil
}

func addMonths(value time.Time, months int) time.Time {
	index := value.Year()*12 + int(value.Month()) - 1 + months
	return time.Date(index/12, time.Month(index%12+1), 1, 0, 0, 0, 0, time.UTC)
}
