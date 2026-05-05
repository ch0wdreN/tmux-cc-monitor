package cleanup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch0wdreN/tmux-cc-monitor/internal/state"
)

// isZeroResult reports whether r is equivalent to a zero-value Result.
// Result contains a []string field so it is not directly comparable with ==.
func isZeroResult(r Result) bool {
	return r.DeletedServerMismatch == 0 &&
		r.DeletedDeadPane == 0 &&
		r.Skipped == 0 &&
		len(r.Errors) == 0
}

// writeState writes a State to the sessions dir as determined by
// state.SessionsDir() (which honors XDG_CONFIG_HOME set via t.Setenv).
func writeState(t *testing.T, s *state.State) string {
	t.Helper()
	if err := state.WriteAtomic(s); err != nil {
		t.Fatalf("writeState: WriteAtomic: %v", err)
	}
	dir, err := state.SessionsDir()
	if err != nil {
		t.Fatalf("writeState: SessionsDir: %v", err)
	}
	// PathFor strips the leading '%' and appends .json.
	paneDigits := s.PaneID[1:]
	return filepath.Join(dir, paneDigits+".json")
}

func baseState(paneID string, pid int) *state.State {
	return &state.State{
		SchemaVersion: state.SchemaVersion,
		TmuxServerPID: pid,
		PaneID:        paneID,
		Session:       "main",
		WindowIndex:   0,
		WindowName:    "editor",
		CWD:           "/tmp",
		Status:        state.StatusIdle,
		LastEvent:     "Stop",
		UpdatedAt:     time.Now(),
	}
}

// TestRun_DeletesServerMismatch: state written with PID 999; run with PID 1234.
// File must be deleted; Result.DeletedServerMismatch == 1.
func TestRun_DeletesServerMismatch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	s := baseState("%1", 999)
	fpath := writeState(t, s)

	result, err := Run(1234, map[string]bool{})
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if result.DeletedServerMismatch != 1 {
		t.Errorf("DeletedServerMismatch = %d, want 1", result.DeletedServerMismatch)
	}
	if result.DeletedDeadPane != 0 || result.Skipped != 0 {
		t.Errorf("unexpected counters: %+v", result)
	}
	if _, statErr := os.Stat(fpath); !os.IsNotExist(statErr) {
		t.Errorf("file %q should have been deleted, but still exists", fpath)
	}
}

// TestRun_KeepsLivePane: state with matching PID and pane in livePaneIDs.
// File must remain; all counters zero.
func TestRun_KeepsLivePane(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	s := baseState("%5", 1234)
	fpath := writeState(t, s)

	result, err := Run(1234, map[string]bool{"%5": true})
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if result.DeletedServerMismatch != 0 || result.DeletedDeadPane != 0 || result.Skipped != 0 {
		t.Errorf("unexpected counters: %+v", result)
	}
	if _, statErr := os.Stat(fpath); statErr != nil {
		t.Errorf("file %q should still exist: %v", fpath, statErr)
	}
}

// TestRun_DeletesDeadPaneAfterThreshold: state with old UpdatedAt for pane
// not in livePaneIDs; file must be deleted.
func TestRun_DeletesDeadPaneAfterThreshold(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Override threshold so we can control the window precisely.
	orig := StaleThreshold
	StaleThreshold = 5 * time.Second
	t.Cleanup(func() { StaleThreshold = orig })

	s := baseState("%99", 1234)
	s.UpdatedAt = time.Now().Add(-10 * time.Second) // 10s ago > 5s threshold
	fpath := writeState(t, s)

	result, err := Run(1234, map[string]bool{})
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if result.DeletedDeadPane != 1 {
		t.Errorf("DeletedDeadPane = %d, want 1", result.DeletedDeadPane)
	}
	if result.DeletedServerMismatch != 0 || result.Skipped != 0 {
		t.Errorf("unexpected counters: %+v", result)
	}
	if _, statErr := os.Stat(fpath); !os.IsNotExist(statErr) {
		t.Errorf("file %q should have been deleted, but still exists", fpath)
	}
}

// TestRun_KeepsRecentDeadPaneFile: state written just now for a dead pane;
// must NOT be deleted (within race window). Result.Skipped == 1.
func TestRun_KeepsRecentDeadPaneFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	orig := StaleThreshold
	StaleThreshold = 5 * time.Second
	t.Cleanup(func() { StaleThreshold = orig })

	s := baseState("%99", 1234)
	s.UpdatedAt = time.Now() // right now — well within threshold
	fpath := writeState(t, s)

	result, err := Run(1234, map[string]bool{})
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
	if result.DeletedServerMismatch != 0 || result.DeletedDeadPane != 0 {
		t.Errorf("unexpected counters: %+v", result)
	}
	if _, statErr := os.Stat(fpath); statErr != nil {
		t.Errorf("file %q should still exist: %v", fpath, statErr)
	}
}

// TestRun_LeavesUnparseableFiles: malformed JSON in the sessions dir must be
// left alone and must not appear in any counter.
func TestRun_LeavesUnparseableFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir, err := state.SessionsDir()
	if err != nil {
		t.Fatalf("SessionsDir: %v", err)
	}
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		t.Fatalf("MkdirAll: %v", mkErr)
	}

	bad := filepath.Join(dir, "bad.json")
	if writeErr := os.WriteFile(bad, []byte("{not valid json"), 0o600); writeErr != nil {
		t.Fatalf("write bad file: %v", writeErr)
	}

	result, err := Run(1234, map[string]bool{})
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if result.DeletedServerMismatch != 0 || result.DeletedDeadPane != 0 ||
		result.Skipped != 0 || len(result.Errors) != 0 {
		t.Errorf("unexpected result for unparseable file: %+v", result)
	}
	if _, statErr := os.Stat(bad); statErr != nil {
		t.Errorf("bad file %q should still exist: %v", bad, statErr)
	}
}

// TestRun_MissingDir: if the sessions directory does not exist, Run must
// return a zero-value Result and nil error.
func TestRun_MissingDir(t *testing.T) {
	// Point XDG_CONFIG_HOME at a directory that does not have a
	// tmux-cc-monitor/sessions subdirectory.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	result, err := Run(1234, map[string]bool{})
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if !isZeroResult(result) {
		t.Errorf("expected zero-value Result for missing dir, got %+v", result)
	}
}

// TestRun_WrongSchemaVersionLeftAlone: a valid JSON file with an unexpected
// schema_version must be left alone and not counted.
func TestRun_WrongSchemaVersionLeftAlone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir, err := state.SessionsDir()
	if err != nil {
		t.Fatalf("SessionsDir: %v", err)
	}
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		t.Fatalf("MkdirAll: %v", mkErr)
	}

	// Write a syntactically valid JSON with schema_version=99 (unknown).
	future := map[string]any{
		"schema_version":  99,
		"tmux_server_pid": 1234,
		"pane_id":         "%7",
		"updated_at":      time.Now().Add(-60 * time.Second),
	}
	data, _ := json.Marshal(future)
	fpath := filepath.Join(dir, "7.json")
	if writeErr := os.WriteFile(fpath, data, 0o600); writeErr != nil {
		t.Fatalf("write future schema file: %v", writeErr)
	}

	result, err := Run(1234, map[string]bool{})
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if !isZeroResult(result) {
		t.Errorf("expected zero-value Result for unknown schema, got %+v", result)
	}
	if _, statErr := os.Stat(fpath); statErr != nil {
		t.Errorf("file %q should still exist: %v", fpath, statErr)
	}
}
