//go:build darwin || linux

package app

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

type windowSize struct {
	rows    uint16
	columns uint16
	xpixel  uint16
	ypixel  uint16
}

func nativeTerminalWidth(file *os.File) int {
	size, ok := nativeTerminalSize(file)
	if !ok {
		return 0
	}
	return int(size.columns)
}

func nativeTerminalHeight(file *os.File) int {
	size, ok := nativeTerminalSize(file)
	if !ok {
		return 0
	}
	return int(size.rows)
}

func nativeTerminalSize(file *os.File) (windowSize, bool) {
	request := uintptr(0x5413)
	if runtime.GOOS == "darwin" {
		request = 0x40087468
	}
	size := windowSize{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), request, uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return windowSize{}, false
	}
	return size, true
}

func nativeIsTerminal(file *os.File) bool {
	return nativeTerminalWidth(file) > 0
}

func prepareTerminal(_ *os.File) (func(), bool) {
	return func() {}, true
}

func prepareInput(file *os.File) (func(), bool) {
	getRequest := uintptr(0x5401)
	setRequest := uintptr(0x5402)
	if runtime.GOOS == "darwin" {
		getRequest = 0x40487413
		setRequest = 0x80487414
	}
	original := syscall.Termios{}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), getRequest, uintptr(unsafe.Pointer(&original))); errno != 0 {
		return func() {}, false
	}
	changed := original
	changed.Lflag &^= syscall.ECHO | syscall.ICANON
	changed.Cc[syscall.VMIN] = 1
	changed.Cc[syscall.VTIME] = 0
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), setRequest, uintptr(unsafe.Pointer(&changed))); errno != 0 {
		return func() {}, false
	}
	return func() {
		syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), setRequest, uintptr(unsafe.Pointer(&original)))
	}, true
}

func prepareHiddenInput(file *os.File) (func(), bool) {
	getRequest := uintptr(0x5401)
	setRequest := uintptr(0x5402)
	if runtime.GOOS == "darwin" {
		getRequest = 0x40487413
		setRequest = 0x80487414
	}
	original := syscall.Termios{}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), getRequest, uintptr(unsafe.Pointer(&original))); errno != 0 {
		return func() {}, false
	}
	changed := original
	changed.Lflag &^= syscall.ECHO
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), setRequest, uintptr(unsafe.Pointer(&changed))); errno != 0 {
		return func() {}, false
	}
	return func() {
		syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), setRequest, uintptr(unsafe.Pointer(&original)))
	}, true
}

func terminalSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGTSTP}
}
