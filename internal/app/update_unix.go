//go:build !windows

package app

import "os"

func replaceExecutable(staged, target string) (bool, error) {
	return false, os.Rename(staged, target)
}
