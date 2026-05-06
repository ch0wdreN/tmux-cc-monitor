// Package hook implements the handler invoked by `tmux-cc-monitor hook <event>`.
// It reads the JSON payload from stdin, classifies the event, and writes a
// per-pane state file atomically.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ch0wdreN/tmux-cc-monitor/internal/errlog"
	"github.com/ch0wdreN/tmux-cc-monitor/internal/state"
	"github.com/ch0wdreN/tmux-cc-monitor/internal/tmuxutil"
)

const (
	maxPayloadBytes  = 1 << 20 // 1 MiB
	maxMessageRunes  = 200
)

// Handle runs the hook for the given event name.
//
// Steps:
//  1. If !tmuxutil.InTmux(): return nil immediately (no-op outside tmux).
//  2. Read all of os.Stdin into a []byte payload (may be empty).
//  3. Parse it as a generic map[string]any for inspection, AND keep
//     the raw bytes as json.RawMessage to embed in the state.
//  4. Resolve fields: cwd, paneID, serverPID, pane info.
//  5. Classify status from event + payload.
//  6. Build state.State and write it atomically.
//  7. Return any error that prevented the write.
//
// Returns nil on success or no-op (outside tmux); error on classified failures.
func Handle(event string) error {
	// Step 1: guard — no-op when not inside tmux.
	if !tmuxutil.InTmux() {
		return nil
	}

	// Step 2: read stdin. Bounded read so a runaway payload does not consume
	// arbitrary memory before we get a chance to truncate; +1 byte lets us
	// detect "fits exactly" vs "exceeded".
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, maxPayloadBytes+1))
	if err != nil {
		return fmt.Errorf("hook %s: read stdin: %w", event, err)
	}

	var payloadTruncated bool
	if len(raw) > maxPayloadBytes {
		raw = raw[:maxPayloadBytes]
		payloadTruncated = true
	}

	// Step 3: parse payload as a generic map for field inspection.
	var payload map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			// Malformed JSON: still proceed; classification will use empty payload.
			payload = nil
		}
	}

	// Only retain the raw payload when it is fully valid JSON. Truncated
	// (>1 MiB cut mid-document) bytes embedded as json.RawMessage would
	// produce a malformed state file, since RawMessage's MarshalJSON does
	// not validate. Same for unparseable payloads — better to drop than to
	// leave a state file that ReadAll will warn-and-skip on every popup.
	var rawPayload json.RawMessage
	if len(raw) > 0 && !payloadTruncated && payload != nil {
		rawPayload = json.RawMessage(raw)
	}

	// Log truncation warning after we have a pane ID candidate; we do it below.

	// Step 4: resolve fields.

	// cwd: prefer payload field, fall back to os.Getwd.
	cwd := stringField(payload, "cwd")
	if cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}

	// paneID: must be present to write a meaningful state file.
	paneID := tmuxutil.CurrentPaneID()
	if paneID == "" {
		if logErr := errlog.Append("", event, "TMUX_PANE is empty; cannot identify pane"); logErr != nil {
			_ = logErr // best-effort
		}
		return fmt.Errorf("hook %s: TMUX_PANE is empty", event)
	}

	// Log truncation warning now that we have paneID.
	if payloadTruncated {
		if logErr := errlog.Append(paneID, "payload-truncated", fmt.Sprintf("hook %s: payload exceeded 1 MiB and was truncated", event)); logErr != nil {
			_ = logErr
		}
	}

	// serverPID.
	serverPID, pidErr := tmuxutil.ServerPID()
	if pidErr != nil {
		// Non-fatal: write file with zero PID; cleanup will treat it accordingly.
		serverPID = 0
	}

	// pane info: iterate list-panes to find our pane.
	var session string
	var windowIndex int
	var windowName string
	var paneIndex int

	panes, listErr := tmuxutil.ListPanes()
	if listErr == nil {
		for _, p := range panes {
			if p.ID == paneID {
				session = p.Session
				windowIndex = p.WindowIndex
				windowName = p.WindowName
				paneIndex = p.Index
				break
			}
		}
		// If not found: leave zero/empty — pane exists from our perspective
		// even if list-panes raced.
	}
	// listErr is intentionally ignored: we still write the file.

	// Step 5: classify.
	status, lastMessage := classifyEvent(event, payload)

	// Step 6: build and write state.
	s := &state.State{
		SchemaVersion: state.SchemaVersion,
		TmuxServerPID: serverPID,
		PaneID:        paneID,
		PaneIndex:     paneIndex,
		Session:       session,
		WindowIndex:   windowIndex,
		WindowName:    windowName,
		CWD:           cwd,
		Status:        status,
		LastEvent:     event,
		LastMessage:   lastMessage,
		RawPayload:    rawPayload,
		UpdatedAt:     time.Now().UTC(),
	}

	// Step 7: write atomically.
	if err := state.WriteAtomic(s); err != nil {
		if logErr := errlog.Append(paneID, event, err.Error()); logErr != nil {
			_ = logErr
		}
		return fmt.Errorf("hook %s: write state: %w", event, err)
	}

	return nil
}

// classifyEvent maps (event name, parsed payload) to a Status and a short
// human-readable message for the popup UI.
// payload may be nil when stdin was empty or unparseable.
func classifyEvent(event string, payload map[string]any) (state.Status, string) {
	switch event {
	case "UserPromptSubmit":
		prompt := stringField(payload, "prompt")
		return state.StatusRunning, truncateRunes(prompt, maxMessageRunes)

	case "Notification":
		notifType := stringField(payload, "notification_type")
		if notifType == "permission_prompt" {
			msg := composePermissionMessage(payload)
			return state.StatusWaitingPermission, truncateRunes(msg, maxMessageRunes)
		}
		// Any other Notification subtype.
		return state.StatusWaitingOther, truncateRunes(notifType, maxMessageRunes)

	case "Stop":
		return state.StatusIdle, ""

	default:
		// Unknown / future event: treat as running with event name as message.
		return state.StatusRunning, truncateRunes(event, maxMessageRunes)
	}
}

// composePermissionMessage builds the lastMessage for a permission_prompt
// Notification. Format: "<tool_name>: <compact_json_of_tool_input>" when both
// fields are present; falls back to just the notification_type string.
func composePermissionMessage(payload map[string]any) string {
	toolName := stringField(payload, "tool_name")

	var toolInputStr string
	if ti, ok := payload["tool_input"]; ok && ti != nil {
		if b, err := json.Marshal(ti); err == nil {
			toolInputStr = string(b)
		}
	}

	if toolName != "" && toolInputStr != "" {
		return toolName + ": " + toolInputStr
	}
	if toolName != "" {
		return toolName
	}
	return stringField(payload, "notification_type")
}

// stringField extracts a string value from a generic payload map.
// Returns "" if payload is nil, the key is absent, or the value is not a string.
func stringField(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	v, ok := payload[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// truncateRunes truncates s to at most n unicode runes.
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
