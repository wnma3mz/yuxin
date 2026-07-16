package main

import (
	"os"

	"github.com/wnma3mz/yuxin/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
