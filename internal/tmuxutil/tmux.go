// Package tmuxutil wraps tmux command invocations used by the hook handler,
// cleanup logic, and TUI. All exec calls use a 5-second timeout.
package tmuxutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const execTimeout = 5 * time.Second

// Pane describes one tmux pane.
type Pane struct {
	ID          string // e.g. "%42"
	Session     string
	WindowIndex int
	WindowName  string
	Index       int // pane index within the window (0-based)
}

// InTmux returns true iff $TMUX is non-empty.
// Use this as a guard in the hook handler so we no-op outside tmux.
func InTmux() bool {
	return os.Getenv("TMUX") != ""
}

// CurrentPaneID returns os.Getenv("TMUX_PANE"), or "" if unset.
// Hook processes inherit this from Claude Code, which inherited it from tmux.
func CurrentPaneID() string {
	return os.Getenv("TMUX_PANE")
}

// ServerPID returns the tmux server PID via `tmux display-message -p '#{pid}'`.
// Used to mark state files with a generation tag so kill-server invalidates them.
func ServerPID() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "tmux", "display-message", "-p", "#{pid}")
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("tmux display-message: %w (stderr: %s)", err, stderr.String())
	}

	raw := strings.TrimSpace(string(out))
	pid, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("tmux display-message: parse pid %q: %w", raw, err)
	}
	return pid, nil
}

// ListPanes returns all panes across all sessions on the current tmux server.
// Uses `tmux list-panes -a -F
// '#{pane_id}\t#{session_name}\t#{window_index}\t#{window_name}\t#{pane_index}'`.
// pane_index is appended last so the existing field order is preserved.
func ListPanes() ([]Pane, error) {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "tmux", "list-panes", "-a",
		"-F", "#{pane_id}\t#{session_name}\t#{window_index}\t#{window_name}\t#{pane_index}")
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes: %w (stderr: %s)", err, stderr.String())
	}

	return parsePanesOutput(string(out))
}

// parsePanesOutput parses the tab-separated output of `tmux list-panes -a`.
// Exposed as an unexported function so tests can exercise parsing without a
// real tmux server.
func parsePanesOutput(s string) ([]Pane, error) {
	var panes []Pane
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 5 {
			return nil, fmt.Errorf("tmux list-panes: malformed line %q (got %d fields, want 5)", line, len(fields))
		}
		winIdx, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("tmux list-panes: parse window_index %q in line %q: %w", fields[2], line, err)
		}
		paneIdx, err := strconv.Atoi(fields[4])
		if err != nil {
			return nil, fmt.Errorf("tmux list-panes: parse pane_index %q in line %q: %w", fields[4], line, err)
		}
		panes = append(panes, Pane{
			ID:          fields[0],
			Session:     fields[1],
			WindowIndex: winIdx,
			WindowName:  fields[3],
			Index:       paneIdx,
		})
	}
	return panes, nil
}

// SendKeys sends text to a target pane followed by Enter.
//
// Strategy:
//   - The text is always sent in literal mode (`tmux send-keys -l <text>`) so
//     tmux key names and control characters are not interpreted (security per
//     Design Doc §10). Because `-l` disables key-name lookup for ALL subsequent
//     arguments in the same invocation, the Enter key must be sent as a separate
//     `send-keys` call without `-l`.
//   - For "small" text (≤ 1024 bytes AND no embedded '\n'):
//     1. `tmux send-keys -t <pane> -l <text>`
//     2. `tmux send-keys -t <pane> Enter`
//   - For larger or multiline text, uses the load-buffer / paste-buffer fallback:
//     1. `tmux load-buffer -b tmux-cc-monitor-tmp -` (text via stdin)
//     2. `tmux paste-buffer -b tmux-cc-monitor-tmp -t <pane> -p -r -d`
//     -p emits bracketed paste codes if the target application has requested
//     bracketed paste mode (so a multi-line prompt is not auto-submitted per
//     line). -r disables tmux's default LF→CR translation, which would
//     otherwise turn every newline into Enter from the consumer's
//     perspective. -d deletes the buffer after paste.
//     3. `tmux send-keys -t <pane> Enter`
//
// Returns an error if any underlying tmux command fails.
func SendKeys(paneID, text string) error {
	if len(text) <= 1024 && !strings.ContainsRune(text, '\n') {
		return sendKeysSmall(paneID, text)
	}
	return sendKeysLarge(paneID, text)
}

func sendKeysSmall(paneID, text string) error {
	if err := tmuxRun("send-keys", "-t", paneID, "-l", text); err != nil {
		return fmt.Errorf("tmux send-keys (text): %w", err)
	}
	// Enter must be a separate call: -l from the previous step would have made
	// "Enter" arrive as 5 literal characters.
	if err := tmuxRun("send-keys", "-t", paneID, "Enter"); err != nil {
		return fmt.Errorf("tmux send-keys (Enter): %w", err)
	}
	return nil
}

func sendKeysLarge(paneID, text string) error {
	const bufName = "tmux-cc-monitor-tmp"

	if err := tmuxRunStdin(text, "load-buffer", "-b", bufName, "-"); err != nil {
		return fmt.Errorf("tmux load-buffer: %w", err)
	}
	if err := tmuxRun("paste-buffer", "-b", bufName, "-t", paneID, "-p", "-r", "-d"); err != nil {
		return fmt.Errorf("tmux paste-buffer: %w", err)
	}
	if err := tmuxRun("send-keys", "-t", paneID, "Enter"); err != nil {
		return fmt.Errorf("tmux send-keys (Enter): %w", err)
	}
	return nil
}

// tmuxRun runs `tmux <args>` with execTimeout. Each call owns its own context
// so the cancel func is released when this helper returns, not when the caller
// returns — Go's defer scoping makes that important: a defer inside an inline
// `{}` block in the caller would otherwise survive until the caller's function
// exits.
func tmuxRun(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "tmux", args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w (stderr: %s)", err, stderr.String())
	}
	return nil
}

// tmuxRunStdin is like tmuxRun but feeds the given string into the tmux
// command's stdin (used for `load-buffer -`).
func tmuxRunStdin(stdin string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "tmux", args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w (stderr: %s)", err, stderr.String())
	}
	return nil
}

// CapturePane returns the last n lines of output from the target pane.
// It runs `tmux capture-pane -p -J -e -t <paneID> -S -<lines>` and strips
// any trailing newlines from the result.
// lines must be > 0; paneID must be non-empty.
func CapturePane(paneID string, lines int) (string, error) {
	if paneID == "" {
		return "", fmt.Errorf("tmuxutil: CapturePane: paneID must not be empty")
	}
	if lines <= 0 {
		return "", fmt.Errorf("tmuxutil: CapturePane: lines must be > 0")
	}

	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "tmux",
		"capture-pane", "-p", "-J", "-e",
		"-t", paneID,
		"-S", fmt.Sprintf("-%d", lines),
	)
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane: %w (stderr: %s)", err, stderr.String())
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// SendLiteral sends text to a target pane in literal mode without appending Enter.
// It runs `tmux send-keys -t <paneID> -l -- <text>`.
// If text is empty the call is a no-op and returns nil.
// paneID must be non-empty.
func SendLiteral(paneID, text string) error {
	if paneID == "" {
		return fmt.Errorf("tmuxutil: SendLiteral: paneID must not be empty")
	}
	if text == "" {
		return nil
	}
	if err := tmuxRun("send-keys", "-t", paneID, "-l", "--", text); err != nil {
		return fmt.Errorf("tmux send-keys (literal): %w", err)
	}
	return nil
}

// SendKeyName sends a single tmux key name (e.g. "Enter", "Up", "C-c") to
// the target pane. It runs `tmux send-keys -t <paneID> -- <name>`.
// Both paneID and name must be non-empty. To send multiple keys, call this
// function once per key.
func SendKeyName(paneID, name string) error {
	if paneID == "" {
		return fmt.Errorf("tmuxutil: SendKeyName: paneID must not be empty")
	}
	if name == "" {
		return fmt.Errorf("tmuxutil: SendKeyName: name must not be empty")
	}
	if err := tmuxRun("send-keys", "-t", paneID, "--", name); err != nil {
		return fmt.Errorf("tmux send-keys (key name): %w", err)
	}
	return nil
}

// paneIDInList reports whether paneID appears in panes.
// Extracted so tests can exercise the match logic without a live tmux server.
func paneIDInList(paneID string, panes []Pane) bool {
	for _, p := range panes {
		if p.ID == paneID {
			return true
		}
	}
	return false
}

// PaneAlive reports whether paneID currently exists on the tmux server.
// It delegates to ListPanes and checks the result; if ListPanes returns an
// error that error is returned as-is with bool false.
// paneID must be non-empty.
func PaneAlive(paneID string) (bool, error) {
	if paneID == "" {
		return false, fmt.Errorf("tmuxutil: PaneAlive: paneID must not be empty")
	}
	panes, err := ListPanes()
	if err != nil {
		return false, err
	}
	return paneIDInList(paneID, panes), nil
}
