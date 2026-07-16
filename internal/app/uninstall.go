package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func runUninstall(input io.Reader, output io.Writer, configPath string, purge bool) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("定位当前程序：%w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	return runUninstallUsing(input, output, executable, configPath, purge, uninstallExecutable)
}

func runUninstallUsing(input io.Reader, output io.Writer, executable, configPath string, purge bool, remover func(string) (bool, error)) error {
	target, err := filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("解析程序路径：%w", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("读取当前程序：%w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("当前程序路径不是普通文件")
	}
	if input == nil {
		return fmt.Errorf("缺少确认输入")
	}

	output = outputWriter(output)
	fmt.Fprintf(output, "将卸载程序：%s\n", target)
	confirmation := "UNINSTALL"
	if purge {
		if strings.TrimSpace(configPath) == "" {
			return fmt.Errorf("--purge 无法确定配置路径")
		}
		configPath, err = filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("解析配置路径：%w", err)
		}
		fmt.Fprintf(output, "同时清除配置：%s\n", configPath)
		confirmation = "PURGE"
	} else {
		fmt.Fprintln(output, "本地配置将保留。")
	}
	fmt.Fprintf(output, "输入 %s 确认：", confirmation)

	scanner := bufio.NewScanner(input)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("读取确认：%w", err)
		}
		fmt.Fprintln(output, "\n已取消卸载。")
		return nil
	}
	if strings.TrimSpace(scanner.Text()) != confirmation {
		fmt.Fprintln(output, "已取消卸载。")
		return nil
	}

	if purge {
		if err := validateConfigForUninstall(configPath); err != nil {
			return err
		}
	}
	pending, err := remover(target)
	if err != nil {
		return fmt.Errorf("删除当前程序：%w", err)
	}
	if purge {
		if err := removeConfigForUninstall(configPath); err != nil {
			return err
		}
	}
	if pending {
		fmt.Fprintln(output, "卸载已安排，退出后完成。")
	} else {
		fmt.Fprintln(output, "Yuxin 已卸载。")
	}
	return nil
}

func removeConfigForUninstall(path string) error {
	if err := validateConfigForUninstall(path); err != nil {
		return err
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("清除配置：%w", err)
	}
	return nil
}

func validateConfigForUninstall(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取配置：%w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("配置路径是目录，拒绝删除")
	}
	if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("配置路径不是普通文件，拒绝删除")
	}
	return nil
}
