//go:build !windows

package timer

import "os"

func listenForQuit(quit chan<- struct{}) {
	buf := make([]byte, 1)
	for {
		n, _ := os.Stdin.Read(buf)
		if n == 0 {
			return
		}
		if buf[0] == 'q' || buf[0] == 'Q' {
			close(quit)
			return
		}
	}
}

func IsTerminal() bool { return false }


