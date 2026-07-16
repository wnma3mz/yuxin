package app

import (
	"fmt"
	"io"
	"strings"
)

const shareCardWidth = 58

// RenderShareCard renders a fixed-width, ANSI-free text card suitable for
// copying into a message or screenshot. Callers choose whether snapshot comes
// from DemoDashboard or the user's local configuration.
func RenderShareCard(snapshot DashboardSnapshot, config Config, card string) (string, error) {
	var rows []string
	switch card {
	case "overview":
		rows = shareOverviewRows(snapshot, config)
	case "workday":
		rows = shareWorkdayRows(snapshot, config)
	default:
		return "", fmt.Errorf("不支持的分享卡片 %q（可选 overview 或 workday）", card)
	}

	lines := []string{shareBorder("余薪 YUXIN · " + shareCardTitle(card))}
	for _, row := range rows {
		lines = append(lines, shareLine(row))
	}
	lines = append(lines, shareDivider())
	lines = append(lines, shareLine("无账号  ·  离线运行  ·  数据只在本地"))
	lines = append(lines, "╰"+strings.Repeat("─", shareCardWidth-2)+"╯")
	return strings.Join(lines, "\n"), nil
}

func writeShareCard(output io.Writer, snapshot DashboardSnapshot, config Config, cardType string) error {
	card, err := RenderShareCard(snapshot, config, cardType)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, card)
	return err
}

func shareOverviewRows(snapshot DashboardSnapshot, config Config) []string {
	countdownLabel := shareCountdownLabel(snapshot.Salary)
	rows := []string{
		shareDataLabel(snapshot),
		shareMetric("今日入账", shareAmount(snapshot.Salary.EarnedToday, config.HideAmounts)),
		shareMetric("今日预计", shareAmount(snapshot.Salary.ExpectedToday, config.HideAmounts)),
		shareMetric("工作进度", fmt.Sprintf("%.1f%%", clampFloat(snapshot.Salary.Progress, 0, 1)*100)),
		shareMetric(countdownLabel, shareRemaining(snapshot.Salary)),
	}
	if snapshot.Holiday != nil {
		rows = append(rows, shareMetric("节假日", holidayText(*snapshot.Holiday)))
	}
	if snapshot.RetirementEnabled {
		if config.HideRetirementDate {
			rows = append(rows, shareMetric("退休信息", "已隐藏"))
		} else if config.RetirementMode == "full" {
			retirement := retirementDate(snapshot.Retirement)
			rows = append(rows, shareMetric("预计退休", retirement))
			rows = append(rows, shareMetric("距离退休", retirementDistance(snapshot)))
		} else {
			rows = append(rows, shareMetric("距离退休", retirementDistance(snapshot)))
		}
	}
	if snapshot.AssetsEnabled {
		rows = append(rows, shareMetric("本地存款", shareAmount(snapshot.TotalAssets, config.HideAmounts)))
		if snapshot.SavingsTarget > 0 {
			value := "已隐藏"
			if !config.HideAmounts {
				value = fmt.Sprintf("%.0f%% · 还差 %s", snapshot.SavingsProgress*100, shareAmount(snapshot.SavingsGap, false))
			}
			rows = append(rows, shareMetric("存款目标", value))
		}
	}
	return rows
}

func shareWorkdayRows(snapshot DashboardSnapshot, config Config) []string {
	status, _ := statusText(snapshot.Salary.Status)
	return []string{
		shareDataLabel(snapshot),
		shareMetric("今日状态", strings.TrimSpace(status)),
		shareMetric(shareCountdownLabel(snapshot.Salary), shareRemaining(snapshot.Salary)),
		shareMetric("已工作", duration(snapshot.Salary.ElapsedSeconds)),
		shareMetric("工作进度", fmt.Sprintf("%.1f%%", clampFloat(snapshot.Salary.Progress, 0, 1)*100)),
		shareMetric("今日入账", shareAmount(snapshot.Salary.EarnedToday, config.HideAmounts)),
	}
}

func shareCardTitle(card string) string {
	if card == "workday" {
		return "工作日倒计时"
	}
	return "概览分享卡"
}

func shareDataLabel(snapshot DashboardSnapshot) string {
	if snapshot.DemoMode {
		return "演示数据 · 不包含个人信息"
	}
	return "本地数据 · 请确认后分享"
}

func shareRemaining(salary SalarySnapshot) string {
	if salary.RemainingSeconds > 0 {
		return duration(salary.RemainingSeconds)
	}
	status, _ := statusText(salary.Status)
	return strings.TrimSpace(strings.TrimLeft(status, "●○✓◐"))
}

func shareCountdownLabel(salary SalarySnapshot) string {
	switch salary.Status {
	case "before-work":
		return "上班倒计时"
	case "working", "lunch-break":
		return "下班倒计时"
	default:
		return "工作状态"
	}
}

func shareAmount(value float64, hidden bool) string {
	if hidden {
		return "已隐藏"
	}
	return money(value)
}

func shareMetric(label, value string) string {
	const labelWidth = 14
	return pad(label, labelWidth, alignLeft) + truncate(value, shareCardWidth-4-labelWidth)
}

func shareLine(value string) string {
	return "│ " + pad(truncate(value, shareCardWidth-4), shareCardWidth-4, alignLeft) + " │"
}

func shareDivider() string {
	return "├" + strings.Repeat("─", shareCardWidth-2) + "┤"
}

func shareBorder(title string) string {
	label := " " + title + " "
	return "╭─" + label + strings.Repeat("─", shareCardWidth-3-displayWidth(label)) + "╮"
}
