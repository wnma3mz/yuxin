//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wnma3mz/yuxin/internal/app"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run scripts/generate-demo-data.go OUTPUT")
		os.Exit(2)
	}
	snapshot, config, err := app.DemoDashboard()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	share, err := app.RenderShareCard(snapshot, config, "overview")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	payload, err := json.Marshal(map[string]string{
		"dashboard": app.RenderDashboard(snapshot, config, 110, false),
		"share":     share,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	output := os.Args[1]
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(output, append([]byte("window.YUXIN_DEMO = "), append(payload, ';', '\n')...), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
