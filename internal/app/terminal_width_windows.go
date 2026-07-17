//go:build windows

package app

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	getConsoleModeProc         = kernel32.NewProc("GetConsoleMode")
	setConsoleModeProc         = kernel32.NewProc("SetConsoleMode")
	getConsoleScreenBufferProc = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

type coordinate struct {
	x int16
	y int16
}

type smallRectangle struct {
	left   int16
	top    int16
	right  int16
	bottom int16
}

type consoleScreenBufferInfo struct {
	size              coordinate
	cursorPosition    coordinate
	attributes        uint16
	window            smallRectangle
	maximumWindowSize coordinate
}

func nativeTerminalWidth(file *os.File) int {
	info, ok := nativeTerminalSize(file)
	if !ok {
		return 0
	}
	return int(info.window.right-info.window.left) + 1
}

func nativeTerminalHeight(file *os.File) int {
	info, ok := nativeTerminalSize(file)
	if !ok {
		return 0
	}
	return int(info.window.bottom-info.window.top) + 1
}

func nativeTerminalSize(file *os.File) (consoleScreenBufferInfo, bool) {
	info := consoleScreenBufferInfo{}
	ok, _, _ := getConsoleScreenBufferProc.Call(file.Fd(), uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return consoleScreenBufferInfo{}, false
	}
	return info, true
}

func nativeIsTerminal(file *os.File) bool {
	var mode uint32
	ok, _, _ := getConsoleModeProc.Call(file.Fd(), uintptr(unsafe.Pointer(&mode)))
	return ok != 0
}

func prepareTerminal(file *os.File) (func(), bool) {
	var original uint32
	ok, _, _ := getConsoleModeProc.Call(file.Fd(), uintptr(unsafe.Pointer(&original)))
	if ok == 0 {
		return func() {}, false
	}
	const enableVirtualTerminalProcessing = 0x0004
	if original&enableVirtualTerminalProcessing != 0 {
		return func() {}, true
	}
	ok, _, _ = setConsoleModeProc.Call(file.Fd(), uintptr(original|enableVirtualTerminalProcessing))
	if ok == 0 {
		return func() {}, false
	}
	return func() {
		setConsoleModeProc.Call(file.Fd(), uintptr(original))
	}, true
}

func prepareInput(file *os.File) (func(), bool) {
	var original uint32
	ok, _, _ := getConsoleModeProc.Call(file.Fd(), uintptr(unsafe.Pointer(&original)))
	if ok == 0 {
		return func() {}, false
	}
	const (
		enableLineInput = 0x0002
		enableEchoInput = 0x0004
	)
	changed := original &^ (enableLineInput | enableEchoInput)
	ok, _, _ = setConsoleModeProc.Call(file.Fd(), uintptr(changed))
	if ok == 0 {
		return func() {}, false
	}
	return func() {
		setConsoleModeProc.Call(file.Fd(), uintptr(original))
	}, true
}

func terminalSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
