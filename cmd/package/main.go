package main

import (
	"archive/zip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	os.Exit(runPackage(os.Args[1:], os.Stderr))
}

func runPackage(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("package", flag.ContinueOnError)
	flags.SetOutput(stderr)
	executable := flags.String("executable", "", "path to the built Yuxin executable")
	platform := flags.String("platform", "", "release platform name")
	outputDir := flags.String("output-dir", "release", "release output directory")
	expectedTag := flags.String("expected-tag", "", "optional v-prefixed Git tag")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *executable == "" || *platform == "" {
		fmt.Fprintln(stderr, "错误：--executable and --platform are required")
		return 2
	}
	if err := packageRelease(*executable, *platform, *outputDir, *expectedTag); err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		return 2
	}
	return 0
}

func packageRelease(executable, platform, outputDir, expectedTag string) error {
	versionBytes, err := os.ReadFile("internal/app/VERSION")
	if err != nil {
		return fmt.Errorf("read VERSION: %w", err)
	}
	version := strings.TrimSpace(string(versionBytes))
	if expectedTag != "" {
		if !strings.HasPrefix(expectedTag, "v") {
			return fmt.Errorf("tag %s must start with v", expectedTag)
		}
		if expectedTag != "v"+version {
			return fmt.Errorf("tag %s does not match VERSION %s", expectedTag, version)
		}
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	base := fmt.Sprintf("yuxin-v%s-%s", version, platform)
	archivePath := filepath.Join(outputDir, archiveFilename(platform))
	if err := createArchive(archivePath, base, platform, executable); err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	content, err := os.ReadFile(archivePath)
	if err != nil {
		return fmt.Errorf("read archive: %w", err)
	}
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(content), filepath.Base(archivePath))
	if err := os.WriteFile(archivePath+".sha256", []byte(checksum), 0o644); err != nil {
		return fmt.Errorf("write checksum: %w", err)
	}
	return nil
}

func archiveFilename(platform string) string {
	return "yuxin-" + platform + ".zip"
}

func createArchive(archivePath, base, platform, executable string) error {
	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	closed := false
	defer func() {
		if !closed {
			_ = writer.Close()
			_ = file.Close()
		}
	}()
	executableName := "yuxin"
	if strings.HasPrefix(platform, "windows-") || strings.EqualFold(filepath.Ext(executable), ".exe") {
		executableName += ".exe"
	}
	type archiveEntry struct {
		path string
		name string
		mode os.FileMode
	}
	entries := []archiveEntry{
		{executable, executableName, 0o755},
		{"internal/app/data/default-config.toml", "yuxin.toml", 0o644},
	}
	holidayPaths, err := filepath.Glob("internal/app/data/holidays-*.json")
	if err != nil {
		return err
	}
	if len(holidayPaths) == 0 {
		return fmt.Errorf("未找到节假日数据")
	}
	latestHolidayPath := holidayPaths[len(holidayPaths)-1]
	entries = append(entries,
		archiveEntry{latestHolidayPath, filepath.Base(latestHolidayPath), 0o644},
		archiveEntry{"README.md", "README.md", 0o644},
		archiveEntry{"LICENSE", "LICENSE", 0o644},
	)
	for _, entry := range entries {
		content, err := os.ReadFile(entry.path)
		if err != nil {
			return err
		}
		if err := writeArchiveEntry(writer, base+"/"+entry.name, content, entry.mode); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	closed = true
	return nil
}

func writeArchiveEntry(writer *zip.Writer, name string, content []byte, mode os.FileMode) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(mode)
	// ZIP stores timestamps with a two-second resolution. A fixed timestamp
	// makes identical inputs produce identical archives and checksums.
	header.SetModTime(time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC))
	destination, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = destination.Write(content)
	return err
}
