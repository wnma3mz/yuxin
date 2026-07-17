//go:build windows

package app

import (
	"os"
	"os/exec"
	"path/filepath"
)

const windowsUpdateScript = "@echo off\r\n" +
	"for /L %%A in (1,1,120) do (\r\n" +
	"  move /Y \"%~2\" \"%~1\" >nul 2>&1\r\n" +
	"  if not errorlevel 1 goto updated\r\n" +
	"  timeout /t 1 /nobreak >nul\r\n" +
	")\r\n" +
	"del /F /Q \"%~2\" >nul 2>&1\r\n" +
	":updated\r\n" +
	"del \"%~f0\"\r\n"

func replaceExecutable(staged, target string) (bool, error) {
	script, err := os.CreateTemp(filepath.Dir(target), ".yuxin-update-*.cmd")
	if err != nil {
		return false, err
	}
	scriptPath := script.Name()
	if _, err := script.WriteString(windowsUpdateScript); err != nil {
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
