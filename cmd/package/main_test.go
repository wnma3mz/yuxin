package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateArchiveIncludesRunnableBinaryAndLocalData(t *testing.T) {
	enterProjectRoot(t)
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
		"yuxin.exe": false, "yuxin.toml": false, "holidays-2026.json": false,
		"README.md": false, "LICENSE": false, "Open-Yuxin.vbs": false,
	}
	for _, entry := range archive.File {
		want[filepath.Base(entry.Name)] = true
		if filepath.Base(entry.Name) == "yuxin.exe" && entry.Mode().Perm() != 0o755 {
			t.Fatalf("executable permissions = %o", entry.Mode().Perm())
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("archive is missing %s", name)
		}
	}
}

func TestDesktopLaunchersMatchPlatforms(t *testing.T) {
	mac := launcherByName(t, "macos-arm64", "Open-Yuxin.app/Contents/MacOS/Open-Yuxin")
	if mac.mode != 0o755 || !strings.Contains(mac.content, `exec "$root/yuxin" web`) {
		t.Fatalf("invalid macOS launcher: %#v", mac)
	}
	plist := launcherByName(t, "macos-arm64", "Open-Yuxin.app/Contents/Info.plist")
	if plist.mode != 0o644 || !strings.Contains(plist.content, "<string>Open-Yuxin</string>") {
		t.Fatalf("invalid macOS plist: %#v", plist)
	}
	var document struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal([]byte(plist.content), &document); err != nil || document.XMLName.Local != "plist" {
		t.Fatalf("macOS plist is not valid XML: root=%q err=%v", document.XMLName.Local, err)
	}

	windows := launcherByName(t, "windows-x86_64", "Open-Yuxin.vbs")
	if windows.mode != 0o644 || !strings.Contains(windows.content, `folder & "\yuxin.exe"`) || !strings.Contains(windows.content, `& " web", 0, False`) {
		t.Fatalf("invalid Windows launcher: %#v", windows)
	}

	linux := launcherByName(t, "linux-x86_64", "Open-Yuxin.desktop")
	if linux.mode != 0o755 || !strings.Contains(linux.content, `entry=\\$1`) || !strings.Contains(linux.content, `\\$GIO_LAUNCHED_DESKTOP_FILE`) || !strings.Contains(linux.content, `exec ./yuxin web" sh %k`) || !strings.Contains(linux.content, "Terminal=false") {
		t.Fatalf("invalid Linux launcher: %#v", linux)
	}

	if entries := desktopLaunchers("plan9-amd64"); len(entries) != 0 {
		t.Fatalf("unsupported launchers = %#v", entries)
	}
}

func launcherByName(t *testing.T, platform, name string) generatedEntry {
	t.Helper()
	for _, entry := range desktopLaunchers(platform) {
		if entry.name == name {
			return entry
		}
	}
	t.Fatalf("desktopLaunchers(%q) missing %q", platform, name)
	return generatedEntry{}
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
