package ui

// mirror_test.go covers the mirror-mode key mapping acceptance criteria:
//
//   - mapKey('q') returns actionSendLiteral so q is forwarded to the target pane.
//   - mapKey(Esc) returns actionSendKeyName("Escape") so Esc is forwarded to the target pane.
//   - mapKey(Ctrl+G) returns actionQuit to exit mirror mode back to the list.
//   - viewMirror output contains "ctrl-g" and does NOT contain "esc back".

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

// TestMapKey_EscForwardedToTargetPane verifies that pressing Esc in mirror mode
// produces actionSendKeyName("Escape"), forwarding Esc to the target pane rather
// than exiting mirror mode. Claude Code uses Esc as an interrupt signal, so mirror
// must not intercept it.
func TestMapKey_EscForwardedToTargetPane(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	got := mapKey(msg)
	if got.action != actionSendKeyName {
		t.Errorf("mapKey(Esc).action = %v, want actionSendKeyName", got.action)
	}
	if got.keyName != "Escape" {
		t.Errorf("mapKey(Esc).keyName = %q, want %q", got.keyName, "Escape")
	}
}

// TestMapKey_CtrlGExitsMirrorMode verifies that pressing Ctrl+G in mirror mode
// produces actionQuit, transitioning back to the list view.
func TestMapKey_CtrlGExitsMirrorMode(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyCtrlG}
	got := mapKey(msg)
	if got.action != actionQuit {
		t.Errorf("mapKey(Ctrl+G).action = %v, want actionQuit", got.action)
	}
}

// TestMirrorFooter_CtrlGPresent verifies the mirror mode footer contains "ctrl-g"
// (the advertised exit shortcut) and does NOT contain "esc back" (Esc is now
// forwarded to the target pane, so it must not appear as an exit hint).
func TestMirrorFooter_CtrlGPresent(t *testing.T) {
	m := newModel(nil, 0)
	m.mode = modeMirror
	m.mirror = &mirrorState{paneID: "%test"}
	m.width, m.height = 80, 24

	out := strings.ToLower(m.View())

	if !strings.Contains(out, "ctrl-g") {
		t.Error(`mirror view does not contain "ctrl-g"; want "ctrl-g → list" or similar footer text`)
	}
	if strings.Contains(out, "esc back") {
		t.Error(`mirror view contains "esc back"; want it removed (esc is forwarded to target pane)`)
	}
}
