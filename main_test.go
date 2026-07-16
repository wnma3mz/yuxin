package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpIsAFirstClassOption(t *testing.T) {
	opts, err := parseArgs([]string{"--help"})
	if err != nil || !opts.showHelp {
		t.Fatalf("parseArgs(--help) = %+v, %v", opts, err)
	}
}

func TestUpdateIsAFirstClassCommand(t *testing.T) {
	opts, err := parseArgs([]string{"update"})
	if err != nil || opts.command != "update" {
		t.Fatalf("parseArgs(update) = %+v, %v", opts, err)
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
		{"unknown"}, {"once", "doctor"}, {"--config"}, {"--config="},
		{"--interval"}, {"--interval", "0"}, {"--interval=invalid"},
	} {
		if _, err := parseArgs(args); err == nil {
			t.Errorf("parseArgs(%q) unexpectedly succeeded", args)
		}
	}
}

func TestRunCommonNonInteractiveCommands(t *testing.T) {
	code, output, stderr := runForTest(t, []string{"--version"}, "")
	if code != 0 || !strings.Contains(output, "0.2.0") || stderr != "" {
		t.Fatalf("version = code %d, output %q, stderr %q", code, output, stderr)
	}
	code, output, stderr = runForTest(t, []string{"--help"}, "")
	if code != 0 || !strings.Contains(output, "安装 GitHub 上的最新正式版") || stderr != "" {
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
		if code != 0 || !strings.Contains(output, command.want) || stderr != "" {
			t.Fatalf("run(%q) = code %d, output %q, stderr %q", command.args, code, output, stderr)
		}
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

	if code := run([]string{"config", "--config", path, "--interval", "2"}, input, output, stderr); code != 0 {
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
	code := run(args, input, output, stderr)
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
