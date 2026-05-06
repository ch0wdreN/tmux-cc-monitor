package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ch0wdreN/tmux-cc-monitor/internal/state"
)

// TestGroupByStatus verifies that groupByStatus puts each status in the correct
// section and sorts within each section by UpdatedAt descending.
func TestGroupByStatus(t *testing.T) {
	now := time.Now()

	older := now.Add(-10 * time.Minute)
	newer := now.Add(-1 * time.Minute)

	states := []*state.State{
		{PaneID: "%1", Status: state.StatusIdle, UpdatedAt: older},
		{PaneID: "%2", Status: state.StatusIdle, UpdatedAt: newer},
		{PaneID: "%3", Status: state.StatusWaitingPermission, UpdatedAt: older},
		{PaneID: "%4", Status: state.StatusWaitingPermission, UpdatedAt: newer},
		{PaneID: "%5", Status: state.StatusRunning, UpdatedAt: older},
		{PaneID: "%6", Status: state.StatusWaitingOther, UpdatedAt: older},
	}

	sections := groupByStatus(states)

	// Section order: WaitingPermission, WaitingOther, Running, Idle
	wantStatuses := []state.Status{
		state.StatusWaitingPermission,
		state.StatusWaitingOther,
		state.StatusRunning,
		state.StatusIdle,
	}
	for i, sec := range sections {
		if sec.status != wantStatuses[i] {
			t.Errorf("sections[%d].status = %q, want %q", i, sec.status, wantStatuses[i])
		}
	}

	// WaitingPermission section: 2 items, sorted newer first
	permSec := sections[0]
	if len(permSec.items) != 2 {
		t.Fatalf("permission section: got %d items, want 2", len(permSec.items))
	}
	if permSec.items[0].PaneID != "%4" {
		t.Errorf("permission[0].PaneID = %q, want %%4 (newer)", permSec.items[0].PaneID)
	}
	if permSec.items[1].PaneID != "%3" {
		t.Errorf("permission[1].PaneID = %q, want %%3 (older)", permSec.items[1].PaneID)
	}

	// WaitingOther section: 1 item
	if len(sections[1].items) != 1 {
		t.Errorf("waiting_other section: got %d items, want 1", len(sections[1].items))
	}
	if sections[1].items[0].PaneID != "%6" {
		t.Errorf("waiting_other[0].PaneID = %q, want %%6", sections[1].items[0].PaneID)
	}

	// Running section: 1 item
	if len(sections[2].items) != 1 {
		t.Errorf("running section: got %d items, want 1", len(sections[2].items))
	}
	if sections[2].items[0].PaneID != "%5" {
		t.Errorf("running[0].PaneID = %q, want %%5", sections[2].items[0].PaneID)
	}

	// Idle section: 2 items, sorted newer first
	idleSec := sections[3]
	if len(idleSec.items) != 2 {
		t.Fatalf("idle section: got %d items, want 2", len(idleSec.items))
	}
	if idleSec.items[0].PaneID != "%2" {
		t.Errorf("idle[0].PaneID = %q, want %%2 (newer)", idleSec.items[0].PaneID)
	}
	if idleSec.items[1].PaneID != "%1" {
		t.Errorf("idle[1].PaneID = %q, want %%1 (older)", idleSec.items[1].PaneID)
	}
}

// TestGroupByStatusEmpty verifies that empty input produces 4 sections each
// with zero items.
func TestGroupByStatusEmpty(t *testing.T) {
	sections := groupByStatus(nil)
	for i, sec := range sections {
		if len(sec.items) != 0 {
			t.Errorf("sections[%d] has %d items, want 0", i, len(sec.items))
		}
	}
}

// TestHumanizeDuration covers the four time bands.
func TestHumanizeDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{1 * time.Second, "1s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{90 * time.Second, "1m"},
		{59*time.Minute + 59*time.Second, "59m"},
		{60 * time.Minute, "1h"},
		{23*time.Hour + 59*time.Minute, "23h"},
		{24 * time.Hour, "1d"},
		{48 * time.Hour, "2d"},
		{-1 * time.Second, "0s"}, // negative clamped to 0
	}
	for _, tc := range tests {
		got := humanizeDuration(tc.d)
		if got != tc.want {
			t.Errorf("humanizeDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// TestTruncateMessage verifies ASCII and multi-byte (Japanese) rune handling.
func TestTruncateMessage(t *testing.T) {
	tests := []struct {
		s    string
		max  int
		want string
	}{
		{"hello", 10, "hello"},            // no truncation needed
		{"hello", 5, "hello"},             // exact fit
		{"hello world", 5, "hell…"},       // truncated at rune boundary
		{"", 5, ""},                        // empty string
		{"abc", 0, ""},                     // max=0 returns empty
		{"abc", 1, "…"},                    // max=1: 0 runes + ellipsis
		{"日本語テスト", 3, "日本…"},         // multi-byte: 2 runes + ellipsis
		{"日本語テスト", 10, "日本語テスト"}, // no truncation
		{"abcde", 4, "abc…"},               // boundary
	}
	for _, tc := range tests {
		got := truncateMessage(tc.s, tc.max)
		if got != tc.want {
			t.Errorf("truncateMessage(%q, %d) = %q, want %q", tc.s, tc.max, got, tc.want)
		}
	}
}

// --- mirror key mapping tests ---

// TestMapKeyReservedKeys verifies that q, Esc, and F1 are NOT forwarded.
func TestMapKeyReservedKeys(t *testing.T) {
	reserved := []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyF1},
	}
	for _, msg := range reserved {
		result := mapKey(msg)
		if result.action == actionSendLiteral || result.action == actionSendKeyName {
			t.Errorf("mapKey(%v): expected not-forwarded, got action=%v keyName=%q literal=%q",
				msg, result.action, result.keyName, result.literal)
		}
	}
}

// TestMapKeyQuitActions verifies that q and Esc produce actionQuit.
func TestMapKeyQuitActions(t *testing.T) {
	tests := []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	}
	for _, msg := range tests {
		result := mapKey(msg)
		if result.action != actionQuit {
			t.Errorf("mapKey(%v): got action=%v, want actionQuit", msg, result.action)
		}
	}
}

// TestMapKeyF1Reserved verifies F1 is reserved (actionReserved, not forwarded).
func TestMapKeyF1Reserved(t *testing.T) {
	result := mapKey(tea.KeyMsg{Type: tea.KeyF1})
	if result.action != actionReserved {
		t.Errorf("mapKey(F1): got action=%v, want actionReserved", result.action)
	}
}

// TestMapKeyValueCollisions is the critical test ensuring that semantic key
// names take priority over their Ctrl* aliases due to switch case ordering.
//
// The aliasing in bubbletea:
//   - KeyEnter == KeyCtrlM (13)
//   - KeyTab   == KeyCtrlI (9)
//   - KeyBackspace == KeyCtrlQuestionMark (127)
//
// mapKey MUST return the semantic key name (Enter/Tab/BSpace), not the
// Ctrl* name (C-m/C-i/C-?), because the semantic KeyType constants are
// evaluated first in the switch.
func TestMapKeyValueCollisions(t *testing.T) {
	tests := []struct {
		name    string
		msg     tea.KeyMsg
		wantKey string
		wantAct keyAction
	}{
		{
			name:    "KeyEnter → Enter (not C-m)",
			msg:     tea.KeyMsg{Type: tea.KeyEnter},
			wantKey: "Enter",
			wantAct: actionSendKeyName,
		},
		{
			name:    "KeyTab → Tab (not C-i)",
			msg:     tea.KeyMsg{Type: tea.KeyTab},
			wantKey: "Tab",
			wantAct: actionSendKeyName,
		},
		{
			name:    "KeyBackspace → BSpace (not C-?)",
			msg:     tea.KeyMsg{Type: tea.KeyBackspace},
			wantKey: "BSpace",
			wantAct: actionSendKeyName,
		},
		{
			name:    "KeyDelete → DC",
			msg:     tea.KeyMsg{Type: tea.KeyDelete},
			wantKey: "DC",
			wantAct: actionSendKeyName,
		},
	}
	for _, tc := range tests {
		result := mapKey(tc.msg)
		if result.action != tc.wantAct {
			t.Errorf("%s: action=%v, want %v", tc.name, result.action, tc.wantAct)
		}
		if result.keyName != tc.wantKey {
			t.Errorf("%s: keyName=%q, want %q", tc.name, result.keyName, tc.wantKey)
		}
	}
}

// TestMapKeyPrintableChars verifies that printable ASCII and UTF-8 chars
// produce actionSendLiteral with the rune string.
func TestMapKeyPrintableChars(t *testing.T) {
	tests := []struct {
		msg     tea.KeyMsg
		wantLit string
	}{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, "a"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Z'}}, "Z"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}, "1"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}, "2"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'あ'}}, "あ"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'🎉'}}, "🎉"},
	}
	for _, tc := range tests {
		result := mapKey(tc.msg)
		if result.action != actionSendLiteral {
			t.Errorf("mapKey(rune %q): action=%v, want actionSendLiteral", tc.wantLit, result.action)
		}
		if result.literal != tc.wantLit {
			t.Errorf("mapKey(rune %q): literal=%q, want %q", tc.wantLit, result.literal, tc.wantLit)
		}
	}
}

// TestMapKeyArrows verifies arrow key names.
func TestMapKeyArrows(t *testing.T) {
	tests := []struct {
		msg     tea.KeyMsg
		wantKey string
	}{
		{tea.KeyMsg{Type: tea.KeyUp}, "Up"},
		{tea.KeyMsg{Type: tea.KeyDown}, "Down"},
		{tea.KeyMsg{Type: tea.KeyLeft}, "Left"},
		{tea.KeyMsg{Type: tea.KeyRight}, "Right"},
	}
	for _, tc := range tests {
		result := mapKey(tc.msg)
		if result.action != actionSendKeyName {
			t.Errorf("mapKey(%v): action=%v, want actionSendKeyName", tc.msg.Type, result.action)
		}
		if result.keyName != tc.wantKey {
			t.Errorf("mapKey(%v): keyName=%q, want %q", tc.msg.Type, result.keyName, tc.wantKey)
		}
	}
}

// TestMapKeyShiftTab verifies Shift+Tab maps to BTab.
func TestMapKeyShiftTab(t *testing.T) {
	result := mapKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	if result.action != actionSendKeyName {
		t.Errorf("mapKey(ShiftTab): action=%v, want actionSendKeyName", result.action)
	}
	if result.keyName != "BTab" {
		t.Errorf("mapKey(ShiftTab): keyName=%q, want BTab", result.keyName)
	}
}

// TestMapKeyCtrlKeys verifies a selection of Ctrl+letter combos.
func TestMapKeyCtrlKeys(t *testing.T) {
	tests := []struct {
		msg     tea.KeyMsg
		wantKey string
	}{
		{tea.KeyMsg{Type: tea.KeyCtrlC}, "C-c"},
		{tea.KeyMsg{Type: tea.KeyCtrlA}, "C-a"},
		{tea.KeyMsg{Type: tea.KeyCtrlZ}, "C-z"},
		{tea.KeyMsg{Type: tea.KeyCtrlU}, "C-u"},
	}
	for _, tc := range tests {
		result := mapKey(tc.msg)
		if result.action != actionSendKeyName {
			t.Errorf("mapKey(%v): action=%v, want actionSendKeyName", tc.msg.Type, result.action)
		}
		if result.keyName != tc.wantKey {
			t.Errorf("mapKey(%v): keyName=%q, want %q", tc.msg.Type, result.keyName, tc.wantKey)
		}
	}
}

// TestMapKeyAltA verifies Alt+a maps to key name "M-a".
func TestMapKeyAltA(t *testing.T) {
	result := mapKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}, Alt: true})
	if result.action != actionSendKeyName {
		t.Errorf("mapKey(alt+a): action=%v, want actionSendKeyName", result.action)
	}
	if result.keyName != "M-a" {
		t.Errorf("mapKey(alt+a): keyName=%q, want M-a", result.keyName)
	}
}

// TestMapKeyAltEnter verifies Alt+Enter maps to "M-Enter".
func TestMapKeyAltEnter(t *testing.T) {
	result := mapKey(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	if result.action != actionSendKeyName {
		t.Errorf("mapKey(alt+enter): action=%v, want actionSendKeyName", result.action)
	}
	if result.keyName != "M-Enter" {
		t.Errorf("mapKey(alt+enter): keyName=%q, want M-Enter", result.keyName)
	}
}

// TestMapKeyShiftArrows verifies Shift+arrow maps to "S-Up" etc.
func TestMapKeyShiftArrows(t *testing.T) {
	tests := []struct {
		msg     tea.KeyMsg
		wantKey string
	}{
		{tea.KeyMsg{Type: tea.KeyShiftUp}, "S-Up"},
		{tea.KeyMsg{Type: tea.KeyShiftDown}, "S-Down"},
		{tea.KeyMsg{Type: tea.KeyShiftLeft}, "S-Left"},
		{tea.KeyMsg{Type: tea.KeyShiftRight}, "S-Right"},
	}
	for _, tc := range tests {
		result := mapKey(tc.msg)
		if result.action != actionSendKeyName {
			t.Errorf("mapKey(%v): action=%v, want actionSendKeyName", tc.msg.Type, result.action)
		}
		if result.keyName != tc.wantKey {
			t.Errorf("mapKey(%v): keyName=%q, want %q", tc.msg.Type, result.keyName, tc.wantKey)
		}
	}
}

// TestMapKeyF13Drop verifies that F13 (no tmux name) produces actionDrop with
// a non-empty warnMsg.
func TestMapKeyF13Drop(t *testing.T) {
	result := mapKey(tea.KeyMsg{Type: tea.KeyF13})
	if result.action != actionDrop {
		t.Errorf("mapKey(F13): action=%v, want actionDrop", result.action)
	}
	if result.warnMsg == "" {
		t.Error("mapKey(F13): expected non-empty warnMsg for dropped key")
	}
}

// TestMapKeyF20Drop verifies that F20 (no tmux name) produces actionDrop.
func TestMapKeyF20Drop(t *testing.T) {
	result := mapKey(tea.KeyMsg{Type: tea.KeyF20})
	if result.action != actionDrop {
		t.Errorf("mapKey(F20): action=%v, want actionDrop", result.action)
	}
}

// TestMapKeyCtrlPgUpDrop verifies that KeyCtrlPgUp (no tmux name) produces actionDrop.
func TestMapKeyCtrlPgUpDrop(t *testing.T) {
	result := mapKey(tea.KeyMsg{Type: tea.KeyCtrlPgUp})
	if result.action != actionDrop {
		t.Errorf("mapKey(CtrlPgUp): action=%v, want actionDrop", result.action)
	}
}

// TestMapKeyCtrlPgDownDrop verifies that KeyCtrlPgDown (no tmux name) produces actionDrop.
func TestMapKeyCtrlPgDownDrop(t *testing.T) {
	result := mapKey(tea.KeyMsg{Type: tea.KeyCtrlPgDown})
	if result.action != actionDrop {
		t.Errorf("mapKey(CtrlPgDown): action=%v, want actionDrop", result.action)
	}
}

// TestMapKeyPasteSendLiteral verifies that paste events are sent as literal text.
func TestMapKeyPasteSendLiteral(t *testing.T) {
	result := mapKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello world"), Paste: true})
	if result.action != actionSendLiteral {
		t.Errorf("mapKey(paste): action=%v, want actionSendLiteral", result.action)
	}
	if result.literal != "hello world" {
		t.Errorf("mapKey(paste): literal=%q, want %q", result.literal, "hello world")
	}
}

// TestMapKeySpace verifies KeySpace produces actionSendLiteral with a space.
func TestMapKeySpace(t *testing.T) {
	result := mapKey(tea.KeyMsg{Type: tea.KeySpace})
	if result.action != actionSendLiteral {
		t.Errorf("mapKey(space): action=%v, want actionSendLiteral", result.action)
	}
	if result.literal != " " {
		t.Errorf("mapKey(space): literal=%q, want \" \"", result.literal)
	}
}

// TestMapKeyFunctionKeys verifies F2-F12 produce actionSendKeyName.
func TestMapKeyFunctionKeys(t *testing.T) {
	tests := []struct {
		keyType tea.KeyType
		want    string
	}{
		{tea.KeyF2, "F2"},
		{tea.KeyF3, "F3"},
		{tea.KeyF4, "F4"},
		{tea.KeyF5, "F5"},
		{tea.KeyF6, "F6"},
		{tea.KeyF7, "F7"},
		{tea.KeyF8, "F8"},
		{tea.KeyF9, "F9"},
		{tea.KeyF10, "F10"},
		{tea.KeyF11, "F11"},
		{tea.KeyF12, "F12"},
	}
	for _, tc := range tests {
		result := mapKey(tea.KeyMsg{Type: tc.keyType})
		if result.action != actionSendKeyName {
			t.Errorf("mapKey(F%v): action=%v, want actionSendKeyName", tc.want, result.action)
		}
		if result.keyName != tc.want {
			t.Errorf("mapKey(F%v): keyName=%q, want %q", tc.want, result.keyName, tc.want)
		}
	}
}

// TestMapKeyPgUpPgDown verifies PgUp/PgDown map to tmux names PPage/NPage.
func TestMapKeyPgUpPgDown(t *testing.T) {
	tests := []struct {
		msg     tea.KeyMsg
		wantKey string
	}{
		{tea.KeyMsg{Type: tea.KeyPgUp}, "PPage"},
		{tea.KeyMsg{Type: tea.KeyPgDown}, "NPage"},
	}
	for _, tc := range tests {
		result := mapKey(tc.msg)
		if result.action != actionSendKeyName {
			t.Errorf("mapKey(%v): action=%v, want actionSendKeyName", tc.msg.Type, result.action)
		}
		if result.keyName != tc.wantKey {
			t.Errorf("mapKey(%v): keyName=%q, want %q", tc.msg.Type, result.keyName, tc.wantKey)
		}
	}
}

// TestMapKeySwitchOrderGuarantee directly verifies that the switch ordering rule
// is enforced: KeyEnter/KeyTab/KeyBackspace/KeyDelete each produce their semantic
// name, not the Ctrl* alias name.
//
// This is the critical regression guard for the value-collision bug described
// in the Design Doc §6.3 "値衝突対策".
func TestMapKeySwitchOrderGuarantee(t *testing.T) {
	// These four have Ctrl* aliases with identical integer values.
	// The switch must resolve them as the semantic names.
	table := []struct {
		desc        string
		msg         tea.KeyMsg
		wantKeyName string
		ctrlAlias   string // what we must NOT produce
	}{
		{
			desc:        "KeyEnter (==KeyCtrlM) → Enter, not C-m",
			msg:         tea.KeyMsg{Type: tea.KeyEnter},
			wantKeyName: "Enter",
			ctrlAlias:   "C-m",
		},
		{
			desc:        "KeyTab (==KeyCtrlI) → Tab, not C-i",
			msg:         tea.KeyMsg{Type: tea.KeyTab},
			wantKeyName: "Tab",
			ctrlAlias:   "C-i",
		},
		{
			desc:        "KeyBackspace (==KeyCtrlQuestionMark, 127) → BSpace, not C-?",
			msg:         tea.KeyMsg{Type: tea.KeyBackspace},
			wantKeyName: "BSpace",
			ctrlAlias:   "C-?",
		},
		{
			desc:        "KeyDelete → DC",
			msg:         tea.KeyMsg{Type: tea.KeyDelete},
			wantKeyName: "DC",
			ctrlAlias:   "", // no alias clash for Delete
		},
	}

	for _, tc := range table {
		result := mapKey(tc.msg)
		if result.action != actionSendKeyName {
			t.Errorf("%s: action=%v, want actionSendKeyName", tc.desc, result.action)
			continue
		}
		if result.keyName != tc.wantKeyName {
			t.Errorf("%s: keyName=%q, want %q", tc.desc, result.keyName, tc.wantKeyName)
		}
		if tc.ctrlAlias != "" && result.keyName == tc.ctrlAlias {
			t.Errorf("%s: got ctrl alias %q, must not produce Ctrl* alias", tc.desc, tc.ctrlAlias)
		}
	}
}

// TestMirrorQuitTriggersReload verifies that exiting mirror mode via q/Esc both
// transitions to modeList and fires a non-nil reload Cmd, so the list view is
// refreshed before being shown again.
func TestMirrorQuitTriggersReload(t *testing.T) {
	cases := []struct {
		desc string
		msg  tea.KeyMsg
	}{
		{"q exits mirror with reload", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}},
		{"Esc exits mirror with reload", tea.KeyMsg{Type: tea.KeyEsc}},
	}

	for _, tc := range cases {
		m := model{
			mode:   modeMirror,
			mirror: &mirrorState{paneID: "%42"},
			width:  80,
			height: 24,
		}

		next, cmd := m.updateMirror(tc.msg)
		nm, ok := next.(model)
		if !ok {
			t.Fatalf("%s: returned model is not of type model", tc.desc)
		}
		if nm.mode != modeList {
			t.Errorf("%s: mode=%v, want modeList", tc.desc, nm.mode)
		}
		if nm.mirror != nil {
			t.Errorf("%s: mirror state must be cleared, got %+v", tc.desc, nm.mirror)
		}
		if cmd == nil {
			t.Errorf("%s: expected non-nil reload Cmd, got nil", tc.desc)
		}
	}
}
