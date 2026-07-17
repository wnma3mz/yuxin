package app

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigWizardEditsRefreshInterval(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("5\n1\ninvalid\n0.5\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RefreshInterval != 500*time.Millisecond || !strings.Contains(output.String(), "0.1 到 3600") {
		t.Fatalf("refresh interval = %s, output %q", updated.RefreshInterval, output.String())
	}
}

func TestConfigWizardEditsSlogan(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("5\n2\n今日有数，下班有期。\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Slogan != "今日有数，下班有期。" {
		t.Fatalf("slogan = %q", updated.Slogan)
	}
}

func TestWizardAmountRejectsValuesAboveLimit(t *testing.T) {
	var output bytes.Buffer
	wizard := configWizard{
		input:  strings.NewReader("1000000000001\n100\n"),
		reader: bufio.NewReader(strings.NewReader("")),
		out:    &output,
	}
	wizard.reader = bufio.NewReader(wizard.input)
	amount, err := wizard.amount("金额", 0, false)
	if err != nil || amount != 100 {
		t.Fatalf("amount = %v, error %v", amount, err)
	}
	if !strings.Contains(output.String(), configNumber(maxMoneyAmount)) {
		t.Fatalf("limit prompt missing from %q", output.String())
	}
}

func TestConfigWizardSetsSingleDeposit(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	input := "4\n3w\n0\n0\n"
	updated, err := configureConfig(strings.NewReader(input), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Assets != 30000 || len(updated.AssetItems) != 1 || updated.AssetItems[0].Name != "存款" || updated.AssetItems[0].Kind != "deposit" {
		t.Fatalf("deposit = %.2f, items = %#v", updated.Assets, updated.AssetItems)
	}
}

func TestConfigWizardChangesOnlySelectedSection(t *testing.T) {
	config := testFullConfig()
	config.StartSecond = 10 * 3600
	config.EndSecond = 19 * 3600
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := saveConfig(config, path); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("1\n1\n9000\n\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.SalaryAmount != 9000 || updated.StartSecond != 10*3600 || updated.Assets != 100000 {
		t.Fatalf("unexpected updated config: %#v", updated)
	}
	loaded, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SalaryAmount != 9000 || loaded.StartSecond != 10*3600 || loaded.Assets != 100000 {
		t.Fatalf("saved config did not round trip: %#v", loaded)
	}
}

func TestConfigWizardAcceptsCompactAssetAmount(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("4\n200k\n0\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Assets != 200000 || len(updated.AssetItems) != 1 || updated.AssetItems[0].Balance != 200000 {
		t.Fatalf("assets = %#v, total %.2f", updated.AssetItems, updated.Assets)
	}
}

func TestEditingDepositCollapsesLegacyAccounts(t *testing.T) {
	config := testFullConfig()
	config.AssetItems = []AssetItem{
		{Name: "工资卡", Kind: "checking", Balance: 100000},
		{Name: "定期", Kind: "deposit", Balance: 300000},
	}
	config.Assets = 400000
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("4\n20w\n0\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.AssetItems) != 1 || updated.AssetItems[0].Name != "存款" || updated.AssetItems[0].Balance != 200000 {
		t.Fatalf("legacy accounts were not simplified: %#v", updated.AssetItems)
	}
}

func TestInvalidClockAndChoicesAreNotSilentlySaved(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	input := "2\n\ninvalid\n10:00\n19:00\n\n0\n"
	updated, err := configureConfig(strings.NewReader(input), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.StartSecond != 10*3600 || updated.EndSecond != 19*3600 {
		t.Fatalf("schedule = %s-%s", clock(updated.StartSecond), clock(updated.EndSecond))
	}
	if !strings.Contains(output.String(), "必须使用 HH:MM") {
		t.Fatal("invalid clock did not produce a validation message")
	}
}

func TestScheduleUsesSingleLunchDuration(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("2\n\n14:00\n22:00\n90\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.LunchEnabled || updated.LunchEnd-updated.LunchStart != 90*60 || updated.LunchStart != 17*3600+15*60 {
		t.Fatalf("lunch = enabled %t, %s-%s", updated.LunchEnabled, clock(updated.LunchStart), clock(updated.LunchEnd))
	}

	var disabledOutput bytes.Buffer
	disabled, err := configureConfig(strings.NewReader("2\n\n\n\n0\n0\n"), &disabledOutput, filepath.Join(t.TempDir(), "config.toml"), config)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.LunchEnabled {
		t.Fatal("zero lunch duration did not disable lunch")
	}
}

func TestZeroDepositClosesAssets(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("4\n0\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.AssetsEnabled || updated.Assets != 0 || len(updated.AssetItems) != 0 || updated.Reserve != 0 || updated.TargetMonthlySpend != 0 {
		t.Fatalf("zero deposit did not close assets: %#v", updated)
	}
}

func TestConfigWizardSetsMonthlySavingsTarget(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("4\n10w\n3000\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Assets != 100000 || updated.TargetMonthlySpend != 3000 {
		t.Fatalf("savings target = %#v", updated)
	}
	if !strings.Contains(output.String(), "目标每月可花") {
		t.Fatalf("target prompt missing: %q", output.String())
	}
}

func TestConfigWizardSetsRetirementFromAgeAndDefaultsToMale(t *testing.T) {
	config := testFullConfig()
	config.ProfileEnabled = false
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("3\n1\n30\n\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.ProfileEnabled || updated.RetirementYears != 0 || updated.RetirementMode != "full" || updated.Sex != "male" {
		t.Fatalf("retirement = %#v", updated)
	}
	if ageOnDate(updated.BirthDate, configDateOnly(time.Now())) != 30 {
		t.Fatalf("birth date was not derived from age: %s", updated.BirthDate)
	}
}

func TestClosingRetirementClearsPersonalProfile(t *testing.T) {
	config := testFullConfig()
	config.BirthDate = mustDate("1990-06-18")
	config.ProgressBirthDate = config.BirthDate
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("3\n0\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProfileEnabled || !updated.BirthDate.IsZero() || updated.Sex != "" || updated.ProgressBirthDate.Format("2006-01-02") == "1990-06-18" {
		t.Fatalf("retirement profile was retained: %#v", updated)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "1990-06-18") || strings.Contains(string(content), "[profile]") {
		t.Fatalf("saved config retained retirement profile:\n%s", content)
	}
}

func TestParseAgeOrBirth(t *testing.T) {
	today := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.Local)
	tests := map[string]string{
		"30":         "1996-07-16",
		"1995-06":    "1995-06-01",
		"1995/6/18":  "1995-06-18",
		"1995年6月":    "1995-06-01",
		"1995年6月18日": "1995-06-18",
	}
	for input, want := range tests {
		birth, err := parseAgeOrBirth(input, today)
		if err != nil || birth.Format("2006-01-02") != want {
			t.Errorf("parseAgeOrBirth(%q) = %s, %v; want %s", input, birth, err, want)
		}
	}
	for _, input := range []string{"0", "101", "2027-01", "not-a-date"} {
		if _, err := parseAgeOrBirth(input, today); err == nil {
			t.Errorf("parseAgeOrBirth(%q) unexpectedly succeeded", input)
		}
	}
}

func TestSaveConfigPreservesMultipleAccounts(t *testing.T) {
	config := testFullConfig()
	config.RefreshInterval = 500 * time.Millisecond
	config.AssetItems = []AssetItem{
		{Name: "工资卡", Kind: "checking", Balance: 12345.67},
		{Name: "定期", Kind: "deposit", Balance: 200000},
	}
	config.Assets = 212345.67
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := saveConfig(config, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RefreshInterval != 500*time.Millisecond || len(loaded.AssetItems) != 2 || loaded.AssetItems[1].Kind != "deposit" || loaded.Assets != config.Assets {
		t.Fatalf("round trip = %#v", loaded)
	}
}

func TestConfigWizardEditsRetirement(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	input := "3\n1\n1990-06\n2\n0\n"
	updated, err := configureConfig(strings.NewReader(input), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.HideAmounts || updated.HideRetirementDate || updated.RetirementMode != "full" || !updated.ProfileEnabled || updated.Sex != "female" || updated.BirthDate.Format("2006-01-02") != "1990-06-01" {
		t.Fatalf("updated display config = %#v", updated)
	}
}

func TestWizardSummaryHonorsPrivacySettings(t *testing.T) {
	config := testFullConfig()
	config.SalaryAmount = 12345
	config.Assets = 98765
	config.BirthDate = mustDate("1992-02-03")
	config.HideAmounts = true
	config.HideRetirementDate = true
	var output bytes.Buffer
	configWizard{out: &output}.summary(config)
	for _, secret := range []string{"12,345", "98,765", "1992-02-03"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("summary leaked %q: %s", secret, output.String())
		}
	}
	if !strings.Contains(output.String(), "¥••••") || strings.Contains(output.String(), "隐私模式") {
		t.Fatalf("summary did not show privacy state: %s", output.String())
	}
}

func TestConfigWizardDoesNotSaveNoOpEdit(t *testing.T) {
	config := testFullConfig()
	config.AssetItems = []AssetItem{{Name: "存款", Kind: "deposit", Balance: config.Assets}}
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("4\n\n0\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "未修改") {
		t.Fatalf("output = %q", output.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("no-op edit wrote config: %v", err)
	}
	if updated.HideAmounts != config.HideAmounts || updated.HideRetirementDate != config.HideRetirementDate {
		t.Fatalf("no-op changed privacy: %#v", updated)
	}
}

func TestPrivacyModeCyclesThroughThreeStates(t *testing.T) {
	config := testFullConfig()
	amounts := cyclePrivacy(config)
	if !amounts.HideAmounts || amounts.HideRetirementDate {
		t.Fatalf("first privacy state = %#v", amounts)
	}
	all := cyclePrivacy(amounts)
	if !all.HideAmounts || !all.HideRetirementDate {
		t.Fatalf("second privacy state = %#v", all)
	}
	off := cyclePrivacy(all)
	if off.HideAmounts || off.HideRetirementDate {
		t.Fatalf("third privacy state = %#v", off)
	}
}
