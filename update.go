package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	latestReleaseURL = "https://api.github.com/repos/wnma3mz/yuxin/releases/latest"
	maxDownloadSize  = 100 << 20
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func runUpdate(output io.Writer) error {
	platform, err := releasePlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	fmt.Fprintln(output, "正在检查 GitHub 最新正式版…")
	client := &http.Client{Timeout: 90 * time.Second}
	releaseData, err := download(client, latestReleaseURL)
	if err != nil {
		return fmt.Errorf("读取最新版本：%w", err)
	}
	var release githubRelease
	if err := json.Unmarshal(releaseData, &release); err != nil {
		return fmt.Errorf("解析最新版本：%w", err)
	}
	comparison, err := compareVersions(version, release.TagName)
	if err != nil {
		return err
	}
	if comparison >= 0 {
		fmt.Fprintf(output, "当前已是最新版 %s。\n", version)
		return nil
	}

	archiveName := "yuxin-" + platform + ".zip"
	archiveURL, ok := releaseAssetURL(release, archiveName)
	if !ok {
		return fmt.Errorf("发布 %s 缺少 %s", release.TagName, archiveName)
	}
	checksumURL, ok := releaseAssetURL(release, archiveName+".sha256")
	if !ok {
		return fmt.Errorf("发布 %s 缺少 %s.sha256", release.TagName, archiveName)
	}
	fmt.Fprintf(output, "正在下载 %s…\n", release.TagName)
	archiveData, err := download(client, archiveURL)
	if err != nil {
		return fmt.Errorf("下载更新：%w", err)
	}
	checksumData, err := download(client, checksumURL)
	if err != nil {
		return fmt.Errorf("下载校验文件：%w", err)
	}
	if err := verifyChecksum(archiveData, checksumData, archiveName); err != nil {
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("定位当前程序：%w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("读取当前程序：%w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(executable), ".yuxin-update-*")
	if err != nil {
		return fmt.Errorf("创建更新文件：%w", err)
	}
	temporaryPath := temporary.Name()
	keepTemporary := false
	defer func() {
		temporary.Close()
		if !keepTemporary {
			os.Remove(temporaryPath)
		}
	}()
	executableName := "yuxin"
	if runtime.GOOS == "windows" {
		executableName += ".exe"
	}
	if err := extractExecutable(archiveData, executableName, temporary); err != nil {
		return err
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o755
	}
	if err := temporary.Chmod(mode); err != nil {
		return fmt.Errorf("设置更新权限：%w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("写入更新：%w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("关闭更新文件：%w", err)
	}
	pending, err := replaceExecutable(temporaryPath, executable)
	if err != nil {
		return fmt.Errorf("替换当前程序：%w", err)
	}
	keepTemporary = pending
	if pending {
		fmt.Fprintf(output, "已下载 %s，退出后将完成替换。\n", release.TagName)
	} else {
		fmt.Fprintf(output, "已更新到 %s。\n", release.TagName)
	}
	return nil
}

func releasePlatform(goos, goarch string) (string, error) {
	arch := map[string]string{"amd64": "x86_64", "arm64": "arm64"}[goarch]
	if arch == "" {
		return "", fmt.Errorf("暂不支持处理器架构 %s", goarch)
	}
	osName := map[string]string{"darwin": "macos", "linux": "linux", "windows": "windows"}[goos]
	if osName == "" || osName != "macos" && arch == "arm64" {
		return "", fmt.Errorf("暂不支持平台 %s/%s", goos, goarch)
	}
	return osName + "-" + arch, nil
}

func releaseAssetURL(release githubRelease, name string) (string, bool) {
	for _, asset := range release.Assets {
		if asset.Name == name && asset.BrowserDownloadURL != "" {
			return asset.BrowserDownloadURL, true
		}
	}
	return "", false
}

func download(client *http.Client, url string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "yuxin/"+version)
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("HTTP %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	if response.ContentLength > maxDownloadSize {
		return nil, fmt.Errorf("下载内容超过 100 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxDownloadSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDownloadSize {
		return nil, fmt.Errorf("下载内容超过 100 MiB")
	}
	return data, nil
}

func verifyChecksum(content, checksumFile []byte, filename string) error {
	fields := strings.Fields(string(checksumFile))
	if len(fields) < 2 || strings.TrimPrefix(fields[1], "*") != filename {
		return fmt.Errorf("校验文件与 %s 不匹配", filename)
	}
	want, err := hex.DecodeString(fields[0])
	if err != nil || len(want) != sha256.Size {
		return fmt.Errorf("SHA-256 格式无效")
	}
	got := sha256.Sum256(content)
	if !bytes.Equal(got[:], want) {
		return fmt.Errorf("%s 的 SHA-256 校验失败", filename)
	}
	return nil
}

func extractExecutable(archiveData []byte, executableName string, output io.Writer) error {
	archive, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
	if err != nil {
		return fmt.Errorf("打开更新包：%w", err)
	}
	found := false
	for _, entry := range archive.File {
		if path.Base(entry.Name) != executableName || entry.FileInfo().IsDir() {
			continue
		}
		if found {
			return fmt.Errorf("更新包包含多个 %s", executableName)
		}
		if entry.UncompressedSize64 > maxDownloadSize {
			return fmt.Errorf("更新程序超过 100 MiB")
		}
		source, err := entry.Open()
		if err != nil {
			return fmt.Errorf("读取更新程序：%w", err)
		}
		_, copyErr := io.Copy(output, io.LimitReader(source, maxDownloadSize+1))
		closeErr := source.Close()
		if copyErr != nil {
			return fmt.Errorf("解压更新程序：%w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("关闭更新程序：%w", closeErr)
		}
		found = true
	}
	if !found {
		return fmt.Errorf("更新包缺少 %s", executableName)
	}
	return nil
}

func compareVersions(current, latest string) (int, error) {
	currentParts, err := parseVersion(current)
	if err != nil {
		return 0, fmt.Errorf("当前版本 %q 无效：%w", current, err)
	}
	latestParts, err := parseVersion(latest)
	if err != nil {
		return 0, fmt.Errorf("最新版本 %q 无效：%w", latest, err)
	}
	for index := range currentParts {
		if currentParts[index] < latestParts[index] {
			return -1, nil
		}
		if currentParts[index] > latestParts[index] {
			return 1, nil
		}
	}
	return 0, nil
}

func parseVersion(value string) ([3]int, error) {
	var result [3]int
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	if strings.ContainsAny(value, "-+") {
		return result, fmt.Errorf("只支持正式语义化版本")
	}
	parts := strings.Split(value, ".")
	if len(parts) != len(result) {
		return result, fmt.Errorf("需要 major.minor.patch")
	}
	for index, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return result, fmt.Errorf("版本号必须是非负整数")
		}
		result[index] = number
	}
	return result, nil
}
