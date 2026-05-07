//go:build windows

package timer

import (
	"os/exec"
	"syscall"
)

// isDaemonAlive checks whether a process with the given PID is still running.
func isDaemonAlive(pid int) bool {
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	// WaitForSingleObject with 0 timeout: WAIT_TIMEOUT (0x102) means still running.
	s, _ := syscall.WaitForSingleObject(h, 0)
	return s == 0x102
}

// detachedProcess removes the new process from the parent's console entirely,
// allowing the spawning terminal to be closed without waiting for the daemon.
const detachedProcess = 0x00000008

func spawnDaemon(exe, onixHome string) {
	cmd := exec.Command(exe, "daemon", onixHome)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess,
		HideWindow:    true,
	}
	_ = cmd.Start()
}


