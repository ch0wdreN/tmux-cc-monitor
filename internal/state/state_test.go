package state_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ch0wdreN/tmux-cc-monitor/internal/state"
)

// sampleState returns a deterministic State for use in tests.
func sampleState(paneID string) *state.State {
	return &state.State{
		SchemaVersion: state.SchemaVersion,
		TmuxServerPID: 12345,
		PaneID:        paneID,
		Session:       "work",
		WindowIndex:   1,
		WindowName:    "api",
		CWD:           "/tmp/proj",
		Status:        state.StatusRunning,
		LastEvent:     "UserPromptSubmit",
		LastMessage:   "hello",
		UpdatedAt:     time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC),
	}
}

// TestWriteAtomic_RoundTrip writes a State and reads it back via ReadAll,
// verifying field-level equality.
func TestWriteAtomic_RoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	want := sampleState("%42")
	if err := state.WriteAtomic(want); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	states, warnings, err := state.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(states) != 1 {
		t.Fatalf("ReadAll returned %d states, want 1", len(states))
	}

	got := states[0]
	if got.PaneID != want.PaneID {
		t.Errorf("PaneID: got %q, want %q", got.PaneID, want.PaneID)
	}
	if got.SchemaVersion != want.SchemaVersion {
		t.Errorf("SchemaVersion: got %d, want %d", got.SchemaVersion, want.SchemaVersion)
	}
	if got.TmuxServerPID != want.TmuxServerPID {
		t.Errorf("TmuxServerPID: got %d, want %d", got.TmuxServerPID, want.TmuxServerPID)
	}
	if got.Session != want.Session {
		t.Errorf("Session: got %q, want %q", got.Session, want.Session)
	}
	if got.WindowIndex != want.WindowIndex {
		t.Errorf("WindowIndex: got %d, want %d", got.WindowIndex, want.WindowIndex)
	}
	if got.WindowName != want.WindowName {
		t.Errorf("WindowName: got %q, want %q", got.WindowName, want.WindowName)
	}
	if got.CWD != want.CWD {
		t.Errorf("CWD: got %q, want %q", got.CWD, want.CWD)
	}
	if got.Status != want.Status {
		t.Errorf("Status: got %q, want %q", got.Status, want.Status)
	}
	if got.LastEvent != want.LastEvent {
		t.Errorf("LastEvent: got %q, want %q", got.LastEvent, want.LastEvent)
	}
	if got.LastMessage != want.LastMessage {
		t.Errorf("LastMessage: got %q, want %q", got.LastMessage, want.LastMessage)
	}
	if !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("UpdatedAt: got %v, want %v", got.UpdatedAt, want.UpdatedAt)
	}
}

// TestWriteAtomic_FileMode verifies that written state files have mode 0600.
func TestWriteAtomic_FileMode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := state.WriteAtomic(sampleState("%7")); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	dir, err := state.SessionsDir()
	if err != nil {
		t.Fatalf("SessionsDir: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "7.json"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode: got %04o, want 0600", perm)
	}
}

// TestWriteAtomic_DirMode verifies that SessionsDir is created with mode 0700.
func TestWriteAtomic_DirMode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := state.WriteAtomic(sampleState("%7")); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	dir, err := state.SessionsDir()
	if err != nil {
		t.Fatalf("SessionsDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat sessions dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("sessions dir mode: got %04o, want 0700", perm)
	}
}

// TestReadAll_SkipsUnknownSchemaVersion verifies that a file with an unknown
// schema_version is skipped and a warning is returned.
func TestReadAll_SkipsUnknownSchemaVersion(t *testing.T) {
	tmpXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpXDG)

	dir, err := state.SessionsDir()
	if err != nil {
		t.Fatalf("SessionsDir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	bad := map[string]any{
		"schema_version":  999,
		"tmux_server_pid": 1,
		"pane_id":         "%1",
		"status":          "idle",
		"updated_at":      "2026-05-06T10:00:00Z",
	}
	data, _ := json.Marshal(bad)
	if err := os.WriteFile(filepath.Join(dir, "1.json"), data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	states, warnings, err := state.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(states) != 0 {
		t.Errorf("expected 0 states, got %d", len(states))
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

// TestReadAll_SkipsMalformedJSON verifies that a non-JSON file is skipped with
// a warning.
func TestReadAll_SkipsMalformedJSON(t *testing.T) {
	tmpXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpXDG)

	dir, err := state.SessionsDir()
	if err != nil {
		t.Fatalf("SessionsDir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "2.json"), []byte("not json {{"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	states, warnings, err := state.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(states) != 0 {
		t.Errorf("expected 0 states, got %d", len(states))
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

// TestPathFor is a table-driven test for PathFor.
func TestPathFor(t *testing.T) {
	// Use a fixed temp dir so we can inspect the returned path.
	tmpXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpXDG)

	dir, err := state.SessionsDir()
	if err != nil {
		t.Fatalf("SessionsDir: %v", err)
	}

	cases := []struct {
		paneID  string
		wantSuf string // expected file base name (empty = error expected)
		wantErr bool
	}{
		{"%42", "42.json", false},
		{"%0", "0.json", false},
		{"%100", "100.json", false},
		// error cases
		{"42", "", true},    // missing leading %
		{"%", "", true},     // no numeric part
		{"%abc", "", true},  // non-digit after %
		{"", "", true},      // empty string
		{"%12a", "", true},  // digits followed by letter
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.paneID, func(t *testing.T) {
			got, err := state.PathFor(tc.paneID)
			if tc.wantErr {
				if err == nil {
					t.Errorf("PathFor(%q) = %q, nil; want error", tc.paneID, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("PathFor(%q): %v", tc.paneID, err)
			}
			want := filepath.Join(dir, tc.wantSuf)
			if got != want {
				t.Errorf("PathFor(%q) = %q, want %q", tc.paneID, got, want)
			}
		})
	}
}

// TestReadAll_MissingDir verifies that ReadAll returns empty results (no error)
// when SessionsDir does not exist.
func TestReadAll_MissingDir(t *testing.T) {
	// Point XDG_CONFIG_HOME at a directory that doesn't contain our subdir.
	tmpXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpXDG)
	// sessions/ subdir is never created, so ReadAll should see a missing dir.

	states, warnings, err := state.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll returned unexpected error: %v", err)
	}
	if len(states) != 0 {
		t.Errorf("expected 0 states, got %d", len(states))
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %v", warnings)
	}
}

// TestReadAll_SortedByPaneID verifies that ReadAll returns states sorted by
// the numeric portion of PaneID (so %2 sorts before %10, not after).
func TestReadAll_SortedByPaneID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	panes := []string{"%9", "%3", "%1", "%20"}
	for _, p := range panes {
		if err := state.WriteAtomic(sampleState(p)); err != nil {
			t.Fatalf("WriteAtomic(%q): %v", p, err)
		}
	}

	states, _, err := state.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	got := make([]string, len(states))
	for i, s := range states {
		got[i] = s.PaneID
	}
	want := []string{"%1", "%3", "%9", "%20"}
	if !slices.Equal(got, want) {
		t.Errorf("states sort order: got %v, want %v", got, want)
	}
}
