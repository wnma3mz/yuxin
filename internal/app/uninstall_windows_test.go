//go:build windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUninstallExecutableHandlesSpecialPathAndCleansScript(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "uninstall path & spaces")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "yuxin.exe")
	if err := os.WriteFile(target, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := windowsTemporaryScripts(t, "yuxin-uninstall-*.ps1")
	pending, err := uninstallExecutable(target)
	if err != nil || !pending {
		t.Fatalf("uninstallExecutable = pending %t, error %v", pending, err)
	}
	waitForWindowsCondition(t, 8*time.Second, func() bool {
		return os.IsNotExist(statError(target)) && noNewWindowsScripts(before, windowsTemporaryScripts(t, "yuxin-uninstall-*.ps1"))
	})
}

func TestUninstallExecutableStartFailurePreservesTargetAndCleansScript(t *testing.T) {
	target := filepath.Join(t.TempDir(), "yuxin.exe")
	if err := os.WriteFile(target, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := windowsTemporaryScripts(t, "yuxin-uninstall-*.ps1")
	t.Setenv("PATH", "")
	pending, err := uninstallExecutable(target)
	if err == nil || pending {
		t.Fatalf("uninstallExecutable = pending %t, error %v", pending, err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "powershell.exe") {
		t.Fatalf("start error = %v", err)
	}
	if content, readErr := os.ReadFile(target); readErr != nil || string(content) != "binary" {
		t.Fatalf("target content = %q, %v", content, readErr)
	}
	after := windowsTemporaryScripts(t, "yuxin-uninstall-*.ps1")
	if !noNewWindowsScripts(before, after) {
		t.Fatalf("failed start left scripts: before=%v after=%v", before, after)
	}
}

func TestWindowsUninstallScriptRetriesAreBoundedAndSelfCleaning(t *testing.T) {
	for _, want := range []string{
		"$attempt -lt 120",
		"Remove-Item -Force -LiteralPath $Target",
		"Start-Sleep -Milliseconds 250",
		"if (-not (Test-Path -LiteralPath $Target))",
		"Remove-Item -Force -LiteralPath $ScriptPath",
	} {
		if !strings.Contains(windowsUninstallScript, want) {
			t.Fatalf("uninstall script missing %q:\n%s", want, windowsUninstallScript)
		}
	}
}

func windowsTemporaryScripts(t *testing.T, pattern string) map[string]bool {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(os.TempDir(), pattern))
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]bool, len(paths))
	for _, path := range paths {
		result[path] = true
	}
	return result
}

func noNewWindowsScripts(before, after map[string]bool) bool {
	for path := range after {
		if !before[path] {
			return false
		}
	}
	return true
}
