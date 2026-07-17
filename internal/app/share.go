package app

import (
	"fmt"
	"io"
	"strings"
)

const shareCardWidth = 62

// RenderShareCard renders a fixed-width, ANSI-free text card suitable for
// copying into a message or screenshot. Callers choose whether snapshot comes
// from DemoDashboard or the user's local configuration.
func RenderShareCard(snapshot DashboardSnapshot, config Config, card string) (string, error) {
	config = normalizedPrivacyConfig(config)
	switch card {
	case "overview":
		return renderOverviewShareCard(snapshot, config), nil
	case "workday":
		return renderWorkdayShareCard(snapshot, config), nil
	default:
		return "", fmt.Errorf("不支持的分享卡片 %q（可选 overview 或 workday）", card)
	}
}

func writeShareCard(output io.Writer, snapshot DashboardSnapshot, config Config, cardType string) error {
	card, err := RenderShareCard(snapshot, config, cardType)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, card)
	return err
}

func renderOverviewShareCard(snapshot DashboardSnapshot, config Config) string {
	rows := []string{
		shareDataLabel(snapshot, config),
		shareDivider(),
		shareLine(shareWorkProgress(snapshot.Salary)),
		shareLine(shareIncome(snapshot.Salary, config.HideAmounts)),
		shareLine(""),
	}
	if snapshot.Holiday != nil {
		rows = append(rows, shareLine("🌴 盼头："+holidayText(*snapshot.Holiday)))
	}
	if snapshot.RetirementEnabled {
		rows = append(rows, shareLine(shareRetirement(snapshot, config)))
	}
	if snapshot.RetirementEnabled && snapshot.AssetsEnabled {
		rows = append(rows, shareLine(""), shareLine(shareDailyBudget(snapshot, config.HideAmounts)))
	}
	lines := []string{shareBorder(shareOverviewTitle(config))}
	lines = append(lines, shareLine(rows[0]))
	lines = append(lines, rows[1:]...)
	lines = append(lines, shareDivider(), shareLine(shareFooter()), shareBottomBorder())
	return strings.Join(lines, "\n")
}

func renderWorkdayShareCard(snapshot DashboardSnapshot, config Config) string {
	lines := []string{shareBorder("余薪 YUXIN · 工作日倒计时")}
	for _, row := range shareWorkdayRows(snapshot, config) {
		lines = append(lines, shareLine(row))
	}
	lines = append(lines, shareDivider(), shareLine(shareFooter()), shareBottomBorder())
	return strings.Join(lines, "\n")
}

func shareOverviewTitle(config Config) string {
	slogan := strings.TrimSpace(config.Slogan)
	slogan = strings.TrimRight(slogan, "。.!！")
	if slogan == "" {
		slogan = strings.TrimRight(defaultSlogan, "。")
	}
	return "余薪 YUXIN · " + slogan
}

func shareWorkProgress(salary SalarySnapshot) string {
	percent := fmt.Sprintf("%.1f%%", clampFloat(salary.Progress, 0, 1)*100)
	suffix := shareProgressSuffix(salary)
	prefix := "⏳ 今日工作进度 ────"
	barWidth := max(8, shareCardWidth-4-displayWidth(prefix)-displayWidth(percent)-displayWidth(suffix)-2)
	filled := int(clampFloat(salary.Progress, 0, 1)*float64(barWidth) + 0.5)
	bar := strings.Repeat("■", filled) + strings.Repeat("□", barWidth-filled)
	return fmt.Sprintf("%s%s %s %s", prefix, bar, percent, suffix)
}

func shareProgressSuffix(salary SalarySnapshot) string {
	switch salary.Status {
	case "before-work":
		return "(距上班 " + shareRemaining(salary) + ")"
	case "working", "lunch-break":
		return "(剩 " + shareRemaining(salary) + ")"
	default:
		return "(" + shareRemaining(salary) + ")"
	}
}

func shareIncome(salary SalarySnapshot, hidden bool) string {
	return fmt.Sprintf("💰 今日已经赚到 ──── %s / %s (预计)",
		shareAmount(salary.EarnedToday, hidden), shareAmount(salary.ExpectedToday, hidden))
}

func shareRetirement(snapshot DashboardSnapshot, config Config) string {
	if config.HideRetirementDate {
		return "👴 退休：信息已隐藏"
	}
	distance := strings.NewReplacer(" 年", "年", " 个月", "个月", " 天", "天").Replace(retirementDistance(snapshot)) + "后"
	value := "👴 退休：" + distance
	if snapshot.SavingsTarget > 0 {
		if config.HideAmounts {
			value += " ── [存款目标已隐藏]"
		} else {
			value += fmt.Sprintf(" ── [存款目标进度: %.0f%%]", clampFloat(snapshot.SavingsProgress, 0, 1)*100)
		}
	}
	return value
}

func shareDailyBudget(snapshot DashboardSnapshot, hidden bool) string {
	if hidden {
		return "💬 躺平生存指南 ──── 每天可花 已隐藏"
	}
	amount := shareAmount(snapshot.DailyUntilRetirement, false)
	return fmt.Sprintf("💬 躺平生存指南 ──── 每天可花 %s (%s)", amount, purchasingPowerQuip(snapshot.DailyUntilRetirement))
}

func shareWorkdayRows(snapshot DashboardSnapshot, config Config) []string {
	status, _ := statusText(snapshot.Salary.Status)
	return []string{
		shareDataLabel(snapshot, config),
		shareMetric("今日状态", strings.TrimSpace(status)),
		shareMetric(shareCountdownLabel(snapshot.Salary), shareRemaining(snapshot.Salary)),
		shareMetric("已工作", duration(snapshot.Salary.ElapsedSeconds)),
		shareMetric("工作进度", fmt.Sprintf("%.1f%%", clampFloat(snapshot.Salary.Progress, 0, 1)*100)),
		shareMetric("今日入账", shareAmount(snapshot.Salary.EarnedToday, config.HideAmounts)),
	}
}

func shareDataLabel(snapshot DashboardSnapshot, config Config) string {
	if snapshot.DemoMode {
		return "[💡] 演示数据 · 已使用固定合成数据"
	}
	if config.HideRetirementDate {
		return "[✓] 本地数据 · 金额、存款和退休信息已隐藏"
	}
	if config.HideAmounts {
		return "[✓] 本地数据 · 金额和存款已隐藏"
	}
	return "[!] 本地数据 · 尚未开启隐私保护"
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
	label := " " + truncate(title, shareCardWidth-5) + " "
	return "╭─" + label + strings.Repeat("─", max(0, shareCardWidth-3-displayWidth(label))) + "╮"
}

func shareBottomBorder() string {
	return "╰" + strings.Repeat("─", shareCardWidth-2) + "╯"
}

func shareFooter() string {
	return "🏠 离线本地运行 · 数据绝不上云 · github.com/wnma3mz/yuxin"
}
