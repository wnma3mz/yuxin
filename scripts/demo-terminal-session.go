//go:build ignore

package main

import (
	"encoding/json"
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

func writeEvent(encoder *json.Encoder, timestamp float64, output string) error {
	return encoder.Encode([]any{timestamp, "o", strings.ReplaceAll(output, "\n", "\r\n")})
}

func run(path string) error {
	snapshot, config, err := app.DemoDashboard()
	if err != nil {
		return err
	}
	start := snapshot.Now
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(map[string]any{
		"version": 2,
		"width":   demoColumns,
		"height":  30,
		"env": map[string]string{
			"SHELL": "/bin/sh",
			"TERM":  "xterm-256color",
		},
	}); err != nil {
		return err
	}

	for second := 0; second < 9; second++ {
		snapshot, err = app.CalculateDashboard(start.Add(time.Duration(second)*time.Second), config)
		if err != nil {
			return err
		}
		snapshot.DemoMode = true
		output := "\x1b[?25l\x1b[?7l\x1b[2J\x1b[H" + app.RenderDashboard(snapshot, config, demoColumns, true)
		if err := writeEvent(encoder, float64(second), output); err != nil {
			return err
		}
	}

	privacyConfig := config
	privacyConfig.HideAmounts = true
	for second := 9; second < 12; second++ {
		snapshot, err = app.CalculateDashboard(start.Add(time.Duration(second)*time.Second), config)
		if err != nil {
			return err
		}
		snapshot.DemoMode = true
		output := "\x1b[?7l\x1b[2J\x1b[H" + app.RenderDashboard(snapshot, privacyConfig, demoColumns, true)
		if err := writeEvent(encoder, float64(second), output); err != nil {
			return err
		}
	}

	share, err := app.RenderShareCard(snapshot, config, "overview")
	if err != nil {
		return err
	}

	var shareScene strings.Builder
	shareScene.WriteString("\x1b[?7l\x1b[2J\x1b[H\x1b[1;97m" + centeredText("一键生成可分享画面") + "\x1b[0m\n")
	shareScene.WriteString("\x1b[90m" + centeredText("固定合成数据，不暴露工资、存款或退休信息") + "\x1b[0m\n\n")
	indent := strings.Repeat(" ", (demoColumns-62)/2)
	for _, line := range strings.Split(share, "\n") {
		shareScene.WriteString(indent + line + "\n")
	}
	return writeEvent(encoder, 12, shareScene.String())
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: demo-terminal-session CAST_PATH")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
