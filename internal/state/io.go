package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// WriteAtomic serializes s to its state file (PathFor(s.PaneID)) atomically.
// It creates SessionsDir (mode 0700) if it does not exist. The file itself is
// written with mode 0600. Atomicity is achieved by writing to a temp file in
// the same directory and then calling os.Rename.
func WriteAtomic(s *State) error {
	target, err := PathFor(s.PaneID)
	if err != nil {
		return fmt.Errorf("state: WriteAtomic: %w", err)
	}

	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("state: WriteAtomic: create sessions dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("state: WriteAtomic: marshal: %w", err)
	}

	// Create the temp file in the same directory so os.Rename is atomic
	// (same filesystem mount point).
	tmp, err := os.CreateTemp(dir, ".state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("state: WriteAtomic: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Best-effort cleanup of the temp file on any error path.
	var writeErr error
	defer func() {
		if writeErr != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		writeErr = fmt.Errorf("state: WriteAtomic: chmod temp file: %w", err)
		_ = tmp.Close()
		return writeErr
	}

	if _, err := tmp.Write(data); err != nil {
		writeErr = fmt.Errorf("state: WriteAtomic: write temp file: %w", err)
		_ = tmp.Close()
		return writeErr
	}

	if err := tmp.Sync(); err != nil {
		writeErr = fmt.Errorf("state: WriteAtomic: sync temp file: %w", err)
		_ = tmp.Close()
		return writeErr
	}

	if err := tmp.Close(); err != nil {
		writeErr = fmt.Errorf("state: WriteAtomic: close temp file: %w", err)
		return writeErr
	}

	if err := os.Rename(tmpName, target); err != nil {
		writeErr = fmt.Errorf("state: WriteAtomic: rename: %w", err)
		return writeErr
	}

	return nil
}

// ReadAll reads every *.json file from SessionsDir and returns the parsed
// states sorted by PaneID.
//
// Files with an unknown SchemaVersion or malformed JSON are skipped; a
// human-readable warning string is appended to warnings for each skipped file.
//
// The returned error is non-nil only for directory-level failures (e.g.,
// os.ReadDir returns an error for a reason other than the directory not
// existing). A missing SessionsDir is treated as an empty directory — no
// states, no warnings, no error.
func ReadAll() (states []*State, warnings []string, err error) {
	dir, err := SessionsDir()
	if err != nil {
		return nil, nil, fmt.Errorf("state: ReadAll: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("state: ReadAll: read dir %q: %w", dir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}

		fpath := filepath.Join(dir, name)
		raw, readErr := os.ReadFile(fpath)
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("state: ReadAll: read %q: %v", fpath, readErr))
			continue
		}

		var s State
		if unmarshalErr := json.Unmarshal(raw, &s); unmarshalErr != nil {
			warnings = append(warnings, fmt.Sprintf("state: ReadAll: parse %q: %v", fpath, unmarshalErr))
			continue
		}

		if s.SchemaVersion != SchemaVersion {
			warnings = append(warnings, fmt.Sprintf(
				"state: ReadAll: skipping %q: unknown schema_version %d (expected %d)",
				fpath, s.SchemaVersion, SchemaVersion,
			))
			continue
		}

		states = append(states, &s)
	}

	// Sort by the numeric part of the pane id so %2 sorts before %10
	// (lexicographic order would put %10 before %2). Files were read
	// sequentially above; ADR1 accepts that for ≤100 sessions, which
	// covers the v0.0.1 target use case.
	sort.Slice(states, func(i, j int) bool {
		iN, _ := strconv.Atoi(strings.TrimPrefix(states[i].PaneID, "%"))
		jN, _ := strconv.Atoi(strings.TrimPrefix(states[j].PaneID, "%"))
		return iN < jN
	})

	return states, warnings, nil
}
