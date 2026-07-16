package app

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

//go:embed VERSION
var versionFile string

var version = strings.TrimSpace(versionFile)

type cliOptions struct {
	command        string
	configPath     string
	configExplicit bool
	interval       time.Duration
	hasInterval    bool
	showVersion    bool
	showHelp       bool
}

// Run executes the Yuxin command and returns its process exit code.
func Run(args []string, stdin, stdout, stderr *os.File) int {
	return run(args, stdin, stdout, stderr)
}

func run(args []string, stdin, stdout, stderr *os.File) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		return 2
	}
	if opts.showVersion {
		fmt.Fprintf(stdout, "余薪 Yuxin %s\n", version)
		return 0
	}
	if opts.showHelp {
		fmt.Fprintln(stdout, usage)
		return 0
	}
	if opts.command == "update" {
		if err := runUpdate(stdout); err != nil {
			fmt.Fprintf(stderr, "更新失败：%v\n", err)
			return 1
		}
		return 0
	}

	path, explicit, err := resolveConfigPath(opts)
	if err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		return 2
	}
	config, source, err := readConfig(path, explicit)
	if opts.command == "config" && errors.Is(err, os.ErrNotExist) {
		config = defaultConfig()
		err = saveConfig(config, path)
		source = path
	}
	if err != nil {
		fmt.Fprintf(stderr, "错误：读取配置 %s：%v\n", path, err)
		return 2
	}
	if err := validateConfig(config); err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		return 2
	}

	if opts.command == "doctor" {
		return runDoctor(stdout, stdin, config, path, source)
	}
	if opts.command == "config" {
		if _, err := configureConfig(stdin, stdout, path, config); err != nil {
			fmt.Fprintf(stderr, "配置未保存：%v\n", err)
			return 2
		}
		return 0
	}
	persistedConfig := config
	if opts.hasInterval {
		config.RefreshInterval = opts.interval
		if err := validateConfig(config); err != nil {
			fmt.Fprintf(stderr, "错误：%v\n", err)
			return 2
		}
	}
	if opts.command == "once" || !isTerminal(stdin) || !isTerminal(stdout) {
		snapshot, err := CalculateDashboard(time.Now(), config)
		if err != nil {
			fmt.Fprintf(stderr, "错误：计算仪表盘：%v\n", err)
			return 2
		}
		frame := RenderDashboard(snapshot, config, terminalWidth(), false)
		fmt.Fprintln(stdout, frame)
		return 0
	}
	return runDashboard(stdin, stdout, stderr, persistedConfig, path, opts.interval, opts.hasInterval)
}

func parseArgs(args []string) (cliOptions, error) {
	var opts cliOptions
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--version":
			opts.showVersion = true
		case arg == "--config":
			index++
			if index >= len(args) || args[index] == "" {
				return opts, errors.New("--config 需要文件路径")
			}
			opts.configPath = args[index]
			opts.configExplicit = true
		case strings.HasPrefix(arg, "--config="):
			opts.configPath = strings.TrimPrefix(arg, "--config=")
			if opts.configPath == "" {
				return opts, errors.New("--config 需要文件路径")
			}
			opts.configExplicit = true
		case arg == "--interval":
			index++
			if index >= len(args) {
				return opts, errors.New("--interval 需要秒数")
			}
			interval, err := parseInterval(args[index])
			if err != nil {
				return opts, err
			}
			opts.interval, opts.hasInterval = interval, true
		case strings.HasPrefix(arg, "--interval="):
			interval, err := parseInterval(strings.TrimPrefix(arg, "--interval="))
			if err != nil {
				return opts, err
			}
			opts.interval, opts.hasInterval = interval, true
		case arg == "once" || arg == "doctor" || arg == "config" || arg == "update":
			if opts.command != "" {
				return opts, fmt.Errorf("只能指定一个命令")
			}
			opts.command = arg
		case arg == "-h" || arg == "--help":
			opts.showHelp = true
		default:
			return opts, fmt.Errorf("未知参数 %q", arg)
		}
	}
	return opts, nil
}

func parseInterval(value string) (time.Duration, error) {
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil || seconds <= 0 {
		return 0, errors.New("刷新间隔必须为正数")
	}
	interval := time.Duration(seconds * float64(time.Second))
	if interval <= 0 {
		return 0, errors.New("刷新间隔太小")
	}
	return interval, nil
}

func resolveConfigPath(opts cliOptions) (string, bool, error) {
	if opts.configExplicit {
		return opts.configPath, true, nil
	}
	if path := os.Getenv("YUXIN_CONFIG"); path != "" {
		return path, true, nil
	}
	directory := os.Getenv("XDG_CONFIG_HOME")
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, fmt.Errorf("无法确定配置目录：%w", err)
		}
		directory = filepath.Join(home, ".config")
	}
	return filepath.Join(directory, "yuxin", "config.toml"), false, nil
}

func readConfig(path string, explicit bool) (Config, string, error) {
	config, err := loadConfig(path)
	if err == nil {
		return config, path, nil
	}
	if !explicit && errors.Is(err, os.ErrNotExist) {
		if err := createDefaultConfig(path); err != nil {
			return Config{}, "", err
		}
		config, err := loadConfig(path)
		return config, path, err
	}
	return Config{}, "", err
}

func runDoctor(stdout, stdin *os.File, config Config, path, source string) int {
	fmt.Fprintf(stdout, "余薪 Yuxin %s\n", version)
	fmt.Fprintf(stdout, "Go: %s  ✓\n", runtime.Version())
	interactive := isTerminal(stdin) && isTerminal(stdout)
	if interactive {
		fmt.Fprintln(stdout, "终端交互: ✓")
	} else {
		fmt.Fprintln(stdout, "终端交互: 非交互（将使用 once 模式）")
	}
	if source == "内置默认值" {
		fmt.Fprintf(stdout, "配置: %s（不存在，使用内置默认值）✓\n", path)
	} else {
		fmt.Fprintf(stdout, "配置: %s  ✓\n", source)
	}
	fmt.Fprintf(stdout, "刷新间隔: %s  ✓\n", formatInterval(config.RefreshInterval))
	fmt.Fprintln(stdout, "仪表盘数据: 本地配置，无网络请求 ✓")
	return 0
}

func runDashboard(stdin, stdout, stderr *os.File, config Config, path string, intervalOverride time.Duration, hasIntervalOverride bool) int {
	for {
		dashboardConfig := config
		if hasIntervalOverride {
			dashboardConfig.RefreshInterval = intervalOverride
		}
		action, code := runDashboardSession(stdin, stdout, stderr, dashboardConfig)
		if code != 0 || action != "edit" {
			return code
		}
		updated, err := configureConfig(stdin, stdout, path, config)
		if err != nil {
			fmt.Fprintf(stderr, "配置未保存：%v\n", err)
			return 2
		}
		config = updated
	}
}

func runDashboardSession(stdin, stdout, stderr *os.File, config Config) (string, int) {
	restoreTerminal, terminalReady := prepareTerminal(stdout)
	if !terminalReady {
		snapshot, err := CalculateDashboard(time.Now(), config)
		if err != nil {
			fmt.Fprintf(stderr, "错误：计算仪表盘：%v\n", err)
			return "", 2
		}
		fmt.Fprintln(stdout, RenderDashboard(snapshot, config, terminalWidth(), false))
		return "", 0
	}
	defer restoreTerminal()
	restoreInput, inputReady := prepareInput(stdin)
	if inputReady {
		defer restoreInput()
	}
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, terminalSignals()...)
	defer signal.Stop(interrupt)

	ticker := time.NewTicker(config.RefreshInterval)
	defer ticker.Stop()
	fmt.Fprint(stdout, "\x1b[?1049h\x1b[?25l")
	defer fmt.Fprint(stdout, "\x1b[?25h\x1b[?1049l")

	details := false
	helpVisible := false
	demoMode := false
	draw := func() error {
		color := os.Getenv("NO_COLOR") == ""
		renderConfig := config
		snapshot, err := CalculateDashboard(time.Now(), renderConfig)
		if demoMode {
			snapshot, renderConfig, err = DemoDashboard()
		}
		if err != nil {
			return err
		}
		frame := renderDashboard(snapshot, renderConfig, terminalWidth(), terminalHeight(), color, details, helpVisible)
		fmt.Fprintf(stdout, "\x1b[H\x1b[2J%s", frame)
		return nil
	}
	if err := draw(); err != nil {
		fmt.Fprintf(stderr, "错误：计算仪表盘：%v\n", err)
		return "", 2
	}
	var keys <-chan byte
	if inputReady {
		keyChannel := make(chan byte)
		keys = keyChannel
		go func() {
			buffer := []byte{0}
			for {
				if _, err := stdin.Read(buffer); err != nil {
					close(keyChannel)
					return
				}
				key := buffer[0]
				keyChannel <- key
				if key == 'e' || key == 'E' || key == 'q' || key == 'Q' || key == 3 {
					return
				}
			}
		}()
	}
	for {
		select {
		case <-ticker.C:
			if err := draw(); err != nil {
				fmt.Fprintf(stderr, "错误：计算仪表盘：%v\n", err)
				return "", 2
			}
		case <-interrupt:
			return "", 0
		case key, open := <-keys:
			if !open {
				keys = nil
				continue
			}
			switch key {
			case 'q', 'Q', 3:
				return "", 0
			case 'e', 'E':
				return "edit", 0
			case 'r', 'R':
			case 's', 'S':
				demoMode = !demoMode
				details = false
				helpVisible = false
			case 'd', 'D':
				details = !details
				helpVisible = false
			case '?':
				helpVisible = !helpVisible
				details = false
			default:
				continue
			}
			if err := draw(); err != nil {
				fmt.Fprintf(stderr, "错误：计算仪表盘：%v\n", err)
				return "", 2
			}
		}
	}
}

func isTerminal(file *os.File) bool {
	return nativeIsTerminal(file)
}

func terminalWidth() int {
	if value, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && value > 0 {
		return value
	}
	if width := nativeTerminalWidth(os.Stdout); width > 0 {
		return width
	}
	return 80
}

func terminalHeight() int {
	if value, err := strconv.Atoi(os.Getenv("LINES")); err == nil && value > 0 {
		return value
	}
	if height := nativeTerminalHeight(os.Stdout); height > 0 {
		return height
	}
	return 24
}

func formatInterval(interval time.Duration) string {
	seconds := float64(interval) / float64(time.Second)
	return strconv.FormatFloat(seconds, 'f', -1, 64) + "s"
}

const usage = `用法：yuxin [once|config|doctor|update] [选项]

命令：
  once                输出一次仪表盘快照
  config              修改本地配置
  doctor              检查运行环境和配置
  update              安装 GitHub 上的最新正式版

选项：
  --config PATH       使用指定 TOML 配置
  --interval SECONDS  临时覆盖刷新间隔
  --version           显示版本
  -h, --help          显示帮助`
