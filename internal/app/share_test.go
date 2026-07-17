package app

import (
	"strings"
	"testing"
)

func TestRenderShareCardOverview(t *testing.T) {
	snapshot, config, err := DemoDashboard()
	if err != nil {
		t.Fatal(err)
	}
	card, err := RenderShareCard(snapshot, config, "overview")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"摸鱼有数，下班有期", "演示数据", "今日工作进度", "今日已经赚到",
		"盼头", "退休", "存款目标进度", "躺平生存指南", "离线本地运行", "github.com/wnma3mz/yuxin",
	} {
		if !strings.Contains(card, want) {
			t.Fatalf("card does not contain %q:\n%s", want, card)
		}
	}
	assertShareCardWidth(t, card)
	if ansiPattern.MatchString(card) {
		t.Fatalf("share card contains ANSI escape codes: %q", card)
	}
}

func TestRenderShareCardWorkday(t *testing.T) {
	snapshot, config, err := DemoDashboard()
	if err != nil {
		t.Fatal(err)
	}
	card, err := RenderShareCard(snapshot, config, "workday")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"工作日倒计时", "今日状态", "下班倒计时", "已工作", "工作进度",
		"离线本地运行", "数据绝不上云", "github.com/wnma3mz/yuxin",
	} {
		if !strings.Contains(card, want) {
			t.Fatalf("card does not contain %q:\n%s", want, card)
		}
	}
	if strings.Contains(card, "预计退休") || strings.Contains(card, "本地存款") {
		t.Fatalf("workday card should stay focused:\n%s", card)
	}
	assertShareCardWidth(t, card)
}

func TestRenderShareCardHonorsPrivacySettings(t *testing.T) {
	snapshot, config, err := DemoDashboard()
	if err != nil {
		t.Fatal(err)
	}
	config.HideAmounts = true
	config.HideRetirementDate = true
	card, err := RenderShareCard(snapshot, config, "overview")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(card, "¥") || strings.Contains(card, "2056") {
		t.Fatalf("privacy card leaked sensitive values:\n%s", card)
	}
	if count := strings.Count(card, "已隐藏"); count < 3 {
		t.Fatalf("expected hidden salary, retirement date, and assets; got %d:\n%s", count, card)
	}
}

func TestRenderShareCardNormalizesLegacyRetirementPrivacy(t *testing.T) {
	snapshot, config, err := DemoDashboard()
	if err != nil {
		t.Fatal(err)
	}
	config.HideAmounts = false
	config.HideRetirementDate = true
	card, err := RenderShareCard(snapshot, config, "overview")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(card, "¥") || strings.Contains(card, "2056") {
		t.Fatalf("legacy retirement privacy leaked sensitive values:\n%s", card)
	}
}

func TestShareCardUsesWorkStateCountdownLabel(t *testing.T) {
	config := defaultConfig()
	before := DashboardSnapshot{Salary: SalarySnapshot{Status: "before-work", RemainingSeconds: 3600}}
	card, err := RenderShareCard(before, config, "overview")
	if err != nil || !strings.Contains(card, "距上班 1h 00m") || strings.Contains(card, "剩 1h 00m") {
		t.Fatalf("before-work share countdown is inconsistent: %v\n%s", err, card)
	}
	after := DashboardSnapshot{Salary: SalarySnapshot{Status: "after-work"}}
	card, err = RenderShareCard(after, config, "overview")
	if err != nil || !strings.Contains(card, "今日工作进度") || !strings.Contains(card, "已经下班") {
		t.Fatalf("after-work share status is inconsistent: %v\n%s", err, card)
	}
}

func TestShareCardDescribesPrivacyStateTruthfully(t *testing.T) {
	snapshot, config, err := DemoDashboard()
	if err != nil {
		t.Fatal(err)
	}
	card, err := RenderShareCard(snapshot, config, "overview")
	if err != nil || !strings.Contains(card, "已使用固定合成数据") || strings.Contains(card, "尚未开启隐私") {
		t.Fatalf("demo card privacy label is misleading: %v\n%s", err, card)
	}

	snapshot.DemoMode = false
	card, err = RenderShareCard(snapshot, config, "overview")
	if err != nil || !strings.Contains(card, "尚未开启隐私保护") {
		t.Fatalf("real card should warn when privacy is off: %v\n%s", err, card)
	}
	config.HideAmounts = true
	card, err = RenderShareCard(snapshot, config, "overview")
	if err != nil || !strings.Contains(card, "金额和存款已隐藏") {
		t.Fatalf("amount privacy label is missing: %v\n%s", err, card)
	}
	config.HideRetirementDate = true
	card, err = RenderShareCard(snapshot, config, "overview")
	if err != nil || !strings.Contains(card, "金额、存款和退休信息已隐藏") {
		t.Fatalf("full privacy label is missing: %v\n%s", err, card)
	}
}

func TestShareCardTruncatesLongCustomSlogan(t *testing.T) {
	snapshot, config, err := DemoDashboard()
	if err != nil {
		t.Fatal(err)
	}
	config.Slogan = strings.Repeat("今天也要准时下班", 5)
	card, err := RenderShareCard(snapshot, config, "overview")
	if err != nil {
		t.Fatal(err)
	}
	assertShareCardWidth(t, card)
}

func TestRenderShareCardRejectsUnknownType(t *testing.T) {
	if _, err := RenderShareCard(DashboardSnapshot{}, Config{}, "poster"); err == nil {
		t.Fatal("expected an error for unknown card type")
	}
}

func assertShareCardWidth(t *testing.T, card string) {
	t.Helper()
	for index, line := range strings.Split(card, "\n") {
		if width := displayWidth(line); width != shareCardWidth {
			t.Fatalf("line %d has width %d, want %d: %q", index+1, width, shareCardWidth, line)
		}
	}
}
