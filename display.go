package main

import (
	"fmt"
	"strings"
	"time"
)

// formatDuration formats remaining seconds for display.
// kind="stopwatch" → "—" (no remaining concept).
// raw=true         → raw integer seconds.
// secs=0           → "done".
func formatDuration(secs int64, kind string, raw bool) string {
	if kind == "stopwatch" {
		return "—"
	}
	if secs == 0 {
		return "done"
	}
	if raw {
		return fmt.Sprintf("%d", secs)
	}
	return fmtHMS(secs)
}

func formatElapsed(secs int64, raw bool) string {
	if raw {
		return fmt.Sprintf("%d", secs)
	}
	return fmtHMS(secs)
}

func fmtHMS(secs int64) string {
	if secs < 0 {
		secs = 0
	}
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func statusLabel(e *TimerEntry) string {
	switch {
	case e.Done:
		return "done"
	case e.PausedAt != nil:
		return "paused"
	case e.ComputeRemaining() == 0 && e.IsRepeating():
		return "firing…"
	default:
		return "running"
	}
}

type tableRow struct {
	id, name, kind, remaining, elapsed, status string
}

func formatTable(entries []TimerEntry, cfg *TimerConfig) string {
	if len(entries) == 0 {
		return ""
	}
	rows := make([]tableRow, len(entries))
	for i, e := range entries {
		status := statusLabel(&e)
		if e.IsRepeating() {
			status += " ↻"
		}
		rows[i] = tableRow{
			id:        e.ID,
			name:      e.Name,
			kind:      e.Kind,
			remaining: formatDuration(e.ComputeRemaining(), e.Kind, cfg.RawSeconds),
			elapsed:   formatElapsed(e.ComputeElapsed(), false),
			status:    status,
		}
	}

	headers := [6]string{"ID", "NAME", "KIND", "REMAINING", "ELAPSED", "STATUS"}
	widths := [6]int{2, 4, 4, 9, 7, 6}
	for _, r := range rows {
		cols := [6]string{r.id, r.name, r.kind, r.remaining, r.elapsed, r.status}
		for j, c := range cols {
			if len(c) > widths[j] {
				widths[j] = len(c)
			}
		}
	}

	var b strings.Builder
	for j, h := range headers {
		if j > 0 {
			b.WriteString("  ")
		}
		b.WriteString(padRight(h, widths[j]))
	}
	b.WriteByte('\n')
	for j, w := range widths {
		if j > 0 {
			b.WriteString("  ")
		}
		b.WriteString(strings.Repeat("─", w))
	}
	b.WriteByte('\n')
	for _, r := range rows {
		cols := [6]string{r.id, r.name, r.kind, r.remaining, r.elapsed, r.status}
		for j, c := range cols {
			if j > 0 {
				b.WriteString("  ")
			}
			b.WriteString(padRight(c, widths[j]))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func runWatch(onixHome, scope string, vis *Config) error {
	firstPrint := true
	var lastLines int
	for {
		s, err := loadState(onixHome, scope)
		if err != nil {
			return err
		}
		active := filterActive(s.Timers)
		table := formatTable(active, &vis.Timer)
		if table == "" {
			table = "No active timers.\n"
		}

		if !firstPrint && lastLines > 0 {
			fmt.Printf("\033[%dA", lastLines)
		}
		firstPrint = false

		fmt.Print(table)
		lastLines = strings.Count(table, "\n")

		time.Sleep(time.Second)
	}
}
