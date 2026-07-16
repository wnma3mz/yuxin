package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

var hiddenWebToken = regexp.MustCompile(`name="token" value="([A-Za-z0-9_-]+)"`)
var hiddenQuitToken = regexp.MustCompile(`(?s)action="/quit".*?name="token" value="([A-Za-z0-9_-]+)"`)

func TestWebDefaultsToSyntheticDataAndRequiresTwoConfirmations(t *testing.T) {
	now := time.Date(2026, time.July, 16, 15, 24, 0, 0, time.Local)
	config := defaultConfig()
	config.SalaryMode = "daily"
	config.SalaryAmount = 98765

	handler, err := newWebHandler(config, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	demoSnapshot, demoConfig, err := DemoDashboard()
	if err != nil {
		t.Fatal(err)
	}
	demoCard, err := RenderShareCard(demoSnapshot, demoConfig, "overview")
	if err != nil {
		t.Fatal(err)
	}
	realSnapshot, err := CalculateDashboard(now, config)
	if err != nil {
		t.Fatal(err)
	}
	realCard, err := RenderShareCard(realSnapshot, config, "overview")
	if err != nil {
		t.Fatal(err)
	}

	publicPage := webRequest(t, handler, http.MethodGet, "/", "")
	if publicPage.Code != http.StatusOK {
		t.Fatalf("public GET / status = %d, body = %q", publicPage.Code, publicPage.Body.String())
	}
	if body := publicPage.Body.String(); !strings.Contains(body, demoCard) || strings.Contains(body, realCard) || hiddenWebToken.MatchString(body) || hiddenQuitToken.MatchString(body) {
		t.Fatalf("public page exposed privileged content:\n%s", body)
	}
	unauthorizedPost := webRequest(t, handler, http.MethodPost, "/mode/confirm", "token=invalid")
	if unauthorizedPost.Code != http.StatusForbidden {
		t.Fatalf("unauthorized POST status = %d, body = %q", unauthorizedPost.Code, unauthorizedPost.Body.String())
	}

	sessionCookie := bootstrapWebSession(t, handler)
	defaultPage := webRequest(t, handler, http.MethodGet, "/", "", sessionCookie)
	if defaultPage.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, body = %q", defaultPage.Code, defaultPage.Body.String())
	}
	if body := defaultPage.Body.String(); !strings.Contains(body, demoCard) || strings.Contains(body, realCard) {
		t.Fatalf("default page did not isolate synthetic data:\n%s", body)
	}
	assertWebSecurityHeaders(t, defaultPage.Header())

	// Knowing the route is deliberately insufficient: the first page token can
	// only open the confirmation screen, and a direct real-mode request fails.
	directReal := webRequest(t, handler, http.MethodPost, "/mode/real", "token=invalid", sessionCookie)
	if directReal.Code != http.StatusForbidden || strings.Contains(directReal.Body.String(), realCard) {
		t.Fatalf("direct real-mode request = %d, body = %q", directReal.Code, directReal.Body.String())
	}

	confirmToken := webTokenFromPage(t, defaultPage.Body.String())
	confirmation := webRequest(t, handler, http.MethodPost, "/mode/confirm", url.Values{"token": {confirmToken}}.Encode(), sessionCookie)
	if confirmation.Code != http.StatusOK {
		t.Fatalf("confirmation status = %d, body = %q", confirmation.Code, confirmation.Body.String())
	}
	if body := confirmation.Body.String(); !strings.Contains(body, "再次确认") || !strings.Contains(body, demoCard) || strings.Contains(body, realCard) {
		t.Fatalf("confirmation page exposed unexpected data:\n%s", body)
	}

	// Confirmation tokens are one-use, preventing replay of the first step.
	replayed := webRequest(t, handler, http.MethodPost, "/mode/confirm", url.Values{"token": {confirmToken}}.Encode(), sessionCookie)
	if replayed.Code != http.StatusForbidden {
		t.Fatalf("replayed confirmation status = %d", replayed.Code)
	}

	realToken := webTokenFromPage(t, confirmation.Body.String())
	realPage := webRequest(t, handler, http.MethodPost, "/mode/real", url.Values{"token": {realToken}}.Encode(), sessionCookie)
	if realPage.Code != http.StatusOK || !strings.Contains(realPage.Body.String(), realCard) {
		t.Fatalf("confirmed real page = %d, body = %q", realPage.Code, realPage.Body.String())
	}
	realCookie := findWebCookie(t, realPage.Result().Cookies(), webModeCookie)
	if !realCookie.HttpOnly || realCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("real-mode cookie lacks privacy attributes: %#v", realCookie)
	}

	persistedRealPage := webRequest(t, handler, http.MethodGet, "/", "", sessionCookie, realCookie)
	if !strings.Contains(persistedRealPage.Body.String(), realCard) {
		t.Fatalf("confirmed real mode was not retained: %q", persistedRealPage.Body.String())
	}

	demoToken := webTokenFromPage(t, realPage.Body.String())
	demoPage := webRequest(t, handler, http.MethodPost, "/mode/demo", url.Values{"token": {demoToken}}.Encode(), sessionCookie, realCookie)
	if demoPage.Code != http.StatusOK || !strings.Contains(demoPage.Body.String(), demoCard) {
		t.Fatalf("switch to demo = %d, body = %q", demoPage.Code, demoPage.Body.String())
	}
	clearedCookie := findWebCookie(t, demoPage.Result().Cookies(), webModeCookie)
	if clearedCookie.MaxAge >= 0 {
		t.Fatalf("demo mode did not clear real-mode cookie: %#v", clearedCookie)
	}
}

func TestWebRejectsNonLocalRequestsAndUnsafeMethods(t *testing.T) {
	handler, err := newWebHandler(defaultConfig(), time.Now)
	if err != nil {
		t.Fatal(err)
	}

	nonLocalHost := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	nonLocalHost.RemoteAddr = "127.0.0.1:12345"
	nonLocalHostResponse := httptest.NewRecorder()
	handler.ServeHTTP(nonLocalHostResponse, nonLocalHost)
	if nonLocalHostResponse.Code != http.StatusForbidden {
		t.Fatalf("non-local host status = %d", nonLocalHostResponse.Code)
	}

	nonLocalPeer := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/", nil)
	nonLocalPeer.RemoteAddr = "192.0.2.10:12345"
	nonLocalPeerResponse := httptest.NewRecorder()
	handler.ServeHTTP(nonLocalPeerResponse, nonLocalPeer)
	if nonLocalPeerResponse.Code != http.StatusForbidden {
		t.Fatalf("non-local peer status = %d", nonLocalPeerResponse.Code)
	}

	unsafeMethod := webRequest(t, handler, http.MethodGet, "/mode/real", "")
	if unsafeMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /mode/real status = %d", unsafeMethod.Code)
	}
	unknownRoute := webRequest(t, handler, http.MethodGet, "/anything", "")
	if unknownRoute.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d", unknownRoute.Code)
	}
}

func TestWebQuitRequiresPageTokenAndStopsServer(t *testing.T) {
	handler, err := newWebHandler(defaultConfig(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	quit := make(chan struct{})
	handler.quit = func() { close(quit) }

	directQuit := webRequest(t, handler, http.MethodPost, "/quit", "token=invalid")
	if directQuit.Code != http.StatusForbidden {
		t.Fatalf("direct quit status = %d", directQuit.Code)
	}
	sessionCookie := bootstrapWebSession(t, handler)
	page := webRequest(t, handler, http.MethodGet, "/", "", sessionCookie)
	match := hiddenQuitToken.FindStringSubmatch(page.Body.String())
	if len(match) != 2 {
		t.Fatalf("page has no quit token: %q", page.Body.String())
	}
	response := webRequest(t, handler, http.MethodPost, "/quit", url.Values{"token": {match[1]}}.Encode(), sessionCookie)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "余薪已退出") {
		t.Fatalf("quit response = %d, body = %q", response.Code, response.Body.String())
	}
	select {
	case <-quit:
	default:
		t.Fatal("quit endpoint did not stop the server")
	}
}

func TestRunWebContextListensOnlyOnLoopbackAndSurvivesBrowserFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var output bytes.Buffer
	var launchPath string
	opener := func(value string) error {
		parsed, err := url.Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Scheme != "file" || parsed.RawQuery != "" || strings.Contains(value, webBootstrapQueryName+"=") {
			t.Fatalf("opener received capability URL instead of private file: %q", value)
		}
		launchPath = filepath.FromSlash(parsed.Path)
		if runtime.GOOS == "windows" {
			launchPath = strings.TrimPrefix(launchPath, string(filepath.Separator))
		}
		content, err := os.ReadFile(launchPath)
		if err != nil {
			t.Fatalf("read bootstrap file: %v", err)
		}
		if text := string(content); !strings.Contains(text, "http://127.0.0.1:") || !strings.Contains(text, "?"+webBootstrapQueryName+"=") {
			t.Fatalf("bootstrap file does not contain the local capability URL: %q", text)
		}
		info, err := os.Stat(launchPath)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatalf("bootstrap permissions = %o", info.Mode().Perm())
		}
		cancel()
		return errors.New("browser unavailable")
	}
	if err := runWebContext(ctx, &output, defaultConfig(), opener); err != nil {
		t.Fatal(err)
	}
	if text := output.String(); !strings.Contains(text, "http://127.0.0.1:") || !strings.Contains(text, "请手动打开：file:") {
		t.Fatalf("runWebContext output = %q", text)
	}
	if _, err := os.Stat(launchPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("bootstrap file was not removed: %v", err)
	}
}

func TestWebPendingTokenCountIsStrictlyLimited(t *testing.T) {
	now := time.Date(2026, time.July, 16, 15, 24, 0, 0, time.Local)
	handler, err := newWebHandler(defaultConfig(), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	for range maxPendingWebTokens + 20 {
		if _, err := handler.issueToken("confirm"); err != nil {
			t.Fatal(err)
		}
		if got := len(handler.pendingToken); got > maxPendingWebTokens {
			t.Fatalf("pending token count = %d, want <= %d", got, maxPendingWebTokens)
		}
	}
	if got := len(handler.pendingToken); got != maxPendingWebTokens {
		t.Fatalf("pending token count after eviction = %d, want %d", got, maxPendingWebTokens)
	}

	now = now.Add(6 * time.Minute)
	if _, err := handler.issueToken("confirm"); err != nil {
		t.Fatal(err)
	}
	if got := len(handler.pendingToken); got != 1 {
		t.Fatalf("pending token count after expiry cleanup = %d, want 1", got)
	}
}

func bootstrapWebSession(t *testing.T, handler *webHandler) *http.Cookie {
	t.Helper()
	key := handler.bootstrapKey
	response := webRequest(t, handler, http.MethodGet, "/?"+url.Values{webBootstrapQueryName: {key}}.Encode(), "")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("bootstrap response = %d, location = %q, body = %q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	cookie := findWebCookie(t, response.Result().Cookies(), webSessionCookie)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Fatalf("bootstrap cookie lacks privacy attributes: %#v", cookie)
	}
	secondUse := webRequest(t, handler, http.MethodGet, "/?"+url.Values{webBootstrapQueryName: {key}}.Encode(), "")
	if secondUse.Code != http.StatusOK || hiddenWebToken.MatchString(secondUse.Body.String()) || hiddenQuitToken.MatchString(secondUse.Body.String()) {
		t.Fatalf("reused bootstrap key retained privileges: status = %d, body = %q", secondUse.Code, secondUse.Body.String())
	}
	return cookie
}

func webRequest(t *testing.T, handler http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "http://127.0.0.1"+path, strings.NewReader(body))
	request.RemoteAddr = "127.0.0.1:12345"
	if body != "" {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func webTokenFromPage(t *testing.T, body string) string {
	t.Helper()
	match := hiddenWebToken.FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("page has no action token: %q", body)
	}
	return match[1]
}

func findWebCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response has no %s cookie: %#v", name, cookies)
	return nil
}

func assertWebSecurityHeaders(t *testing.T, header http.Header) {
	t.Helper()
	for name, want := range map[string]string{
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if policy := header.Get("Content-Security-Policy"); !strings.Contains(policy, "default-src 'none'") || !strings.Contains(policy, "form-action 'self'") {
		t.Errorf("Content-Security-Policy = %q", policy)
	}
}
