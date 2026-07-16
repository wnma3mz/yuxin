package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHelpIsAFirstClassOption(t *testing.T) {
	opts, err := parseArgs([]string{"--help"})
	if err != nil || !opts.showHelp {
		t.Fatalf("parseArgs(--help) = %+v, %v", opts, err)
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
