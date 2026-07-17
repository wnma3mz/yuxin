//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/wnma3mz/yuxin/internal/app"
)

const demoColumns = 110

func centeredText(text string) string {
	width := 0
	for _, r := range text {
		if r > 255 {
			width += 2
		} else {
			width++
		}
	}
	return strings.Repeat(" ", max(0, (demoColumns-width)/2)) + text
}

func run() error {
	snapshot, config, err := app.DemoDashboard()
	if err != nil {
		return err
	}
	share, err := app.RenderShareCard(snapshot, config, "overview")
	if err != nil {
		return err
	}

	defer fmt.Print("\x1b[?25h")
	fmt.Print("\x1b[?25l\x1b[2J\x1b[H" + app.RenderDashboard(snapshot, config, demoColumns, true))
	time.Sleep(5 * time.Second)
	privacyConfig := config
	privacyConfig.HideAmounts = true
	fmt.Print("\x1b[2J\x1b[H" + app.RenderDashboard(snapshot, privacyConfig, demoColumns, true))
	time.Sleep(3 * time.Second)

	var shareScene strings.Builder
	shareScene.WriteString("\x1b[2J\x1b[H\x1b[1;97m" + centeredText("一键生成可分享画面") + "\x1b[0m\n")
	shareScene.WriteString("\x1b[90m" + centeredText("固定合成数据，不暴露工资、存款或退休信息") + "\x1b[0m\n\n")
	indent := strings.Repeat(" ", (demoColumns-62)/2)
	for _, line := range strings.Split(share, "\n") {
		shareScene.WriteString(indent + line + "\n")
	}
	fmt.Print(shareScene.String())
	time.Sleep(3 * time.Second)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
