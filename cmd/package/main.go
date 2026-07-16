package main

import (
	"archive/zip"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	executable := flag.String("executable", "", "path to the built Yuxin executable")
	platform := flag.String("platform", "", "release platform name")
	outputDir := flag.String("output-dir", "release", "release output directory")
	expectedTag := flag.String("expected-tag", "", "optional v-prefixed Git tag")
	flag.Parse()
	if *executable == "" || *platform == "" {
		fatalf("--executable and --platform are required")
	}
	versionBytes, err := os.ReadFile("VERSION")
	if err != nil {
		fatalf("read VERSION: %v", err)
	}
	version := strings.TrimSpace(string(versionBytes))
	if strings.HasPrefix(*expectedTag, "v") && *expectedTag != "v"+version {
		fatalf("tag %s does not match VERSION %s", *expectedTag, version)
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fatalf("create output directory: %v", err)
	}
	base := fmt.Sprintf("yuxin-v%s-%s", version, *platform)
	archivePath := filepath.Join(*outputDir, base+".zip")
	if err := createArchive(archivePath, base, *platform, *executable); err != nil {
		fatalf("create archive: %v", err)
	}
	content, err := os.ReadFile(archivePath)
	if err != nil {
		fatalf("read archive: %v", err)
	}
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(content), filepath.Base(archivePath))
	if err := os.WriteFile(archivePath+".sha256", []byte(checksum), 0o644); err != nil {
		fatalf("write checksum: %v", err)
	}
}

func createArchive(archivePath, base, platform, executable string) error {
	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	executableName := "yuxin"
	if strings.HasPrefix(platform, "windows-") || strings.EqualFold(filepath.Ext(executable), ".exe") {
		executableName += ".exe"
	}
	entries := []struct {
		path string
		name string
		mode os.FileMode
	}{
		{executable, executableName, 0o755},
		{"data/default-config.toml", "yuxin.toml", 0o644},
		{"data/holidays-2026.json", "holidays-2026.json", 0o644},
	}
	for _, entry := range entries {
		content, err := os.ReadFile(entry.path)
		if err != nil {
			writer.Close()
			file.Close()
			return err
		}
		header := &zip.FileHeader{Name: base + "/" + entry.name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		header.SetModTime(time.Now().UTC())
		destination, err := writer.CreateHeader(header)
		if err != nil {
			writer.Close()
			file.Close()
			return err
		}
		if _, err := destination.Write(content); err != nil {
			writer.Close()
			file.Close()
			return err
		}
	}
	if err := writer.Close(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func fatalf(format string, values ...any) {
	fmt.Fprintf(os.Stderr, "错误："+format+"\n", values...)
	os.Exit(2)
}
