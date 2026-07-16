//go:build !windows

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceExecutable(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "yuxin")
	staged := filepath.Join(directory, "update")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	pending, err := replaceExecutable(staged, target)
	if err != nil || pending {
		t.Fatalf("replaceExecutable = pending %t, error %v", pending, err)
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != "new" {
		t.Fatalf("replacement content = %q, %v", content, err)
	}
}
