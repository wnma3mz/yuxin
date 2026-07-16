package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	webModeCookie         = "yuxin_web_mode"
	webSessionCookie      = "yuxin_web_session"
	maxPendingWebTokens   = 64
	webBootstrapQueryName = "key"
)

var webPageTemplate = template.Must(template.New("web-page").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>余薪 Yuxin</title>
</head>
<body>
  <h1>余薪 Yuxin</h1>
  <p>{{.Notice}}</p>
  <pre>{{.Card}}</pre>
  {{if .Authorized}}
    <form method="post" action="{{.Action}}">
      <input type="hidden" name="token" value="{{.Token}}">
      <button type="submit">{{.Button}}</button>
    </form>
    <form method="post" action="/quit">
      <input type="hidden" name="token" value="{{.QuitToken}}">
      <button type="submit">退出余薪</button>
    </form>
  {{end}}
</body>
</html>`))

var webConfirmTemplate = template.Must(template.New("web-confirm").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>确认显示真实数据 · 余薪 Yuxin</title>
</head>
<body>
  <h1>再次确认</h1>
  <p>下一步会在本机页面显示你的真实数据。截图分享前请检查隐私字段。</p>
  <pre>{{.Card}}</pre>
  <form method="post" action="/mode/real">
    <input type="hidden" name="token" value="{{.Token}}">
    <button type="submit">确认显示真实数据</button>
  </form>
  <p><a href="/">取消，继续使用演示数据</a></p>
  <form method="post" action="/quit">
    <input type="hidden" name="token" value="{{.QuitToken}}">
    <button type="submit">退出余薪</button>
  </form>
</body>
</html>`))

type webPageData struct {
	Notice     string
	Card       string
	Action     string
	Token      string
	Button     string
	QuitToken  string
	Authorized bool
}

type webToken struct {
	stage   string
	expires time.Time
}

type webHandler struct {
	config           Config
	now              func() time.Time
	bootstrapKey     string
	webSession       string
	realSession      string
	mu               sync.Mutex
	bootstrapped     bool
	pendingToken     map[string]webToken
	bootstrapCleanup func()
	quit             func()
}

// runWeb starts the browser entry point and blocks until an interrupt arrives.
// It deliberately binds an IPv4 loopback address instead of exposing user data
// on the local network.
func runWeb(stdout io.Writer, config Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), terminalSignals()...)
	defer stop()
	return runWebContext(ctx, stdout, config, openWebBrowser)
}

func runWebContext(ctx context.Context, stdout io.Writer, config Config, opener func(string) error) error {
	handler, err := newWebHandler(config, time.Now)
	if err != nil {
		return err
	}
	quit := make(chan struct{})
	var quitOnce sync.Once
	handler.quit = func() { quitOnce.Do(func() { close(quit) }) }
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("启动本机页面: %w", err)
	}
	publicURL := "http://" + listener.Addr().String() + "/"
	capabilityURL := publicURL + "?" +
		url.Values{webBootstrapQueryName: {handler.bootstrapKey}}.Encode()
	launchURL, cleanupBootstrap, err := createWebBootstrapFile(capabilityURL)
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("创建本机引导页: %w", err)
	}
	defer cleanupBootstrap()
	handler.bootstrapCleanup = cleanupBootstrap

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	serveError := make(chan error, 1)
	go func() {
		serveError <- server.Serve(listener)
	}()

	fmt.Fprintf(stdout, "余薪本机页面：%s\n", publicURL)
	fmt.Fprintf(stdout, "若未自动打开，请手动打开：%s\n", launchURL)
	if err := opener(launchURL); err != nil {
		fmt.Fprintf(stdout, "未能自动打开浏览器：%v\n", err)
	}

	shutdown := func() error {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("关闭本机页面: %w", err)
		}
		return nil
	}
	select {
	case err := <-serveError:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("本机页面服务: %w", err)
		}
		return nil
	case <-ctx.Done():
		return shutdown()
	case <-quit:
		return shutdown()
	}
}

func newWebHandler(config Config, now func() time.Time) (*webHandler, error) {
	bootstrapKey, err := randomWebToken()
	if err != nil {
		return nil, fmt.Errorf("生成本机启动密钥: %w", err)
	}
	webSession, err := randomWebToken()
	if err != nil {
		return nil, fmt.Errorf("生成本机会话: %w", err)
	}
	realSession, err := randomWebToken()
	if err != nil {
		return nil, fmt.Errorf("生成本机会话: %w", err)
	}
	if now == nil {
		now = time.Now
	}
	return &webHandler{
		config:       config,
		now:          now,
		bootstrapKey: bootstrapKey,
		webSession:   webSession,
		realSession:  realSession,
		pendingToken: make(map[string]webToken),
	}, nil
}

func (handler *webHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	setWebSecurityHeaders(writer.Header())
	if !webRequestIsLocal(request) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}
	if request.Method == http.MethodPost && !handler.hasWebSession(request) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}

	switch request.URL.Path {
	case "/":
		if request.Method != http.MethodGet {
			methodNotAllowed(writer, http.MethodGet)
			return
		}
		if handler.bootstrap(writer, request) {
			return
		}
		authorized := handler.hasWebSession(request)
		handler.renderPage(writer, authorized && handler.realMode(request), authorized)
	case "/mode/confirm":
		if request.Method != http.MethodPost {
			methodNotAllowed(writer, http.MethodPost)
			return
		}
		handler.renderConfirmation(writer, request)
	case "/mode/real":
		if request.Method != http.MethodPost {
			methodNotAllowed(writer, http.MethodPost)
			return
		}
		handler.enableRealMode(writer, request)
	case "/mode/demo":
		if request.Method != http.MethodPost {
			methodNotAllowed(writer, http.MethodPost)
			return
		}
		handler.enableDemoMode(writer, request)
	case "/quit":
		if request.Method != http.MethodPost {
			methodNotAllowed(writer, http.MethodPost)
			return
		}
		handler.quitWeb(writer, request)
	default:
		http.NotFound(writer, request)
	}
}

func (handler *webHandler) bootstrap(writer http.ResponseWriter, request *http.Request) bool {
	key := request.URL.Query().Get(webBootstrapQueryName)
	if key == "" {
		return false
	}
	handler.mu.Lock()
	valid := !handler.bootstrapped && key == handler.bootstrapKey
	if valid {
		handler.bootstrapped = true
		handler.bootstrapKey = ""
	}
	cleanup := handler.bootstrapCleanup
	if valid {
		handler.bootstrapCleanup = nil
	}
	handler.mu.Unlock()
	if !valid {
		return false
	}
	if cleanup != nil {
		cleanup()
	}
	http.SetCookie(writer, &http.Cookie{
		Name:     webSessionCookie,
		Value:    handler.webSession,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(writer, request, "/", http.StatusSeeOther)
	return true
}

func (handler *webHandler) renderPage(writer http.ResponseWriter, realMode, authorized bool) {
	var (
		snapshot DashboardSnapshot
		config   Config
		err      error
	)
	data := webPageData{Authorized: authorized}
	if realMode {
		config = handler.config
		snapshot, err = CalculateDashboard(handler.now(), config)
		data.Notice = "当前显示本机真实数据。分享前请检查隐私字段。"
		data.Action = "/mode/demo"
		data.Button = "切回隐私演示"
	} else {
		snapshot, config, err = DemoDashboard()
		data.Notice = "默认使用合成数据，可安全用于截图分享。"
		data.Action = "/mode/confirm"
		data.Button = "显示真实数据"
	}
	if err != nil {
		http.Error(writer, "生成分享卡片失败", http.StatusInternalServerError)
		return
	}
	data.Card, err = RenderShareCard(snapshot, config, "overview")
	if err != nil {
		http.Error(writer, "生成分享卡片失败", http.StatusInternalServerError)
		return
	}
	if authorized {
		stage := "confirm"
		if realMode {
			stage = "demo"
		}
		data.Token, err = handler.issueToken(stage)
		if err != nil {
			http.Error(writer, "生成本机会话失败", http.StatusInternalServerError)
			return
		}
		data.QuitToken, err = handler.issueToken("quit")
		if err != nil {
			http.Error(writer, "生成本机会话失败", http.StatusInternalServerError)
			return
		}
	}
	writeWebTemplate(writer, webPageTemplate, data)
}

func (handler *webHandler) renderConfirmation(writer http.ResponseWriter, request *http.Request) {
	if !handler.consumeRequestToken(writer, request, "confirm") {
		return
	}
	snapshot, config, err := DemoDashboard()
	if err != nil {
		http.Error(writer, "生成分享卡片失败", http.StatusInternalServerError)
		return
	}
	card, err := RenderShareCard(snapshot, config, "overview")
	if err != nil {
		http.Error(writer, "生成分享卡片失败", http.StatusInternalServerError)
		return
	}
	token, err := handler.issueToken("real")
	if err != nil {
		http.Error(writer, "生成本机会话失败", http.StatusInternalServerError)
		return
	}
	quitToken, err := handler.issueToken("quit")
	if err != nil {
		http.Error(writer, "生成本机会话失败", http.StatusInternalServerError)
		return
	}
	writeWebTemplate(writer, webConfirmTemplate, struct {
		Card      string
		Token     string
		QuitToken string
	}{card, token, quitToken})
}

func (handler *webHandler) enableRealMode(writer http.ResponseWriter, request *http.Request) {
	if !handler.consumeRequestToken(writer, request, "real") {
		return
	}
	http.SetCookie(writer, &http.Cookie{
		Name:     webModeCookie,
		Value:    handler.realSession,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	handler.renderPage(writer, true, true)
}

func (handler *webHandler) enableDemoMode(writer http.ResponseWriter, request *http.Request) {
	if !handler.consumeRequestToken(writer, request, "demo") {
		return
	}
	http.SetCookie(writer, &http.Cookie{
		Name:     webModeCookie,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	handler.renderPage(writer, false, true)
}

func (handler *webHandler) quitWeb(writer http.ResponseWriter, request *http.Request) {
	if !handler.consumeRequestToken(writer, request, "quit") {
		return
	}
	fmt.Fprintln(writer, "<!doctype html><html lang=\"zh-CN\"><meta charset=\"utf-8\"><title>余薪已退出</title><p>余薪已退出，可以关闭此页面。</p></html>")
	if handler.quit != nil {
		handler.quit()
	}
}

func (handler *webHandler) realMode(request *http.Request) bool {
	cookie, err := request.Cookie(webModeCookie)
	return err == nil && cookie.Value == handler.realSession
}

func (handler *webHandler) hasWebSession(request *http.Request) bool {
	cookie, err := request.Cookie(webSessionCookie)
	return err == nil && cookie.Value == handler.webSession
}

func (handler *webHandler) issueToken(stage string) (string, error) {
	token, err := randomWebToken()
	if err != nil {
		return "", err
	}
	now := handler.now()
	handler.mu.Lock()
	defer handler.mu.Unlock()
	for value, pending := range handler.pendingToken {
		if !pending.expires.After(now) {
			delete(handler.pendingToken, value)
		}
	}
	for len(handler.pendingToken) >= maxPendingWebTokens {
		var oldestValue string
		var oldestExpiry time.Time
		for value, pending := range handler.pendingToken {
			if oldestValue == "" || pending.expires.Before(oldestExpiry) {
				oldestValue = value
				oldestExpiry = pending.expires
			}
		}
		delete(handler.pendingToken, oldestValue)
	}
	handler.pendingToken[token] = webToken{stage: stage, expires: now.Add(5 * time.Minute)}
	return token, nil
}

func (handler *webHandler) consumeRequestToken(writer http.ResponseWriter, request *http.Request, stage string) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 4<<10)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "invalid request", http.StatusBadRequest)
		return false
	}
	token := request.PostForm.Get("token")
	now := handler.now()
	handler.mu.Lock()
	pending, found := handler.pendingToken[token]
	if found {
		delete(handler.pendingToken, token)
	}
	handler.mu.Unlock()
	if !found || pending.stage != stage || !pending.expires.After(now) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func randomWebToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func createWebBootstrapFile(capabilityURL string) (string, func(), error) {
	file, err := os.CreateTemp("", "yuxin-launch-*.html")
	if err != nil {
		return "", nil, err
	}
	path := file.Name()
	var once sync.Once
	cleanup := func() {
		once.Do(func() { _ = os.Remove(path) })
	}
	fail := func(err error) (string, func(), error) {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	if err := file.Chmod(0o600); err != nil {
		return fail(err)
	}
	escapedURL := html.EscapeString(capabilityURL)
	content := `<!doctype html><meta charset="utf-8"><meta name="referrer" content="no-referrer">` +
		`<meta http-equiv="refresh" content="0;url=` + escapedURL + `">` +
		`<title>正在打开余薪</title><a href="` + escapedURL + `">打开余薪</a>`
	if _, err := file.WriteString(content); err != nil {
		return fail(err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	urlPath := filepath.ToSlash(path)
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	return (&url.URL{Scheme: "file", Path: urlPath}).String(), cleanup, nil
}

func webRequestIsLocal(request *http.Request) bool {
	host := request.Host
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	host = strings.Trim(host, "[]")
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return false
	}
	remoteHost, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return false
	}
	return net.ParseIP(remoteHost).IsLoopback()
}

func setWebSecurityHeaders(header http.Header) {
	header.Set("Content-Security-Policy", "default-src 'none'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Type", "text/html; charset=utf-8")
}

func methodNotAllowed(writer http.ResponseWriter, allow string) {
	writer.Header().Set("Allow", allow)
	http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
}

func writeWebTemplate(writer http.ResponseWriter, page *template.Template, data any) {
	if err := page.Execute(writer, data); err != nil {
		// Headers or part of the response might already have been written. The
		// templates are static, so this path only guards unexpected writer errors.
		return
	}
}

func openWebBrowser(localURL string) error {
	if _, err := url.ParseRequestURI(localURL); err != nil {
		return err
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", localURL)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", localURL)
	case "linux":
		command = exec.Command("xdg-open", localURL)
	default:
		return fmt.Errorf("不支持自动打开 %s", runtime.GOOS)
	}
	if err := command.Start(); err != nil {
		return err
	}
	go func() { _ = command.Wait() }()
	return nil
}
