package ui

// mirror_test.go covers the Phase 4 acceptance criteria from Design Doc §5:
//
//  - mapKey('q') returns actionSendLiteral so q is forwarded to the target pane.
//  - mapKey(Esc) returns actionQuit so Esc exits mirror mode back to the list.
//  - viewMirror output contains "esc" and does NOT contain "q quit".
//
// These tests are the canonical regression guard for the v0.0.2 behaviour
// change that removed q as a mirror-exit shortcut.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestMapKey_QForwardedToTargetPane verifies that pressing q in mirror mode
// produces actionSendLiteral with literal "q", meaning the key is forwarded
// to the target pane rather than exiting mirror mode.
func TestMapKey_QForwardedToTargetPane(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	got := mapKey(msg)
	if got.action != actionSendLiteral {
		t.Errorf("mapKey('q').action = %v, want actionSendLiteral", got.action)
	}
	if got.literal != "q" {
		t.Errorf("mapKey('q').literal = %q, want %q", got.literal, "q")
	}
}

// TestMapKey_EscExitsMirrorMode verifies that pressing Esc in mirror mode
// produces actionQuit, transitioning back to the list view.
func TestMapKey_EscExitsMirrorMode(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	got := mapKey(msg)
	if got.action != actionQuit {
		t.Errorf("mapKey(Esc).action = %v, want actionQuit", got.action)
	}
}

// TestMirrorFooter_EscPresentQQuitAbsent verifies the mirror mode footer
// contains "esc" (the advertised exit shortcut) and does NOT contain
// "q quit" (which was removed in v0.0.2 when q became a forwarded key).
func TestMirrorFooter_EscPresentQQuitAbsent(t *testing.T) {
	m := newModel(nil, 0)
	m.mode = modeMirror
	m.mirror = &mirrorState{paneID: "%test"}
	m.width, m.height = 80, 24

	out := strings.ToLower(m.View())

	if strings.Contains(out, "q quit") {
		t.Error(`mirror view contains "q quit"; want it removed (q is forwarded to target pane in v0.0.2)`)
	}
	if !strings.Contains(out, "esc") {
		t.Error(`mirror view does not contain "esc"; want "esc back" or similar footer text`)
	}
}
