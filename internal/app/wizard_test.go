package app

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigWizardEditsRefreshInterval(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("3\ninvalid\n0.5\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RefreshInterval != 500*time.Millisecond || !strings.Contains(output.String(), "0.1 到 3600") {
		t.Fatalf("refresh interval = %s, output %q", updated.RefreshInterval, output.String())
	}
}

func TestConfigWizardAddsAndDeletesAccount(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	input := "5\n3\na\n定期\n2\n3w\nd\n2\n0\n0\n0\n"
	updated, err := configureConfig(strings.NewReader(input), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.AssetItems) != 1 || updated.AssetItems[0].Name != "当前余额" {
		t.Fatalf("accounts = %#v", updated.AssetItems)
	}
}

func TestWizardSecretAndYesNoWithPlainInput(t *testing.T) {
	var output bytes.Buffer
	wizard := configWizard{
		input:  strings.NewReader("secret\nmaybe\nyes\n"),
		reader: bufio.NewReader(strings.NewReader("secret\nmaybe\nyes\n")),
		out:    &output,
	}
	// Keep input and reader on the same stream, as configureConfig does.
	stream := strings.NewReader("secret\nmaybe\nyes\n")
	wizard.input = stream
	wizard.reader = bufio.NewReader(stream)
	secret, err := wizard.secret("密码")
	if err != nil || secret != "secret" {
		t.Fatalf("secret = %q, %v", secret, err)
	}
	answer, err := wizard.yesNo("确认", false)
	if err != nil || !answer || !strings.Contains(output.String(), "请输入 y 或 n") {
		t.Fatalf("yesNo = %t, %v, output %q", answer, err, output.String())
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
	updated, err := configureConfig(strings.NewReader("5\n1\n200k\n0\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Assets != 200000 || len(updated.AssetItems) != 1 || updated.AssetItems[0].Balance != 200000 {
		t.Fatalf("assets = %#v, total %.2f", updated.AssetItems, updated.Assets)
	}
}

func TestEditingCurrentBalancePreservesOtherAccounts(t *testing.T) {
	config := testFullConfig()
	config.AssetItems = []AssetItem{
		{Name: "工资卡", Kind: "checking", Balance: 100000},
		{Name: "定期", Kind: "deposit", Balance: 300000},
	}
	config.Assets = assetTotal(config.AssetItems)
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("5\n1\n20w\n0\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.AssetItems) != 2 || updated.AssetItems[0].Balance != 200000 || updated.AssetItems[1].Balance != 300000 {
		t.Fatalf("editing current balance lost accounts: %#v", updated.AssetItems)
	}
}

func TestInvalidClockAndChoicesAreNotSilentlySaved(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	input := "2\n\ninvalid\n10:00\n19:00\n\n\n\n0\n"
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

func TestClosingAssetsRequiresConfirmation(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("5\n4\n\n0\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.AssetsEnabled || len(updated.AssetItems) != 1 {
		t.Fatalf("assets were deleted without confirmation: %#v", updated.AssetItems)
	}
}

func TestConfigWizardAcceptsManualRetirementProfile(t *testing.T) {
	config := testFullConfig()
	path := filepath.Join(t.TempDir(), "config.toml")
	var output bytes.Buffer
	updated, err := configureConfig(strings.NewReader("4\n2\n1992-02-03\n2\n1\n0\n"), &output, path, config)
	if err != nil {
		t.Fatal(err)
	}
	if updated.BirthDate.Format("2006-01-02") != "1992-02-03" || updated.Sex != "female" || updated.FemaleTrack != "50" {
		t.Fatalf("profile = %s %s %s", updated.BirthDate, updated.Sex, updated.FemaleTrack)
	}
}

func TestParseIdentityNumber(t *testing.T) {
	birth, sex, err := parseIdentityNumber("11010519491231002X")
	if err != nil {
		t.Fatal(err)
	}
	if birth.Format("2006-01-02") != "1949-12-31" || sex != "female" {
		t.Fatalf("identity = %s %s", birth, sex)
	}
	if _, _, err := parseIdentityNumber("110105194912310021"); err == nil {
		t.Fatal("invalid checksum unexpectedly accepted")
	}
}

func TestSaveConfigPreservesMultipleAccounts(t *testing.T) {
	config := testFullConfig()
	config.RefreshInterval = 500 * time.Millisecond
	config.AssetItems = []AssetItem{
		{Name: "工资卡", Kind: "checking", Balance: 12345.67},
		{Name: "定期", Kind: "deposit", Balance: 200000},
	}
	config.Assets = assetTotal(config.AssetItems)
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
