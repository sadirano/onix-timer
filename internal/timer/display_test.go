package timer

import (
	"github.com/sadirano/onix-timer/internal/config"
	"strings"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		secs int64
		kind string
		raw  bool
		want string
	}{
		{0, "countdown", false, "done"},
		{0, "stopwatch", false, "—"},
		{90, "countdown", false, "1:30"},
		{3661, "countdown", false, "1:01:01"},
		{90, "countdown", true, "90"},
		{1500, "countdown", false, "25:00"},
	}
	for _, c := range cases {
		got := formatDuration(c.secs, c.kind, c.raw)
		if got != c.want {
			t.Errorf("formatDuration(%d, %q, %v): want %q, got %q", c.secs, c.kind, c.raw, c.want, got)
		}
	}
}

func TestFormatElapsed(t *testing.T) {
	if got := formatElapsed(65, false); got != "1:05" {
		t.Errorf("want 1:05, got %q", got)
	}
	if got := formatElapsed(65, true); got != "65" {
		t.Errorf("want 65, got %q", got)
	}
}

func TestFmtHMS(t *testing.T) {
	cases := []struct {
		secs int64
		want string
	}{
		{0, "0:00"},
		{59, "0:59"},
		{60, "1:00"},
		{3599, "59:59"},
		{3600, "1:00:00"},
		{3661, "1:01:01"},
	}
	for _, c := range cases {
		got := fmtHMS(c.secs)
		if got != c.want {
			t.Errorf("fmtHMS(%d): want %q, got %q", c.secs, c.want, got)
		}
	}
}

func TestStatusLabel(t *testing.T) {
	now := time.Now()
	e := TimerEntry{Kind: "countdown"}
	if statusLabel(&e) != "running" {
		t.Error("expected running")
	}
	e.PausedAt = &now
	if statusLabel(&e) != "paused" {
		t.Error("expected paused")
	}
	e.PausedAt = nil
	e.Done = true
	if statusLabel(&e) != "done" {
		t.Error("expected done")
	}
}

func TestFormatTable(t *testing.T) {
	now := time.Now().Add(-30 * time.Second)
	entries := []TimerEntry{
		{
			ID:         "t1",
			Name:       "standup",
			Kind:       "countdown",
			DurationS:  300,
			StartedAt:  &now,
			RemainingS: 300,
		},
	}
	cfg := &Timerconfig.Config{RawSeconds: false}
	table := formatTable(entries, cfg)

	if !strings.Contains(table, "t1") {
		t.Error("table should contain timer ID")
	}
	if !strings.Contains(table, "standup") {
		t.Error("table should contain timer name")
	}
	if !strings.Contains(table, "countdown") {
		t.Error("table should contain kind")
	}
	if !strings.Contains(table, "REMAINING") {
		t.Error("table should contain header")
	}
	if !strings.Contains(table, "─") {
		t.Error("table should contain separator")
	}
}

func TestFormatTableEmpty(t *testing.T) {
	if got := formatTable(nil, &Timerconfig.Config{}); got != "" {
		t.Errorf("empty table should be empty string, got %q", got)
	}
}

func TestPadRight(t *testing.T) {
	if got := padRight("hi", 5); got != "hi   " {
		t.Errorf("want 'hi   ', got %q", got)
	}
	if got := padRight("hello world", 5); got != "hello world" {
		t.Error("longer string should not be truncated")
	}
}


