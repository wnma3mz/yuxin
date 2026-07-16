package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateArchiveIncludesRunnableBinaryAndLocalData(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(original, "..", ".."))
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(original)
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
		"README.md": false, "LICENSE": false,
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

func TestArchiveFilenameIsStableAcrossVersions(t *testing.T) {
	if got := archiveFilename("macos-arm64"); got != "yuxin-macos-arm64.zip" {
		t.Fatalf("archive filename = %q", got)
	}
}
