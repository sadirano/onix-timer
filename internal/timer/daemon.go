package timer

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const daemonIdleTimeout = 60 * time.Second

func daemonPIDPath(onixHome string) string {
	return filepath.Join(timerDir(onixHome), "daemon.pid")
}

// ensureDaemon checks whether the background daemon is alive and spawns one if not.
func ensureDaemon(onixHome string) {
	pidPath := daemonPIDPath(onixHome)
	if data, err := os.ReadFile(pidPath); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			if isDaemonAlive(pid) {
				return
			}
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	spawnDaemon(exe, onixHome)
}

// runDaemon is the entry point for the background daemon process.
// It polls all scope state files every second and fires notifications when timers expire.
func RunDaemon(onixHome string) {
	pidPath := daemonPIDPath(onixHome)

	// Exit immediately if another daemon is already alive.
	if data, err := os.ReadFile(pidPath); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			if isDaemonAlive(pid) {
				return
			}
		}
	}

	_ = os.MkdirAll(timerDir(onixHome), 0o755)
	_ = os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644)
	defer os.Remove(pidPath)

	idleSince := time.Now()
	for {
		if tickAllScopes(onixHome) {
			idleSince = time.Now()
		} else if time.Since(idleSince) > daemonIdleTimeout {
			return
		}
		time.Sleep(time.Second)
	}
}

// tickAllScopes processes every *.json state file in the timer directory.
// Returns true if any scope had active (non-done, non-paused) timers.
func tickAllScopes(onixHome string) bool {
	files, err := filepath.Glob(filepath.Join(timerDir(onixHome), "*.json"))
	if err != nil || len(files) == 0 {
		return false
	}
	hadActive := false
	for _, f := range files {
		scope := strings.TrimSuffix(filepath.Base(f), ".json")
		if processScope(onixHome, scope) {
			hadActive = true
		}
	}
	return hadActive
}

// processScope fires any due timers in a single scope and saves updated state.
// Returns true if there are still active timers in this scope.
func processScope(onixHome, scope string) bool {
	s, err := loadState(onixHome, scope)
	if err != nil {
		return false
	}

	now := time.Now()
	modified := false
	hasActive := false

	for i := range s.Timers {
		e := &s.Timers[i]
		if e.Done || e.Kind == "stopwatch" || e.PausedAt != nil {
			continue
		}
		// Migrate timers that predate the daemon (no NextFireAt stored).
		// Compute remaining from StartedAt and set NextFireAt so future ticks are fast.
		if e.NextFireAt == nil {
			rem := e.ComputeRemaining()
			if rem > 0 {
				fireAt := now.Add(time.Duration(rem) * time.Second)
				e.NextFireAt = &fireAt
				modified = true
				hasActive = true
				continue
			}
			// rem == 0: timer should have already fired — fall through to fire it now.
		}
		if e.NextFireAt != nil && now.Before(*e.NextFireAt) {
			hasActive = true
			continue
		}

		// Timer has fired.
		if !e.NotifyDisabled {
			showToast(e.Name + " — done!")
		}
		runOnDone(e.OnDone)

		if e.IsRepeating() {
			e.RepeatFired++
			next := now.Add(time.Duration(e.RepeatEveryS) * time.Second)
			bounded := (e.RepeatTimes > 0 && e.RepeatFired >= e.RepeatTimes) ||
				(e.RepeatUntil != nil && next.After(*e.RepeatUntil))
			if bounded {
				e.Done = true
				e.NextFireAt = nil
			} else {
				e.NextFireAt = &next
				hasActive = true
			}
		} else {
			e.Done = true
			e.NextFireAt = nil
		}
		modified = true
	}

	if modified {
		_ = saveState(onixHome, scope, s)
	}
	return hasActive
}


