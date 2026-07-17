package app

import (
	"fmt"
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
	for _, expected := range []string{"余薪 YUXIN", defaultSlogan, "今日入账", "¥286.37", "距离下班", "🏁 退休倒计时", "💰 存款", "实时存款余额", "每天可花", "每月可花", "[p] 隐私", "[v] 视图", "[q] 退出"} {
		if !strings.Contains(output, expected) {
			t.Errorf("rendered output does not contain %q", expected)
		}
	}
	if strings.Contains(output, "\x1b[") {
		t.Fatal("plain rendering contains ANSI escapes")
	}
	if strings.Contains(output, "[s] ") || strings.Contains(output, "[d] ") {
		t.Fatalf("旧视图快捷键仍在底栏:\n%s", output)
	}
}

func TestCustomSloganIsCenteredAndHiddenWhenNarrow(t *testing.T) {
	config := defaultConfig()
	config.Slogan = "自定义口号"
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	if output := RenderDashboard(snapshot, config, 80, false); !strings.Contains(output, config.Slogan) {
		t.Fatalf("wide dashboard omitted slogan:\n%s", output)
	}
	if output := RenderDashboard(snapshot, config, 60, false); strings.Contains(output, config.Slogan) {
		t.Fatalf("narrow dashboard should hide slogan:\n%s", output)
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
	for _, forbidden := range []string{"0001-01", "退休倒计时", "实时存款余额", "每天可花"} {
		if strings.Contains(output, forbidden) {
			t.Errorf("disabled output contains %q", forbidden)
		}
	}
}

func TestBeforeAndAfterWorkUseMatchingCountdownText(t *testing.T) {
	config := defaultConfig()
	before, err := CalculateDashboard(time.Date(2026, time.July, 16, 8, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	beforeOutput := RenderDashboard(before, config, 80, false)
	if !strings.Contains(beforeOutput, "尚未上班") || !strings.Contains(beforeOutput, "距离上班 1h 00m") || strings.Contains(beforeOutput, "距离下班") {
		t.Fatalf("before-work output has inconsistent countdown:\n%s", beforeOutput)
	}

	after, err := CalculateDashboard(time.Date(2026, time.July, 16, 19, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	afterOutput := RenderDashboard(after, config, 80, false)
	if !strings.Contains(afterOutput, "已经下班") || strings.Contains(afterOutput, "距离下班") || strings.Contains(afterOutput, "距离上班") {
		t.Fatalf("after-work output has inconsistent countdown:\n%s", afterOutput)
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
		if strings.Contains(output, "每天可花") || strings.Contains(output, "当前存款") {
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
	if got := color("状态", "32", true); !strings.Contains(got, "\x1b[32m") {
		t.Fatalf("enabled color = %q", got)
	}
	if got := truncate("余薪 YUXIN", 6); got != "余薪 …" {
		t.Fatalf("truncate() = %q", got)
	}
	if runeWidth('\u0301') != 0 || runeWidth('薪') != 2 || runeWidth('⏳') != 2 || runeWidth('\uFFFD') != 1 || runeWidth('A') != 1 {
		t.Fatal("rune width variants failed")
	}
}

func TestPurchasingPowerQuipRanges(t *testing.T) {
	for _, test := range []struct {
		budget float64
		want   string
	}{
		{0.99, "馒头自由还差一点"},
		{1, "加蛋勉强，馒头管够"},
		{3, "够一瓶快乐水"},
		{6, "沙县小吃友情试吃"},
		{12, "勉强点个沙县外卖"},
		{25, "工作日午餐自由"},
		{50, "疯狂星期四，肆意疯狂"},
		{100, "脱离温饱，略有小康"},
		{200, "恭喜，退休生活开始体面"},
	} {
		if got := purchasingPowerQuip(test.budget); got != test.want {
			t.Errorf("purchasingPowerQuip(%.2f) = %q, want %q", test.budget, got, test.want)
		}
	}
}

func TestPurchasingPowerQuipAppearsInDepositTitleOnly(t *testing.T) {
	config := testFullConfig()
	config.Assets = 10000
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := renderDashboard(snapshot, config, 110, 30, false, false)
	want := "存款 · " + purchasingPowerQuip(snapshot.DailyUntilRetirement)
	if !strings.Contains(output, want) {
		t.Fatalf("deposit title missing %q:\n%s", want, output)
	}
	config.HideAmounts = true
	hidden := renderDashboard(snapshot, config, 110, 30, false, false)
	if strings.Contains(hidden, purchasingPowerQuip(snapshot.DailyUntilRetirement)) {
		t.Fatalf("privacy mode exposed purchasing-power title:\n%s", hidden)
	}
	config.HideAmounts = false
	config.HideRetirementDate = true
	hiddenRetirement := renderDashboard(snapshot, config, 110, 30, false, false)
	if strings.Contains(hiddenRetirement, purchasingPowerQuip(snapshot.DailyUntilRetirement)) {
		t.Fatalf("retirement privacy exposed purchasing-power title:\n%s", hiddenRetirement)
	}
	for _, sensitive := range []string{money(snapshot.TotalAssets), money(snapshot.DailyUntilRetirement), money(snapshot.Salary.EarnedToday)} {
		if strings.Contains(hiddenRetirement, sensitive) {
			t.Fatalf("legacy retirement privacy exposed %q:\n%s", sensitive, hiddenRetirement)
		}
	}
	if !strings.Contains(hiddenRetirement, "¥••••") {
		t.Fatalf("legacy retirement privacy did not render hidden placeholders:\n%s", hiddenRetirement)
	}
	compact := renderDashboard(snapshot, testFullConfig(), 80, 20, false, false)
	if strings.Contains(compact, "存款 ·") {
		t.Fatalf("compact layout included purchasing-power title:\n%s", compact)
	}
}

func TestStatusAndFormattingVariants(t *testing.T) {
	for status, expected := range map[string]string{
		"working": "正在上班", "before-work": "尚未上班", "lunch-break": "午休中",
		"after-work": "已经下班", "rest-day": "今日休息", "custom": "custom",
	} {
		text, _ := statusText(status)
		if !strings.Contains(text, expected) {
			t.Errorf("statusText(%q) = %q", status, text)
		}
	}
	for status, expected := range map[string]string{
		"before-work": "摸鱼先热身",
		"after-work":  "快乐已到账",
	} {
		text, _ := workStatus(DashboardSnapshot{Salary: SalarySnapshot{Status: status}}, defaultConfig())
		if !strings.Contains(text, expected) {
			t.Errorf("workStatus(%q) = %q", status, text)
		}
	}
	for status, expected := range map[string]string{
		"working": "/ 秒", "lunch-break": "余额暂停跳动", "after-work": "已封盘",
		"before-work": "尚未开张", "rest-day": "工资今天也休息",
	} {
		if got := incomeHintForDisplay(SalarySnapshot{Status: status, HourlyRate: 36}, false); !strings.Contains(got, expected) {
			t.Errorf("incomeHintForDisplay(%q) = %q", status, got)
		}
	}
	for seconds, expected := range map[int]string{0: "0s", 59: "59s", 60: "1m 00s", 3600: "1h 00m", 90061: "25h 01m"} {
		if got := duration(seconds); got != expected {
			t.Errorf("duration(%d) = %q, want %q", seconds, got, expected)
		}
	}
	if money(-1.999) != "-¥2.00" || money(999.999) != "¥1,000.00" {
		t.Fatal("money rounding variants failed")
	}
}

func TestFooterShowsPrivacyAndViewState(t *testing.T) {
	config := defaultConfig()
	snapshot := DashboardSnapshot{}
	if got := dashboardFooter(config, snapshot, false, 100); !strings.Contains(got, "[p] 隐私") || !strings.Contains(got, "[v] 视图") {
		t.Fatalf("default footer = %q", got)
	}
	config.HideAmounts = true
	if got := dashboardFooter(config, snapshot, false, 100); !strings.Contains(got, "隐私·金额") {
		t.Fatalf("amount privacy footer = %q", got)
	}
	config.HideRetirementDate = true
	if got := dashboardFooter(config, snapshot, false, 100); !strings.Contains(got, "隐私·全部") {
		t.Fatalf("full privacy footer = %q", got)
	}
	snapshot.DemoMode = true
	if got := dashboardFooter(config, snapshot, false, 100); !strings.Contains(got, "[v] 演示") {
		t.Fatalf("demo footer = %q", got)
	}
	snapshot.DemoMode = false
	if got := dashboardFooter(config, snapshot, true, 100); !strings.Contains(got, "[v] 详情") {
		t.Fatalf("details footer = %q", got)
	}
	if got := dashboardFooter(config, snapshot, true, 30); strings.Contains(got, "·全部") || strings.Contains(got, "详情") {
		t.Fatalf("narrow footer did not fall back = %q", got)
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
	for _, expected := range []string{"三点几啦，饮茶先！", "◆", "已过 25 天", "还剩 71 天"} {
		if !strings.Contains(output, expected) {
			t.Errorf("rendered output does not contain %q", expected)
		}
	}
}

func TestResponsiveDashboardLayout(t *testing.T) {
	config := testFullConfig()
	now := time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local)
	snapshot, err := CalculateDashboard(now, config)
	if err != nil {
		t.Fatal(err)
	}

	wide := RenderDashboard(snapshot, config, 100, false)
	foundSideBySide := false
	for _, line := range strings.Split(wide, "\n") {
		if strings.Contains(line, "退休倒计时") && strings.Contains(line, "存款") {
			foundSideBySide = true
			break
		}
	}
	if !foundSideBySide {
		t.Fatal("100-column layout did not render retirement and assets side by side")
	}

	medium := RenderDashboard(snapshot, config, 80, false)
	for _, line := range strings.Split(medium, "\n") {
		if strings.Contains(line, "退休倒计时") && strings.Contains(line, "存款") {
			t.Fatal("80-column layout unexpectedly rendered panels side by side")
		}
	}

	narrow := RenderDashboard(snapshot, config, 60, false)
	for _, expected := range []string{"本地数据 ✓", "下班", "未来", "实时存款余额"} {
		if !strings.Contains(narrow, expected) {
			t.Errorf("60-column output does not contain %q", expected)
		}
	}
	for _, forbidden := range []string{"09:00", "18:00", "◆"} {
		if strings.Contains(narrow, forbidden) {
			t.Errorf("60-column output contains %q", forbidden)
		}
	}
}

func TestDashboardDetailsView(t *testing.T) {
	config := defaultConfig()
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}

	details := renderDashboard(snapshot, config, 80, 24, false, true)
	if !strings.Contains(details, "计算口径") || strings.Contains(details, "当前存款") {
		t.Fatalf("unexpected details view:\n%s", details)
	}
}

func TestDemoDashboardIsClearlyMarked(t *testing.T) {
	snapshot, config, err := DemoDashboard()
	if err != nil {
		t.Fatal(err)
	}
	output := RenderDashboard(snapshot, config, 80, false)
	for _, expected := range []string{"余薪 YUXIN · 演示模式", "演示数据 ✓", "¥258,521.38", "距离退休"} {
		if !strings.Contains(output, expected) {
			t.Errorf("demo output does not contain %q", expected)
		}
	}
	tiny := RenderDashboard(snapshot, config, 23, false)
	if !strings.Contains(tiny, "演示模式") {
		t.Fatalf("tiny demo output is not clearly marked:\n%s", tiny)
	}
}

func TestShortTerminalUsesCompactPanels(t *testing.T) {
	config := testFullConfig()
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := renderDashboard(snapshot, config, 100, 23, false, false)
	if !strings.Contains(output, "未来") || strings.Contains(output, "╭─ 存款") || strings.Contains(output, "◆") {
		t.Fatalf("short terminal did not use compact layout:\n%s", output)
	}
}

func TestStandard80By24TerminalDoesNotOverflow(t *testing.T) {
	config := testFullConfig()
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := renderDashboard(snapshot, config, 80, 24, false, false)
	lines := strings.Split(output, "\n")
	if len(lines) > 24 || !strings.Contains(output, "╭─ 未来") || !strings.Contains(output, "退休还有") || !strings.Contains(output, "实时存款余额") {
		t.Fatalf("80x24 layout uses %d lines:\n%s", len(lines), output)
	}
}

func TestPrivacySettingsHideDashboardFields(t *testing.T) {
	config := testFullConfig()
	config.HideAmounts = true
	config.HideRetirementDate = true
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := RenderDashboard(snapshot, config, 100, false)
	for _, leaked := range []string{"¥286", "¥100,000", snapshot.Retirement.RetirementMonth.Format("2006-01")} {
		if strings.Contains(output, leaked) {
			t.Fatalf("privacy output leaked %q:\n%s", leaked, output)
		}
	}
	if !strings.Contains(output, "¥••••") || !strings.Contains(output, "退休信息") || strings.Contains(output, "距离退休") || strings.Contains(output, "退休进度") {
		t.Fatalf("privacy settings were not applied:\n%s", output)
	}
}

func TestAmountOnlyPrivacyKeepsRetirementData(t *testing.T) {
	config := testFullConfig()
	config.HideAmounts = true
	config.HideRetirementDate = false
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := RenderDashboard(snapshot, config, 100, false)
	if !strings.Contains(output, "¥••••") || !strings.Contains(output, "距离退休") || !strings.Contains(output, "退休进度") {
		t.Fatalf("amount-only privacy hid the wrong fields:\n%s", output)
	}
	for _, leaked := range []string{"¥227", "¥100,227"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("amount-only privacy leaked %q:\n%s", leaked, output)
		}
	}
}

func TestRetirementShowsIntegerTotalUnitsAndProgress(t *testing.T) {
	config := testFullConfig()
	config.RetirementMode = "countdown"
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := RenderDashboard(snapshot, config, 80, false)
	days := snapshot.Retirement.RemainingDays
	expectedValues := []string{
		fmt.Sprintf("%d 年", int(float64(days)/365.2425)),
		fmt.Sprintf("%d 个月", int(float64(days)/30.436875)),
		commaInt(days) + " 天",
		"退休进度",
	}
	for _, expected := range expectedValues {
		if !strings.Contains(output, expected) {
			t.Fatalf("retirement panel missing %q:\n%s", expected, output)
		}
	}
	for _, expected := range []string{"🏁 退休倒计时", "距离退休", "按月计算", "按天计算"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("退休卡片缺少 %q:\n%s", expected, output)
		}
	}
	progressIndex := strings.Index(output, "退休进度")
	yearsIndex := strings.Index(output, expectedValues[0])
	if progressIndex < 0 || yearsIndex < 0 || progressIndex > yearsIndex {
		t.Fatalf("退休进度未显示在倒计时数据之前:\n%s", output)
	}
	for _, hidden := range []string{"预计退休", "法定", "个工作日"} {
		if strings.Contains(output, hidden) {
			t.Fatalf("countdown mode contains %q:\n%s", hidden, output)
		}
	}
	if got := retirementDistance(DashboardSnapshot{
		Now:        time.Date(2026, time.July, 16, 0, 0, 0, 0, time.Local),
		Retirement: RetirementSnapshot{RetirementMonth: time.Date(2058, time.January, 1, 0, 0, 0, 0, time.Local)},
	}); got != "31 年 5 个月 16 天" {
		t.Fatalf("retirementDistance = %q", got)
	}
}

func TestFuturePanelsUseParallelCopyAndMatchingHeights(t *testing.T) {
	config := testFullConfig()
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := renderDashboard(snapshot, config, 110, 30, false, false)
	for _, expected := range []string{
		"🏁 退休倒计时", "💰 存款 ·", "如果现在躺平：",
		"├─ 每天可花", "└─ 每月可花",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("未来卡片缺少 %q:\n%s", expected, output)
		}
	}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "└─ 按天计算") && !strings.Contains(line, "└─ 每月可花") {
			t.Fatalf("并排卡片未对齐:\n%s", output)
		}
	}
	if strings.Contains(output, "每年可花") {
		t.Fatalf("存款卡片不应重复展示年度换算:\n%s", output)
	}
}

func TestFuturePanelsUseSemanticColors(t *testing.T) {
	config := testFullConfig()
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := renderDashboard(snapshot, config, 110, 30, true, false)
	for _, expected := range []string{
		"\x1b[36m🏁 退休倒计时",
		"\x1b[32m💰 存款",
		"\x1b[33m" + fmt.Sprintf("%d 年", int(float64(snapshot.Retirement.RemainingDays)/averageDaysPerYear)),
		"\x1b[32m" + displayMoney(snapshot.LiveBalance, false),
		"\x1b[36m" + displayMoney(snapshot.DailyUntilRetirement, false),
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("未来卡片缺少语义色 %q:\n%s", expected, output)
		}
	}

	config.HideAmounts = true
	hidden := renderDashboard(snapshot, config, 110, 30, true, false)
	if strings.Contains(hidden, "\x1b[32m¥••••") || strings.Contains(hidden, "\x1b[36m¥••••") {
		t.Fatalf("隐私占位符不应使用金额强调色:\n%s", hidden)
	}
}

func TestSavingsTargetShowsProgressAndGap(t *testing.T) {
	config := testFullConfig()
	config.TargetMonthlySpend = 3000
	snapshot, err := CalculateDashboard(time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local), config)
	if err != nil {
		t.Fatal(err)
	}
	output := RenderDashboard(snapshot, config, 110, false)
	for _, expected := range []string{"存款目标", "每天", "每月", "¥3,000.00", "每年", "¥36,000.00", "目标进度", "距离目标还差"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("savings target missing %q:\n%s", expected, output)
		}
	}

	config.HideAmounts = true
	hidden := RenderDashboard(snapshot, config, 110, false)
	if strings.Contains(hidden, "¥3,000.00") || !strings.Contains(hidden, "存款目标") || !strings.Contains(hidden, "已隐藏") {
		t.Fatalf("savings target privacy failed:\n%s", hidden)
	}
}

func TestHolidayTextActiveAndFirstPeriod(t *testing.T) {
	active := HolidaySnapshot{
		Name: "国庆节", DaysUntil: 0,
		Start: time.Date(2026, time.October, 1, 0, 0, 0, 0, time.Local),
		End:   time.Date(2026, time.October, 7, 0, 0, 0, 0, time.Local),
	}
	if got := holidayText(active); !strings.Contains(got, "共 7 天") {
		t.Fatalf("active holiday text = %q", got)
	}
	upcoming := HolidaySnapshot{
		Name: "元旦", DaysUntil: 3,
		Start: time.Date(2027, time.January, 1, 0, 0, 0, 0, time.Local),
	}
	if got := holidayText(upcoming); !strings.Contains(got, "下个假期") || !strings.Contains(got, "还有 3 天") {
		t.Fatalf("first holiday text = %q", got)
	}
}
