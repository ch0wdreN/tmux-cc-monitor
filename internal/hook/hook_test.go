package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ch0wdreN/tmux-cc-monitor/internal/state"
)

// ---------------------------------------------------------------------------
// classifyEvent tests
// ---------------------------------------------------------------------------

func TestClassify_UserPromptSubmit(t *testing.T) {
	payload := map[string]any{"prompt": "hello world"}
	status, msg := classifyEvent("UserPromptSubmit", payload)

	if status != state.StatusRunning {
		t.Errorf("status: got %q, want %q", status, state.StatusRunning)
	}
	if msg != "hello world" {
		t.Errorf("lastMessage: got %q, want %q", msg, "hello world")
	}
}

func TestClassify_NotificationPermissionPrompt(t *testing.T) {
	payload := map[string]any{
		"notification_type": "permission_prompt",
		"tool_name":         "Bash",
		"tool_input":        map[string]any{"command": "rm -rf /"},
	}
	status, msg := classifyEvent("Notification", payload)

	if status != state.StatusWaitingAction {
		t.Errorf("status: got %q, want %q", status, state.StatusWaitingAction)
	}
	if !strings.Contains(msg, "Bash") {
		t.Errorf("lastMessage %q does not contain tool_name %q", msg, "Bash")
	}
}

func TestClassify_NotificationIdle(t *testing.T) {
	payload := map[string]any{
		"notification_type": "idle_prompt",
	}
	status, msg := classifyEvent("Notification", payload)

	if status != state.StatusIdle {
		t.Errorf("status: got %q, want %q", status, state.StatusIdle)
	}
	if msg != "idle_prompt" {
		t.Errorf("lastMessage: got %q, want %q", msg, "idle_prompt")
	}
}

func TestClassify_NotificationElicitation(t *testing.T) {
	payload := map[string]any{
		"notification_type": "elicitation_dialog",
	}
	status, msg := classifyEvent("Notification", payload)

	if status != state.StatusWaitingAction {
		t.Errorf("status: got %q, want %q", status, state.StatusWaitingAction)
	}
	if msg != "elicitation_dialog" {
		t.Errorf("lastMessage: got %q, want %q", msg, "elicitation_dialog")
	}
}

// TestClassifyEvent_Notification covers all notification_type branches defined
// in Design Doc §6.5.
func TestClassifyEvent_Notification(t *testing.T) {
	tests := []struct {
		name           string
		notifType      string
		wantStatus     state.Status
		wantMessageHas string
	}{
		{"permission_prompt → waiting_action", "permission_prompt", state.StatusWaitingAction, ""},
		{"idle_prompt → idle", "idle_prompt", state.StatusIdle, "idle_prompt"},
		{"elicitation_dialog → waiting_action", "elicitation_dialog", state.StatusWaitingAction, "elicitation_dialog"},
		{"elicitation_response → running", "elicitation_response", state.StatusRunning, "elicitation_response"},
		{"elicitation_complete → running", "elicitation_complete", state.StatusRunning, "elicitation_complete"},
		{"auth_success → idle", "auth_success", state.StatusIdle, "auth_success"},
		{"unknown_future_type → waiting_other", "unknown_future_type", state.StatusWaitingOther, "unknown_future_type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]any{"notification_type": tt.notifType}
			gotStatus, gotMsg := classifyEvent("Notification", payload)
			if gotStatus != tt.wantStatus {
				t.Errorf("status: got %q, want %q", gotStatus, tt.wantStatus)
			}
			if tt.wantMessageHas != "" && !strings.Contains(gotMsg, tt.wantMessageHas) {
				t.Errorf("lastMessage %q does not contain %q", gotMsg, tt.wantMessageHas)
			}
		})
	}
}

// TestClassifyEvent_PermissionPromptMessage verifies that composePermissionMessage
// embeds tool_name and tool_input fields into the lastMessage.
func TestClassifyEvent_PermissionPromptMessage(t *testing.T) {
	payload := map[string]any{
		"notification_type": "permission_prompt",
		"tool_name":         "Bash",
		"tool_input":        map[string]any{"command": "ls"},
	}
	gotStatus, gotMsg := classifyEvent("Notification", payload)

	if gotStatus != state.StatusWaitingAction {
		t.Errorf("status: got %q, want %q", gotStatus, state.StatusWaitingAction)
	}
	if !strings.Contains(gotMsg, "Bash") {
		t.Errorf("lastMessage %q does not contain tool_name %q", gotMsg, "Bash")
	}
	if !strings.Contains(gotMsg, "ls") {
		t.Errorf("lastMessage %q does not contain command %q", gotMsg, "ls")
	}
}

func TestClassify_Stop(t *testing.T) {
	status, msg := classifyEvent("Stop", nil)

	if status != state.StatusIdle {
		t.Errorf("status: got %q, want %q", status, state.StatusIdle)
	}
	if msg != "" {
		t.Errorf("lastMessage: got %q, want empty", msg)
	}
}

func TestClassify_Unknown(t *testing.T) {
	status, msg := classifyEvent("WhateverElse", nil)

	if status != state.StatusRunning {
		t.Errorf("status: got %q, want %q", status, state.StatusRunning)
	}
	if msg != "WhateverElse" {
		t.Errorf("lastMessage: got %q, want %q", msg, "WhateverElse")
	}
}

func TestClassify_LongPromptTruncation(t *testing.T) {
	// Build a 500-rune prompt using multi-byte characters to prove we truncate
	// by rune count, not byte count.
	longPrompt := strings.Repeat("あ", 500) // each 'あ' is 3 bytes in UTF-8
	payload := map[string]any{"prompt": longPrompt}

	_, msg := classifyEvent("UserPromptSubmit", payload)

	runes := []rune(msg)
	if len(runes) != maxMessageRunes {
		t.Errorf("lastMessage rune count: got %d, want %d", len(runes), maxMessageRunes)
	}
}

// ---------------------------------------------------------------------------
// Handle integration-ish test
// ---------------------------------------------------------------------------

// TestHandle_NoTmuxNoOp verifies that Handle is a no-op when $TMUX is unset:
// no state file must be created.
func TestHandle_NoTmuxNoOp(t *testing.T) {
	// Isolate the config directory so no real state files are touched.
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Remove $TMUX so InTmux() returns false.
	t.Setenv("TMUX", "")

	// Redirect stdin to an empty reader.
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	_ = w.Close() // EOF immediately
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()

	if err := Handle("UserPromptSubmit"); err != nil {
		t.Errorf("Handle returned error: %v", err)
	}

	// Verify no state files were written.
	sessionsDir := filepath.Join(tmpDir, "tmux-cc-monitor", "sessions")
	entries, readErr := os.ReadDir(sessionsDir)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatalf("ReadDir: %v", readErr)
	}
	if len(entries) > 0 {
		t.Errorf("expected no state files, found %d", len(entries))
	}
}
