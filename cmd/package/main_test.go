package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateArchiveIncludesRunnableBinaryAndLocalData(t *testing.T) {
	enterProjectRoot(t)
	holidayPaths, err := filepath.Glob("internal/app/data/holidays-*.json")
	if err != nil || len(holidayPaths) == 0 {
		t.Fatalf("holiday data = %q, %v", holidayPaths, err)
	}
	latestHoliday := filepath.Base(holidayPaths[len(holidayPaths)-1])
	executable := filepath.Join(t.TempDir(), "yuxin.bin")
	if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "yuxin.zip")
	if err := createArchive(archivePath, "yuxin-v0.2.0-windows-x86_64", "windows-x86_64", executable); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	want := map[string]bool{
		"yuxin.exe": false, "yuxin.toml": false, latestHoliday: false,
		"README.md": false, "LICENSE": false,
	}
	holidayFiles := 0
	for _, entry := range archive.File {
		name := filepath.Base(entry.Name)
		want[name] = true
		if strings.HasPrefix(name, "holidays-") && strings.HasSuffix(name, ".json") {
			holidayFiles++
		}
		if name == "yuxin.exe" && entry.Mode().Perm() != 0o755 {
			t.Fatalf("executable permissions = %o", entry.Mode().Perm())
		}
	}
	if holidayFiles != 1 {
		t.Fatalf("archive contains %d holiday data files, want 1", holidayFiles)
	}
	for name, found := range want {
		if !found {
			t.Errorf("archive is missing %s", name)
		}
	}
}

func TestRunPackageCreatesArchiveAndChecksum(t *testing.T) {
	enterProjectRoot(t)
	executable := filepath.Join(t.TempDir(), "yuxin")
	if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	var stderr bytes.Buffer
	code := runPackage([]string{
		"--executable", executable,
		"--platform", "linux-x86_64",
		"--output-dir", output,
		"--expected-tag", "v" + readProjectVersion(t),
	}, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("runPackage = %d, stderr %q", code, stderr.String())
	}
	for _, name := range []string{"yuxin-linux-x86_64.zip", "yuxin-linux-x86_64.zip.sha256"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestRunPackageReportsInvalidArguments(t *testing.T) {
	enterProjectRoot(t)
	for name, args := range map[string][]string{
		"missing required": {},
		"unknown flag":     {"--unknown"},
		"tag mismatch": {
			"--executable", "missing", "--platform", "linux-x86_64", "--expected-tag", "v99.0.0",
		},
		"tag without prefix": {
			"--executable", "missing", "--platform", "linux-x86_64", "--expected-tag", readProjectVersion(t),
		},
	} {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := runPackage(args, &stderr); code != 2 || strings.TrimSpace(stderr.String()) == "" {
				t.Fatalf("runPackage(%q) = %d, stderr %q", args, code, stderr.String())
			}
		})
	}
}

func enterProjectRoot(t *testing.T) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Clean(filepath.Join(original, "..", ".."))); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
}

func readProjectVersion(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile("internal/app/VERSION")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(content))
}

func TestArchiveFilenameIsStableAcrossVersions(t *testing.T) {
	if got := archiveFilename("macos-arm64"); got != "yuxin-macos-arm64.zip" {
		t.Fatalf("archive filename = %q", got)
	}
}

func TestCreateArchiveIsReproducible(t *testing.T) {
	enterProjectRoot(t)
	executable := filepath.Join(t.TempDir(), "yuxin")
	if err := os.WriteFile(executable, []byte("stable binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(t.TempDir(), "first.zip")
	second := filepath.Join(t.TempDir(), "second.zip")
	for _, destination := range []string{first, second} {
		if err := createArchive(destination, "yuxin-v1.2.3-linux-x86_64", "linux-x86_64", executable); err != nil {
			t.Fatal(err)
		}
	}
	firstContent, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondContent, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstContent, secondContent) {
		t.Fatal("identical inputs produced different archives")
	}
}
