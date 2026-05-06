package ui

import (
	"strings"
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
		{PaneID: "%3", Status: state.StatusWaitingAction, UpdatedAt: older},
		{PaneID: "%4", Status: state.StatusWaitingAction, UpdatedAt: newer},
		{PaneID: "%5", Status: state.StatusRunning, UpdatedAt: older},
		{PaneID: "%6", Status: state.StatusWaitingOther, UpdatedAt: older},
	}

	sections := groupByStatus(states)

	// Section order: WaitingAction, WaitingOther, Running, Idle
	wantStatuses := []state.Status{
		state.StatusWaitingAction,
		state.StatusWaitingOther,
		state.StatusRunning,
		state.StatusIdle,
	}
	for i, sec := range sections {
		if sec.status != wantStatuses[i] {
			t.Errorf("sections[%d].status = %q, want %q", i, sec.status, wantStatuses[i])
		}
	}

	// WaitingAction section: 2 items, sorted newer first
	permSec := sections[0]
	if len(permSec.items) != 2 {
		t.Fatalf("action section: got %d items, want 2", len(permSec.items))
	}
	if permSec.items[0].PaneID != "%4" {
		t.Errorf("action[0].PaneID = %q, want %%4 (newer)", permSec.items[0].PaneID)
	}
	if permSec.items[1].PaneID != "%3" {
		t.Errorf("action[1].PaneID = %q, want %%3 (older)", permSec.items[1].PaneID)
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

// TestHumanizeDuration covers the four time bands with boundary values.
func TestHumanizeDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "<1m"},
		{"30s", 30 * time.Second, "<1m"},
		{"59s", 59 * time.Second, "<1m"},
		{"1m", time.Minute, "1m"},
		{"59m", 59 * time.Minute, "59m"},
		{"1h", time.Hour, "1h"},
		{"23h", 23 * time.Hour, "23h"},
		{"24h", 24 * time.Hour, "1d"},
		{"negative", -time.Second, "<1m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := humanizeDuration(tt.d); got != tt.want {
				t.Errorf("humanizeDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
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

// TestMapKeyReservedKeys verifies that Ctrl+G and F1 are NOT forwarded to the
// target pane. Note: both q and Esc are intentionally NOT in this list —
// they are forwarded to the target pane (q as actionSendLiteral, Esc as
// actionSendKeyName("Escape")).
func TestMapKeyReservedKeys(t *testing.T) {
	reserved := []tea.KeyMsg{
		{Type: tea.KeyCtrlG},
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

// TestMapKeyQuitActions verifies that Ctrl+G produces actionQuit.
// Note: q and Esc are NOT in this list — both are forwarded to the target pane
// (q as actionSendLiteral, Esc as actionSendKeyName("Escape")). Only Ctrl+G
// exits mirror mode.
func TestMapKeyQuitActions(t *testing.T) {
	tests := []tea.KeyMsg{
		{Type: tea.KeyCtrlG},
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

// mirrorModel returns a model already in mirror mode targeting the given pane.
// All Update tests for mirror-mode messages start from this state.
func mirrorModel(paneID string) model {
	return model{
		mode:   modeMirror,
		mirror: &mirrorState{paneID: paneID},
		width:  80,
		height: 24,
	}
}

// TestUpdateWindowSizeInMirrorTriggersRecapture verifies that a resize event
// in mirror mode produces a non-nil Cmd (which schedules a fresh capture-pane
// against the new dimensions) and that the model fields are updated.
func TestUpdateWindowSizeInMirrorTriggersRecapture(t *testing.T) {
	m := mirrorModel("%42")
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	nm := next.(model)

	if nm.width != 100 || nm.height != 40 {
		t.Errorf("dimensions not updated: width=%d height=%d, want 100/40", nm.width, nm.height)
	}
	if cmd == nil {
		t.Error("expected non-nil capture Cmd after resize in mirror mode")
	}
}

// TestUpdateWindowSizeInListIsNoCmd verifies that resizes in list mode do not
// launch any capture work — the next View call repaints with new dimensions.
func TestUpdateWindowSizeInListIsNoCmd(t *testing.T) {
	m := model{mode: modeList, width: 80, height: 24}
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	nm := next.(model)

	if nm.width != 120 || nm.height != 50 {
		t.Errorf("dimensions not updated: width=%d height=%d, want 120/50", nm.width, nm.height)
	}
	if cmd != nil {
		t.Error("expected nil Cmd in list-mode resize, got non-nil")
	}
}

// TestUpdatePaneAliveDeadShowsBannerAndSchedulesReturn verifies that when the
// PaneAlive check reports false, the banner is set and a return-to-list Cmd
// is scheduled.
func TestUpdatePaneAliveDeadShowsBannerAndSchedulesReturn(t *testing.T) {
	m := mirrorModel("%42")
	next, cmd := m.Update(paneAliveResultMsg{alive: false})
	nm := next.(model)

	if nm.mirror == nil {
		t.Fatal("mirror state was cleared too early; expected dead-banner phase")
	}
	if !nm.mirror.deadBanner {
		t.Error("deadBanner not set on alive=false")
	}
	if cmd == nil {
		t.Error("expected non-nil return-to-list Cmd on alive=false")
	}
}

// TestUpdatePaneAliveTrueIsNoOp verifies that PaneAlive=true does not change
// model state or fire any Cmd — the next regular tick handles the next capture.
func TestUpdatePaneAliveTrueIsNoOp(t *testing.T) {
	m := mirrorModel("%42")
	m.mirror.deadBanner = false

	next, cmd := m.Update(paneAliveResultMsg{alive: true})
	nm := next.(model)

	if nm.mirror.deadBanner {
		t.Error("deadBanner must remain false on alive=true")
	}
	if cmd != nil {
		t.Error("expected nil Cmd on alive=true; tick handles next capture")
	}
}

// TestUpdateReturnToListClearsMirror verifies that the post-banner timeout
// transitions out of mirror mode and clears mirror state.
func TestUpdateReturnToListClearsMirror(t *testing.T) {
	m := mirrorModel("%42")
	m.mirror.deadBanner = true

	next, cmd := m.Update(returnToListMsg{})
	nm := next.(model)

	if nm.mode != modeList {
		t.Errorf("mode=%v, want modeList", nm.mode)
	}
	if nm.mirror != nil {
		t.Errorf("mirror state must be cleared, got %+v", nm.mirror)
	}
	if cmd != nil {
		t.Error("returnToListMsg must not fire any further Cmd")
	}
}

// TestUpdateMirrorTickInMirrorReschedules verifies that a tick in mirror mode
// returns a non-nil Cmd (which is tea.Batch of a re-tick + a re-capture).
func TestUpdateMirrorTickInMirrorReschedules(t *testing.T) {
	m := mirrorModel("%42")
	_, cmd := m.Update(mirrorTickMsg{})
	if cmd == nil {
		t.Error("expected non-nil Cmd (Batch of next tick + capture) in mirror mode")
	}
}

// TestUpdateMirrorTickInListIsNoCmd verifies that an in-flight tick that
// arrives after the user already returned to list mode does not reschedule
// itself — the tick chain must stop when modeMirror exits.
func TestUpdateMirrorTickInListIsNoCmd(t *testing.T) {
	m := model{mode: modeList, width: 80, height: 24}
	_, cmd := m.Update(mirrorTickMsg{})
	if cmd != nil {
		t.Error("tick after exit must not produce a Cmd; tick chain leaked")
	}
}

// TestUpdateCaptureResultErrorChecksPaneAlive verifies that a capture failure
// triggers a PaneAlive check, which is how mirror discovers a dead pane.
func TestUpdateCaptureResultErrorChecksPaneAlive(t *testing.T) {
	m := mirrorModel("%42")
	_, cmd := m.Update(captureResultMsg{err: errCapture("boom")})
	if cmd == nil {
		t.Error("expected non-nil PaneAlive Cmd after capture error")
	}
}

// TestUpdateCaptureResultSuccessUpdatesContent verifies that a successful
// capture updates mirror.content and clears any prior warnMsg.
func TestUpdateCaptureResultSuccessUpdatesContent(t *testing.T) {
	m := mirrorModel("%42")
	m.mirror.warnMsg = "stale warn"

	next, cmd := m.Update(captureResultMsg{content: "hello world"})
	nm := next.(model)

	if nm.mirror.content != "hello world" {
		t.Errorf("content=%q, want %q", nm.mirror.content, "hello world")
	}
	if nm.mirror.warnMsg != "" {
		t.Errorf("warnMsg should be cleared after successful capture, got %q", nm.mirror.warnMsg)
	}
	if cmd != nil {
		t.Error("successful capture must not fire any further Cmd")
	}
}

// TestUpdateSendErrSetsBanner verifies that a send-keys failure surfaces in
// the mirror's sendErr field for the View to render.
func TestUpdateSendErrSetsBanner(t *testing.T) {
	m := mirrorModel("%42")
	next, _ := m.Update(sendErrMsg("send failed: timeout"))
	nm := next.(model)

	if nm.mirror.sendErr != "send failed: timeout" {
		t.Errorf("sendErr=%q, want %q", nm.mirror.sendErr, "send failed: timeout")
	}
}

// TestUpdateMirrorSendLiteralReturnsCmd verifies that a printable-rune key in
// mirror mode produces a Cmd that will run SendLiteral + re-capture.
func TestUpdateMirrorSendLiteralReturnsCmd(t *testing.T) {
	m := mirrorModel("%42")
	next, cmd := m.updateMirror(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	nm := next.(model)

	if nm.mode != modeMirror {
		t.Errorf("mode=%v, want modeMirror (printable rune must not exit mirror)", nm.mode)
	}
	if cmd == nil {
		t.Error("expected non-nil Cmd for actionSendLiteral")
	}
}

// TestUpdateMirrorSendKeyNameReturnsCmd verifies that a key-name key (e.g. Up)
// produces a Cmd that will run SendKeyName + re-capture.
func TestUpdateMirrorSendKeyNameReturnsCmd(t *testing.T) {
	m := mirrorModel("%42")
	next, cmd := m.updateMirror(tea.KeyMsg{Type: tea.KeyUp})
	nm := next.(model)

	if nm.mode != modeMirror {
		t.Errorf("mode=%v, want modeMirror (Up must not exit mirror)", nm.mode)
	}
	if cmd == nil {
		t.Error("expected non-nil Cmd for actionSendKeyName")
	}
}

// TestUpdateMirrorDropSetsWarn verifies that a dropped key (e.g. F13) sets a
// non-empty warnMsg in mirror state and returns no Cmd.
func TestUpdateMirrorDropSetsWarn(t *testing.T) {
	m := mirrorModel("%42")
	next, cmd := m.updateMirror(tea.KeyMsg{Type: tea.KeyF13})
	nm := next.(model)

	if nm.mirror.warnMsg == "" {
		t.Error("expected warnMsg to be set after F13 drop")
	}
	if cmd != nil {
		t.Error("dropped key must not fire any Cmd")
	}
}

// TestViewList_RunningHeaderUsesGreenStyle verifies that viewList renders the
// "Running" section header using styleSectionRunning (bright green + bold).
func TestViewList_RunningHeaderUsesGreenStyle(t *testing.T) {
	now := time.Now()
	states := []*state.State{
		{
			PaneID:    "%10",
			Status:    state.StatusRunning,
			CWD:       "/home/user/project",
			UpdatedAt: now.Add(-2 * time.Minute),
		},
	}
	m := newModel(states, 0)
	m.width = 120
	m.height = 40

	output := m.viewList()

	want := styleSectionRunning.Render("── Running ──")
	if !strings.Contains(output, want) {
		t.Errorf("viewList() output does not contain styleSectionRunning-rendered header.\nwant substring: %q\ngot: %q", want, output)
	}
}

// TestGroupByStatus_ActionWaitingTitle verifies that sections[0].title is
// "Action Waiting".
func TestGroupByStatus_ActionWaitingTitle(t *testing.T) {
	sections := groupByStatus(nil)
	if sections[0].title != "Action Waiting" {
		t.Errorf("sections[0].title = %q, want %q", sections[0].title, "Action Waiting")
	}
}

// TestUpdate_RedrawTickMsg verifies that Update(redrawTickMsg{}) returns the
// model unchanged and a non-nil Cmd (the next scheduled tick).
func TestUpdate_RedrawTickMsg(t *testing.T) {
	m := newModel(nil, 0)
	m.cursor = 0

	newM, cmd := m.Update(redrawTickMsg{})

	nm, ok := newM.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", newM)
	}
	if nm.cursor != m.cursor {
		t.Errorf("cursor changed: got %d, want %d", nm.cursor, m.cursor)
	}
	if cmd == nil {
		t.Error("Update(redrawTickMsg) returned nil cmd, want next tick")
	}
}

// TestModel_InitSchedulesRedrawTick verifies that Init returns a non-nil Cmd
// (tea.Batch containing at minimum scheduleRedrawTick).
func TestModel_InitSchedulesRedrawTick(t *testing.T) {
	m := newModel(nil, 0)
	if cmd := m.Init(); cmd == nil {
		t.Error("Init returned nil cmd, want batch with redraw tick")
	}
}

// errCapture is a small typed error for TestUpdateCaptureResultErrorChecksPaneAlive.
type errCapture string

func (e errCapture) Error() string { return string(e) }

// TestMirrorQuitTriggersReload verifies that exiting mirror mode via Ctrl+G
// transitions to modeList and fires a non-nil reload Cmd, so the list view is
// refreshed before being shown again.
// Note: q and Esc no longer exit mirror mode; both are forwarded to the target pane.
func TestMirrorQuitTriggersReload(t *testing.T) {
	cases := []struct {
		desc string
		msg  tea.KeyMsg
	}{
		{"Ctrl+G exits mirror with reload", tea.KeyMsg{Type: tea.KeyCtrlG}},
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
