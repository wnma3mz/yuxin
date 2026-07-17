//go:build windows

package app

import (
	"os"
	"os/exec"
	"path/filepath"
)

const windowsUpdateScript = "param([string] $Target, [string] $Staged, [string] $ScriptPath)\r\n" +
	"$ErrorActionPreference = \"SilentlyContinue\"\r\n" +
	"for ($attempt = 0; $attempt -lt 120 -and (Test-Path -LiteralPath $Staged); $attempt++) {\r\n" +
	"    Move-Item -Force -LiteralPath $Staged -Destination $Target\r\n" +
	"    if (Test-Path -LiteralPath $Staged) { Start-Sleep -Milliseconds 250 }\r\n" +
	"}\r\n" +
	"if (Test-Path -LiteralPath $Staged) { Remove-Item -Force -LiteralPath $Staged }\r\n" +
	"Remove-Item -Force -LiteralPath $ScriptPath\r\n"

func replaceExecutable(staged, target string) (bool, error) {
	script, err := os.CreateTemp(filepath.Dir(target), ".yuxin-update-*.ps1")
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
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath, target, staged, scriptPath)
	if err := command.Start(); err != nil {
		os.Remove(scriptPath)
		return false, err
	}
	return true, nil
}
