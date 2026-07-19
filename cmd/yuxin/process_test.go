package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCLIProcessBoundaries(t *testing.T) {
	root := projectRoot(t)
	binaryName := "yuxin"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(t.TempDir(), binaryName)
	build := exec.Command("go", "build", "-trimpath", "-o", binary, "./cmd/yuxin")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	t.Run("version and invalid command exit codes", func(t *testing.T) {
		stdout, stderr, code := runCLIProcess(t, root, binary, "", "--version")
		if code != 0 || !strings.Contains(stdout, "余薪 Yuxin") || stderr != "" {
			t.Fatalf("version: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		_, stderr, code = runCLIProcess(t, root, binary, "", "definitely-unknown")
		if code != 2 || !strings.Contains(stderr, "未知参数") {
			t.Fatalf("invalid command: code=%d stderr=%q", code, stderr)
		}
	})

	t.Run("anonymous share validates explicit private config", func(t *testing.T) {
		privateConfig := filepath.Join(t.TempDir(), "private.toml")
		if err := os.WriteFile(privateConfig, []byte("not valid toml\nsecret = 987654321\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runCLIProcess(t, root, binary, "", "share", "--config", privateConfig)
		if code != 2 || !strings.Contains(stderr, "读取配置") || strings.Contains(stdout, "987654321") || strings.Contains(stderr, "987654321") {
			t.Fatalf("share: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})

	t.Run("clear requires exact process input", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "config.toml")
		defaultConfig, err := os.ReadFile(filepath.Join(root, "internal", "app", "data", "default-config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(configPath, defaultConfig, 0o600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runCLIProcess(t, root, binary, "delete\n", "config", "clear", "--config", configPath)
		if code != 0 || stderr != "" || !strings.Contains(stdout, "已取消") {
			t.Fatalf("cancel clear: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if _, err := os.Stat(configPath); err != nil {
			t.Fatalf("cancelled clear removed config: %v", err)
		}
		stdout, stderr, code = runCLIProcess(t, root, binary, "DELETE\n", "config", "clear", "--config", configPath)
		if code != 0 || stderr != "" || !strings.Contains(stdout, "配置已清除") {
			t.Fatalf("confirmed clear: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("confirmed clear left config: %v", err)
		}
	})
}

func runCLIProcess(t *testing.T, root, binary, input string, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = root
	command.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("CLI %q did not exit before timeout", args)
	}
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("run CLI %q: %v", args, err)
	}
	return stdout.String(), stderr.String(), exitError.ExitCode()
}

func projectRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(directory, "..", ".."))
}
