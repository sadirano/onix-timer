package timer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadStateMissing(t *testing.T) {
	s, err := loadState(t.TempDir(), "global")
	if err != nil {
		t.Fatal(err)
	}
	if s.NextID != 1 {
		t.Fatalf("want NextID=1, got %d", s.NextID)
	}
}

func TestLoadStateCorrupt(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "timer")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "global.json"), []byte("not json{"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := loadState(dir, "global")
	if err != nil {
		t.Fatal(err)
	}
	if s.NextID != 1 {
		t.Fatalf("corrupt file should yield fresh state, got NextID=%d", s.NextID)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	s := &State{
		Version: 1,
		NextID:  5,
		Timers: []TimerEntry{{
			ID:         "t1",
			Name:       "Test",
			Kind:       "countdown",
			DurationS:  300,
			StartedAt:  &now,
			RemainingS: 295,
		}},
		Recents: []string{"5m"},
	}
	if err := saveState(dir, "global", s); err != nil {
		t.Fatal(err)
	}
	got, err := loadState(dir, "global")
	if err != nil {
		t.Fatal(err)
	}
	if got.NextID != 5 {
		t.Fatalf("NextID: want 5, got %d", got.NextID)
	}
	if len(got.Timers) != 1 || got.Timers[0].ID != "t1" {
		t.Fatal("timer not preserved")
	}
	if got.Timers[0].StartedAt == nil || !got.Timers[0].StartedAt.Equal(now) {
		t.Fatalf("StartedAt not preserved: %v", got.Timers[0].StartedAt)
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "timer", "global.json")
	s := &State{Version: 1, NextID: 1}
	if err := saveState(dir, "global", s); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file not created: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Fatal("temp file should be gone after rename")
	}
}

func TestAddRecent(t *testing.T) {
	s := &State{}
	addRecent(s, "25m")
	addRecent(s, "5m")
	addRecent(s, "25m") // duplicate — should move to front
	if s.Recents[0] != "25m" {
		t.Fatalf("want 25m at front, got %s", s.Recents[0])
	}
	if len(s.Recents) != 2 {
		t.Fatalf("want 2 recents, got %d", len(s.Recents))
	}
}

func TestAddRecentCap(t *testing.T) {
	s := &State{}
	for i := 0; i < 25; i++ {
		addRecent(s, fmt.Sprintf("%dm", i+1))
	}
	if len(s.Recents) > 20 {
		t.Fatalf("recents should be capped at 20, got %d", len(s.Recents))
	}
}

func TestFindByIDOrName(t *testing.T) {
	s := &State{Timers: []TimerEntry{
		{ID: "t1", Name: "standup"},
		{ID: "t2", Name: "deploy check"},
	}}
	if e := findByIDOrName(s, "t1"); e == nil || e.ID != "t1" {
		t.Fatal("should find by ID")
	}
	if e := findByIDOrName(s, "stan"); e == nil || e.Name != "standup" {
		t.Fatal("should find by name prefix")
	}
	if e := findByIDOrName(s, "STAN"); e == nil {
		t.Fatal("name match should be case-insensitive")
	}
	if e := findByIDOrName(s, "missing"); e != nil {
		t.Fatal("should return nil for missing")
	}
}

func TestComputeRemaining(t *testing.T) {
	now := time.Now()
	past := now.Add(-30 * time.Second)
	e := TimerEntry{
		Kind:       "countdown",
		DurationS:  60,
		RemainingS: 60,
		StartedAt:  &past,
	}
	rem := e.ComputeRemaining()
	if rem < 28 || rem > 32 {
		t.Fatalf("expected ~30s remaining, got %d", rem)
	}
}

func TestComputeRemainingPaused(t *testing.T) {
	now := time.Now()
	past := now.Add(-10 * time.Second)
	e := TimerEntry{
		Kind:       "countdown",
		DurationS:  60,
		RemainingS: 50,
		StartedAt:  &past,
		PausedAt:   &now,
	}
	if rem := e.ComputeRemaining(); rem != 50 {
		t.Fatalf("paused timer should return frozen RemainingS=50, got %d", rem)
	}
}

func TestComputeRemainingDone(t *testing.T) {
	e := TimerEntry{Kind: "countdown", Done: true, RemainingS: 100}
	if rem := e.ComputeRemaining(); rem != 0 {
		t.Fatalf("done timer should return 0, got %d", rem)
	}
}

func TestFilterActive(t *testing.T) {
	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now().Add(-30 * time.Minute)
	timers := []TimerEntry{
		{ID: "t1", Kind: "countdown", Done: false},
		{ID: "t2", Kind: "countdown", Done: true, StartedAt: &old},    // old done → filtered
		{ID: "t3", Kind: "countdown", Done: true, StartedAt: &recent}, // recent done → kept
	}
	active := filterActive(timers)
	if len(active) != 2 {
		t.Fatalf("want 2 active, got %d", len(active))
	}
	ids := map[string]bool{}
	for _, e := range active {
		ids[e.ID] = true
	}
	if !ids["t1"] || !ids["t3"] {
		t.Fatal("expected t1 and t3 in active list")
	}
}


