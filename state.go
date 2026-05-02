package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type State struct {
	Version int          `json:"version"`
	Timers  []TimerEntry `json:"timers"`
	Recents []string     `json:"recents"`
	NextID  int          `json:"next_id"`
}

type TimerEntry struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Kind           string     `json:"kind"` // "countdown" | "stopwatch" | "repeat"
	DurationS      int64      `json:"duration_s"`
	StartedAt      *time.Time `json:"started_at"`
	PausedAt       *time.Time `json:"paused_at"`
	RemainingS     int64      `json:"remaining_s"`
	RepeatEveryS   int64      `json:"repeat_every_s"`   // 0 = one-shot
	NextFireAt     *time.Time `json:"next_fire_at"`     // daemon fires when now >= NextFireAt
	OnDone         string     `json:"on_done"`
	Laps           []LapEntry `json:"laps"`
	RawInput       string     `json:"raw_input"`
	NotifyDisabled bool       `json:"notify_disabled,omitempty"`
	Done           bool       `json:"done"`
}

type LapEntry struct {
	At       time.Time `json:"at"`
	ElapsedS int64     `json:"elapsed_s"`
}

func (e *TimerEntry) IsRepeating() bool { return e.RepeatEveryS > 0 }

func (e *TimerEntry) ComputeRemaining() int64 {
	if e.Done || e.Kind == "stopwatch" {
		return 0
	}
	if e.PausedAt != nil {
		return e.RemainingS
	}
	if e.NextFireAt != nil {
		r := int64(time.Until(*e.NextFireAt).Seconds())
		if r < 0 {
			return 0
		}
		return r
	}
	if e.StartedAt == nil {
		return e.RemainingS
	}
	elapsed := int64(time.Since(*e.StartedAt).Seconds())
	r := e.RemainingS - elapsed
	if r < 0 {
		return 0
	}
	return r
}

func (e *TimerEntry) ComputeElapsed() int64 {
	if e.StartedAt == nil {
		return 0
	}
	if e.PausedAt != nil {
		return int64(e.PausedAt.Sub(*e.StartedAt).Seconds())
	}
	return int64(time.Since(*e.StartedAt).Seconds())
}

// timerDir returns ~/.onix/timer/ — the folder for all timer state and config files.
func timerDir(onixHome string) string {
	if onixHome == "" {
		home, _ := os.UserHomeDir()
		onixHome = filepath.Join(home, ".onix")
	}
	return filepath.Join(onixHome, "timer")
}

// stateFilePath returns the path for a given scope's state file.
// scope "global" → global.json; any other value → <scope>.json.
func stateFilePath(onixHome, scope string) string {
	if scope == "" {
		scope = "global"
	}
	return filepath.Join(timerDir(onixHome), scope+".json")
}

func loadState(onixHome, scope string) (*State, error) {
	path := stateFilePath(onixHome, scope)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{Version: 1, NextID: 1}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return &State{Version: 1, NextID: 1}, nil // corrupt → start fresh
	}
	if s.NextID < 1 {
		s.NextID = 1
	}
	return &s, nil
}

func saveState(onixHome, scope string, s *State) error {
	path := stateFilePath(onixHome, scope)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir timer dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write state tmp: %w", err)
	}
	return os.Rename(tmp, path)
}

func addRecent(s *State, raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	const maxRecents = 20
	filtered := make([]string, 0, len(s.Recents))
	for _, r := range s.Recents {
		if r != raw {
			filtered = append(filtered, r)
		}
	}
	s.Recents = append([]string{raw}, filtered...)
	if len(s.Recents) > maxRecents {
		s.Recents = s.Recents[:maxRecents]
	}
}

func findByIDOrName(s *State, ref string) *TimerEntry {
	ref = strings.TrimSpace(ref)
	for i := range s.Timers {
		if s.Timers[i].ID == ref {
			return &s.Timers[i]
		}
	}
	lower := strings.ToLower(ref)
	for i := range s.Timers {
		if strings.HasPrefix(strings.ToLower(s.Timers[i].Name), lower) {
			return &s.Timers[i]
		}
	}
	return nil
}

func findByID(s *State, id string) *TimerEntry {
	for i := range s.Timers {
		if s.Timers[i].ID == id {
			return &s.Timers[i]
		}
	}
	return nil
}

func removeByID(s *State, id string) bool {
	for i, e := range s.Timers {
		if e.ID == id {
			s.Timers = append(s.Timers[:i], s.Timers[i+1:]...)
			return true
		}
	}
	return false
}

func filterActive(timers []TimerEntry) []TimerEntry {
	out := make([]TimerEntry, 0, len(timers))
	cutoff := time.Now().Add(-time.Hour)
	for _, e := range timers {
		if e.Done {
			if e.StartedAt != nil && e.StartedAt.After(cutoff) {
				out = append(out, e)
			}
			continue
		}
		out = append(out, e)
	}
	return out
}
