//go:build windows

package timer

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32watch      = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32watch.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32watch.NewProc("SetConsoleMode")
)

func consoleMode(h uintptr) (uint32, bool) {
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode)))
	return mode, r != 0
}

// listenForQuit switches stdin to raw (no line-buffering) and closes quit when
// the user presses q, Q, or Ctrl+C.
func listenForQuit(quit chan<- struct{}) {
	h := uintptr(syscall.Handle(os.Stdin.Fd()))

	oldMode, ok := consoleMode(h)
	if !ok {
		return // not an interactive console
	}
	// Clear ENABLE_LINE_INPUT (0x0002) and ENABLE_ECHO_INPUT (0x0004).
	procSetConsoleMode.Call(h, uintptr(oldMode&^uint32(0x0006)))
	defer procSetConsoleMode.Call(h, uintptr(oldMode))

	buf := make([]byte, 1)
	for {
		n, _ := os.Stdin.Read(buf)
		if n == 0 {
			return
		}
		if buf[0] == 'q' || buf[0] == 'Q' || buf[0] == 3 { // 3 = Ctrl+C
			close(quit)
			return
		}
	}
}

// isTerminal reports whether stdin is an interactive console.
func IsTerminal() bool {
	_, ok := consoleMode(uintptr(syscall.Handle(os.Stdin.Fd())))
	return ok
}


