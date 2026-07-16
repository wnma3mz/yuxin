package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHelpIsAFirstClassOption(t *testing.T) {
	opts, err := parseArgs([]string{"--help"})
	if err != nil || !opts.showHelp {
		t.Fatalf("parseArgs(--help) = %+v, %v", opts, err)
	}
}

func TestUpdateIsAFirstClassCommand(t *testing.T) {
	opts, err := parseArgs([]string{"update", "--force"})
	if err != nil || opts.command != "update" {
		t.Fatalf("parseArgs(update --force) = %+v, %v", opts, err)
	}
	if !opts.forceUpdate {
		t.Fatal("--force was not retained")
	}
}

func TestUninstallIsAFirstClassCommand(t *testing.T) {
	opts, err := parseArgs([]string{"uninstall", "--purge", "--config", "private.toml"})
	if err != nil || opts.command != "uninstall" || !opts.purge || opts.configPath != "private.toml" {
		t.Fatalf("parseArgs(uninstall --purge) = %+v, %v", opts, err)
	}
}

func TestShareAndConfigTransferArguments(t *testing.T) {
	opts, err := parseArgs([]string{"share", "--real", "--card", "workday"})
	if err != nil || opts.command != "share" || !opts.shareReal || opts.shareCard != "workday" {
		t.Fatalf("parseArgs(share) = %+v, %v", opts, err)
	}
	opts, err = parseArgs([]string{"config", "export", "backup.toml"})
	if err != nil || opts.configAction != "export" || opts.actionPath != "backup.toml" {
		t.Fatalf("parseArgs(config export) = %+v, %v", opts, err)
	}
	opts, err = parseArgs([]string{"config", "export", "update"})
	if err != nil || opts.actionPath != "update" {
		t.Fatalf("reserved-word export path = %+v, %v", opts, err)
	}
	for _, args := range [][]string{{"config", "export"}, {"config", "import"}, {"share", "--card"}} {
		if _, err := parseArgs(args); err == nil {
			t.Errorf("parseArgs(%q) unexpectedly succeeded", args)
		}
	}
}

func TestNullDeviceIsNotATerminal(t *testing.T) {
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if isTerminal(file) {
		t.Fatal("null device was detected as an interactive terminal")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	t.Setenv("YUXIN_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/yuxin-xdg")
	path, explicit, err := resolveConfigPath(cliOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if explicit || path != filepath.Join("/tmp/yuxin-xdg", "yuxin", "config.toml") {
		t.Fatalf("resolveConfigPath() = %q, %t", path, explicit)
	}
}

func TestExplicitAndEnvironmentConfigPaths(t *testing.T) {
	explicit, isExplicit, err := resolveConfigPath(cliOptions{configPath: "custom.toml", configExplicit: true})
	if err != nil || !isExplicit || explicit != "custom.toml" {
		t.Fatalf("explicit path = %q, %t, %v", explicit, isExplicit, err)
	}
	t.Setenv("YUXIN_CONFIG", "environment.toml")
	fromEnvironment, isExplicit, err := resolveConfigPath(cliOptions{})
	if err != nil || !isExplicit || fromEnvironment != "environment.toml" {
		t.Fatalf("environment path = %q, %t, %v", fromEnvironment, isExplicit, err)
	}
}

func TestParseArgsRejectsInvalidInput(t *testing.T) {
	for _, args := range [][]string{
		{"unknown"}, {"web"}, {"once", "doctor"}, {"--config"}, {"--config="},
		{"--interval"}, {"--interval", "0"}, {"--interval=invalid"},
	} {
		if _, err := parseArgs(args); err == nil {
			t.Errorf("parseArgs(%q) unexpectedly succeeded", args)
		}
	}
}

func TestRunCommonNonInteractiveCommands(t *testing.T) {
	code, output, stderr := runForTest(t, []string{"--version"}, "")
	if code != 0 || !strings.Contains(output, version) || stderr != "" {
		t.Fatalf("version = code %d, output %q, stderr %q", code, output, stderr)
	}
	code, output, stderr = runForTest(t, []string{"--help"}, "")
	if code != 0 || !strings.Contains(output, "安装 GitHub 上的最新正式版") || strings.Contains(output, "\t") || stderr != "" {
		t.Fatalf("help = code %d, output %q, stderr %q", code, output, stderr)
	}
	code, _, stderr = runForTest(t, []string{"unknown"}, "")
	if code != 2 || !strings.Contains(stderr, "未知参数") {
		t.Fatalf("invalid command = code %d, stderr %q", code, stderr)
	}
}

func TestRunOnceAndDoctorWithDefaultConfig(t *testing.T) {
	for _, command := range []struct {
		args []string
		want string
	}{
		{[]string{"once", "--config", "data/default-config.toml"}, "今日入账"},
		{[]string{"doctor", "--config", "data/default-config.toml"}, "仪表盘数据"},
	} {
		code, output, stderr := runForTest(t, command.args, "")
		if code != 0 || !strings.Contains(output, command.want) || stderr != "" && !strings.Contains(stderr, "节假日数据") {
			t.Fatalf("run(%q) = code %d, output %q, stderr %q", command.args, code, output, stderr)
		}
	}
}

func TestRunShareDefaultsToSyntheticData(t *testing.T) {
	code, output, stderr := runForTest(t, []string{"share", "--config", "data/default-config.toml"}, "")
	if code != 0 || stderr != "" || !strings.Contains(output, "演示数据") || !strings.Contains(output, "无账号") {
		t.Fatalf("share = code %d, output %q, stderr %q", code, output, stderr)
	}
	invalid := filepath.Join(t.TempDir(), "invalid.toml")
	if err := os.WriteFile(invalid, []byte("not valid toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, output, stderr = runForTest(t, []string{"share", "--config", invalid}, "")
	if code != 0 || stderr != "" || !strings.Contains(output, "演示数据") {
		t.Fatalf("synthetic share with invalid config = code %d, output %q, stderr %q", code, output, stderr)
	}
	code, output, stderr = runForTest(t, []string{"share", "--real", "--card=workday", "--config", "data/default-config.toml"}, "")
	if code != 0 || !strings.Contains(output, "本地数据") || !strings.Contains(stderr, "隐私提示") {
		t.Fatalf("real share = code %d, output %q, stderr %q", code, output, stderr)
	}
}

func TestRunConfigExportImportAndClear(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.toml")
	backupPath := filepath.Join(directory, "backup.toml")
	config := defaultConfig()
	config.SalaryAmount = 12345
	if err := saveConfig(config, configPath); err != nil {
		t.Fatal(err)
	}
	code, output, stderr := runForTest(t, []string{"config", "export", backupPath, "--config", configPath}, "")
	if code != 0 || stderr != "" || !strings.Contains(output, "敏感信息") {
		t.Fatalf("export = code %d, output %q, stderr %q", code, output, stderr)
	}
	config.SalaryAmount = 23456
	if err := saveConfig(config, backupPath); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runForTest(t, []string{"config", "import", backupPath, "--config", configPath}, "")
	if code != 0 || stderr != "" {
		t.Fatalf("import = code %d, stderr %q", code, stderr)
	}
	loaded, err := loadConfig(configPath)
	if err != nil || loaded.SalaryAmount != 23456 {
		t.Fatalf("imported config = %#v, %v", loaded, err)
	}
	code, output, stderr = runForTest(t, []string{"config", "clear", "--config", configPath}, "DELETE\n")
	if code != 0 || stderr != "" || !strings.Contains(output, "配置已清除") {
		t.Fatalf("clear = code %d, output %q, stderr %q", code, output, stderr)
	}
}

func TestRunReportsDataCommandErrors(t *testing.T) {
	directory := t.TempDir()
	invalidConfig := filepath.Join(directory, "invalid.toml")
	if err := os.WriteFile(invalidConfig, []byte("[salary]\namount = \"0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		args  []string
		input string
		want  string
	}{
		{[]string{"config", "import", filepath.Join(directory, "missing.toml"), "--config", filepath.Join(directory, "target.toml")}, "", "导入失败"},
		{[]string{"config", "clear", "--config", directory}, "DELETE\n", "清理失败"},
		{[]string{"config", "export", directory, "--config", "data/default-config.toml"}, "", "导出失败"},
		{[]string{"share", "--card", "unknown", "--config", "data/default-config.toml"}, "", "生成分享卡片失败"},
		{[]string{"once", "--config", invalidConfig}, "", "读取配置"},
	}
	for _, test := range tests {
		code, _, stderr := runForTest(t, test.args, test.input)
		if code == 0 || !strings.Contains(stderr, test.want) {
			t.Errorf("run(%q) = %d, stderr %q; want %q", test.args, code, stderr, test.want)
		}
	}
}

func TestHolidayDataReminderUsesCurrentYear(t *testing.T) {
	var output bytes.Buffer
	remindHolidayData(&output, time.Date(2026, time.July, 16, 0, 0, 0, 0, time.Local))
	if output.Len() != 0 {
		t.Fatalf("bundled year reminder = %q", output.String())
	}
	remindHolidayData(&output, time.Date(2027, time.January, 1, 0, 0, 0, 0, time.Local))
	if !strings.Contains(output.String(), "2027 年") || !strings.Contains(output.String(), "update --force") {
		t.Fatalf("missing-year reminder = %q", output.String())
	}
}

func TestReadConfigCreatesImplicitDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	config, source, err := readConfig(path, false)
	if err != nil || source != path {
		t.Fatalf("readConfig = source %q, error %v", source, err)
	}
	if config.ProfileEnabled || config.AssetsEnabled {
		t.Fatalf("created config enabled optional modules: %#v", config)
	}
	missing := filepath.Join(t.TempDir(), "missing.toml")
	if _, _, err := readConfig(missing, true); !os.IsNotExist(err) {
		t.Fatalf("explicit missing config error = %v", err)
	}
}

func TestTerminalSizeEnvironmentOverrides(t *testing.T) {
	t.Setenv("COLUMNS", "91")
	t.Setenv("LINES", "37")
	if terminalWidth() != 91 || terminalHeight() != 37 {
		t.Fatalf("terminal size = %dx%d", terminalWidth(), terminalHeight())
	}
}

func TestConfigCommandCreatesAnExplicitMissingConfig(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "custom.toml")
	input, err := os.CreateTemp(directory, "input")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if _, err := input.WriteString("0\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := input.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	output, err := os.CreateTemp(directory, "output")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	stderr, err := os.CreateTemp(directory, "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()

	if code := runAt([]string{"config", "--config", path, "--interval", "2"}, input, output, stderr, testCLITime()); code != 0 {
		t.Fatalf("run(config) = %d", code)
	}
	config, err := loadConfig(path)
	if err != nil {
		t.Fatalf("created config cannot be loaded: %v", err)
	}
	if config.RefreshInterval.String() != "1s" {
		t.Fatalf("temporary interval was persisted: %s", config.RefreshInterval)
	}
}

func runForTest(t *testing.T, args []string, inputText string) (int, string, string) {
	t.Helper()
	directory := t.TempDir()
	input, err := os.CreateTemp(directory, "input")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if _, err := input.WriteString(inputText); err != nil {
		t.Fatal(err)
	}
	if _, err := input.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	output, err := os.CreateTemp(directory, "output")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	stderr, err := os.CreateTemp(directory, "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	code := runAt(args, input, output, stderr, testCLITime())
	read := func(file *os.File) string {
		if _, err := file.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	return code, read(output), read(stderr)
}

func testCLITime() time.Time {
	return time.Date(2026, time.July, 16, 15, 0, 0, 0, time.Local)
}
