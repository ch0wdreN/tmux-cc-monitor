package ui

import (
	"testing"
	"time"

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
