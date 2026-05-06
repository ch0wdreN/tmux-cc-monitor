package state_test

// This file mechanizes the ADR-0003 invariant — that the state-classification
// code path must not depend on `tmux capture-pane` — and the corresponding
// Acceptance Criterion in the v0.1.0 popup-mirror Design Doc §5
// (docs/design-doc/20260506_tmux_cc_monitor_popup_mirror_design.md).
//
// The popup mirror feature intentionally uses capture-pane, but only inside
// the UI / tmuxutil layer. The directories listed below form the
// state-classification path and must remain capture-pane-free. Comments are
// intentionally not exempt: re-introducing the string even in a comment is
// treated as a violation, to minimize the risk of silently regressing the
// separation between the two paths.
//
// A shell-side equivalent of this check lives at
// scripts/check-no-capture-pane-in-state.sh and is wired into
// `task check-state-purity` / `task verify`.

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNoCapturePaneInStateClassificationPaths walks every *.go file
// (including *_test.go) under the state-classification directories and
// fails if it finds the literal string "capture-pane" anywhere in the
// file contents. This test file itself is the one deliberate exception:
// it is the Go-side mechanization of the invariant and so necessarily
// contains the needle.
func TestNoCapturePaneInStateClassificationPaths(t *testing.T) {
	const needle = "capture-pane"

	// Resolve the repo root from this test file's location, so the test
	// is independent of the test runner's working directory.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed; cannot locate this test file")
	}
	// thisFile == <repo>/internal/state/purity_test.go
	thisFileAbs, err := filepath.Abs(thisFile)
	if err != nil {
		t.Fatalf("abs %s: %v", thisFile, err)
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFileAbs)))

	targets := []string{
		filepath.Join(repoRoot, "internal", "state"),
		filepath.Join(repoRoot, "internal", "hook"),
		filepath.Join(repoRoot, "internal", "cleanup"),
		filepath.Join(repoRoot, "internal", "errlog"),
	}

	checked := 0
	for _, dir := range targets {
		// Skip directories that don't exist yet — packages may not all
		// be present at every point in history.
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		} else if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}

		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			// Skip this test file itself — it is the deliberate
			// exception that must contain the needle.
			absPath, absErr := filepath.Abs(path)
			if absErr == nil && absPath == thisFileAbs {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), needle) {
				// Report each offending line for actionable output.
				rel, relErr := filepath.Rel(repoRoot, path)
				if relErr != nil {
					rel = path
				}
				for i, line := range strings.Split(string(data), "\n") {
					if strings.Contains(line, needle) {
						t.Errorf("%s:%d: %q must not appear in state-classification paths (ADR-0003): %s",
							rel, i+1, needle, strings.TrimSpace(line))
					}
				}
			}
			checked++
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}

	if checked == 0 {
		t.Fatal("no Go files were checked; target directories may be missing or misconfigured")
	}
}
