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

func exportConfig(source Config, destination string, stdout io.Writer) error {
	destinationPath, err := absoluteConfigPath(destination)
	if err != nil {
		return fmt.Errorf("导出配置: %w", err)
	}
	if err := rejectDirectory(destinationPath); err != nil {
		return fmt.Errorf("导出配置: %w", err)
	}
	stdout = outputWriter(stdout)
	fmt.Fprintln(stdout, "提醒：导出的配置可能包含薪资、资产和出生日期等敏感信息，请妥善保管。")
	if err := saveConfig(source, destinationPath); err != nil {
		return fmt.Errorf("导出配置: %w", err)
	}

	fmt.Fprintf(stdout, "已导出配置：%s\n", destinationPath)
	return nil
}

func importConfig(sourcePath, targetPath string, stdout io.Writer) error {
	source, err := absoluteConfigPath(sourcePath)
	if err != nil {
		return fmt.Errorf("导入配置: %w", err)
	}
	target, err := absoluteConfigPath(targetPath)
	if err != nil {
		return fmt.Errorf("导入配置: %w", err)
	}
	if samePath(source, target) {
		return fmt.Errorf("导入配置: 源文件与目标配置是同一文件")
	}
	if err := requireRegularFile(source); err != nil {
		return fmt.Errorf("导入配置: %w", err)
	}
	if err := rejectDirectory(target); err != nil {
		return fmt.Errorf("导入配置: %w", err)
	}

	// Parse and validate the complete source before saveConfig touches the target.
	config, err := loadConfig(source)
	if err != nil {
		return fmt.Errorf("导入配置: 源文件无效: %w", err)
	}
	if err := saveConfig(config, target); err != nil {
		return fmt.Errorf("导入配置: %w", err)
	}

	fmt.Fprintf(outputWriter(stdout), "已导入配置：%s\n", target)
	return nil
}

func clearConfig(path string, input io.Reader, stdout io.Writer) error {
	target, err := absoluteConfigPath(path)
	if err != nil {
		return fmt.Errorf("清除配置: %w", err)
	}
	stdout = outputWriter(stdout)
	fmt.Fprintf(stdout, "将要清除的配置：%s\n", target)

	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(stdout, "配置文件不存在，无需清除。")
		return nil
	}
	if err != nil {
		return fmt.Errorf("清除配置: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("清除配置: 目标是目录，拒绝删除")
	}
	if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("清除配置: 目标不是普通文件，拒绝删除")
	}
	if input == nil {
		return fmt.Errorf("清除配置: 缺少确认输入")
	}

	fmt.Fprint(stdout, "输入 DELETE 确认清除：")
	scanner := bufio.NewScanner(input)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("清除配置: 读取确认: %w", err)
		}
		fmt.Fprintln(stdout, "\n已取消，配置未删除。")
		return nil
	}
	if strings.TrimSpace(scanner.Text()) != "DELETE" {
		fmt.Fprintln(stdout, "已取消，配置未删除。")
		return nil
	}
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("清除配置: %w", err)
	}
	fmt.Fprintln(stdout, "配置已清除。")
	return nil
}

func absoluteConfigPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("路径不能为空")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("解析路径: %w", err)
	}
	return filepath.Clean(absolute), nil
}

func samePath(first, second string) bool {
	if first == second {
		return true
	}
	firstInfo, firstErr := os.Stat(first)
	secondInfo, secondErr := os.Stat(second)
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}

func requireRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("源路径不是普通文件")
	}
	return nil
}

func rejectDirectory(path string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("目标路径是目录")
	}
	return nil
}

func outputWriter(writer io.Writer) io.Writer {
	if writer == nil {
		return io.Discard
	}
	return writer
}
