//go:build darwin || linux

package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCLIInterruptAtConfigPromptPreservesConfig(t *testing.T) {
	root := projectRoot(t)
	binary := filepath.Join(t.TempDir(), "yuxin")
	build := exec.Command("go", "build", "-trimpath", "-o", binary, "./cmd/yuxin")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	configPath := filepath.Join(t.TempDir(), "config.toml")
	before, err := os.ReadFile(filepath.Join(root, "internal", "app", "data", "default-config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, before, 0o600); err != nil {
		t.Fatal(err)
	}

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinReader.Close()
	defer stdinWriter.Close()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutReader.Close()
	defer stdoutWriter.Close()
	command := exec.Command(binary, "config", "--config", configPath)
	command.Dir = root
	command.Stdin = stdinReader
	command.Stdout = stdoutWriter
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_ = stdinReader.Close()
	_ = stdoutWriter.Close()

	promptSeen := make(chan struct{})
	readDone := make(chan struct{})
	collector := &promptCollector{needle: "选择要修改的部分", seen: promptSeen}
	go func() {
		_, _ = io.Copy(collector, stdoutReader)
		close(readDone)
	}()
	select {
	case <-promptSeen:
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("CLI did not reach config prompt; stdout=%q stderr=%q", collector.String(), stderr.String())
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		_ = command.Process.Kill()
		t.Fatal(err)
	}
	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	select {
	case err := <-wait:
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Fatalf("interrupt result = %v", err)
		}
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("CLI did not exit after interrupt")
	}
	_ = stdoutReader.Close()
	<-readDone
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("interrupt at the initial prompt changed the config")
	}
}

type promptCollector struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	needle string
	seen   chan struct{}
	once   sync.Once
}

func (collector *promptCollector) Write(value []byte) (int, error) {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	count, err := collector.buffer.Write(value)
	if strings.Contains(collector.buffer.String(), collector.needle) {
		collector.once.Do(func() { close(collector.seen) })
	}
	return count, err
}

func (collector *promptCollector) String() string {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	return collector.buffer.String()
}
