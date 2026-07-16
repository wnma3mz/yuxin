package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallRequiresExactConfirmationAndPreservesConfig(t *testing.T) {
	for name, confirmation := range map[string]string{"wrong": "uninstall\n", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			executable := filepath.Join(directory, "yuxin")
			configPath := filepath.Join(directory, "config.toml")
			if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(configPath, []byte("config"), 0o600); err != nil {
				t.Fatal(err)
			}
			called := false
			var output bytes.Buffer
			err := runUninstallUsing(strings.NewReader(confirmation), &output, executable, configPath, false, func(string) (bool, error) {
				called = true
				return false, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if called || !strings.Contains(output.String(), "已取消") {
				t.Fatalf("called = %t, output = %q", called, output.String())
			}
			if _, err := os.Stat(configPath); err != nil {
				t.Fatalf("config was removed: %v", err)
			}
		})
	}
}

func TestUninstallDefaultRemovesOnlyExecutable(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "yuxin")
	configPath := filepath.Join(directory, "config.toml")
	if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := runUninstallUsing(strings.NewReader("UNINSTALL\n"), &output, executable, configPath, false, func(target string) (bool, error) {
		return false, os.Remove(target)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(executable); !os.IsNotExist(err) {
		t.Fatalf("executable still exists: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config was removed: %v", err)
	}
	if !strings.Contains(output.String(), "本地配置将保留") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestUninstallPurgeRemovesConfigAndHandlesPendingRemoval(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "yuxin")
	configPath := filepath.Join(directory, "config.toml")
	if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := runUninstallUsing(strings.NewReader("PURGE\n"), &output, executable, configPath, true, func(string) (bool, error) {
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config still exists: %v", err)
	}
	if !strings.Contains(output.String(), "退出后完成") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestUninstallReportsRemovalAndUnsafeConfigErrors(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "yuxin")
	if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("denied")
	if err := runUninstallUsing(strings.NewReader("UNINSTALL\n"), nil, executable, "", false, func(string) (bool, error) {
		return false, wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("removal error = %v", err)
	}
	if err := runUninstallUsing(strings.NewReader("PURGE\n"), nil, executable, directory, true, func(string) (bool, error) {
		return false, nil
	}); err == nil || !strings.Contains(err.Error(), "目录") {
		t.Fatalf("unsafe config error = %v", err)
	}
}
