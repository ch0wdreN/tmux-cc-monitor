package errlog

import (
	"os"
	"strings"
	"testing"
)

// TestAppendAndCount writes 3 records and verifies RecentCount returns 3.
func TestAppendAndCount(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for i := range 3 {
		if err := Append("pane1", "Stop", strings.Repeat("x", i+1)); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	n, err := RecentCount()
	if err != nil {
		t.Fatalf("RecentCount: %v", err)
	}
	if n != 3 {
		t.Errorf("RecentCount = %d, want 3", n)
	}
}

// TestRotation verifies that writing beyond 1 MiB causes rotation:
// the active file is small/empty and <path>.1 exists with the bulk data.
func TestRotation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Each Append call writes ~100 bytes of overhead plus the message.
	// Use a 2 KiB message; 600 calls ≈ 1.2 MiB, comfortably over the threshold.
	longMsg := strings.Repeat("a", 2048)
	for range 600 {
		if err := Append("%1", "Notification", longMsg); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}

	// .1 must exist.
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("rotated file %s.1 not found: %v", path, err)
	}

	// Active file must be smaller than the threshold (rotation may happen
	// mid-run so the active file may have a few records, but not the bulk).
	info, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("stat active file: %v", err)
	}
	if err == nil && info.Size() >= rotateThresholdBytes {
		t.Errorf("active file size %d >= threshold %d after rotation", info.Size(), rotateThresholdBytes)
	}
}

// TestRecentCount_MissingFile verifies that a missing log file returns (0, nil).
func TestRecentCount_MissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// No Append calls — file does not exist.
	n, err := RecentCount()
	if err != nil {
		t.Fatalf("RecentCount on missing file: %v", err)
	}
	if n != 0 {
		t.Errorf("RecentCount = %d, want 0", n)
	}
}

// TestAppend_NewlineInMessage verifies that embedded newlines become " | "
// so the record stays on a single line.
func TestAppend_NewlineInMessage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	msg := "line one\nline two\nline three"
	if err := Append("%5", "UserPromptSubmit", msg); err != nil {
		t.Fatalf("Append: %v", err)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)

	// The record must contain " | " separators instead of literal newlines.
	if !strings.Contains(content, " | ") {
		t.Errorf("expected \" | \" separator in record, got: %q", content)
	}

	// The file must have exactly one newline (the trailing record terminator).
	if strings.Count(content, "\n") != 1 {
		t.Errorf("expected exactly 1 newline in file, got %d; content: %q",
			strings.Count(content, "\n"), content)
	}
}
