package main

import (
	"testing"
	"time"
)

func TestParseTimerSpec(t *testing.T) {
	cases := []struct {
		input   string
		kind    string
		wantDur time.Duration
		wantErr bool
	}{
		// Bare minutes
		{"25", "countdown", 25 * time.Minute, false},
		{"1.5", "countdown", 90 * time.Second, false}, // 1.5 minutes = 90s

		// Shorthand
		{"25m", "countdown", 25 * time.Minute, false},
		{"25min", "countdown", 25 * time.Minute, false},
		{"90s", "countdown", 90 * time.Second, false},
		{"2h", "countdown", 2 * time.Hour, false},
		{"1.5h", "countdown", 90 * time.Minute, false},

		// Compound
		{"2h30m", "countdown", 2*time.Hour + 30*time.Minute, false},
		{"1h 30m 20s", "countdown", 1*time.Hour + 30*time.Minute + 20*time.Second, false},
		{"2 hours 30 minutes", "countdown", 2*time.Hour + 30*time.Minute, false},
		{"30 minutes", "countdown", 30 * time.Minute, false},

		// Colon notation
		{"27:30", "countdown", 27*time.Minute + 30*time.Second, false},
		{"1:30:00", "countdown", 1*time.Hour + 30*time.Minute, false},
		{"0:45", "countdown", 45 * time.Second, false},

		// Repeat
		{"every 5m", "repeat", 5 * time.Minute, false},
		{"every 1h", "repeat", time.Hour, false},
		{"every 2h30m", "repeat", 2*time.Hour + 30*time.Minute, false},

		// From+every
		// Can't assert StartsAt wall time precisely, just check kind + repeat
		{"from 9am every 30m", "repeat", 30 * time.Minute, false},
		{"every 2h from 1pm", "repeat", 2 * time.Hour, false},

		// Bad input
		{"", "", 0, true},
		{"not-a-timer", "", 0, true},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			pt, err := ParseTimerSpec(c.input)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", c.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", c.input, err)
			}
			if pt.Kind != c.kind {
				t.Fatalf("kind: want %q, got %q", c.kind, pt.Kind)
			}
			if c.wantDur > 0 {
				got := pt.Duration
				if pt.Kind == "repeat" {
					got = pt.RepeatEvery
				}
				if got != c.wantDur {
					t.Fatalf("duration: want %v, got %v", c.wantDur, got)
				}
			}
		})
	}
}

func TestParseFlagsName(t *testing.T) {
	pos, pt, err := ParseFlags([]string{"--name", "my timer", "25m"})
	if err != nil {
		t.Fatal(err)
	}
	if pt.Name != "my timer" {
		t.Fatalf("want name 'my timer', got %q", pt.Name)
	}
	if len(pos) != 1 || pos[0] != "25m" {
		t.Fatalf("unexpected positional: %v", pos)
	}
}

func TestParseFlagsEvery(t *testing.T) {
	_, pt, err := ParseFlags([]string{"--every", "1h"})
	if err != nil {
		t.Fatal(err)
	}
	if pt.RepeatEvery != time.Hour {
		t.Fatalf("want 1h repeat, got %v", pt.RepeatEvery)
	}
}

func TestParseFlagsAt(t *testing.T) {
	_, pt, err := ParseFlags([]string{"--at", "23:59"})
	if err != nil {
		t.Fatal(err)
	}
	if pt.StartsAt == nil {
		t.Fatal("StartsAt should be set")
	}
	if pt.StartsAt.Hour() != 23 || pt.StartsAt.Minute() != 59 {
		t.Fatalf("unexpected StartsAt: %v", pt.StartsAt)
	}
}

func TestParseFlagsExec(t *testing.T) {
	_, pt, err := ParseFlags([]string{"--exec", "echo done"})
	if err != nil {
		t.Fatal(err)
	}
	if pt.OnDone != "echo done" {
		t.Fatalf("want 'echo done', got %q", pt.OnDone)
	}
}

func TestParseSimpleDuration(t *testing.T) {
	cases := []struct {
		input string
		want  time.Duration
	}{
		{"30m", 30 * time.Minute},
		{"1h", time.Hour},
		{"90s", 90 * time.Second},
		{"1.5h", 90 * time.Minute},
		{"30", 30 * time.Minute},
	}
	for _, c := range cases {
		got, err := ParseSimpleDuration(c.input)
		if err != nil {
			t.Errorf("ParseSimpleDuration(%q): %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseSimpleDuration(%q): want %v, got %v", c.input, c.want, got)
		}
	}
}

func TestParseWallTime(t *testing.T) {
	cases := []struct {
		input  string
		hour   int
		minute int
	}{
		{"3pm", 15, 0},
		{"9am", 9, 0},
		{"3:30pm", 15, 30},
		{"14:30", 14, 30},
		{"9:00", 9, 0},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got, err := ParseWallTime(c.input)
			if err != nil {
				t.Fatalf("ParseWallTime(%q): %v", c.input, err)
			}
			if got.Hour() != c.hour || got.Minute() != c.minute {
				t.Fatalf("want %02d:%02d, got %02d:%02d", c.hour, c.minute, got.Hour(), got.Minute())
			}
		})
	}
}

func TestAutoName(t *testing.T) {
	cases := []struct {
		pt   ParsedTimer
		n    int
		want string
	}{
		{ParsedTimer{Kind: "stopwatch"}, 1, "Stopwatch #1"},
		{ParsedTimer{Kind: "countdown", Duration: 25 * time.Minute}, 3, "25m #3"},
		{ParsedTimer{Kind: "repeat", RepeatEvery: time.Hour}, 2, "every 1h #2"},
	}
	for _, c := range cases {
		got := AutoName(&c.pt, c.n)
		if got != c.want {
			t.Errorf("AutoName: want %q, got %q", c.want, got)
		}
	}
}

func TestTrailingTextAsName(t *testing.T) {
	cases := []struct {
		input string
		name  string
	}{
		{"25m standup", "standup"},
		{"1h focus", "focus"},
		{"30m pomodoro", "pomodoro"},
		{"1h30m deep work", "deep work"},
		{"30 minutes meeting", "meeting"},
		{"2 hours review", "review"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			pt, err := ParseTimerSpec(c.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pt.Name != c.name {
				t.Fatalf("want name %q, got %q", c.name, pt.Name)
			}
		})
	}
}

func TestRepeatTimerName(t *testing.T) {
	cases := []struct {
		input    string
		name     string
		interval time.Duration
	}{
		{"every 2m standup", "standup", 2 * time.Minute},
		{"every 1h focus", "focus", time.Hour},
		{"every 30m deep work", "deep work", 30 * time.Minute},
		{"every 30 minutes meeting", "meeting", 30 * time.Minute},
		{"every 2m", "", 2 * time.Minute},                       // no name → empty
		{"from 9am every 30m standup", "standup", 30 * time.Minute},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			pt, err := ParseTimerSpec(c.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pt.Kind != "repeat" {
				t.Fatalf("want kind=repeat, got %q", pt.Kind)
			}
			if pt.RepeatEvery != c.interval {
				t.Fatalf("want interval %v, got %v", c.interval, pt.RepeatEvery)
			}
			if pt.Name != c.name {
				t.Fatalf("want name %q, got %q", c.name, pt.Name)
			}
		})
	}
}

func TestParseFlagsNotifyOverride(t *testing.T) {
	_, pt, err := ParseFlags([]string{"--no-notify", "25m"})
	if err != nil {
		t.Fatal(err)
	}
	if pt.NotifyOverride == nil || *pt.NotifyOverride != false {
		t.Fatal("expected NotifyOverride=false for --no-notify")
	}

	_, pt2, err := ParseFlags([]string{"--notify", "25m"})
	if err != nil {
		t.Fatal(err)
	}
	if pt2.NotifyOverride == nil || *pt2.NotifyOverride != true {
		t.Fatal("expected NotifyOverride=true for --notify")
	}

	_, pt3, _ := ParseFlags([]string{"25m"})
	if pt3.NotifyOverride != nil {
		t.Fatal("expected NotifyOverride=nil when flag not given")
	}
}

func TestFormatDurationHuman(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{25 * time.Minute, "25m"},
		{time.Hour, "1h"},
		{90 * time.Minute, "1h30m"},
		{30 * time.Second, "30s"},
		{1*time.Hour + 30*time.Minute + 20*time.Second, "1h30m20s"},
	}
	for _, c := range cases {
		got := FormatDurationHuman(c.d)
		if got != c.want {
			t.Errorf("FormatDurationHuman(%v): want %q, got %q", c.d, c.want, got)
		}
	}
}
