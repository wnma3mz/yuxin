//go:build !windows

package main

import "os"

func replaceExecutable(staged, target string) (bool, error) {
	return false, os.Rename(staged, target)
}
