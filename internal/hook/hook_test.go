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

	if status != state.StatusWaitingPermission {
		t.Errorf("status: got %q, want %q", status, state.StatusWaitingPermission)
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

	if status != state.StatusWaitingOther {
		t.Errorf("status: got %q, want %q", status, state.StatusWaitingOther)
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

	if status != state.StatusWaitingOther {
		t.Errorf("status: got %q, want %q", status, state.StatusWaitingOther)
	}
	if msg != "elicitation_dialog" {
		t.Errorf("lastMessage: got %q, want %q", msg, "elicitation_dialog")
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
