package app

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func RenderDashboard(snapshot DashboardSnapshot, config Config, terminalWidth int, useColor bool) string {
	return renderDashboard(snapshot, config, terminalWidth, 0, useColor, false)
}

func renderDashboard(snapshot DashboardSnapshot, config Config, terminalWidth, terminalHeight int, useColor, details bool) string {
	config = normalizedPrivacyConfig(config)
	width := terminalWidth
	if width <= 0 {
		width = 80
	}
	if width < 24 {
		return renderTiny(snapshot, config, width)
	}
	if width > 110 {
		width = 110
	}
	inner := width - 4
	status, statusColor := workStatus(snapshot, config)
	nowText := snapshot.Now.Format("2006-01-02 15:04:05")
	title := "余薪 YUXIN"
	if snapshot.DemoMode {
		title += " · 演示模式"
		if width < 40 {
			title = "YUXIN · 演示模式"
		}
	}
	topLeft := "╭─ " + title + " "
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
	var headerContent string
	if width >= 70 {
		status = color(status, statusColor, useColor)
		headerContent = threeColumns(status, config.Slogan, refresh, inner)
	} else {
		statusWidth := inner - displayWidth(refresh) - 1
		status = color(truncate(status, statusWidth), statusColor, useColor)
		headerContent = pad(status, statusWidth, alignLeft) + " " + refresh
	}
	header := "│ " + headerContent + " │"
	lines := []string{top, header, "├" + strings.Repeat("─", width-2) + "┤"}
	lines = append(lines,
		"│"+pad("今日入账", width-2, alignCenter)+"│",
		"│"+pad(color(displayMoney(snapshot.Salary.EarnedToday, config.HideAmounts), "1;38;5;214", useColor), width-2, alignCenter)+"│",
		"│"+pad(color(incomeHintForDisplay(snapshot.Salary, config.HideAmounts), "90", useColor), width-2, alignCenter)+"│",
	)

	if width >= 70 {
		metrics := threeColumns(
			"已工作 "+duration(snapshot.Salary.ElapsedSeconds),
			remainingText(snapshot.Salary),
			"今日预计 "+displayMoney(snapshot.Salary.ExpectedToday, config.HideAmounts),
			inner,
		)
		lines = append(lines, "│ "+metrics+" │")
	} else {
		compact := remainingText(snapshot.Salary) + "  预计 " + displayMoney(snapshot.Salary.ExpectedToday, config.HideAmounts)
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
		if config.HideRetirementDate {
			retirementRows = append(retirementRows, metric("退休信息", "已隐藏", leftWidth-4))
		} else {
			days := max(0, snapshot.Retirement.RemainingDays)
			retirementRows = append(retirementRows,
				retirementProgress(snapshot.Retirement.Progress, leftWidth-4, useColor),
				metric("距离退休还有", fmt.Sprintf("%d 年", int(float64(days)/averageDaysPerYear)), leftWidth-4),
				metric("距离退休还有", fmt.Sprintf("%d 个月", int(float64(days)/averageDaysPerMonth)), leftWidth-4),
				metric("距离退休还有", commaInt(days)+" 天", leftWidth-4),
			)
		}
	}
	assetRows := []string{}
	assetTitle := "存款"
	if snapshot.AssetsEnabled {
		assetRows = append(assetRows,
			metric("实时存款余额", displayMoney(snapshot.LiveBalance, config.HideAmounts), rightWidth-4),
		)
		if snapshot.RetirementEnabled && snapshot.Retirement.RemainingDays > 0 {
			assetRows = append(assetRows,
				metric("撑到退休每天可花", displayMoney(snapshot.DailyUntilRetirement, config.HideAmounts), rightWidth-4),
				metric("撑到退休每月可花", displayMoney(snapshot.DailyUntilRetirement*averageDaysPerMonth, config.HideAmounts), rightWidth-4),
				metric("撑到退休每年可花", displayMoney(snapshot.DailyUntilRetirement*averageDaysPerYear, config.HideAmounts), rightWidth-4),
			)
			if !config.HideAmounts && !config.HideRetirementDate {
				assetTitle += " · " + purchasingPowerQuip(snapshot.DailyUntilRetirement)
			}
		}
	}
	goalRows := []string{}
	if snapshot.SavingsTarget > 0 {
		goalRows = append(goalRows, threeColumns(
			"每天 "+displayMoney(config.TargetMonthlySpend/averageDaysPerMonth, config.HideAmounts),
			"每月 "+displayMoney(config.TargetMonthlySpend, config.HideAmounts),
			"每年 "+displayMoney(config.TargetMonthlySpend*12, config.HideAmounts),
			width-4,
		))
		if config.HideAmounts {
			goalRows = append(goalRows,
				metric("目标进度", "已隐藏", width-4),
				metric("距离目标还差", displayMoney(snapshot.SavingsGap, true), width-4),
			)
		} else {
			gap := displayMoney(snapshot.SavingsGap, false)
			if snapshot.SavingsGap <= 0 {
				gap = "已达成"
			}
			goalRows = append(goalRows,
				savingsTargetProgress(snapshot.SavingsProgress, width-4, useColor),
				metric("距离目标还差", gap, width-4),
			)
		}
	}
	showPanels := !details
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
	if len(goalRows) > 0 {
		fullPanelHeight += len(goalRows) + 2
	}
	compactLayout := width < 70 || (terminalHeight > 0 && (terminalHeight < 24 || len(lines)+fullPanelHeight+1 > terminalHeight))
	if showPanels && compactLayout {
		compactRows := []string{}
		if snapshot.RetirementEnabled {
			text := "退休信息 已隐藏"
			if !config.HideRetirementDate {
				text = "退休还有 " + retirementDistance(snapshot)
			}
			compactRows = append(compactRows, text)
		}
		if snapshot.AssetsEnabled {
			text := "实时存款余额 " + displayMoney(snapshot.LiveBalance, config.HideAmounts)
			if snapshot.RetirementEnabled && snapshot.Retirement.RemainingDays > 0 {
				text += " · 撑到退休每天可花 " + displayMoney(snapshot.DailyUntilRetirement, config.HideAmounts)
			}
			compactRows = append(compactRows, text)
			if snapshot.SavingsTarget > 0 {
				target := "存款目标 已隐藏"
				if !config.HideAmounts {
					target = fmt.Sprintf("存款目标 %.0f%% · 还差 %s", snapshot.SavingsProgress*100, displayMoney(snapshot.SavingsGap, false))
				}
				compactRows = append(compactRows, target)
			}
		}
		if len(compactRows) > 0 {
			lines = append(lines, panel("未来", compactRows, width)...)
		}
	} else if showPanels && sideBySide {
		lines = append(lines, joinPanels(
			panel("退休倒计时", retirementRows, leftWidth),
			panel(assetTitle, assetRows, rightWidth),
		)...)
		if len(goalRows) > 0 {
			lines = append(lines, panel("存款目标", goalRows, width)...)
		}
	} else if showPanels {
		if snapshot.RetirementEnabled {
			lines = append(lines, panel("退休倒计时", retirementRows, width)...)
		}
		if snapshot.AssetsEnabled {
			lines = append(lines, panel(assetTitle, assetRows, width)...)
		}
		if len(goalRows) > 0 {
			lines = append(lines, panel("存款目标", goalRows, width)...)
		}
	}
	if details {
		lines = append(lines, panel("计算口径", []string{
			fmt.Sprintf("时薪 %s · 日薪 %s", displayMoney(snapshot.Salary.HourlyRate, config.HideAmounts), displayMoney(snapshot.Salary.DailyRate, config.HideAmounts)),
			"退休天数口径：距离预计退休月第一天。",
			"退休年、月为剩余天数按 365.2425 天和 30.436875 天向下取整。",
			"退休进度统一按 18 岁起计，不需要收集参加工作时间。",
			"存款目标：目标月支出 × 距退休天数 ÷ 30.436875，不含收益率和通胀。",
			"今日入账按秒更新；未来口径不含个税、奖金、利息、通胀和养老金。",
		}, width)...)
	}
	footer := dashboardFooter(config, snapshot, details, width)
	lines = append(lines, pad(truncate(footer, width), width, alignCenter))
	return strings.Join(lines, "\n")
}

func dashboardFooter(config Config, snapshot DashboardSnapshot, details bool, width int) string {
	privacy := "[p] 隐私"
	if config.HideRetirementDate {
		privacy += "·全部"
	} else if config.HideAmounts {
		privacy += "·金额"
	}
	view := "[v] 视图"
	if snapshot.DemoMode {
		view = "[v] 演示"
	} else if details {
		view = "[v] 详情"
	}
	parts := []string{"[e] 配置", privacy}
	if width >= 70 {
		parts = append(parts, "[r] 刷新")
	}
	parts = append(parts, view, "[q] 退出")
	footer := strings.Join(parts, "  ")
	if displayWidth(footer) <= width {
		return footer
	}
	return "[e] 配置  [p] 隐私  [v] 视图  [q] 退出"
}

func renderTiny(snapshot DashboardSnapshot, config Config, width int) string {
	title := "余薪 YUXIN"
	if snapshot.DemoMode {
		title += " · 演示模式"
	}
	lines := []string{
		title,
		displayMoney(snapshot.Salary.EarnedToday, config.HideAmounts),
		remainingText(snapshot.Salary),
	}
	if snapshot.AssetsEnabled {
		lines = append(lines, "实时存款余额 "+displayMoney(snapshot.LiveBalance, config.HideAmounts))
	}
	if snapshot.RetirementEnabled {
		if config.HideRetirementDate {
			lines = append(lines, "退休信息 已隐藏")
		} else {
			lines = append(lines, "退休还有 "+retirementDistance(snapshot))
		}
		if snapshot.AssetsEnabled {
			lines = append(lines, "撑到退休每天可花 "+displayMoney(snapshot.DailyUntilRetirement, config.HideAmounts))
		}
	}
	if snapshot.SavingsTarget > 0 {
		if config.HideAmounts {
			lines = append(lines, "存款目标 已隐藏")
		} else {
			lines = append(lines, fmt.Sprintf("存款目标 %.0f%% 还差%s", snapshot.SavingsProgress*100, displayMoney(snapshot.SavingsGap, false)))
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

func incomeHintForDisplay(salary SalarySnapshot, hideAmounts bool) string {
	switch salary.Status {
	case "working":
		return "↗ +" + displayMoney(salary.HourlyRate/3600, hideAmounts) + " / 秒"
	case "lunch-break":
		return "午休中，余额暂停跳动"
	case "after-work":
		return "今日入账已封盘"
	case "before-work":
		return "今日尚未开张"
	default:
		return "工资今天也休息"
	}
}

func displayMoney(value float64, hidden bool) string {
	if hidden {
		return "¥••••"
	}
	return money(value)
}

func purchasingPowerQuip(dailyBudget float64) string {
	switch {
	case dailyBudget < 1:
		return "馒头自由还差一点"
	case dailyBudget < 3:
		return "加蛋勉强，馒头管够"
	case dailyBudget < 6:
		return "够一瓶快乐水"
	case dailyBudget < 12:
		return "沙县小吃友情试吃"
	case dailyBudget < 25:
		return "勉强点个沙县外卖"
	case dailyBudget < 50:
		return "工作日午餐自由"
	case dailyBudget < 100:
		return "疯狂星期四，肆意疯狂"
	case dailyBudget < 200:
		return "脱离温饱，略有小康"
	default:
		return "恭喜，退休生活开始体面"
	}
}

func retirementDistance(snapshot DashboardSnapshot) string {
	start := normalizedDate(snapshot.Now)
	end := normalizedDate(snapshot.Retirement.RetirementMonth)
	if !end.After(start) {
		return "0 年 0 个月 0 天"
	}
	years := end.Year() - start.Year()
	cursor := start.AddDate(years, 0, 0)
	if cursor.After(end) {
		years--
		cursor = start.AddDate(years, 0, 0)
	}
	months := (end.Year()-cursor.Year())*12 + int(end.Month()-cursor.Month())
	monthCursor := cursor.AddDate(0, months, 0)
	if monthCursor.After(end) {
		months--
		monthCursor = cursor.AddDate(0, months, 0)
	}
	days := max(0, daysBetween(monthCursor, end))
	return fmt.Sprintf("%d 年 %d 个月 %d 天", years, months, days)
}

func remainingText(salary SalarySnapshot) string {
	switch salary.Status {
	case "before-work":
		if salary.RemainingSeconds > 0 {
			return "距离上班 " + duration(salary.RemainingSeconds)
		}
	case "working", "lunch-break":
		if salary.RemainingSeconds > 0 {
			return "距离下班 " + duration(salary.RemainingSeconds)
		}
	}
	if salary.RemainingSeconds > 0 && salary.Status == "" {
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
		return "✓ 已经下班", "36"
	case "rest-day":
		return "○ 今日休息", "90"
	default:
		return status, "0"
	}
}

func workStatus(snapshot DashboardSnapshot, config Config) (string, string) {
	status, statusColor := statusText(snapshot.Salary.Status)
	switch snapshot.Salary.Status {
	case "before-work":
		return status + "（摸鱼先热身）", statusColor
	case "after-work":
		return status + "（快乐已到账）", statusColor
	case "working":
	default:
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
	note := fmt.Sprintf("已过 %d 天 · 还剩 %d 天", holiday.DaysSincePrevious, holiday.DaysUntil)
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
	const label = "退休进度"
	percent := fmt.Sprintf("%.1f%%", clampFloat(progress, 0, 1)*100)
	barWidth := max(4, width-displayWidth(label)-displayWidth(percent)-2)
	return label + " " + progressBar(progress, barWidth, useColor) + " " + percent
}

func savingsTargetProgress(progress float64, width int, useColor bool) string {
	const label = "目标进度"
	percent := fmt.Sprintf("%.0f%%", clampFloat(progress, 0, 1)*100)
	barWidth := max(4, width-displayWidth(label)-displayWidth(percent)-2)
	return label + " " + progressBar(progress, barWidth, useColor) + " " + percent
}
