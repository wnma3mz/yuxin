//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func replaceExecutable(staged, target string) (bool, error) {
	script, err := os.CreateTemp(filepath.Dir(target), ".yuxin-update-*.cmd")
	if err != nil {
		return false, err
	}
	scriptPath := script.Name()
	content := "@echo off\r\n" +
		":retry\r\n" +
		"move /Y \"%~2\" \"%~1\" >nul 2>&1\r\n" +
		"if errorlevel 1 (\r\n" +
		"  timeout /t 1 /nobreak >nul\r\n" +
		"  goto retry\r\n" +
		")\r\n" +
		"del \"%~f0\"\r\n"
	if _, err := script.WriteString(content); err != nil {
		script.Close()
		os.Remove(scriptPath)
		return false, err
	}
	if err := script.Close(); err != nil {
		os.Remove(scriptPath)
		return false, err
	}
	command := exec.Command("cmd.exe", "/C", "start", "", "/B", scriptPath, target, staged)
	if err := command.Start(); err != nil {
		os.Remove(scriptPath)
		return false, err
	}
	return true, nil
}
