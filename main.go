package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	onixHome := strings.TrimSpace(os.Getenv("ONIX_HOME"))
	vis := loadConfig(onixHome)
	args := os.Args[1:]

	if len(args) == 0 {
		scope, _, hasContext := resolveScope(nil)
		if !hasContext {
			scope = promptScopeIfNeeded(onixHome, scope)
		}
		s, _ := loadState(onixHome, scope)
		if len(filterActive(s.Timers)) == 0 {
			printHelp()
			return
		}
		if err := runLS(onixHome, scope, &vis, false, false); err != nil {
			fatal("%v", err)
		}
		return
	}

	subcmd := strings.ToLower(strings.TrimSpace(args[0]))
	rest := args[1:]

	// daemon is an internal subcommand invoked by ensureDaemon; it receives onixHome as rest[0].
	if subcmd == "daemon" {
		effectiveHome := onixHome
		if len(rest) > 0 && strings.TrimSpace(rest[0]) != "" {
			effectiveHome = strings.TrimSpace(rest[0])
		}
		runDaemon(effectiveHome)
		return
	}

	switch subcmd {
	case "start":
		scope, cleaned, hasContext := resolveScope(rest)
		if !hasContext {
			scope = promptScopeIfNeeded(onixHome, scope)
		}
		if err := runStart(cleaned, onixHome, scope, &vis); err != nil {
			fatal("%v", err)
		}
	case "stop":
		scope, cleaned, hasContext := resolveScope(rest)
		if !hasContext {
			scope = promptScopeIfNeeded(onixHome, scope)
		}
		if err := runStop(cleaned, onixHome, scope, &vis); err != nil {
			fatal("%v", err)
		}
	case "cancel", "rm", "delete":
		scope, cleaned, hasContext := resolveScope(rest)
		if !hasContext {
			scope = promptScopeIfNeeded(onixHome, scope)
		}
		if err := runCancel(cleaned, onixHome, scope); err != nil {
			fatal("%v", err)
		}
	case "reset":
		scope, cleaned, hasContext := resolveScope(rest)
		if !hasContext {
			scope = promptScopeIfNeeded(onixHome, scope)
		}
		if err := runReset(cleaned, onixHome, scope, &vis); err != nil {
			fatal("%v", err)
		}
	case "lap":
		scope, cleaned, hasContext := resolveScope(rest)
		if !hasContext {
			scope = promptScopeIfNeeded(onixHome, scope)
		}
		if err := runLap(cleaned, onixHome, scope); err != nil {
			fatal("%v", err)
		}
	case "status":
		scope, _, hasContext := resolveScope(rest)
		if !hasContext {
			scope = promptScopeIfNeeded(onixHome, scope)
		}
		if err := runStatus(onixHome, scope, &vis); err != nil {
			fatal("%v", err)
		}
	case "ls", "list":
		scope, cleaned, hasContext := resolveScope(rest)
		if !hasContext {
			scope = promptScopeIfNeeded(onixHome, scope)
		}
		raw := hasFlag(cleaned, "--raw")
		watch := hasFlag(cleaned, "--watch")
		if err := runLS(onixHome, scope, &vis, raw, watch); err != nil {
			fatal("%v", err)
		}
	case "scopes":
		if err := runScopes(onixHome); err != nil {
			fatal("%v", err)
		}
	case "help", "-h", "--help":
		printHelp()
	default:
		// Implicit start: treat all args as start spec
		scope, cleaned, hasContext := resolveScope(args)
		if !hasContext {
			scope = promptScopeIfNeeded(onixHome, scope)
		}
		if err := runStart(cleaned, onixHome, scope, &vis); err != nil {
			fatal("%v", err)
		}
	}
}

// resolveScope extracts the timer scope from env vars and/or --global/-g flag.
// hasContext is true when any explicit context was found; false means the "global"
// default was used with no user intent — callers may then prompt for a scope.
// Priority: --global/-g flag > ONIX_TARGET env > ONIX_ALIAS env > "global"
func resolveScope(args []string) (scope string, remaining []string, hasContext bool) {
	target := strings.TrimSpace(os.Getenv("ONIX_TARGET"))
	alias := strings.TrimSpace(os.Getenv("ONIX_ALIAS"))

	remaining = make([]string, 0, len(args))
	global := false
	for _, a := range args {
		if a == "--global" || a == "-g" {
			global = true
		} else {
			remaining = append(remaining, a)
		}
	}

	if global {
		return "global", remaining, true
	}
	if target != "" {
		return sanitizeScope(target), remaining, true
	}
	if alias != "" {
		return sanitizeScope(alias), remaining, true
	}
	return "global", remaining, false
}

// promptScopeIfNeeded shows a scope picker when stdin is interactive and more
// than one scope exists. Returns current unchanged when there is nothing to pick.
func promptScopeIfNeeded(onixHome, current string) string {
	if !isTerminal() {
		return current
	}
	files, _ := filepath.Glob(filepath.Join(timerDir(onixHome), "*.json"))
	var others []string
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		if name != "global" {
			others = append(others, name)
		}
	}
	if len(others) == 0 {
		return current
	}
	all := append(others, "global")
	fmt.Printf("No scope set. Available: %s\n", strings.Join(all, ", "))
	fmt.Printf("Scope (default: global): ")
	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(input)
	if input == "" {
		return "global"
	}
	return sanitizeScope(input)
}

// sanitizeScope converts an arbitrary string into a safe filename slug.
func sanitizeScope(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "global"
	}
	return result
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "onix-timer: "+format+"\n", a...)
	os.Exit(1)
}

func printHelp() {
	fmt.Print(`onix-timer — smart CLI timer for the Onix ecosystem

USAGE
  timer [start] <spec>                   Start a countdown (implicit start)
  timer start --stopwatch [name]         Count-up stopwatch
  timer start --every <dur> [name]                    Repeat on interval (infinite)
  timer start --every <dur> --times <n> [name]        Repeat exactly N times
  timer start --every <dur> --until <time> [name]     Repeat until wall-clock time
  timer start --at <time> [--every <d>]               Delayed start, optionally repeating
  timer stop <id|name>                   Pause / resume timer
  timer cancel <id|name>                 Delete timer
  timer reset <id|name>                  Restart from original duration
  timer lap <id|name>                    Record split on stopwatch
  timer ls [--raw] [--watch]             List timers; --watch refreshes live (q to quit)
  timer status                           One-line summary of active timers
  timer scopes                           List all scopes and their active timer count
  timer help                             Show this help

SPEC EXAMPLES
  25m                   25 minutes
  1h30m                 1 hour 30 minutes
  27:30                 27 minutes 30 seconds (MM:SS)
  1:30:00               HH:MM:SS
  90s                   90 seconds
  25                    25 minutes (bare number)
  every 1h              repeat every hour, first fires in 1h
  every 2h from 1pm     repeat every 2h starting at 13:00
  from 9am every 30m    same, alternate word order
  at 14:30              one-shot delayed start

FLAGS (with start)
  --name, -n <name>   Set timer name
  --stopwatch         Count up instead of down
  --every <dur>       Repeat on this interval
  --at <time>         Delay start (HH:MM or 3pm format)
  --exec <cmd>        Run shell command on each expiry
  --times <n>         Fire repeat timer exactly N times then stop
  --until <time>      Stop repeat timer after this wall-clock time
  --notify            Force notifications on (overrides config)
  --no-notify         Disable notifications for this timer
  --global, -g        Use global timer scope (ignore project context)

FLAGS (with ls)
  --raw               TSV output with raw seconds
  --watch             Refresh table every second
  --global, -g        Show global timers

CONTROLS (in fzf picker)
  Enter               Show timer details
  ctrl-s              Stop / resume selected timer
  ctrl-d              Cancel / delete selected timer
  ctrl-r              Reset selected timer
`)
}
