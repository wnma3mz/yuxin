package app

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func RenderDashboard(snapshot DashboardSnapshot, config Config, terminalWidth int, useColor bool) string {
	return renderDashboard(snapshot, config, terminalWidth, 0, useColor, false, false)
}

func renderDashboard(snapshot DashboardSnapshot, config Config, terminalWidth, terminalHeight int, useColor, details, helpVisible bool) string {
	width := terminalWidth
	if width <= 0 {
		width = 80
	}
	if width < 24 {
		return renderTiny(snapshot, width)
	}
	if width > 110 {
		width = 110
	}
	inner := width - 4
	status, statusColor := workStatus(snapshot, config)
	nowText := snapshot.Now.Format("2006-01-02 15:04:05")
	topLeft := "╭─ 余薪 YUXIN "
	topRight := " ─╮"
	if width >= 40 {
		topRight = " " + nowText + topRight
	}
	top := topLeft + strings.Repeat("─", max(1, width-displayWidth(topLeft)-displayWidth(topRight))) + topRight

	refresh := "刷新 " + formatInterval(config.RefreshInterval)
	if width >= 46 {
		dataLabel := "本地数据 ✓"
		if snapshot.DemoMode {
			dataLabel = "演示数据 ✓"
		}
		refresh += "  " + dataLabel
	}
	statusWidth := inner - displayWidth(refresh) - 1
	status = color(truncate(status, statusWidth), statusColor, useColor)
	header := "│ " + pad(status, statusWidth, alignLeft) + " " + refresh + " │"
	lines := []string{top, header, "├" + strings.Repeat("─", width-2) + "┤"}
	lines = append(lines,
		"│"+pad("今日入账", width-2, alignCenter)+"│",
		"│"+pad(color(money(snapshot.Salary.EarnedToday), "1;38;5;214", useColor), width-2, alignCenter)+"│",
		"│"+pad(color(incomeHint(snapshot.Salary), "90", useColor), width-2, alignCenter)+"│",
	)

	if width >= 70 {
		metrics := threeColumns(
			"已工作 "+duration(snapshot.Salary.ElapsedSeconds),
			remainingText(snapshot.Salary),
			"今日预计 "+money(snapshot.Salary.ExpectedToday),
			inner,
		)
		lines = append(lines, "│ "+metrics+" │")
	} else {
		compact := "下班 " + duration(snapshot.Salary.RemainingSeconds) + "  预计 " + money(snapshot.Salary.ExpectedToday)
		lines = append(lines, "│ "+pad(truncate(compact, inner), inner, alignLeft)+" │")
	}

	percent := fmt.Sprintf("%5.1f%%", clampFloat(snapshot.Salary.Progress, 0, 1)*100)
	progress := ""
	if width >= 70 {
		start := clock(config.StartSecond)
		end := clock(config.EndSecond)
		barWidth := max(8, inner-displayWidth(start)-displayWidth(end)-displayWidth(percent)-3)
		bar := progressBar(snapshot.Salary.Progress, barWidth, useColor)
		progress = start + " " + bar + " " + percent + " " + end
	} else {
		barWidth := max(4, inner-displayWidth(percent)-1)
		progress = progressBar(snapshot.Salary.Progress, barWidth, useColor) + " " + percent
	}
	lines = append(lines, "│ "+pad(truncate(progress, inner), inner, alignLeft)+" │")
	if snapshot.Holiday != nil {
		showTimeline := width >= 70 && (terminalHeight <= 0 || terminalHeight >= 25)
		for _, holidayLine := range holidayLines(*snapshot.Holiday, snapshot.Now, inner, useColor, showTimeline) {
			lines = append(lines, "│ "+pad(truncate(holidayLine, inner), inner, alignLeft)+" │")
		}
	} else if !snapshot.HolidayDataAvailable {
		lines = append(lines, "│ "+pad("当前版本未附带本年度节假日 · 请更新 Yuxin", inner, alignLeft)+" │")
	}
	lines = append(lines, "╰"+strings.Repeat("─", width-2)+"╯")

	sideBySide := snapshot.RetirementEnabled && snapshot.AssetsEnabled && width >= 100
	leftWidth, rightWidth := width, width
	if sideBySide {
		leftWidth = (width - 1) / 2
		rightWidth = width - leftWidth - 1
	}
	retirementRows := []string{}
	if snapshot.RetirementEnabled {
		retirementRows = append(retirementRows,
			metric("预计退休", retirementDate(snapshot.Retirement), leftWidth-4),
			metric("距离退休", commaInt(snapshot.Retirement.RemainingDays)+" 天", leftWidth-4),
			metric("剩余工作日", commaInt(snapshot.RemainingWorkdays)+" 天", leftWidth-4),
		)
		if snapshot.AssetsEnabled {
			retirementRows = append(retirementRows, metric("现在退休每天可花", money(snapshot.DailyUntilRetirement), leftWidth-4))
		}
		retirementRows = append(retirementRows, retirementProgress(snapshot.Retirement.Progress, leftWidth-4, useColor))
	}
	assetRows := []string{}
	if snapshot.AssetsEnabled {
		assetRows = append(assetRows,
			metric("配置余额", money(snapshot.TotalAssets), rightWidth-4),
			metric("今日工资", "+"+money(snapshot.Salary.EarnedToday), rightWidth-4),
			metric("实时余额", money(snapshot.LiveBalance), rightWidth-4),
			metric("应急保留金", money(config.Reserve), rightWidth-4),
			metric("可支配余额", money(snapshot.SpendableAssets), rightWidth-4),
		)
	}
	showPanels := !details && !helpVisible
	fullPanelHeight := 0
	if sideBySide {
		fullPanelHeight = max(len(retirementRows), len(assetRows)) + 2
	} else {
		if snapshot.RetirementEnabled {
			fullPanelHeight += len(retirementRows) + 2
		}
		if snapshot.AssetsEnabled {
			fullPanelHeight += len(assetRows) + 2
		}
	}
	compactLayout := width < 70 || (terminalHeight > 0 && (terminalHeight < 24 || len(lines)+fullPanelHeight+1 > terminalHeight))
	if showPanels && compactLayout {
		compactRows := []string{}
		if snapshot.RetirementEnabled {
			compactRows = append(compactRows, fmt.Sprintf("退休 %s · %s 天",
				snapshot.Retirement.RetirementMonth.Format("2006-01"),
				commaInt(snapshot.Retirement.RemainingDays)))
		}
		if snapshot.RetirementEnabled && snapshot.AssetsEnabled {
			compactRows = append(compactRows, "现在退休每天可花 "+money(snapshot.DailyUntilRetirement))
		}
		if len(compactRows) > 0 {
			lines = append(lines, panel("未来", compactRows, width)...)
		}
	} else if showPanels && sideBySide {
		lines = append(lines, joinPanels(
			panel("退休倒计时", retirementRows, leftWidth),
			panel("资产续航", assetRows, rightWidth),
		)...)
	} else if showPanels {
		if snapshot.RetirementEnabled {
			lines = append(lines, panel("退休倒计时", retirementRows, width)...)
		}
		if snapshot.AssetsEnabled {
			lines = append(lines, panel("资产续航", assetRows, width)...)
		}
	}
	if details {
		lines = append(lines, panel("计算口径", []string{
			fmt.Sprintf("时薪 %s · 日薪 %s", money(snapshot.Salary.HourlyRate), money(snapshot.Salary.DailyRate)),
			"退休天数口径：距离预计退休月第一天；工作日口径使用随包节假日数据。",
			"退休进度统一按 18 岁起计，不需要收集参加工作时间。",
			"今日工资与实时余额按秒更新；未来口径不含个税、奖金、利息、通胀和养老金。",
		}, width)...)
	}
	if helpVisible {
		lines = append(lines, panel("快捷键", []string{
			"e 编辑配置   r 立即刷新   s 隐私演示",
			"d 计算口径   ? 帮助       q 退出",
		}, width)...)
	}
	footer := "[e] 配置  [r] 刷新  [s] 演示  [d] 详情  [?] 帮助  [q] 退出"
	if width < 70 {
		footer = "[e] 配置  [s] 演示  [q] 退出"
	}
	lines = append(lines, pad(truncate(footer, width), width, alignCenter))
	return strings.Join(lines, "\n")
}

func renderTiny(snapshot DashboardSnapshot, width int) string {
	title := "余薪 YUXIN"
	if snapshot.DemoMode {
		title += " · 演示"
	}
	lines := []string{
		title,
		money(snapshot.Salary.EarnedToday),
		remainingText(snapshot.Salary),
	}
	if snapshot.RetirementEnabled {
		lines = append(lines, fmt.Sprintf("退休还有 %s 天", commaInt(snapshot.Retirement.RemainingDays)))
		if snapshot.AssetsEnabled {
			lines = append(lines, "每天可花 "+money(snapshot.DailyUntilRetirement))
		}
	}
	if snapshot.Holiday != nil {
		lines = append(lines, holidayText(*snapshot.Holiday))
	}
	lines = append(lines, "Ctrl+C 退出")
	for index := range lines {
		lines[index] = truncate(lines[index], width)
	}
	return strings.Join(lines, "\n")
}

func incomeHint(salary SalarySnapshot) string {
	switch salary.Status {
	case "working":
		return "↗ +" + money(salary.HourlyRate/3600) + " / 秒"
	case "lunch-break":
		return "午休暂停增长"
	case "after-work":
		return "今日已结算"
	case "before-work":
		return "等待上班"
	default:
		return "今天好好休息"
	}
}

func remainingText(salary SalarySnapshot) string {
	if salary.RemainingSeconds > 0 {
		return "距离下班 " + duration(salary.RemainingSeconds)
	}
	status, _ := statusText(salary.Status)
	return "工作状态 " + strings.TrimSpace(strings.TrimLeft(status, "●○✓◐"))
}

func statusText(status string) (string, string) {
	switch status {
	case "working":
		return "● 正在上班", "32"
	case "before-work":
		return "○ 尚未上班", "90"
	case "lunch-break":
		return "◐ 午休中", "33"
	case "after-work":
		return "✓ 今日下班", "36"
	case "rest-day":
		return "○ 今日休息", "90"
	default:
		return status, "0"
	}
}

func workStatus(snapshot DashboardSnapshot, config Config) (string, string) {
	status, statusColor := statusText(snapshot.Salary.Status)
	if snapshot.Salary.Status != "working" {
		return status, statusColor
	}
	message := ""
	nowSecond := snapshot.Now.Hour()*3600 + snapshot.Now.Minute()*60 + snapshot.Now.Second()
	switch {
	case snapshot.Salary.RemainingSeconds > 0 && snapshot.Salary.RemainingSeconds <= 10*60:
		message = "蓄势待发，准备冲刺起跑"
	case snapshot.Now.Hour() == 15:
		message = "三点几啦，饮茶先！"
	case nowSecond-config.StartSecond < 3600:
		message = "元气满满地摸鱼"
	}
	if message != "" {
		status += " (" + message + ")"
	}
	return status, statusColor
}

func holidayLines(holiday HolidaySnapshot, now time.Time, width int, useColor, showTimeline bool) []string {
	if holiday.DaysUntil <= 0 {
		remaining := max(1, daysBetween(now, holiday.End)+1)
		return []string{color(fmt.Sprintf("假期中 %s · 还剩 %d 天", holiday.Name, remaining), "36", useColor)}
	}
	if !holiday.HasPrevious || !showTimeline {
		return []string{color(holidayText(holiday), "36", useColor)}
	}
	left := holiday.PreviousName + " " + holiday.PreviousEnd.Format("01-02")
	right := holiday.Start.Format("01-02") + " " + holiday.Name
	barWidth := max(8, width-displayWidth(left)-displayWidth(right)-4)
	progress := clampFloat(holiday.IntervalProgress, 0, 1)
	marker := int(math.Round(progress * float64(barWidth-1)))
	passed := color(strings.Repeat("━", marker), "36", useColor)
	today := color("◆", "1;33", useColor)
	remaining := color(strings.Repeat("─", barWidth-marker-1), "90", useColor)
	timeline := left + " " + color("●", "36", useColor) + passed + today + remaining + color("○", "90", useColor) + " " + right
	note := fmt.Sprintf(
		"今天 %.1f%% · 已过 %d 天 / 还剩 %d 天",
		progress*100,
		holiday.DaysSincePrevious,
		holiday.DaysUntil,
	)
	return []string{timeline, pad(note, width, alignCenter)}
}

func holidayText(holiday HolidaySnapshot) string {
	if holiday.DaysUntil <= 0 && !holiday.End.IsZero() {
		remaining := int(holiday.End.Sub(holiday.Start).Hours()/24) + 1
		return fmt.Sprintf("假期中 %s · 共 %d 天", holiday.Name, max(1, remaining))
	}
	if holiday.HasPrevious {
		return fmt.Sprintf("距%s最后一天 %d 天 · 距%s %d 天",
			holiday.PreviousName, holiday.DaysSincePrevious, holiday.Name, holiday.DaysUntil)
	}
	return fmt.Sprintf("下个假期 %s · %s · 还有 %d 天", holiday.Name, holiday.Start.Format("01月02日"), holiday.DaysUntil)
}

func retirementDate(retirement RetirementSnapshot) string {
	value := retirement.RetirementMonth.Format("2006-01")
	if !retirement.IsEstimate && retirement.StatutoryAge != "" {
		value += " · 法定 " + retirement.StatutoryAge
	}
	return value
}

func retirementProgress(progress float64, width int, useColor bool) string {
	const label = "退休进度（18岁起）"
	percent := fmt.Sprintf("%.1f%%", clampFloat(progress, 0, 1)*100)
	barWidth := max(4, width-displayWidth(label)-displayWidth(percent)-2)
	return label + " " + progressBar(progress, barWidth, useColor) + " " + percent
}

func panel(title string, rows []string, width int) []string {
	titleText := " " + title + " "
	top := "╭─" + titleText + strings.Repeat("─", max(0, width-3-displayWidth(titleText))) + "╮"
	result := []string{top}
	for _, row := range rows {
		result = append(result, "│ "+pad(truncate(row, width-4), width-4, alignLeft)+" │")
	}
	return append(result, "╰"+strings.Repeat("─", width-2)+"╯")
}

func joinPanels(left, right []string) []string {
	height := max(len(left), len(right))
	leftWidth := 0
	for _, line := range left {
		leftWidth = max(leftWidth, displayWidth(line))
	}
	result := make([]string, 0, height)
	for index := 0; index < height; index++ {
		leftLine := ""
		if index < len(left) {
			leftLine = left[index]
		}
		rightLine := ""
		if index < len(right) {
			rightLine = right[index]
		}
		result = append(result, pad(leftLine, leftWidth, alignLeft)+" "+rightLine)
	}
	return result
}

func metric(label, value string, width int) string {
	return pad(label, max(displayWidth(label)+1, width-displayWidth(value)), alignLeft) + value
}

func threeColumns(left, center, right string, width int) string {
	leftWidth := width / 3
	rightWidth := width / 3
	centerWidth := width - leftWidth - rightWidth
	return pad(truncate(left, leftWidth), leftWidth, alignLeft) +
		pad(truncate(center, centerWidth), centerWidth, alignCenter) +
		pad(truncate(right, rightWidth), rightWidth, alignRight)
}

func progressBar(progress float64, width int, useColor bool) string {
	filled := int(math.Round(clampFloat(progress, 0, 1) * float64(width)))
	return color(strings.Repeat("█", filled), "32", useColor) +
		color(strings.Repeat("░", width-filled), "90", useColor)
}

func money(value float64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	whole := int64(value)
	fraction := int(math.Round((value - float64(whole)) * 100))
	if fraction == 100 {
		whole++
		fraction = 0
	}
	return fmt.Sprintf("%s¥%s.%02d", sign, commaInt64(whole), fraction)
}

func commaInt(value int) string { return commaInt64(int64(value)) }

func commaInt64(value int64) string {
	sign := ""
	if value < 0 {
		sign, value = "-", -value
	}
	digits := fmt.Sprintf("%d", value)
	for index := len(digits) - 3; index > 0; index -= 3 {
		digits = digits[:index] + "," + digits[index:]
	}
	return sign + digits
}

func duration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	hours, rest := seconds/3600, seconds%3600
	minutes, secs := rest/60, rest%60
	if hours > 0 {
		return fmt.Sprintf("%dh %02dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %02ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

func clock(seconds int) string {
	return fmt.Sprintf("%02d:%02d", seconds/3600, (seconds%3600)/60)
}

type alignment int

const (
	alignLeft alignment = iota
	alignCenter
	alignRight
)

func pad(value string, width int, alignment alignment) string {
	missing := max(0, width-displayWidth(value))
	switch alignment {
	case alignRight:
		return strings.Repeat(" ", missing) + value
	case alignCenter:
		left := missing / 2
		return strings.Repeat(" ", left) + value + strings.Repeat(" ", missing-left)
	default:
		return value + strings.Repeat(" ", missing)
	}
}

func truncate(value string, width int) string {
	if displayWidth(value) <= width {
		return value
	}
	plain := ansiPattern.ReplaceAllString(value, "")
	var result strings.Builder
	used := 0
	for _, char := range plain {
		charWidth := runeWidth(char)
		if used+charWidth > max(0, width-1) {
			break
		}
		result.WriteRune(char)
		used += charWidth
	}
	return result.String() + "…"
}

func displayWidth(value string) int {
	plain := ansiPattern.ReplaceAllString(value, "")
	width := 0
	for _, char := range plain {
		width += runeWidth(char)
	}
	return width
}

func runeWidth(char rune) int {
	if char == '\u200d' || unicode.Is(unicode.Mn, char) || unicode.Is(unicode.Me, char) {
		return 0
	}
	if char >= 0x1100 && (char <= 0x115f || char == 0x2329 || char == 0x232a ||
		(char >= 0x2e80 && char <= 0xa4cf) || (char >= 0xac00 && char <= 0xd7a3) ||
		(char >= 0xf900 && char <= 0xfaff) || (char >= 0xfe10 && char <= 0xfe6f) ||
		(char >= 0xff00 && char <= 0xff60) || (char >= 0x1f300 && char <= 0x1faff)) {
		return 2
	}
	if char == utf8.RuneError {
		return 1
	}
	return 1
}

func color(value, code string, enabled bool) string {
	if !enabled {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func clampFloat(value, low, high float64) float64 {
	return math.Min(high, math.Max(low, value))
}
