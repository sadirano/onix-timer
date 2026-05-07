//go:build !windows

package timer

import (
	"os"
	"os/exec"
	"syscall"
)

func isDaemonAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func spawnDaemon(exe, onixHome string) {
	cmd := exec.Command(exe, "daemon", onixHome)
	_ = cmd.Start()
}


