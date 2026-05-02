package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParsedTimer holds the result of parsing a natural language timer spec.
type ParsedTimer struct {
	Kind           string        // "countdown" | "stopwatch" | "repeat"
	Duration       time.Duration // primary duration (0 for stopwatch)
	RepeatEvery    time.Duration // non-zero for --every / "every Xm" specs
	StartsAt       *time.Time    // non-nil for --at / "at Xpm" specs
	OnDone         string        // --exec command
	Name           string        // user-supplied name
	RawInput       string        // preserved for recents
	NotifyOverride *bool         // nil = use config default; true/false = per-timer override
}

var (
	reColon     = regexp.MustCompile(`^(\d{1,2}):(\d{2})(?::(\d{2}))?$`)
	reBare      = regexp.MustCompile(`^(\d+(?:\.\d+)?)$`)
	reEvery     = regexp.MustCompile(`(?i)^every\s+(.+?)(?:\s+from\s+(.+))?$`)
	reFromEvery = regexp.MustCompile(`(?i)^from\s+(.+?)\s+every\s+(.+)$`)
	reAt        = regexp.MustCompile(`(?i)^at\s+(.+)$`)
	// Single-letter units (h/m/s) must be adjacent to the digits — no spaces allowed.
	// This prevents "5 standup" from matching "5 s" (treating 's' as seconds).
	// Full-word units (hours, minutes, etc.) still allow a preceding space.
	reCompound = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)(?:\s+(hours?|hrs?|minutes?|mins?|seconds?|secs?)|(h|m|s))`)
)

// ParseFlags extracts named flags from args and returns remaining positional args
// plus a partially-filled ParsedTimer.
func ParseFlags(args []string) (positional []string, pt ParsedTimer, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			i++
			if i >= len(args) {
				return nil, pt, fmt.Errorf("--name requires a value")
			}
			pt.Name = args[i]
		case "--stopwatch", "-sw":
			pt.Kind = "stopwatch"
		case "--every", "--repeat", "-e":
			i++
			if i >= len(args) {
				return nil, pt, fmt.Errorf("--every requires a duration")
			}
			d, e := ParseSimpleDuration(args[i])
			if e != nil {
				return nil, pt, fmt.Errorf("--every: %w", e)
			}
			pt.RepeatEvery = d
		case "--at":
			i++
			if i >= len(args) {
				return nil, pt, fmt.Errorf("--at requires a time")
			}
			t, e := ParseWallTime(args[i])
			if e != nil {
				return nil, pt, fmt.Errorf("--at: %w", e)
			}
			pt.StartsAt = &t
		case "--exec":
			i++
			if i >= len(args) {
				return nil, pt, fmt.Errorf("--exec requires a command")
			}
			pt.OnDone = args[i]
		case "--notify":
			b := true
			pt.NotifyOverride = &b
		case "--no-notify":
			b := false
			pt.NotifyOverride = &b
		default:
			positional = append(positional, args[i])
		}
	}
	return
}

// ParseTimerSpec parses positional text (after flags are stripped) into a ParsedTimer.
func ParseTimerSpec(spec string) (ParsedTimer, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ParsedTimer{}, fmt.Errorf("empty spec")
	}

	// "every 5m" / "every 2h from 1pm" / "every 30m standup"
	if m := reEvery.FindStringSubmatch(spec); m != nil {
		d, name, err := parseIntervalWithName(m[1])
		if err != nil {
			return ParsedTimer{}, fmt.Errorf("every interval: %w", err)
		}
		pt := ParsedTimer{Kind: "repeat", RepeatEvery: d, Duration: d, Name: name}
		if m[2] != "" {
			t, err := ParseWallTime(m[2])
			if err != nil {
				return ParsedTimer{}, fmt.Errorf("every from: %w", err)
			}
			pt.StartsAt = &t
		}
		return pt, nil
	}

	// "from 9am every 30m" / "from 9am every 30m standup"
	if m := reFromEvery.FindStringSubmatch(spec); m != nil {
		t, err := ParseWallTime(m[1])
		if err != nil {
			return ParsedTimer{}, fmt.Errorf("from time: %w", err)
		}
		d, name, err := parseIntervalWithName(m[2])
		if err != nil {
			return ParsedTimer{}, fmt.Errorf("every interval: %w", err)
		}
		return ParsedTimer{Kind: "repeat", RepeatEvery: d, Duration: d, StartsAt: &t, Name: name}, nil
	}

	// "at 3pm" or "at 14:30"
	if m := reAt.FindStringSubmatch(spec); m != nil {
		t, err := ParseWallTime(m[1])
		if err != nil {
			return ParsedTimer{}, fmt.Errorf("at time: %w", err)
		}
		dur := time.Until(t)
		if dur < 0 {
			dur = 0
		}
		return ParsedTimer{Kind: "countdown", Duration: dur, StartsAt: &t}, nil
	}

	// "27:30" (MM:SS) or "1:30:00" (HH:MM:SS)
	if m := reColon.FindStringSubmatch(spec); m != nil {
		var h, min, sec int
		if m[3] == "" {
			min, _ = strconv.Atoi(m[1])
			sec, _ = strconv.Atoi(m[2])
		} else {
			h, _ = strconv.Atoi(m[1])
			min, _ = strconv.Atoi(m[2])
			sec, _ = strconv.Atoi(m[3])
		}
		d := time.Duration(h)*time.Hour + time.Duration(min)*time.Minute + time.Duration(sec)*time.Second
		return ParsedTimer{Kind: "countdown", Duration: d}, nil
	}

	// Compound / word durations: "2h30m", "1.5h", "30 minutes", "2 hours 30 minutes"
	// Also extracts any non-duration text as a name hint (e.g. "25m standup" → name="standup").
	if d, rest, ok := parseCompoundDurationWithRemainder(spec); ok {
		return ParsedTimer{Kind: "countdown", Duration: d, Name: rest}, nil
	}

	// Bare number → minutes
	if m := reBare.FindStringSubmatch(spec); m != nil {
		f, _ := strconv.ParseFloat(m[1], 64)
		d := time.Duration(f * float64(time.Minute))
		return ParsedTimer{Kind: "countdown", Duration: d}, nil
	}

	return ParsedTimer{}, fmt.Errorf("unrecognized timer spec %q", spec)
}

// parseIntervalWithName extracts a duration and an optional trailing name from a repeat
// interval string. Examples: "2m" → (2m,""), "30m standup" → (30m,"standup"),
// "5 standup" → (5m,"standup") (bare number treated as minutes).
func parseIntervalWithName(s string) (time.Duration, string, error) {
	s = strings.TrimSpace(s)
	if d, name, ok := parseCompoundDurationWithRemainder(s); ok {
		return d, name, nil
	}
	// Bare number optionally followed by a name: "5" → 5m, "5 standup" → 5m + "standup"
	fields := strings.Fields(s)
	if len(fields) > 0 {
		if m := reBare.FindStringSubmatch(fields[0]); m != nil {
			f, _ := strconv.ParseFloat(m[1], 64)
			name := strings.Join(fields[1:], " ")
			return time.Duration(f * float64(time.Minute)), name, nil
		}
	}
	return 0, "", fmt.Errorf("cannot parse duration %q", s)
}

// ParseSimpleDuration parses a short duration string like "25m", "1h", "90s", "1.5h".
func ParseSimpleDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if d, ok := parseCompoundDuration(s); ok {
		return d, nil
	}
	if m := reBare.FindStringSubmatch(s); m != nil {
		f, _ := strconv.ParseFloat(m[1], 64)
		return time.Duration(f * float64(time.Minute)), nil
	}
	return 0, fmt.Errorf("cannot parse duration %q", s)
}

func parseCompoundDuration(s string) (time.Duration, bool) {
	d, _, ok := parseCompoundDurationWithRemainder(s)
	return d, ok
}

// parseCompoundDurationWithRemainder extracts a duration AND returns the non-duration text
// (trimmed), which callers can use as a timer name hint.
func parseCompoundDurationWithRemainder(s string) (d time.Duration, remainder string, ok bool) {
	lower := strings.ToLower(s)
	matches := reCompound.FindAllStringSubmatch(lower, -1)
	if len(matches) == 0 {
		return 0, "", false
	}
	var total time.Duration
	for _, m := range matches {
		val, _ := strconv.ParseFloat(m[1], 64)
		// m[2] = full-word unit (hours/minutes/seconds); m[3] = single-char unit (h/m/s)
		unit := m[2]
		if unit == "" {
			unit = m[3]
		}
		switch {
		case strings.HasPrefix(unit, "h"):
			total += time.Duration(val * float64(time.Hour))
		case strings.HasPrefix(unit, "m"):
			total += time.Duration(val * float64(time.Minute))
		case strings.HasPrefix(unit, "s"):
			total += time.Duration(val * float64(time.Second))
		}
	}
	if total == 0 {
		return 0, "", false
	}
	// Remove all duration tokens to get the remaining text (potential name)
	rest := reCompound.ReplaceAllString(s, " ")
	rest = strings.Join(strings.Fields(rest), " ")
	return total, rest, true
}

// ParseWallTime parses a wall-clock time like "3pm", "14:30", "9:30am".
// If the result is in the past, adds 24h so it always refers to a future time.
func ParseWallTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	now := time.Now()

	// Normalise to lowercase for matching, but Go's time.Parse is case-insensitive for AM/PM.
	formats := []string{"3:04pm", "3pm", "15:04:05", "15:04"}
	for _, f := range formats {
		if t, err := time.Parse(f, strings.ToLower(s)); err == nil {
			result := time.Date(now.Year(), now.Month(), now.Day(),
				t.Hour(), t.Minute(), t.Second(), 0, now.Location())
			if result.Before(now) {
				result = result.Add(24 * time.Hour)
			}
			return result, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q (use HH:MM or 3pm)", s)
}

// AutoName generates a name when none is provided by the user.
func AutoName(pt *ParsedTimer, n int) string {
	switch pt.Kind {
	case "stopwatch":
		return fmt.Sprintf("Stopwatch #%d", n)
	case "repeat":
		return fmt.Sprintf("every %s #%d", FormatDurationHuman(pt.RepeatEvery), n)
	default:
		return fmt.Sprintf("%s #%d", FormatDurationHuman(pt.Duration), n)
	}
}

// FormatDurationHuman formats a duration as a compact human string like "25m", "1h30m".
func FormatDurationHuman(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h > 0 && m > 0 && s > 0:
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case h > 0 && s > 0:
		return fmt.Sprintf("%dh%ds", h, s)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	case m > 0 && s > 0:
		return fmt.Sprintf("%dm%ds", m, s)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
