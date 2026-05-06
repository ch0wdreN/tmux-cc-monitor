// Package errlog provides an append-only error log for hook write failures.
// Records are written by the hook handler on failure and read by the UI to
// display an error count in the footer.
package errlog

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const rotateThresholdBytes = 1 << 20 // 1 MiB

// mu serialises Append and RecentCount within a single process. It does NOT
// provide cross-process exclusion — concurrent hook subprocesses race on the
// rotation Stat+Rename and may both observe "size exceeded" simultaneously.
// That is acceptable per Design Doc §9 ("最大 1 世代だけ保持"): both renames
// would land on the same .1 path and the latter wins. Per-record write is
// atomic via O_APPEND for sub-PIPE_BUF lines (which all our records are).
var mu sync.Mutex

// Path returns the path to the active error log file.
// It respects $XDG_CONFIG_HOME; otherwise defaults to ~/.config.
func Path() (string, error) {
	base, err := configBase()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "tmux-cc-monitor", "hook-errors.log"), nil
}

// Append writes one error record to the log file.
// Format (tab-separated, one line):
//
//	<RFC3339 timestamp>\t<pane_id>\t<event>\t<message>
//
// Newlines within message are replaced with " | " to keep one record per line.
// The parent directory (0700) and the file (0600) are created on first write.
// After the write, if the file size exceeds rotateThresholdBytes the file is
// rotated: the current file is renamed to <path>.1 (overwriting any previous
// .1), and the next Append call will create a fresh active file.
func Append(paneID, event, message string) error {
	path, err := Path()
	if err != nil {
		return fmt.Errorf("errlog.Append: resolve path: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("errlog.Append: mkdir: %w", err)
	}

	// Sanitise message: collapse embedded newlines.
	sanitised := strings.ReplaceAll(message, "\n", " | ")

	line := fmt.Sprintf("%s\t%s\t%s\t%s\n",
		time.Now().UTC().Format(time.RFC3339),
		paneID,
		event,
		sanitised,
	)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("errlog.Append: open: %w", err)
	}
	if _, err := f.WriteString(line); err != nil {
		_ = f.Close()
		return fmt.Errorf("errlog.Append: write: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("errlog.Append: close: %w", err)
	}

	// Check size and rotate if necessary.
	info, err := os.Stat(path)
	if err != nil {
		// Non-fatal: the write succeeded; we just cannot rotate.
		return nil
	}
	if info.Size() > rotateThresholdBytes {
		if err := os.Rename(path, path+".1"); err != nil {
			// Non-fatal: rotation failure does not invalidate the append.
			_ = err
		}
	}

	return nil
}

// RecentCount returns the number of records in the active log file.
// It counts newlines, which equals the number of records because Append
// guarantees each record ends with exactly one newline.
// If the file does not exist, (0, nil) is returned.
func RecentCount() (int, error) {
	path, err := Path()
	if err != nil {
		return 0, fmt.Errorf("errlog.RecentCount: resolve path: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("errlog.RecentCount: open: %w", err)
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() != "" {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("errlog.RecentCount: scan: %w", err)
	}
	return count, nil
}

// configBase returns the XDG_CONFIG_HOME directory, falling back to ~/.config.
func configBase() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}
