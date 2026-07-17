//go:build darwin || linux

package app

import (
	"syscall"
	"testing"
)

func TestTerminalSignalsLeaveSuspendToUnix(t *testing.T) {
	for _, signal := range terminalSignals() {
		if signal == syscall.SIGTSTP {
			t.Fatal("SIGTSTP must retain its normal Unix suspend behavior")
		}
	}
}
