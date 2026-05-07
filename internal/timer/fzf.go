package timer

import (
	"github.com/sadirano/onix-timer/internal/config"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func fzfAvailable() bool {
	_, err := exec.LookPath("fzf")
	return err == nil
}

func parseFzfExpectOutput(out []byte) (key string, lines []string) {
	raw := strings.TrimRight(string(out), "\n")
	parts := strings.SplitN(raw, "\n", 2)
	if len(parts) == 0 {
		return "", nil
	}
	key = strings.TrimSpace(parts[0])
	if len(parts) < 2 {
		return key, nil
	}
	for _, line := range strings.Split(parts[1], "\n") {
		if l := strings.TrimSpace(line); l != "" {
			lines = append(lines, l)
		}
	}
	return
}

// fzfSelectTimer shows active timers in fzf. Returns the selected timer ID and key pressed.
func fzfSelectTimer(entries []TimerEntry, vis *config.Config) (id, key string, err error) {
	var buf bytes.Buffer
	for _, e := range entries {
		rem := formatDuration(e.ComputeRemaining(), e.Kind, false)
		ela := formatElapsed(e.ComputeElapsed(), false)
		repeat := ""
		if e.IsRepeating() {
			repeat = " ↻"
		}
		fmt.Fprintf(&buf, "%s\t%-22s\t%-10s\t%-8s\t%s%s\n",
			e.ID, e.Name, rem, ela, statusLabel(&e), repeat)
	}

	fzfArgs := []string{
		"--delimiter", "\t",
		"--with-nth", "2,3,4,5",
		"--nth", "1,2",
		"--prompt", vis.Timer.FZF.Prompt,
		"--header", "NAME                    REMAINING   ELAPSED   STATUS",
		"--expect=ctrl-s,ctrl-d,ctrl-r",
	}
	if vis.Timer.FZF.Layout != "" && vis.Timer.FZF.Layout != "default" {
		fzfArgs = append(fzfArgs, "--layout", vis.Timer.FZF.Layout)
	}

	cmd := exec.Command("fzf", fzfArgs...)
	cmd.Stdin = &buf
	cmd.Stderr = nil
	out, cmdErr := cmd.Output()
	if cmdErr != nil {
		if _, ok := cmdErr.(*exec.ExitError); ok {
			return "", "", nil // user dismissed with Esc
		}
		return "", "", fmt.Errorf("fzf: %w", cmdErr)
	}

	keyOut, lines := parseFzfExpectOutput(out)
	if len(lines) == 0 {
		return "", keyOut, nil
	}
	parts := strings.SplitN(lines[0], "\t", 2)
	return strings.TrimSpace(parts[0]), keyOut, nil
}

// fzfSelectRecent shows recent timer specs in fzf and returns the selected entry.
func fzfSelectRecent(recents []string, vis *config.Config) (string, error) {
	if !fzfAvailable() {
		return "", nil
	}
	var buf bytes.Buffer
	for _, r := range recents {
		fmt.Fprintln(&buf, r)
	}

	cmd := exec.Command("fzf",
		"--prompt", vis.Timer.FZF.Prompt,
		"--header", "Recent timers — Enter to start",
	)
	cmd.Stdin = &buf
	out, cmdErr := cmd.Output()
	if cmdErr != nil {
		if _, ok := cmdErr.(*exec.ExitError); ok {
			return "", nil
		}
		return "", fmt.Errorf("fzf: %w", cmdErr)
	}
	return strings.TrimSpace(string(out)), nil
}

// runLSWithFZF shows the timer list in fzf and dispatches key-bound actions.
func runLSWithFZF(entries []TimerEntry, onixHome, scope string, vis *config.Config, s *State) error {
	id, key, err := fzfSelectTimer(entries, vis)
	if err != nil {
		return err
	}
	if id == "" {
		return nil
	}

	switch key {
	case "ctrl-s":
		return runStop([]string{id}, onixHome, scope, vis)
	case "ctrl-d":
		return runCancel([]string{id}, onixHome, scope)
	case "ctrl-r":
		return runReset([]string{id}, onixHome, scope, vis)
	default:
		// Enter: show detail view
		e := findByIDOrName(s, id)
		if e != nil {
			printTimerDetail(e, vis)
		}
	}
	return nil
}

func printTimerDetail(e *TimerEntry, vis *config.Config) {
	fmt.Printf("ID:        %s\n", e.ID)
	fmt.Printf("Name:      %s\n", e.Name)
	fmt.Printf("Kind:      %s\n", e.Kind)
	status := statusLabel(e)
	if e.IsRepeating() {
		status += fmt.Sprintf(" ↻ (every %s)", FormatDurationHuman(time.Duration(e.RepeatEveryS)*time.Second))
	}
	fmt.Printf("Status:    %s\n", status)
	if e.Kind != "stopwatch" {
		fmt.Printf("Remaining: %s\n", formatDuration(e.ComputeRemaining(), e.Kind, vis.Timer.RawSeconds))
	}
	fmt.Printf("Elapsed:   %s\n", formatElapsed(e.ComputeElapsed(), false))
	if e.OnDone != "" {
		fmt.Printf("On done:   %s\n", e.OnDone)
	}
	if len(e.Laps) > 0 {
		fmt.Printf("Laps:      %d\n", len(e.Laps))
		for i, l := range e.Laps {
			fmt.Printf("  %d: %s\n", i+1, formatElapsed(l.ElapsedS, false))
		}
	}
}


