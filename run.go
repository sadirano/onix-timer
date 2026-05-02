package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func runStart(args []string, onixHome, scope string, vis *Config) error {
	positional, pt, err := ParseFlags(args)
	if err != nil {
		return err
	}

	rawSpec := strings.Join(positional, " ")

	if pt.Kind == "stopwatch" {
		if rawSpec != "" && pt.Name == "" {
			pt.Name = strings.TrimSpace(rawSpec)
		}
		pt.RawInput = "stopwatch"
	} else if pt.RepeatEvery > 0 {
		if rawSpec != "" && pt.Name == "" {
			pt.Name = strings.TrimSpace(rawSpec)
		}
		pt.Kind = "repeat"
		pt.Duration = pt.RepeatEvery
		pt.RawInput = fmt.Sprintf("every %s", FormatDurationHuman(pt.RepeatEvery))
	} else if rawSpec == "" && pt.StartsAt == nil {
		s, _ := loadState(onixHome, scope)
		if len(s.Recents) > 0 {
			picked, pickErr := fzfSelectRecent(s.Recents, vis)
			if pickErr != nil || picked == "" {
				return pickErr
			}
			rawSpec = picked
		} else {
			return fmt.Errorf("no spec given — try: timer start 25m")
		}
	}

	if pt.Kind != "stopwatch" && pt.Kind != "repeat" && rawSpec != "" {
		specPt, parseErr := ParseTimerSpec(rawSpec)
		if parseErr != nil {
			return parseErr
		}
		if pt.Kind == "" {
			pt.Kind = specPt.Kind
		}
		if pt.Duration == 0 {
			pt.Duration = specPt.Duration
		}
		if pt.RepeatEvery == 0 {
			pt.RepeatEvery = specPt.RepeatEvery
		}
		if pt.StartsAt == nil {
			pt.StartsAt = specPt.StartsAt
		}
		if pt.Name == "" && specPt.Name != "" {
			pt.Name = specPt.Name
		}
		pt.RawInput = rawSpec
		if pt.RepeatEvery > 0 {
			pt.Kind = "repeat"
			pt.Duration = pt.RepeatEvery
		}
	}

	if pt.Kind == "" {
		pt.Kind = "countdown"
	}
	if pt.RepeatEvery > 0 && pt.Kind != "stopwatch" {
		pt.Kind = "repeat"
	}

	s, err := loadState(onixHome, scope)
	if err != nil {
		return err
	}

	id := fmt.Sprintf("t%d", s.NextID)
	s.NextID++

	if pt.Name == "" {
		pt.Name = AutoName(&pt, s.NextID-1)
	}

	notifyDisabled := !vis.Timer.Notify
	if pt.NotifyOverride != nil {
		notifyDisabled = !*pt.NotifyOverride
	}

	now := time.Now()
	entry := TimerEntry{
		ID:             id,
		Name:           pt.Name,
		Kind:           pt.Kind,
		DurationS:      int64(pt.Duration.Seconds()),
		StartedAt:      &now,
		RemainingS:     int64(pt.Duration.Seconds()),
		RepeatEveryS:   int64(pt.RepeatEvery.Seconds()),
		OnDone:         pt.OnDone,
		RawInput:       pt.RawInput,
		NotifyDisabled: notifyDisabled,
		RepeatTimes:    pt.RepeatTimes,
		RepeatUntil:    pt.RepeatUntil,
	}

	// Always set NextFireAt so the daemon knows when to fire.
	switch {
	case pt.Kind == "stopwatch":
		// no NextFireAt for stopwatches
	case pt.StartsAt != nil:
		entry.NextFireAt = pt.StartsAt
	case pt.Kind == "repeat":
		fireAt := now.Add(pt.RepeatEvery)
		entry.NextFireAt = &fireAt
	default: // countdown
		fireAt := now.Add(pt.Duration)
		entry.NextFireAt = &fireAt
	}

	s.Timers = append(s.Timers, entry)
	if pt.RawInput != "" {
		addRecent(s, pt.RawInput)
	}

	if err := saveState(onixHome, scope, s); err != nil {
		return err
	}

	ensureDaemon(onixHome)

	var displaySecs int64
	if entry.NextFireAt != nil {
		if d := int64(time.Until(*entry.NextFireAt).Seconds()); d > 0 {
			displaySecs = d
		}
	} else {
		displaySecs = entry.DurationS
	}

	switch pt.Kind {
	case "stopwatch":
		fmt.Printf("Started stopwatch: %s  [id=%s]\n", entry.Name, id)
	case "repeat":
		bounds := ""
		if entry.RepeatTimes > 0 {
			bounds += fmt.Sprintf(", %d×", entry.RepeatTimes)
		}
		if entry.RepeatUntil != nil {
			bounds += fmt.Sprintf(", until %s", entry.RepeatUntil.Format("15:04"))
		}
		fmt.Printf("Started: %s  [every %s%s]  [id=%s]\n", entry.Name, FormatDurationHuman(pt.RepeatEvery), bounds, id)
	default:
		fmt.Printf("Started: %s  [%s]  [id=%s]\n", entry.Name, formatDuration(displaySecs, entry.Kind, vis.Timer.RawSeconds), id)
	}
	return nil
}

func runStop(args []string, onixHome, scope string, vis *Config) error {
	ref, err := resolveRef(args, "stop")
	if err != nil {
		return err
	}
	s, err := loadState(onixHome, scope)
	if err != nil {
		return err
	}
	e := findByIDOrName(s, ref)
	if e == nil {
		return fmt.Errorf("timer %q not found", ref)
	}
	if e.Done {
		return fmt.Errorf("timer %q is already done", ref)
	}

	if e.PausedAt != nil {
		// Resume: restore NextFireAt so the daemon picks it back up.
		now := time.Now()
		e.StartedAt = &now
		e.PausedAt = nil
		if e.IsRepeating() {
			fireAt := now.Add(time.Duration(e.RepeatEveryS) * time.Second)
			e.NextFireAt = &fireAt
		} else {
			fireAt := now.Add(time.Duration(e.RemainingS) * time.Second)
			e.NextFireAt = &fireAt
		}
		if err := saveState(onixHome, scope, s); err != nil {
			return err
		}
		ensureDaemon(onixHome)
		fmt.Printf("Resumed: %s  (%s remaining)\n", e.Name, formatDuration(e.RemainingS, e.Kind, vis.Timer.RawSeconds))
	} else {
		// Pause: snapshot remaining and clear NextFireAt so the daemon ignores it.
		now := time.Now()
		e.RemainingS = e.ComputeRemaining()
		e.PausedAt = &now
		e.NextFireAt = nil
		fmt.Printf("Paused: %s  (%s remaining)\n", e.Name, formatDuration(e.RemainingS, e.Kind, vis.Timer.RawSeconds))
		if err := saveState(onixHome, scope, s); err != nil {
			return err
		}
	}
	return nil
}

func runCancel(args []string, onixHome, scope string) error {
	ref, err := resolveRef(args, "cancel")
	if err != nil {
		return err
	}
	s, err := loadState(onixHome, scope)
	if err != nil {
		return err
	}
	e := findByIDOrName(s, ref)
	if e == nil {
		return fmt.Errorf("timer %q not found", ref)
	}
	name, id := e.Name, e.ID
	removeByID(s, id)
	fmt.Printf("Cancelled: %s  [id=%s]\n", name, id)
	return saveState(onixHome, scope, s)
}

func runReset(args []string, onixHome, scope string, vis *Config) error {
	ref, err := resolveRef(args, "reset")
	if err != nil {
		return err
	}
	s, err := loadState(onixHome, scope)
	if err != nil {
		return err
	}
	e := findByIDOrName(s, ref)
	if e == nil {
		return fmt.Errorf("timer %q not found", ref)
	}

	now := time.Now()
	e.StartedAt = &now
	e.PausedAt = nil
	e.RemainingS = e.DurationS
	e.Done = false

	if e.IsRepeating() {
		fireAt := now.Add(time.Duration(e.RepeatEveryS) * time.Second)
		e.NextFireAt = &fireAt
	} else {
		fireAt := now.Add(time.Duration(e.DurationS) * time.Second)
		e.NextFireAt = &fireAt
	}

	if err := saveState(onixHome, scope, s); err != nil {
		return err
	}
	ensureDaemon(onixHome)

	fmt.Printf("Reset: %s  [%s]\n", e.Name, FormatDurationHuman(time.Duration(e.DurationS)*time.Second))
	return nil
}

func runLap(args []string, onixHome, scope string) error {
	ref, err := resolveRef(args, "lap")
	if err != nil {
		return err
	}
	s, err := loadState(onixHome, scope)
	if err != nil {
		return err
	}
	e := findByIDOrName(s, ref)
	if e == nil {
		return fmt.Errorf("timer %q not found", ref)
	}
	if e.Kind != "stopwatch" {
		return fmt.Errorf("lap is only for stopwatches")
	}
	elapsed := e.ComputeElapsed()
	e.Laps = append(e.Laps, LapEntry{At: time.Now(), ElapsedS: elapsed})
	fmt.Printf("Lap %d: %s  elapsed=%s\n", len(e.Laps), e.Name, formatElapsed(elapsed, false))
	return saveState(onixHome, scope, s)
}

func runStatus(onixHome, scope string, vis *Config) error {
	s, err := loadState(onixHome, scope)
	if err != nil {
		return err
	}
	active := filterActive(s.Timers)
	if len(active) == 0 {
		fmt.Println("No active timers.")
		return nil
	}
	ensureDaemon(onixHome)
	for _, e := range active {
		repeat := ""
		if e.IsRepeating() {
			repeat = " ↻"
		}
		rem := formatDuration(e.ComputeRemaining(), e.Kind, vis.Timer.RawSeconds)
		fmt.Printf("[%s] %s — %s%s\n", e.ID, e.Name, rem, repeat)
	}
	return nil
}

func runLS(onixHome, scope string, vis *Config, raw, watch bool) error {
	s, err := loadState(onixHome, scope)
	if err != nil {
		return err
	}
	active := filterActive(s.Timers)
	if len(active) == 0 {
		fmt.Println("No active timers. Start one with: timer start 25m")
		return nil
	}
	ensureDaemon(onixHome)

	if raw {
		for _, e := range active {
			fmt.Printf("%s\t%s\t%s\t%d\t%s\n",
				e.ID, e.Name, e.Kind, e.ComputeRemaining(), statusLabel(&e))
		}
		return nil
	}

	if watch {
		return runWatch(onixHome, scope, vis)
	}

	if fzfAvailable() {
		return runLSWithFZF(active, onixHome, scope, vis, s)
	}

	fmt.Print(formatTable(active, &vis.Timer))
	return nil
}

func runClean(onixHome, scope string) error {
	s, err := loadState(onixHome, scope)
	if err != nil {
		return err
	}
	before := len(s.Timers)
	active := s.Timers[:0]
	for _, e := range s.Timers {
		if !e.Done {
			active = append(active, e)
		}
	}
	s.Timers = active
	removed := before - len(s.Timers)
	if err := saveState(onixHome, scope, s); err != nil {
		return err
	}
	fmt.Printf("Removed %d done timer(s).\n", removed)
	return nil
}

func runScopes(onixHome string) error {
	files, _ := filepath.Glob(filepath.Join(timerDir(onixHome), "*.json"))
	if len(files) == 0 {
		fmt.Println("No scopes found.")
		return nil
	}
	const col = 20
	fmt.Printf("%-*s  %s\n", col, "SCOPE", "ACTIVE")
	fmt.Printf("%-*s  %s\n", col, strings.Repeat("─", col), "──────")
	for _, f := range files {
		scope := strings.TrimSuffix(filepath.Base(f), ".json")
		s, err := loadState(onixHome, scope)
		if err != nil {
			continue
		}
		count := 0
		for _, e := range s.Timers {
			if !e.Done {
				count++
			}
		}
		fmt.Printf("%-*s  %d\n", col, scope, count)
	}
	return nil
}

func resolveRef(args []string, cmd string) (string, error) {
	positional := make([]string, 0, len(args))
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
		}
	}
	if len(positional) == 0 {
		return "", fmt.Errorf("%s requires a timer id or name", cmd)
	}
	return strings.Join(positional, " "), nil
}
