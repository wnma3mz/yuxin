package app

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExportConfigWritesNormalizedPrivateFile(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "backup", "..", "backup", "config.toml")
	config := defaultConfig()
	config.SalaryAmount = 16800
	config.HideAmounts = true

	var output bytes.Buffer
	if err := exportConfig(config, destination, &output); err != nil {
		t.Fatalf("exportConfig: %v", err)
	}
	exported, err := loadConfig(destination)
	if err != nil {
		t.Fatalf("load exported config: %v", err)
	}
	if exported.SalaryAmount != config.SalaryAmount || !exported.HideAmounts {
		t.Fatalf("exported config = %#v", exported)
	}
	absolute, _ := filepath.Abs(destination)
	if !strings.Contains(output.String(), "敏感信息") || !strings.Contains(output.String(), filepath.Clean(absolute)) {
		t.Fatalf("export output = %q", output.String())
	}
	assertPrivatePermissions(t, destination)
}

func TestExportConfigFailurePreservesExistingDestination(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "config.toml")
	const original = "keep me"
	if err := os.WriteFile(destination, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.SalaryAmount = 0

	var output bytes.Buffer
	if err := exportConfig(config, destination, &output); err == nil {
		t.Fatal("exportConfig unexpectedly accepted invalid config")
	}
	if !strings.Contains(output.String(), "敏感信息") {
		t.Fatalf("export warning was not shown before failure: %q", output.String())
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Fatalf("destination changed to %q", content)
	}
}

func TestImportConfigValidatesThenAtomicallySaves(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source.toml")
	target := filepath.Join(directory, "nested", "config.toml")
	config := defaultConfig()
	config.SalaryAmount = 18888
	config.HideRetirementDate = true
	if err := saveConfig(config, source); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := importConfig(source, target, &output); err != nil {
		t.Fatalf("importConfig: %v", err)
	}
	got, err := loadConfig(target)
	if err != nil {
		t.Fatal(err)
	}
	if got.SalaryAmount != config.SalaryAmount || !got.HideRetirementDate {
		t.Fatalf("imported config = %#v", got)
	}
	absolute, _ := filepath.Abs(target)
	if !strings.Contains(output.String(), absolute) {
		t.Fatalf("import output = %q", output.String())
	}
	assertPrivatePermissions(t, target)
}

func TestImportConfigInvalidSourcePreservesTarget(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "invalid.toml")
	target := filepath.Join(directory, "config.toml")
	if err := os.WriteFile(source, []byte("[salary]\namount = \"0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const original = "original target"
	if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := importConfig(source, target, nil); err == nil || !strings.Contains(err.Error(), "源文件无效") {
		t.Fatalf("importConfig error = %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Fatalf("target changed to %q", content)
	}
}

func TestImportConfigRejectsSameFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := saveConfig(defaultConfig(), path); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)

	if err := importConfig(path, filepath.Join(filepath.Dir(path), ".", "config.toml"), nil); err == nil || !strings.Contains(err.Error(), "同一文件") {
		t.Fatalf("same-path import error = %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, before) {
		t.Fatal("same-path import changed config")
	}

	alias := filepath.Join(filepath.Dir(path), "alias.toml")
	if err := os.Link(path, alias); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	if err := importConfig(alias, path, nil); err == nil || !strings.Contains(err.Error(), "同一文件") {
		t.Fatalf("same-file import error = %v", err)
	}
}

func TestTransferRejectsEmptyPathsAndDirectories(t *testing.T) {
	if err := exportConfig(defaultConfig(), "", nil); err == nil {
		t.Fatal("exportConfig accepted an empty path")
	}
	if err := importConfig("", "target", nil); err == nil {
		t.Fatal("importConfig accepted an empty source path")
	}
	directory := t.TempDir()
	if err := exportConfig(defaultConfig(), directory, nil); err == nil {
		t.Fatal("exportConfig accepted a directory")
	}
	source := filepath.Join(t.TempDir(), "source.toml")
	if err := saveConfig(defaultConfig(), source); err != nil {
		t.Fatal(err)
	}
	if err := importConfig(source, directory, nil); err == nil {
		t.Fatal("importConfig accepted a target directory")
	}
}

func TestClearConfigRequiresExactConfirmationAndShowsAbsolutePath(t *testing.T) {
	for name, confirmation := range map[string]string{
		"different text": "delete\n",
		"empty input":    "",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte("config"), 0o600); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := clearConfig(path, strings.NewReader(confirmation), &output); err != nil {
				t.Fatalf("clearConfig: %v", err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("unconfirmed clear removed config: %v", err)
			}
			absolute, _ := filepath.Abs(path)
			if !strings.Contains(output.String(), absolute) || !strings.Contains(output.String(), "已取消") {
				t.Fatalf("clear output = %q", output.String())
			}
		})
	}
}

func TestClearConfigDeletesOnlyConfirmedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := clearConfig(path, strings.NewReader(" DELETE \n"), &output); err != nil {
		t.Fatalf("clearConfig: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("confirmed clear stat error = %v", err)
	}
	if !strings.Contains(output.String(), "配置已清除") {
		t.Fatalf("clear output = %q", output.String())
	}
}

func TestClearConfigMissingFileIsFriendly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.toml")
	var output bytes.Buffer
	if err := clearConfig(path, nil, &output); err != nil {
		t.Fatalf("clearConfig missing file: %v", err)
	}
	if !strings.Contains(output.String(), "不存在") {
		t.Fatalf("clear output = %q", output.String())
	}
}

func TestClearConfigRefusesDirectory(t *testing.T) {
	directory := t.TempDir()
	if err := clearConfig(directory, strings.NewReader("DELETE\n"), nil); err == nil {
		t.Fatal("clearConfig deleted a directory")
	}
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		t.Fatalf("directory was damaged: info=%v err=%v", info, err)
	}
}

func assertPrivatePermissions(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("permissions = %o, want 600", got)
	}
}
