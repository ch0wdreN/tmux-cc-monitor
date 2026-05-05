// Package cleanup removes stale per-pane state files at popup launch.
// It is called once by main before the TUI starts and operates purely on the
// filesystem — it does not invoke tmux itself.
package cleanup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ch0wdreN/tmux-cc-monitor/internal/state"
)

// StaleThreshold is the minimum age (relative to State.UpdatedAt) that a
// dead-pane file must have before it is eligible for deletion.
// Keeping recently-written files prevents a race where a newly-started pane
// writes its state file before tmux list-panes has returned an up-to-date
// result.  See Design Doc §8.4.
// Exposed as a var (not const) so tests can override without rebuilding.
var StaleThreshold = 5 * time.Second

// Result reports what cleanup did. It is returned to the caller so the TUI
// footer or debug output can surface the numbers.
type Result struct {
	// DeletedServerMismatch is the count of files removed because their
	// tmux_server_pid did not match the current server PID.
	DeletedServerMismatch int

	// DeletedDeadPane is the count of files removed because their pane is no
	// longer alive AND the file's UpdatedAt is older than StaleThreshold.
	DeletedDeadPane int

	// Skipped is the count of dead-pane files NOT removed because UpdatedAt
	// was within StaleThreshold (race-window guard).
	Skipped int

	// Errors collects non-fatal per-file error messages (e.g., a file that
	// met a deletion rule but os.Remove failed).  The scan continues after
	// each such error.
	Errors []string
}

// Run scans the sessions directory and deletes stale state files.
//
// Deletion rules are evaluated in order:
//  1. If state.TmuxServerPID != currentServerPID → delete unconditionally.
//     A kill-server invalidates every prior generation; mtime is irrelevant.
//  2. Else if state.PaneID is NOT in livePaneIDs AND
//     time.Since(state.UpdatedAt) > StaleThreshold → delete.
//     Waiting for StaleThreshold prevents racing with a pane that just
//     started and whose state file appeared before list-panes updated.
//
// livePaneIDs must be a set of currently-alive pane IDs returned by
// "tmux list-panes -a" (e.g. {"%42": true}).  The caller — typically
// main.go — is responsible for invoking tmuxutil.ListPanes and passing the
// result here.
//
// If currentServerPID is 0 (caller could not determine the tmux server PID),
// rule 1 would delete every file whose TmuxServerPID is non-zero.  Callers
// should ensure they pass a valid PID; passing 0 is safe but aggressive.
//
// A missing sessions directory is treated as empty: the function returns a
// zero-value Result and a nil error.
//
// Per-file delete errors are collected into Result.Errors and do not abort
// the scan.
func Run(currentServerPID int, livePaneIDs map[string]bool) (Result, error) {
	sessionsDir, err := state.SessionsDir()
	if err != nil {
		return Result{}, fmt.Errorf("cleanup: resolve sessions dir: %w", err)
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{}, nil
		}
		return Result{}, fmt.Errorf("cleanup: read dir %q: %w", sessionsDir, err)
	}

	var result Result
	now := time.Now()

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}

		fpath := filepath.Join(sessionsDir, name)

		raw, readErr := os.ReadFile(fpath)
		if readErr != nil {
			// Cannot read — leave it alone; state.ReadAll will warn about it.
			continue
		}

		var s state.State
		if unmarshalErr := json.Unmarshal(raw, &s); unmarshalErr != nil {
			// Malformed JSON — conservative: leave it alone.
			continue
		}

		if s.SchemaVersion != state.SchemaVersion {
			// Unknown schema — conservative: leave it alone.
			continue
		}

		// Rule 1: server generation mismatch — delete unconditionally.
		if s.TmuxServerPID != currentServerPID {
			if removeErr := os.Remove(fpath); removeErr != nil {
				result.Errors = append(result.Errors,
					fmt.Sprintf("cleanup: remove %q (server mismatch): %v", fpath, removeErr))
			} else {
				result.DeletedServerMismatch++
			}
			continue
		}

		// Rule 2: pane no longer alive AND file is old enough.
		if !livePaneIDs[s.PaneID] {
			age := now.Sub(s.UpdatedAt)
			if age > StaleThreshold {
				if removeErr := os.Remove(fpath); removeErr != nil {
					result.Errors = append(result.Errors,
						fmt.Sprintf("cleanup: remove %q (dead pane): %v", fpath, removeErr))
				} else {
					result.DeletedDeadPane++
				}
			} else {
				result.Skipped++
			}
		}
	}

	return result, nil
}
