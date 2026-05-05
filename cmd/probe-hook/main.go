// Package main implements probe-hook, a diagnostic Claude Code hook binary.
//
// Install it as a Claude Code hook to capture the exact payload structure,
// environment variables, and stdin content that Claude Code delivers on each
// hook event (UserPromptSubmit, Notification, Stop).  Logs are written to
//
//	$XDG_CONFIG_HOME/tmux-cc-monitor/probe/<timestamp>-<pid>.log
//	(falls back to $HOME/.config if XDG_CONFIG_HOME is unset)
//
// To install, add to your ~/.claude/settings.json:
//
//	"hooks": {
//	  "UserPromptSubmit": [
//	    { "matcher": "", "hooks": [{ "type": "command", "command": "probe-hook UserPromptSubmit" }] }
//	  ],
//	  "Notification": [
//	    { "matcher": "", "hooks": [{ "type": "command", "command": "probe-hook Notification" }] }
//	  ],
//	  "Stop": [
//	    { "matcher": "", "hooks": [{ "type": "command", "command": "probe-hook Stop" }] }
//	  ]
//	}
//
// This binary is DIAGNOSTIC ONLY and must never be left installed in production
// use; it writes every hook payload to disk in plaintext.
//
// This binary always exits 0.  A non-zero exit from a hook causes Claude Code
// to surface an error to the user or block the triggering action, which is
// undesirable for a passive probe.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// highlightVars are the env vars we surface first because they are most
// relevant to tmux / Claude Code hook integration investigation.
var highlightVars = []string{
	"TMUX",
	"TMUX_PANE",
	"CLAUDE_PROJECT_DIR",
	"CLAUDE_PLUGIN_ROOT",
	"CLAUDE_PLUGIN_DATA",
	"CLAUDE_ENV_FILE",
	"CLAUDE_CODE_REMOTE",
}

func main() {
	// Always exit 0 — diagnostic probes must not disrupt Claude Code.
	if err := run(); err != nil {
		// Best-effort: try to append the error to stderr so it's visible in
		// Claude Code's internal logs without returning non-zero.
		fmt.Fprintf(os.Stderr, "probe-hook: %v\n", err)
	}
	os.Exit(0)
}

func run() error {
	now := time.Now()

	// 1. Determine log directory.
	configBase := os.Getenv("XDG_CONFIG_HOME")
	if configBase == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("UserHomeDir: %w", err)
		}
		configBase = filepath.Join(home, ".config")
	}
	logDir := filepath.Join(configBase, "tmux-cc-monitor", "probe")

	// 2. Create parent directory (0700 — private; logs may contain secrets).
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", logDir, err)
	}

	// 3. Build filename: RFC3339Nano with colons replaced by hyphens.
	ts := now.UTC().Format(time.RFC3339Nano)
	ts = strings.ReplaceAll(ts, ":", "-")
	logPath := filepath.Join(logDir, fmt.Sprintf("%s-%d.log", ts, os.Getpid()))

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	w := f // write everything through w so we can swap to a buffer if needed

	// 4. Header.
	cwd, _ := os.Getwd()
	argJSON, _ := json.Marshal(os.Args)
	fmt.Fprintf(w, "=== probe-hook log ===\n")
	fmt.Fprintf(w, "timestamp:   %s\n", now.UTC().Format(time.RFC3339Nano))
	fmt.Fprintf(w, "pid:         %d\n", os.Getpid())
	fmt.Fprintf(w, "ppid:        %d\n", syscall.Getppid())
	fmt.Fprintf(w, "argv:        %s\n", argJSON)
	fmt.Fprintf(w, "working_dir: %s\n", cwd)
	fmt.Fprintf(w, "\n")

	// 5. Environment — highlighted vars first, then full alphabetical dump.
	fmt.Fprintf(w, "=== environment (highlighted) ===\n")
	for _, key := range highlightVars {
		val, ok := os.LookupEnv(key)
		if ok {
			fmt.Fprintf(w, "%s=%s\n", key, val)
		} else {
			fmt.Fprintf(w, "%s=<unset>\n", key)
		}
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "=== environment (all, sorted) ===\n")
	envs := os.Environ()
	sort.Strings(envs)
	for _, e := range envs {
		fmt.Fprintf(w, "%s\n", e)
	}
	fmt.Fprintf(w, "\n")

	// 6. Stdin.
	stdinBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(w, "=== stdin ===\n<read error: %v>\n\n", err)
		return nil
	}

	fmt.Fprintf(w, "=== stdin ===\n")
	fmt.Fprintf(w, "<length: %d bytes>\n", len(stdinBytes))
	if len(stdinBytes) == 0 {
		fmt.Fprintf(w, "<empty>\n")
	} else {
		fmt.Fprintf(w, "<raw>\n%s\n", stdinBytes)
	}
	fmt.Fprintf(w, "\n")

	// 7. Prettified JSON if stdin parses.
	if len(stdinBytes) > 0 {
		var v any
		if jsonErr := json.Unmarshal(stdinBytes, &v); jsonErr == nil {
			pretty, _ := json.MarshalIndent(v, "", "  ")
			fmt.Fprintf(w, "=== stdin (prettified) ===\n%s\n\n", pretty)
		} else {
			fmt.Fprintf(w, "=== stdin (not valid JSON) ===\n%v\n\n", jsonErr)
		}
	}

	return nil
}
