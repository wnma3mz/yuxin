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

	snapshot, err = app.CalculateDashboard(start.Add(12*time.Second), config)
	if err != nil {
		return err
	}
	snapshot.DemoMode = true
	output := "\x1b[?7l\x1b[2J\x1b[H" + app.RenderDashboard(snapshot, config, demoColumns, true)
	return writeEvent(encoder, 12, output)
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
