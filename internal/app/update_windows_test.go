//go:build windows

package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReplaceExecutableCompletesInBackgroundAndCleansUp(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "update path & spaces")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "yuxin.exe")
	staged := filepath.Join(directory, "staged update.exe")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}

	pending, err := replaceExecutable(staged, target)
	if err != nil || !pending {
		t.Fatalf("replaceExecutable = pending %t, error %v", pending, err)
	}

	waitForWindowsCondition(t, 8*time.Second, func() bool {
		content, readErr := os.ReadFile(target)
		stagedErr := statError(staged)
		scripts, _ := filepath.Glob(filepath.Join(directory, ".yuxin-update-*.cmd"))
		return readErr == nil && bytes.Equal(content, []byte("new")) && os.IsNotExist(stagedErr) && len(scripts) == 0
	})
	content, readErr := os.ReadFile(target)
	if readErr != nil || string(content) != "new" {
		t.Fatalf("replacement content = %q, %v", content, readErr)
	}
}

func TestReplaceExecutableStartFailureLeavesInputsAndNoScript(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "yuxin.exe")
	staged := filepath.Join(directory, "update.exe")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")
	pending, err := replaceExecutable(staged, target)
	if err == nil || pending {
		t.Fatalf("replaceExecutable = pending %t, error %v", pending, err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cmd.exe") {
		t.Fatalf("start error = %v", err)
	}
	for path, want := range map[string]string{target: "old", staged: "new"} {
		content, readErr := os.ReadFile(path)
		if readErr != nil || string(content) != want {
			t.Fatalf("%s content = %q, %v", path, content, readErr)
		}
	}
	if scripts, _ := filepath.Glob(filepath.Join(directory, ".yuxin-update-*.cmd")); len(scripts) != 0 {
		t.Fatalf("failed start left scripts: %q", scripts)
	}
}

func TestWindowsUpdateScriptRetriesAreBoundedAndCleanUp(t *testing.T) {
	for _, want := range []string{
		"for /L %%A in (1,1,120)",
		"if not errorlevel 1 goto updated",
		"del /F /Q \"%~2\"",
		"del \"%~f0\"",
	} {
		if !strings.Contains(windowsUpdateScript, want) {
			t.Fatalf("update script missing %q:\n%s", want, windowsUpdateScript)
		}
	}
	if strings.Contains(strings.ToLower(windowsUpdateScript), "goto retry") {
		t.Fatal("update script contains an unbounded retry loop")
	}
}

func waitForWindowsCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !condition() {
		t.Fatal("condition was not satisfied before timeout")
	}
}

func statError(path string) error {
	_, err := os.Stat(path)
	return err
}
