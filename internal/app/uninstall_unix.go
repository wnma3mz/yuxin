//go:build !windows

package app

import "os"

func uninstallExecutable(target string) (bool, error) {
	return false, os.Remove(target)
}
