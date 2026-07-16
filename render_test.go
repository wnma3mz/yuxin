package main

import (
	"strings"
	"testing"
	"time"
)

func TestRenderDashboardPlainSnapshot(t *testing.T) {
	config := defaultConfig()
	now := time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local)
	snapshot := DashboardSnapshot{
		Now: now,
		Salary: SalarySnapshot{
			EarnedToday:      286.37,
			ExpectedToday:    363.64,
			HourlyRate:       45.45,
			ElapsedSeconds:   6*3600 + 12*60,
			RemainingSeconds: 1*3600 + 48*60,
			Progress:         0.775,
			Status:           "working",
		},
		Retirement: RetirementSnapshot{
			RetirementMonth: time.Date(2058, time.January, 1, 0, 0, 0, 0, time.Local),
			RemainingDays:   11492,
			StatutoryAge:    "63 岁",
			Progress:        0.501,
		},
		RetirementEnabled:    true,
		AssetsEnabled:        true,
		TotalAssets:          100000,
		LiveBalance:          100286.37,
		SpendableAssets:      100286.37,
		DailyUntilRetirement: 8.73,
		RemainingWorkdays:    8000,
		HolidayDataAvailable: true,
	}

	output := RenderDashboard(snapshot, config, 80, false)
	for _, expected := range []string{"余薪 YUXIN", "今日入账", "¥286.37", "距离下班", "退休倒计时", "资产续航", "[q] 退出"} {
		if !strings.Contains(output, expected) {
			t.Errorf("rendered output does not contain %q", expected)
		}
	}
	if strings.Contains(output, "\x1b[") {
		t.Fatal("plain rendering contains ANSI escapes")
	}
}

func TestDisabledModulesAreHidden(t *testing.T) {
	config := defaultConfig()
	config.ProfileEnabled = false
	config.RetirementYears = 0
	config.AssetsEnabled = false
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 10, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := RenderDashboard(snapshot, config, 80, false)
	for _, forbidden := range []string{"0001-01", "退休倒计时", "资产续航", "实时余额", "每天可花"} {
		if strings.Contains(output, forbidden) {
			t.Errorf("disabled output contains %q", forbidden)
		}
	}
}

func TestDisabledAssetsDoNotLeakDailyBudget(t *testing.T) {
	config := defaultConfig()
	config.AssetsEnabled = false
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 10, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{40, 80} {
		output := RenderDashboard(snapshot, config, width, false)
		if strings.Contains(output, "每天可花") || strings.Contains(output, "资产续航") {
			t.Fatalf("width %d leaked disabled assets:\n%s", width, output)
		}
	}
}

func TestVeryNarrowOutputNeverExceedsTerminal(t *testing.T) {
	config := defaultConfig()
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 10, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	for _, requested := range []int{20, 31, 40} {
		output := RenderDashboard(snapshot, config, requested, false)
		for _, line := range strings.Split(output, "\n") {
			if width := displayWidth(line); width > requested {
				t.Fatalf("requested %d, got line width %d: %q", requested, width, line)
			}
		}
	}
}

func TestRenderDashboardUsesRequestedWidth(t *testing.T) {
	config := defaultConfig()
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 10, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := RenderDashboard(snapshot, config, 72, false)
	for _, line := range strings.Split(output, "\n") {
		if width := displayWidth(line); width > 72 {
			t.Fatalf("line is %d columns wide, want at most 72: %q", width, line)
		}
	}
}

func TestFormattingHelpers(t *testing.T) {
	if got := money(1234567.895); got != "¥1,234,567.90" {
		t.Fatalf("money() = %q", got)
	}
	if got := duration(3723); got != "1h 02m" {
		t.Fatalf("duration() = %q", got)
	}
	if got := displayWidth("余薪 YUXIN"); got != 10 {
		t.Fatalf("displayWidth() = %d", got)
	}
}

func TestWorkSloganAndHolidayTimeline(t *testing.T) {
	config := defaultConfig()
	now := time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local)
	snapshot, err := CalculateDashboard(now, config)
	if err != nil {
		t.Fatal(err)
	}
	output := RenderDashboard(snapshot, config, 100, false)
	for _, expected := range []string{"三点几啦，饮茶先！", "◆", "今天 26.0%", "已过 25 天", "还剩 71 天"} {
		if !strings.Contains(output, expected) {
			t.Errorf("rendered output does not contain %q", expected)
		}
	}
}

func TestResponsiveDashboardLayout(t *testing.T) {
	config := defaultConfig()
	now := time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local)
	snapshot, err := CalculateDashboard(now, config)
	if err != nil {
		t.Fatal(err)
	}

	wide := RenderDashboard(snapshot, config, 100, false)
	foundSideBySide := false
	for _, line := range strings.Split(wide, "\n") {
		if strings.Contains(line, "退休倒计时") && strings.Contains(line, "资产续航") {
			foundSideBySide = true
			break
		}
	}
	if !foundSideBySide {
		t.Fatal("100-column layout did not render retirement and assets side by side")
	}

	medium := RenderDashboard(snapshot, config, 80, false)
	for _, line := range strings.Split(medium, "\n") {
		if strings.Contains(line, "退休倒计时") && strings.Contains(line, "资产续航") {
			t.Fatal("80-column layout unexpectedly rendered panels side by side")
		}
	}

	narrow := RenderDashboard(snapshot, config, 60, false)
	for _, expected := range []string{"本地数据 ✓", "下班", "未来", "现在退休每天可花"} {
		if !strings.Contains(narrow, expected) {
			t.Errorf("60-column output does not contain %q", expected)
		}
	}
	for _, forbidden := range []string{"09:00", "18:00", "◆", "资产续航", "实时余额"} {
		if strings.Contains(narrow, forbidden) {
			t.Errorf("60-column output contains %q", forbidden)
		}
	}
}

func TestDashboardDetailsAndHelpAreInteractiveViews(t *testing.T) {
	config := defaultConfig()
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}

	details := renderDashboard(snapshot, config, 80, 24, false, true, false)
	if !strings.Contains(details, "计算口径") || strings.Contains(details, "资产续航") {
		t.Fatalf("unexpected details view:\n%s", details)
	}
	help := renderDashboard(snapshot, config, 80, 24, false, false, true)
	if !strings.Contains(help, "快捷键") || !strings.Contains(help, "q 退出") {
		t.Fatalf("unexpected help view:\n%s", help)
	}
}

func TestShortTerminalUsesCompactPanels(t *testing.T) {
	config := defaultConfig()
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := renderDashboard(snapshot, config, 100, 23, false, false, false)
	if !strings.Contains(output, "未来") || strings.Contains(output, "资产续航") || strings.Contains(output, "◆") {
		t.Fatalf("short terminal did not use compact layout:\n%s", output)
	}
}

func TestStandard80By24TerminalDoesNotOverflow(t *testing.T) {
	config := defaultConfig()
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := renderDashboard(snapshot, config, 80, 24, false, false, false)
	lines := strings.Split(output, "\n")
	if len(lines) > 24 || !strings.Contains(output, "未来") || strings.Contains(output, "资产续航") {
		t.Fatalf("80x24 layout uses %d lines:\n%s", len(lines), output)
	}
}
