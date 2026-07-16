package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadRepositoryConfig(t *testing.T) {
	config, err := loadConfig("data/default-config.toml")
	if err != nil {
		t.Fatalf("loadConfig(data/default-config.toml): %v", err)
	}

	if config.RefreshInterval != time.Second {
		t.Errorf("RefreshInterval = %s, want 1s", config.RefreshInterval)
	}
	if config.RetirementYears != 30 {
		t.Errorf("RetirementYears = %d, want 30", config.RetirementYears)
	}
	if got := config.RetirementStart.Format("2006-01-02"); got != "2026-07-16" {
		t.Errorf("RetirementStart = %s, want 2026-07-16", got)
	}
	if config.SalaryMode != "monthly" || config.SalaryAmount != 8000 || config.MonthlyWorkdays != 22 {
		t.Errorf("salary config = %q %.2f %.2f", config.SalaryMode, config.SalaryAmount, config.MonthlyWorkdays)
	}
	if len(config.Workdays) != 5 || !config.Workdays[time.Monday] || !config.Workdays[time.Friday] {
		t.Errorf("Workdays = %#v, want Monday through Friday", config.Workdays)
	}
	if config.Assets != 100000 || config.Reserve != 0 {
		t.Errorf("assets/reserve = %.2f/%.2f, want 100000/0", config.Assets, config.Reserve)
	}
}

func TestLoadConfigUsesDefaultsForMinimalFile(t *testing.T) {
	path := writeTestConfig(t, "version = 1\n")
	config, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	want := defaultConfig()
	if config.SalaryAmount != want.SalaryAmount || config.Assets != want.Assets || config.Sex != want.Sex {
		t.Errorf("minimal config did not preserve defaults: %#v", config)
	}
}

func TestLoadConfigSumsAssetsAndParsesAmountSuffixes(t *testing.T) {
	path := writeTestConfig(t, `
reserve = "20w"

[[assets]]
balance = "20万"

[[assets]]
balance = "200k"
`)
	config, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if config.Assets != 400000 {
		t.Errorf("Assets = %.2f, want 400000", config.Assets)
	}
	if config.Reserve != 200000 {
		t.Errorf("Reserve = %.2f, want 200000", config.Reserve)
	}
}

func TestParseAmount(t *testing.T) {
	tests := map[string]float64{
		"20w":     200000,
		"20万":     200000,
		"200k":    200000,
		"200,000": 200000,
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := parseAmount(input)
			if err != nil {
				t.Fatalf("parseAmount(%q): %v", input, err)
			}
			if got != want {
				t.Errorf("parseAmount(%q) = %.2f, want %.2f", input, got, want)
			}
		})
	}

	for _, input := range []string{"nope", "NaN", "Inf"} {
		if _, err := parseAmount(input); err == nil {
			t.Errorf("parseAmount(%q) unexpectedly succeeded", input)
		}
	}
}

func TestLoadConfigValidation(t *testing.T) {
	tests := map[string]string{
		"future version":     "version = 2\n",
		"unknown key":        "mystery = true\n",
		"unknown table":      "[mystery]\n",
		"duplicate key":      "refresh_interval = 1\nrefresh_interval = 2\n",
		"duplicate balance":  "[[assets]]\nbalance = \"10w\"\nbalance = \"20w\"\n",
		"refresh interval":   "refresh_interval = 0\n",
		"salary mode":        "[salary]\nmode = \"weekly\"\n",
		"empty workdays":     "[schedule]\nworkdays = []\n",
		"workday range":      "[schedule]\nworkdays = [0, 8]\n",
		"work time":          "[schedule]\nstart = \"18:00\"\nend = \"09:00\"\n",
		"lunch time":         "[schedule]\nlunch_start = \"08:00\"\n",
		"negative asset":     "[[assets]]\nbalance = \"-1\"\n",
		"one negative asset": "[[assets]]\nbalance = \"-1\"\n[[assets]]\nbalance = \"2\"\n",
		"future birth":       "[profile]\nbirth_date = \"2999-01-01\"\n",
		"female track":       "[profile]\nsex = \"female\"\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := loadConfig(writeTestConfig(t, content)); err == nil {
				t.Fatal("loadConfig unexpectedly succeeded")
			}
		})
	}
}

func TestDisabledDefaults(t *testing.T) {
	config, err := loadConfig(writeTestConfig(t, "assets_enabled = false\nprofile_enabled = false\n"))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if config.Assets != 0 || config.ProfileEnabled {
		t.Errorf("disabled defaults = assets %.2f, profile %t", config.Assets, config.ProfileEnabled)
	}
}

func TestExplicitProfileRequiresBirthDateAndSex(t *testing.T) {
	for name, content := range map[string]string{
		"missing sex":   "[profile]\nbirth_date = \"1980-01-01\"\n",
		"missing birth": "[profile]\nsex = \"male\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadConfig(writeTestConfig(t, content)); err == nil {
				t.Fatal("loadConfig unexpectedly inherited a default profile field")
			}
		})
	}
}

func TestExplicitAssetsOverrideDisabledDefault(t *testing.T) {
	config, err := loadConfig(writeTestConfig(t, "assets_enabled = false\n[[assets]]\nbalance = \"200000\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !config.AssetsEnabled || config.Assets != 200000 {
		t.Fatalf("explicit assets = enabled %t, balance %.2f", config.AssetsEnabled, config.Assets)
	}
}

func TestQuotedHashIsNotAComment(t *testing.T) {
	config, err := loadConfig(writeTestConfig(t, "[salary]\nmode = \"monthly#not-comment\"\n"))
	if err == nil || !strings.Contains(err.Error(), "薪资模式") {
		t.Fatalf("loadConfig error = %v, want salary mode validation error", err)
	}
	if config.SalaryMode != "monthly#not-comment" {
		t.Errorf("SalaryMode = %q, quoted hash was stripped", config.SalaryMode)
	}
}

func TestParseAmountDoesNotReturnNonFiniteValue(t *testing.T) {
	for _, input := range []string{"1e999", "-1e999"} {
		got, err := parseAmount(input)
		if err == nil || (!math.IsInf(got, 0) && got != 0) {
			t.Errorf("parseAmount(%q) = %v, %v", input, got, err)
		}
	}
}

func TestSaveConfigDoesNotDamageExistingFileOnValidationError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	config := defaultConfig()
	if err := saveConfig(config, path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config.SalaryAmount = 0
	if err := saveConfig(config, path); err == nil {
		t.Fatal("invalid config unexpectedly saved")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("failed save changed existing config")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions = %o, want 600", info.Mode().Perm())
	}
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
