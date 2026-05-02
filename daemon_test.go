package main

import (
	"testing"
	"time"
)

func pastEntry(id string) TimerEntry {
	start := time.Now().Add(-2 * time.Minute)
	fire := time.Now().Add(-time.Second)
	return TimerEntry{
		ID:         id,
		Name:       "test-" + id,
		Kind:       "countdown",
		DurationS:  60,
		StartedAt:  &start,
		RemainingS: 60,
		NextFireAt: &fire,
	}
}

func TestProcessScopeCountdownFires(t *testing.T) {
	dir := t.TempDir()
	s := &State{Version: 1, NextID: 2, Timers: []TimerEntry{pastEntry("t1")}}
	if err := saveState(dir, "global", s); err != nil {
		t.Fatal(err)
	}

	hasActive := processScope(dir, "global")
	if hasActive {
		t.Fatal("fired countdown — no active timers should remain")
	}

	got, _ := loadState(dir, "global")
	e := got.Timers[0]
	if !e.Done {
		t.Fatal("timer should be marked Done after firing")
	}
	if e.NextFireAt != nil {
		t.Fatal("NextFireAt should be nil after firing")
	}
}

func TestProcessScopeCountdownNotYetDue(t *testing.T) {
	dir := t.TempDir()
	start := time.Now()
	future := time.Now().Add(10 * time.Minute)
	s := &State{Version: 1, NextID: 2, Timers: []TimerEntry{{
		ID: "t1", Name: "test", Kind: "countdown",
		DurationS: 600, StartedAt: &start, RemainingS: 600,
		NextFireAt: &future,
	}}}
	saveState(dir, "global", s)

	if !processScope(dir, "global") {
		t.Fatal("timer not yet due — should still be active")
	}

	got, _ := loadState(dir, "global")
	if got.Timers[0].Done {
		t.Fatal("timer should not be done yet")
	}
}

func TestProcessScopeRepeatReschedules(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().Add(-5 * time.Minute)
	fire := time.Now().Add(-time.Second)
	s := &State{Version: 1, NextID: 2, Timers: []TimerEntry{{
		ID: "t1", Name: "standup", Kind: "repeat",
		DurationS: 1800, RepeatEveryS: 1800,
		StartedAt: &start, RemainingS: 1800,
		NextFireAt: &fire,
	}}}
	saveState(dir, "global", s)

	if !processScope(dir, "global") {
		t.Fatal("repeat timer should still be active after firing")
	}

	got, _ := loadState(dir, "global")
	e := got.Timers[0]
	if e.Done {
		t.Fatal("repeat timer should not be done after first fire")
	}
	if e.RepeatFired != 1 {
		t.Fatalf("RepeatFired: want 1, got %d", e.RepeatFired)
	}
	if e.NextFireAt == nil {
		t.Fatal("NextFireAt should be rescheduled")
	}
	if !e.NextFireAt.After(time.Now()) {
		t.Fatal("rescheduled NextFireAt should be in the future")
	}
}

func TestProcessScopeRepeatTimesExhausted(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().Add(-5 * time.Minute)
	fire := time.Now().Add(-time.Second)
	s := &State{Version: 1, NextID: 2, Timers: []TimerEntry{{
		ID: "t1", Name: "standup", Kind: "repeat",
		DurationS: 300, RepeatEveryS: 300,
		StartedAt: &start, RemainingS: 300,
		NextFireAt:  &fire,
		RepeatTimes: 3, RepeatFired: 2, // this fire is the 3rd = last
	}}}
	saveState(dir, "global", s)

	if processScope(dir, "global") {
		t.Fatal("all repeats exhausted — should have no active timers")
	}

	got, _ := loadState(dir, "global")
	e := got.Timers[0]
	if !e.Done {
		t.Fatal("repeat timer should be Done when RepeatTimes exhausted")
	}
	if e.RepeatFired != 3 {
		t.Fatalf("RepeatFired: want 3, got %d", e.RepeatFired)
	}
}

func TestProcessScopeRepeatTimesNotYetExhausted(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().Add(-5 * time.Minute)
	fire := time.Now().Add(-time.Second)
	s := &State{Version: 1, NextID: 2, Timers: []TimerEntry{{
		ID: "t1", Name: "standup", Kind: "repeat",
		DurationS: 300, RepeatEveryS: 300,
		StartedAt: &start, RemainingS: 300,
		NextFireAt:  &fire,
		RepeatTimes: 5, RepeatFired: 2, // fires left: 3rd of 5
	}}}
	saveState(dir, "global", s)

	if !processScope(dir, "global") {
		t.Fatal("repeat timer with remaining fires should still be active")
	}

	got, _ := loadState(dir, "global")
	if got.Timers[0].Done {
		t.Fatal("timer should not be done — still has fires remaining")
	}
	if got.Timers[0].RepeatFired != 3 {
		t.Fatalf("RepeatFired: want 3, got %d", got.Timers[0].RepeatFired)
	}
}

func TestProcessScopeRepeatUntilExceeded(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().Add(-5 * time.Minute)
	fire := time.Now().Add(-time.Second)
	// RepeatUntil is 5 min from now; RepeatEveryS is 30 min → next fire > until
	until := time.Now().Add(5 * time.Minute)
	s := &State{Version: 1, NextID: 2, Timers: []TimerEntry{{
		ID: "t1", Name: "test", Kind: "repeat",
		DurationS: 1800, RepeatEveryS: 1800,
		StartedAt: &start, RemainingS: 1800,
		NextFireAt:  &fire,
		RepeatUntil: &until,
	}}}
	saveState(dir, "global", s)

	if processScope(dir, "global") {
		t.Fatal("next fire exceeds RepeatUntil — should be done")
	}

	got, _ := loadState(dir, "global")
	if !got.Timers[0].Done {
		t.Fatal("timer should be Done when next fire would exceed RepeatUntil")
	}
}

func TestProcessScopeMigration(t *testing.T) {
	dir := t.TempDir()
	// Pre-daemon timer: has no NextFireAt, but still has remaining time.
	start := time.Now().Add(-10 * time.Second)
	s := &State{Version: 1, NextID: 2, Timers: []TimerEntry{{
		ID: "t1", Name: "old", Kind: "countdown",
		DurationS: 300, StartedAt: &start, RemainingS: 300,
		// NextFireAt intentionally absent
	}}}
	saveState(dir, "global", s)

	if !processScope(dir, "global") {
		t.Fatal("migrated timer should be active")
	}

	got, _ := loadState(dir, "global")
	if got.Timers[0].NextFireAt == nil {
		t.Fatal("migration should have populated NextFireAt")
	}
	if got.Timers[0].Done {
		t.Fatal("migrated timer with remaining time should not be done")
	}
}

func TestProcessScopePausedIgnored(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().Add(-time.Minute)
	paused := time.Now()
	fire := time.Now().Add(-time.Second) // past, but paused — must not fire
	s := &State{Version: 1, NextID: 2, Timers: []TimerEntry{{
		ID: "t1", Name: "test", Kind: "countdown",
		DurationS: 300, StartedAt: &start, RemainingS: 200,
		PausedAt: &paused, NextFireAt: &fire,
	}}}
	saveState(dir, "global", s)

	processScope(dir, "global")

	got, _ := loadState(dir, "global")
	if got.Timers[0].Done {
		t.Fatal("paused timer must not be fired")
	}
}

func TestProcessScopeStopwatchIgnored(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().Add(-time.Minute)
	fire := time.Now().Add(-time.Second)
	s := &State{Version: 1, NextID: 2, Timers: []TimerEntry{{
		ID: "t1", Name: "workout", Kind: "stopwatch",
		StartedAt: &start, NextFireAt: &fire,
	}}}
	saveState(dir, "global", s)

	processScope(dir, "global")

	got, _ := loadState(dir, "global")
	if got.Timers[0].Done {
		t.Fatal("stopwatch must not be fired by the daemon")
	}
}

func TestProcessScopeAlreadyDoneIgnored(t *testing.T) {
	dir := t.TempDir()
	fire := time.Now().Add(-time.Second)
	s := &State{Version: 1, NextID: 2, Timers: []TimerEntry{{
		ID: "t1", Name: "test", Kind: "countdown",
		Done: true, NextFireAt: &fire,
	}}}
	saveState(dir, "global", s)

	processScope(dir, "global")

	got, _ := loadState(dir, "global")
	// RepeatFired and NextFireAt should be untouched for done timers
	if got.Timers[0].RepeatFired != 0 {
		t.Fatal("Done timer should not have RepeatFired incremented")
	}
}
