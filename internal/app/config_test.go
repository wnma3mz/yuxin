package app

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
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
	if config.RetirementYears != 0 || config.ProfileEnabled {
		t.Errorf("retirement defaults = years %d, profile %t; want disabled", config.RetirementYears, config.ProfileEnabled)
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
	if config.AssetsEnabled || config.Assets != 0 || config.Reserve != 0 {
		t.Errorf("asset defaults = enabled %t, assets %.2f, reserve %.2f; want disabled", config.AssetsEnabled, config.Assets, config.Reserve)
	}
	if config.RetirementMode != "full" || config.RetirementUnit != "days" || config.HideAmounts || config.HideRetirementDate {
		t.Errorf("display defaults = mode %q, unit %q, hide amounts %t, hide retirement %t", config.RetirementMode, config.RetirementUnit, config.HideAmounts, config.HideRetirementDate)
	}
}

func TestCreateDefaultConfigUsesPrivateFocusedDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	if err := createDefaultConfig(path); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.ProfileEnabled || config.AssetsEnabled || config.RetirementYears != 0 {
		t.Fatalf("optional modules should be disabled: %#v", config)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("default config permissions = %o", info.Mode().Perm())
	}
	if err := createDefaultConfig(path); err != nil {
		t.Fatalf("creating an existing default should be harmless: %v", err)
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

func TestScalarAndBooleanParserErrors(t *testing.T) {
	if _, err := parseBool("yes"); err == nil {
		t.Fatal("parseBool accepted a non-boolean value")
	}
	for _, value := range []string{"", "\"unterminated"} {
		if _, err := scalarValue(value); err == nil {
			t.Fatalf("scalarValue(%q) unexpectedly succeeded", value)
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
		"retirement mode":    "retirement_mode = \"verbose\"\n",
		"retirement unit":    "retirement_unit = \"weeks\"\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := loadConfig(writeTestConfig(t, content)); err == nil {
				t.Fatal("loadConfig unexpectedly succeeded")
			}
		})
	}
}

func TestLoadConfigRejectsUnsafeOrUnboundedAssets(t *testing.T) {
	for name, content := range map[string]string{
		"escape":    "[[assets]]\nname = \"bad\\u001bname\"\n",
		"newline":   "[[assets]]\nname = \"bad\\nname\"\n",
		"long name": "[[assets]]\nname = \"" + strings.Repeat("余", maxAssetNameRunes+1) + "\"\n",
		"too many":  strings.Repeat("[[assets]]\n", maxAssetItems+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadConfig(writeTestConfig(t, content)); err == nil {
				t.Fatal("loadConfig unexpectedly accepted unsafe assets")
			}
		})
	}
}

func TestLoadConfigRejectsOversizedOrNonRegularFile(t *testing.T) {
	oversized := writeTestConfig(t, strings.Repeat("# x\n", maxConfigFileSize/4+1))
	if _, err := loadConfig(oversized); err == nil || !strings.Contains(err.Error(), "不能超过") {
		t.Fatalf("oversized config error = %v", err)
	}
	if _, err := loadConfig(t.TempDir()); err == nil || !strings.Contains(err.Error(), "普通文件") {
		t.Fatalf("directory config error = %v", err)
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
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestDisplayAndPrivacyConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	config := defaultConfig()
	config.RetirementMode = "countdown"
	config.RetirementUnit = "workdays"
	config.HideAmounts = true
	config.HideRetirementDate = true
	if err := saveConfig(config, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RetirementMode != "countdown" || loaded.RetirementUnit != "workdays" || !loaded.HideAmounts || !loaded.HideRetirementDate {
		t.Fatalf("round trip display config = %#v", loaded)
	}
}

func TestValidateConfigRejectsInvalidFields(t *testing.T) {
	tests := map[string]func(*Config){
		"retirement years": func(config *Config) { config.RetirementYears = 101 },
		"future progress birth": func(config *Config) {
			config.ProgressBirthDate = configDateOnly(time.Now()).AddDate(0, 0, 1)
		},
		"retirement mode": func(config *Config) { config.RetirementMode = "verbose" },
		"retirement unit": func(config *Config) { config.RetirementUnit = "weeks" },
		"salary mode":     func(config *Config) { config.SalaryMode = "weekly" },
		"salary amount":   func(config *Config) { config.SalaryAmount = 0 },
		"monthly workdays": func(config *Config) {
			config.MonthlyWorkdays = 0
		},
		"empty workdays": func(config *Config) { config.Workdays = nil },
		"work hours":     func(config *Config) { config.EndSecond = config.StartSecond },
		"lunch hours":    func(config *Config) { config.LunchStart = config.StartSecond - 1 },
		"missing profile birth": func(config *Config) {
			config.ProfileEnabled = true
			config.BirthDate = time.Time{}
		},
		"invalid profile sex": func(config *Config) {
			config.ProfileEnabled = true
			config.Sex = "other"
		},
		"future profile birth": func(config *Config) {
			config.ProfileEnabled = true
			config.BirthDate = configDateOnly(time.Now()).AddDate(0, 0, 1)
		},
		"female track": func(config *Config) {
			config.ProfileEnabled = true
			config.Sex = "female"
			config.FemaleTrack = "51"
		},
		"negative reserve": func(config *Config) { config.Reserve = -1 },
		"negative assets":  func(config *Config) { config.Assets = -1 },
		"negative account": func(config *Config) {
			config.AssetItems = []AssetItem{{Balance: -1}}
		},
		"too many accounts": func(config *Config) {
			config.AssetItems = make([]AssetItem, maxAssetItems+1)
		},
		"account control": func(config *Config) {
			config.AssetItems = []AssetItem{{Name: "bad\x1bname"}}
		},
		"account name length": func(config *Config) {
			config.AssetItems = []AssetItem{{Name: strings.Repeat("余", maxAssetNameRunes+1)}}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			config := defaultConfig()
			mutate(&config)
			if err := validateConfig(config); err == nil {
				t.Fatal("validateConfig unexpectedly succeeded")
			}
		})
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
