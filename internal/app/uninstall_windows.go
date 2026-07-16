//go:build windows

package app

import (
	"os"
	"os/exec"
)

func uninstallExecutable(target string) (bool, error) {
	script, err := os.CreateTemp("", "yuxin-uninstall-*.ps1")
	if err != nil {
		return false, err
	}
	scriptPath := script.Name()
	content := "param([string] $Target, [string] $ScriptPath)\r\n" +
		"$ErrorActionPreference = \"SilentlyContinue\"\r\n" +
		"$InstallDirectory = Split-Path -Parent $Target\r\n" +
		"for ($attempt = 0; $attempt -lt 120 -and (Test-Path -LiteralPath $Target); $attempt++) {\r\n" +
		"    Remove-Item -Force -LiteralPath $Target\r\n" +
		"    if (Test-Path -LiteralPath $Target) { Start-Sleep -Milliseconds 250 }\r\n" +
		"}\r\n" +
		"$DefaultDirectory = Join-Path $env:LOCALAPPDATA \"Yuxin\\bin\"\r\n" +
		"if ([string]::Equals($InstallDirectory.TrimEnd('\\'), $DefaultDirectory.TrimEnd('\\'), [StringComparison]::OrdinalIgnoreCase)) {\r\n" +
		"    $UserPath = [Environment]::GetEnvironmentVariable(\"Path\", \"User\")\r\n" +
		"    $Entries = @($UserPath -split ';' | Where-Object { $_ -and -not [string]::Equals($_.TrimEnd('\\'), $InstallDirectory.TrimEnd('\\'), [StringComparison]::OrdinalIgnoreCase) })\r\n" +
		"    [Environment]::SetEnvironmentVariable(\"Path\", ($Entries -join ';'), \"User\")\r\n" +
		"}\r\n" +
		"Remove-Item -Force -LiteralPath $ScriptPath\r\n"
	if _, err := script.WriteString(content); err != nil {
		script.Close()
		os.Remove(scriptPath)
		return false, err
	}
	if err := script.Close(); err != nil {
		os.Remove(scriptPath)
		return false, err
	}
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath, target, scriptPath)
	if err := command.Start(); err != nil {
		os.Remove(scriptPath)
		return false, err
	}
	return true, nil
}
