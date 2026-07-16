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
	entries := []struct {
		path string
		name string
		mode os.FileMode
	}{
		{executable, executableName, 0o755},
		{"internal/app/data/default-config.toml", "yuxin.toml", 0o644},
		{"internal/app/data/holidays-2026.json", "holidays-2026.json", 0o644},
		{"README.md", "README.md", 0o644},
		{"LICENSE", "LICENSE", 0o644},
	}
	for _, entry := range entries {
		content, err := os.ReadFile(entry.path)
		if err != nil {
			return err
		}
		if err := writeArchiveEntry(writer, base+"/"+entry.name, content, entry.mode); err != nil {
			return err
		}
	}
	for _, entry := range desktopLaunchers(platform) {
		if err := writeArchiveEntry(writer, base+"/"+entry.name, []byte(entry.content), entry.mode); err != nil {
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

type generatedEntry struct {
	name    string
	content string
	mode    os.FileMode
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

func desktopLaunchers(platform string) []generatedEntry {
	switch {
	case strings.HasPrefix(platform, "macos-"):
		return []generatedEntry{
			{name: "Open-Yuxin.app/Contents/Info.plist", mode: 0o644, content: macOSLauncherPlist},
			{name: "Open-Yuxin.app/Contents/MacOS/Open-Yuxin", mode: 0o755, content: "#!/bin/sh\nroot=$(CDPATH= cd \"$(dirname \"$0\")/../../..\" && pwd)\nexec \"$root/yuxin\" web\n"},
		}
	case strings.HasPrefix(platform, "windows-"):
		return []generatedEntry{{name: "Open-Yuxin.vbs", mode: 0o644, content: `Set shell = CreateObject("WScript.Shell")
Set files = CreateObject("Scripting.FileSystemObject")
folder = files.GetParentFolderName(WScript.ScriptFullName)
shell.Run Chr(34) & folder & "\yuxin.exe" & Chr(34) & " web", 0, False
`}}
	case strings.HasPrefix(platform, "linux-"):
		return []generatedEntry{{name: "Open-Yuxin.desktop", mode: 0o755, content: linuxLauncherDesktop}}
	default:
		return nil
	}
}

const linuxLauncherDesktop = `[Desktop Entry]
Type=Application
Name=Yuxin
Comment=Open the local Yuxin dashboard
Exec=sh -c "entry=\\$1; [ -n \\"\\$entry\\" ] || entry=\\$GIO_LAUNCHED_DESKTOP_FILE; [ -n \\"\\$entry\\" ] || entry=\\$PWD/Open-Yuxin.desktop; case \\"\\$entry\\" in file://*) entry=\\${entry#file://} ;; esac; cd \\"\\$(dirname \\"\\$entry\\")\\" && exec ./yuxin web" sh %k
Terminal=false
Categories=Utility;
`

const macOSLauncherPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleDisplayName</key><string>Yuxin</string>
<key>CFBundleExecutable</key><string>Open-Yuxin</string>
<key>CFBundleIdentifier</key><string>com.wnma3mz.yuxin.launcher</string>
<key>CFBundleName</key><string>Yuxin</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleVersion</key><string>1</string>
</dict></plist>
`
