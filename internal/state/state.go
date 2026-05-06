// Package state defines the per-pane state type and path helpers for
// tmux-cc-monitor. It is the foundational package; it must not import any
// other internal/* package.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unicode"
)

// SchemaVersion is the current state-file schema version written by this
// binary. ReadAll skips files whose schema_version differs from this value.
const SchemaVersion = 1

// Status represents the lifecycle state of a Claude Code session.
type Status string

const (
	// StatusRunning indicates Claude Code is actively processing a prompt.
	StatusRunning Status = "running"
	// StatusWaitingAction indicates Claude Code is waiting for a user action
	// (e.g. permission_prompt, elicitation_dialog, or similar interactive events).
	StatusWaitingAction Status = "waiting_action"
	// StatusWaitingOther indicates Claude Code sent a Notification that is not
	// a permission request (e.g. idle notice, long-running task notice).
	StatusWaitingOther Status = "waiting_other"
	// StatusIdle indicates Claude Code has finished responding (Stop hook).
	StatusIdle Status = "idle"
)

// State holds the most-recent known state for a single tmux pane running
// Claude Code. It is serialized as JSON to
// ~/.config/tmux-cc-monitor/sessions/<N>.json.
type State struct {
	SchemaVersion int             `json:"schema_version"`
	TmuxServerPID int             `json:"tmux_server_pid"`
	PaneID        string          `json:"pane_id"`
	PaneIndex     int             `json:"pane_index"`
	Session       string          `json:"session"`
	WindowIndex   int             `json:"window_index"`
	WindowName    string          `json:"window_name"`
	CWD           string          `json:"cwd"`
	Status        Status          `json:"status"`
	LastEvent     string          `json:"last_event"`
	LastMessage   string          `json:"last_message"`
	RawPayload    json.RawMessage `json:"raw_payload,omitempty"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// SessionsDir returns the absolute path of the sessions directory,
// ~/.config/tmux-cc-monitor/sessions/.
// It honors $XDG_CONFIG_HOME when set; otherwise falls back to
// $HOME/.config.
func SessionsDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("state: resolve home dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "tmux-cc-monitor", "sessions"), nil
}

// PathFor returns the absolute path of the state file for paneID.
// paneID must be of the form "%N" where N is one or more ASCII digits.
// The leading "%" is stripped; the file is "<sessionsDir>/<N>.json".
func PathFor(paneID string) (string, error) {
	if len(paneID) == 0 {
		return "", fmt.Errorf("state: pane id must not be empty")
	}
	if paneID[0] != '%' {
		return "", fmt.Errorf("state: pane id %q must start with '%%'", paneID)
	}
	digits := paneID[1:]
	if len(digits) == 0 {
		return "", fmt.Errorf("state: pane id %q has no numeric part", paneID)
	}
	for _, r := range digits {
		if !unicode.IsDigit(r) {
			return "", fmt.Errorf("state: pane id %q contains non-digit characters after '%%'", paneID)
		}
	}
	dir, err := SessionsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, digits+".json"), nil
}
