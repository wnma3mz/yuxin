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
		"概览分享卡", "演示数据", "今日入账", "下班倒计时",
		"节假日", "预计退休", "本地资产", "无账号", "离线运行", "数据只在本地",
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
		"无账号", "离线运行", "数据只在本地",
	} {
		if !strings.Contains(card, want) {
			t.Fatalf("card does not contain %q:\n%s", want, card)
		}
	}
	if strings.Contains(card, "预计退休") || strings.Contains(card, "本地资产") {
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
	if count := strings.Count(card, "已隐藏"); count < 4 {
		t.Fatalf("expected hidden salary, retirement date, and assets; got %d:\n%s", count, card)
	}
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
