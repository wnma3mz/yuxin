package app

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleasePlatform(t *testing.T) {
	tests := []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "macos-arm64"},
		{"darwin", "amd64", "macos-x86_64"},
		{"linux", "amd64", "linux-x86_64"},
		{"windows", "amd64", "windows-x86_64"},
	}
	for _, test := range tests {
		got, err := releasePlatform(test.goos, test.goarch)
		if err != nil || got != test.want {
			t.Errorf("releasePlatform(%q, %q) = %q, %v", test.goos, test.goarch, got, err)
		}
	}
	if _, err := releasePlatform("linux", "arm64"); err == nil {
		t.Fatal("unsupported platform unexpectedly succeeded")
	}
	if _, err := releasePlatform("plan9", "amd64"); err == nil {
		t.Fatal("unsupported operating system unexpectedly succeeded")
	}
}

func TestCompareVersions(t *testing.T) {
	for _, test := range []struct {
		current, latest string
		want            int
	}{
		{"0.1.0", "v0.2.0", -1},
		{"0.2.0", "v0.2.0", 0},
		{"1.0.0", "v0.9.9", 1},
	} {
		got, err := compareVersions(test.current, test.latest)
		if err != nil || got != test.want {
			t.Errorf("compareVersions(%q, %q) = %d, %v", test.current, test.latest, got, err)
		}
	}
	if _, err := compareVersions("dev", "v0.2.0"); err == nil {
		t.Fatal("invalid version unexpectedly succeeded")
	}
	if _, err := compareVersions("0.2.0", "v0.3.0-beta.1"); err == nil {
		t.Fatal("prerelease version unexpectedly succeeded")
	}
}

func TestReleaseAssetURL(t *testing.T) {
	var release githubRelease
	release.Assets = append(release.Assets, struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}{Name: "yuxin-macos-arm64.zip", BrowserDownloadURL: "https://example.invalid/yuxin.zip"})
	url, ok := releaseAssetURL(release, "yuxin-macos-arm64.zip")
	if !ok || url == "" {
		t.Fatalf("releaseAssetURL = %q, %t", url, ok)
	}
	if _, ok := releaseAssetURL(release, "missing.zip"); ok {
		t.Fatal("missing release asset unexpectedly found")
	}
}

func TestDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.UserAgent() != "yuxin/"+version || request.Header.Get("Accept") == "" {
			t.Errorf("unexpected request headers: %#v", request.Header)
		}
		response.Write([]byte("release"))
	}))
	defer server.Close()
	content, err := download(server.Client(), server.URL)
	if err != nil || string(content) != "release" {
		t.Fatalf("download = %q, %v", content, err)
	}

	errorServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "unavailable", http.StatusServiceUnavailable)
	}))
	defer errorServer.Close()
	if _, err := download(errorServer.Client(), errorServer.URL); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("HTTP error = %v", err)
	}

	largeServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Length", fmt.Sprint(maxDownloadSize+1))
	}))
	defer largeServer.Close()
	if _, err := download(largeServer.Client(), largeServer.URL); err == nil {
		t.Fatal("oversized download unexpectedly succeeded")
	}
}

func TestVerifyChecksum(t *testing.T) {
	content := []byte("release archive")
	checksum := sha256.Sum256(content)
	filename := "yuxin-macos-arm64.zip"
	checksumFile := []byte(fmt.Sprintf("%x  %s\n", checksum, filename))
	if err := verifyChecksum(content, checksumFile, filename); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum([]byte("changed"), checksumFile, filename); err == nil {
		t.Fatal("changed archive unexpectedly passed checksum validation")
	}
	if err := verifyChecksum(content, checksumFile, "other.zip"); err == nil {
		t.Fatal("mismatched checksum filename unexpectedly succeeded")
	}
	if err := verifyChecksum(content, []byte("not-a-hash  "+filename), filename); err == nil {
		t.Fatal("invalid checksum unexpectedly succeeded")
	}
}

func TestExtractExecutable(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.Create("yuxin-v0.2.0-macos-arm64/yuxin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("binary")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := extractExecutable(archive.Bytes(), "yuxin", &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "binary" {
		t.Fatalf("extracted content = %q", output.String())
	}
	if err := extractExecutable([]byte("not a zip"), "yuxin", &bytes.Buffer{}); err == nil {
		t.Fatal("invalid archive unexpectedly succeeded")
	}
	if err := extractExecutable(archive.Bytes(), "yuxin.exe", &bytes.Buffer{}); err == nil {
		t.Fatal("missing executable unexpectedly succeeded")
	}
}

func TestRunUpdateFromLatestReleaseMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(response, `{"tag_name":%q,"assets":[]}`, "v"+version)
	}))
	defer server.Close()
	var output bytes.Buffer
	if err := runUpdateFrom(&output, server.Client(), server.URL, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "当前已是最新版") {
		t.Fatalf("update output = %q", output.String())
	}
}

func TestRunUpdateFromRejectsBadOrIncompleteRelease(t *testing.T) {
	for name, body := range map[string]string{
		"invalid JSON":  `{`,
		"missing asset": `{"tag_name":"v99.0.0","assets":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.Write([]byte(body))
			}))
			defer server.Close()
			if err := runUpdateFrom(&bytes.Buffer{}, server.Client(), server.URL, false); err == nil {
				t.Fatal("incomplete release unexpectedly succeeded")
			}
		})
	}
}

func TestRunUpdateUsingDownloadsAndVerifiesRelease(t *testing.T) {
	archive := []byte("archive bytes")
	digest := sha256.Sum256(archive)
	platform, err := releasePlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	archiveName := "yuxin-" + platform + ".zip"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/latest":
			fmt.Fprintf(response, `{"tag_name":"v99.0.0","assets":[{"name":%q,"browser_download_url":%q},{"name":%q,"browser_download_url":%q}]}`,
				archiveName, server.URL+"/archive", archiveName+".sha256", server.URL+"/checksum")
		case "/archive":
			response.Write(archive)
		case "/checksum":
			fmt.Fprintf(response, "%x  %s\n", digest, archiveName)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	called := false
	installer := func(content []byte, tag string, output io.Writer) error {
		called = true
		if !bytes.Equal(content, archive) || tag != "v99.0.0" {
			t.Fatalf("installer received %q, %q", content, tag)
		}
		fmt.Fprintln(output, "installed")
		return nil
	}
	var output bytes.Buffer
	if err := runUpdateUsing(&output, server.Client(), server.URL+"/latest", false, installer); err != nil {
		t.Fatal(err)
	}
	if !called || !strings.Contains(output.String(), "正在下载") || !strings.Contains(output.String(), "installed") {
		t.Fatalf("called %t, output %q", called, output.String())
	}
}

func TestForcedUpdateDoesNotStopAtEqualVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		fmt.Fprintf(response, `{"tag_name":%q,"assets":[]}`, "v"+version)
	}))
	defer server.Close()
	err := runUpdateFrom(&bytes.Buffer{}, server.Client(), server.URL, true)
	if err == nil || !strings.Contains(err.Error(), "缺少") {
		t.Fatalf("forced update error = %v", err)
	}
}

func TestForcedUpdateCanReinstallOlderLatestRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		fmt.Fprint(response, `{"tag_name":"v0.1.0","assets":[]}`)
	}))
	defer server.Close()
	if err := runUpdateFrom(&bytes.Buffer{}, server.Client(), server.URL, false); err != nil {
		t.Fatalf("non-forced newer local version: %v", err)
	}
	err := runUpdateFrom(&bytes.Buffer{}, server.Client(), server.URL, true)
	if err == nil || !strings.Contains(err.Error(), "缺少") {
		t.Fatalf("forced reinstall error = %v", err)
	}
}

func TestInstallUpdateUsesVerifiedArchiveExecutable(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.Create("yuxin-v99.0.0/yuxin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("new binary")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "yuxin")
	if err := os.WriteFile(target, []byte("old binary"), 0o751); err != nil {
		t.Fatal(err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, pending := range []bool{false, true} {
		var output bytes.Buffer
		replacer := func(temporary, executable string) (bool, error) {
			if executable != resolvedTarget {
				t.Fatalf("target = %q", executable)
			}
			content, err := os.ReadFile(temporary)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != "new binary" {
				t.Fatalf("temporary content = %q", content)
			}
			return pending, nil
		}
		if err := installUpdate(archive.Bytes(), "v99.0.0", &output, target, replacer); err != nil {
			t.Fatal(err)
		}
		if pending != strings.Contains(output.String(), "退出后") {
			t.Fatalf("pending %t output = %q", pending, output.String())
		}
	}
}

func TestInstallUpdatePreservesReplacementError(t *testing.T) {
	target := filepath.Join(t.TempDir(), "yuxin")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, _ := writer.Create("yuxin")
	entry.Write([]byte("new"))
	writer.Close()
	want := fmt.Errorf("replace denied")
	err := installUpdate(archive.Bytes(), "v99.0.0", &bytes.Buffer{}, target, func(string, string) (bool, error) {
		return false, want
	})
	if err == nil || !strings.Contains(err.Error(), want.Error()) {
		t.Fatalf("installUpdate error = %v", err)
	}
}
