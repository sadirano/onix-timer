package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	return buf.String()
}

func TestResolveRef(t *testing.T) {
	cases := []struct {
		args    []string
		want    string
		wantErr bool
	}{
		{[]string{"standup"}, "standup", false},
		{[]string{"my", "timer"}, "my timer", false},
		{[]string{"-g", "t1"}, "t1", false},    // flags stripped
		{[]string{"--global", "t1"}, "t1", false},
		{[]string{}, "", true},
		{[]string{"-g"}, "", true}, // only a flag, no ref
	}
	for _, c := range cases {
		ref, err := resolveRef(c.args, "stop")
		if c.wantErr {
			if err == nil {
				t.Errorf("resolveRef(%v): expected error", c.args)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveRef(%v): unexpected error: %v", c.args, err)
			continue
		}
		if ref != c.want {
			t.Errorf("resolveRef(%v): want %q, got %q", c.args, c.want, ref)
		}
	}
}

func TestRunScopesEmpty(t *testing.T) {
	dir := t.TempDir()
	out := captureStdout(func() { runScopes(dir) }) //nolint:errcheck
	if !strings.Contains(out, "No scopes") {
		t.Errorf("expected 'No scopes' message, got: %q", out)
	}
}

func TestRunScopesLists(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	s := &State{
		Version: 1, NextID: 2,
		Timers: []TimerEntry{
			{ID: "t1", Name: "standup", Kind: "countdown", DurationS: 300, StartedAt: &now},
		},
	}
	saveState(dir, "work", s)
	saveState(dir, "global", &State{Version: 1, NextID: 1})

	out := captureStdout(func() { runScopes(dir) }) //nolint:errcheck

	if !strings.Contains(out, "work") {
		t.Errorf("expected 'work' scope in output, got: %q", out)
	}
	if !strings.Contains(out, "global") {
		t.Errorf("expected 'global' scope in output, got: %q", out)
	}
}

func TestRunCleanRemovesDone(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	s := &State{
		Version: 1, NextID: 3,
		Timers: []TimerEntry{
			{ID: "t1", Name: "running", Kind: "countdown", DurationS: 300, StartedAt: &now},
			{ID: "t2", Name: "done-one", Kind: "countdown", Done: true},
			{ID: "t3", Name: "done-two", Kind: "countdown", Done: true},
		},
	}
	saveState(dir, "global", s)

	out := captureStdout(func() { runClean(dir, "global") }) //nolint:errcheck

	if !strings.Contains(out, "2") {
		t.Errorf("expected '2' in clean output, got: %q", out)
	}

	got, _ := loadState(dir, "global")
	if len(got.Timers) != 1 || got.Timers[0].ID != "t1" {
		t.Fatalf("expected only running timer remaining, got: %+v", got.Timers)
	}
}

func TestRunCleanNothingToDo(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	s := &State{
		Version: 1, NextID: 2,
		Timers: []TimerEntry{
			{ID: "t1", Name: "standup", Kind: "countdown", DurationS: 300, StartedAt: &now},
		},
	}
	saveState(dir, "global", s)

	captureStdout(func() { runClean(dir, "global") }) //nolint:errcheck

	got, _ := loadState(dir, "global")
	if len(got.Timers) != 1 {
		t.Fatal("clean with no done timers should not remove anything")
	}
}
