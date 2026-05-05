// tmux-cc-monitor wires together the hook handler and popup TUI.
//
// Usage:
//
//	tmux-cc-monitor hook <event>   — called by Claude Code hooks
//	tmux-cc-monitor ui             — called from a tmux keybinding to open the popup
//
// All business logic lives in internal/*; this file is only routing and
// top-level error reporting.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ch0wdreN/tmux-cc-monitor/internal/cleanup"
	"github.com/ch0wdreN/tmux-cc-monitor/internal/errlog"
	"github.com/ch0wdreN/tmux-cc-monitor/internal/hook"
	"github.com/ch0wdreN/tmux-cc-monitor/internal/state"
	"github.com/ch0wdreN/tmux-cc-monitor/internal/tmuxutil"
	"github.com/ch0wdreN/tmux-cc-monitor/internal/ui"
)

const usageText = `tmux-cc-monitor — monitor and steer parallel Claude Code sessions across tmux panes

Usage:
  tmux-cc-monitor hook <event>    Called by Claude Code hooks (UserPromptSubmit / Notification / Stop)
  tmux-cc-monitor ui              Launch the popup TUI (call from a tmux keybinding)

See README.md and docs/design-doc/ for setup instructions.`

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	switch args[0] {
	case "hook":
		runHook(args[1:])
	case "ui":
		runUI()
	case "help", "--help", "-h":
		printUsage(os.Stdout)
		os.Exit(0)
	default:
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, usageText)
}

func runHook(args []string) {
	if len(args) == 0 {
		printUsage(os.Stderr)
		os.Exit(2)
	}
	eventName := args[0]

	if err := hook.Handle(eventName); err != nil {
		fmt.Fprintln(os.Stderr, "tmux-cc-monitor hook:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runUI() {
	if !tmuxutil.InTmux() {
		fmt.Fprintln(os.Stderr, "tmux-cc-monitor ui must be run inside tmux")
		os.Exit(1)
	}

	serverPID, err := tmuxutil.ServerPID()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tmux-cc-monitor ui:", err)
		os.Exit(1)
	}

	panes, err := tmuxutil.ListPanes()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	livePaneIDs := make(map[string]bool, len(panes))
	for _, p := range panes {
		livePaneIDs[p.ID] = true
	}

	// Best-effort: ignore cleanup errors and result for v0.0.1.
	cleanup.Run(serverPID, livePaneIDs) //nolint:errcheck

	states, warnings, err := state.ReadAll()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}

	errCount, err := errlog.RecentCount()
	if err != nil {
		errCount = 0
	}

	if err := ui.Run(states, errCount); err != nil {
		fmt.Fprintln(os.Stderr, "tmux-cc-monitor ui:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
