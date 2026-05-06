// Package ui implements the bubbletea TUI for tmux-cc-monitor.
// It renders a grouped list of Claude Code panes, lets the user select one,
// and enter a mirror mode where the target pane is displayed via capture-pane
// and key presses are forwarded via send-keys.
package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ch0wdreN/tmux-cc-monitor/internal/cleanup"
	"github.com/ch0wdreN/tmux-cc-monitor/internal/errlog"
	"github.com/ch0wdreN/tmux-cc-monitor/internal/state"
	"github.com/ch0wdreN/tmux-cc-monitor/internal/tmuxutil"
)

// Run launches the bubbletea TUI.
//
// states are the pre-loaded, cleanup-filtered pane states.
// errlogCount is the number of recent hook errors, shown in the footer.
//
// Run returns nil on clean quit (q/esc/ctrl+c in list mode). It returns an
// error only if the bubbletea runtime itself fails to start or run.
func Run(states []*state.State, errlogCount int) error {
	m := newModel(states, errlogCount)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// --- interaction modes ---

type mode int

const (
	modeList   mode = iota // browsing the grouped pane list
	modeMirror             // mirroring a selected pane
)

// --- message types ---

// reloadDoneMsg is sent when an async state reload completes.
type reloadDoneMsg struct {
	states      []*state.State
	errlogCount int
}

// redrawTickMsg fires periodically to refresh the elapsed-time display
// without performing any I/O. State reload is a separate path (r key).
type redrawTickMsg struct{}

// --- list data structures ---

// section groups panes by a single status value.
type section struct {
	title  string
	status state.Status
	items  []*state.State
}

// groupByStatus returns the 4 sections in the fixed display order:
// waiting_action → waiting_other → running → idle.
// Each section's items are sorted by UpdatedAt descending (most recent first).
func groupByStatus(states []*state.State) [4]section {
	sections := [4]section{
		{title: "Action Waiting", status: state.StatusWaitingAction},
		{title: "Waiting (other)", status: state.StatusWaitingOther},
		{title: "Running", status: state.StatusRunning},
		{title: "Idle", status: state.StatusIdle},
	}
	for _, s := range states {
		switch s.Status {
		case state.StatusWaitingAction:
			sections[0].items = append(sections[0].items, s)
		case state.StatusWaitingOther:
			sections[1].items = append(sections[1].items, s)
		case state.StatusRunning:
			sections[2].items = append(sections[2].items, s)
		case state.StatusIdle:
			sections[3].items = append(sections[3].items, s)
		}
	}
	for i := range sections {
		sort.Slice(sections[i].items, func(a, b int) bool {
			return sections[i].items[a].UpdatedAt.After(sections[i].items[b].UpdatedAt)
		})
	}
	return sections
}

// humanizeDuration converts a duration to a compact human-readable string.
// Rules: <1m → "<1m", <1h → "Nm", <24h → "Nh", else "Nd".
func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "<1m"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// truncateMessage truncates s to at most max runes. If truncated it appends
// "…" (U+2026, 1 rune). Handles multi-byte runes correctly.
func truncateMessage(s string, max int) string {
	if max <= 0 {
		return ""
	}
	count := utf8.RuneCountInString(s)
	if count <= max {
		return s
	}
	// collect max-1 runes then append ellipsis
	var b strings.Builder
	b.Grow(max * 4)
	i := 0
	for _, r := range s {
		if i >= max-1 {
			break
		}
		b.WriteRune(r)
		i++
	}
	b.WriteRune('…')
	return b.String()
}

// --- bubbletea model ---

// model is the bubbletea application state.
type model struct {
	mode        mode
	sections    [4]section
	errlogCount int

	// list-mode state
	cursor    int            // index into flatItems
	flatItems []*state.State // ordered list of selectable items (no headers)

	// mirror-mode state (non-nil while in modeMirror)
	mirror *mirrorState

	// terminal size
	width  int
	height int
}

func newModel(states []*state.State, errlogCount int) model {
	sections := groupByStatus(states)
	flat := makeFlatItems(sections)

	return model{
		mode:        modeList,
		sections:    sections,
		errlogCount: errlogCount,
		flatItems:   flat,
		width:       80,
		height:      24,
	}
}

// makeFlatItems builds an ordered slice of selectable items across all sections.
func makeFlatItems(sections [4]section) []*state.State {
	var items []*state.State
	for _, sec := range sections {
		items = append(items, sec.items...)
	}
	return items
}

// reloadStates performs a full state reload including cleanup. It is designed
// to be called inside a tea.Cmd goroutine so it does not block the UI.
func reloadStates() tea.Msg {
	serverPID, err := tmuxutil.ServerPID()
	if err != nil {
		// Best-effort: proceed with PID=0 which will be aggressive on cleanup.
		serverPID = 0
	}

	panes, err := tmuxutil.ListPanes()
	if err == nil {
		livePaneIDs := make(map[string]bool, len(panes))
		for _, p := range panes {
			livePaneIDs[p.ID] = true
		}
		cleanup.Run(serverPID, livePaneIDs) //nolint:errcheck
	}

	// Forward state.ReadAll warnings (schema_version mismatch, JSON corruption,
	// etc.) to hook-errors.log. Stderr is unsafe inside a popup because it would
	// corrupt the rendered TUI; the log is the only durable observation channel
	// for reload-time anomalies.
	states, warnings, _ := state.ReadAll() //nolint:errcheck
	for _, w := range warnings {
		_ = errlog.Append("", "reload-warning", w)
	}

	errCount, err := errlog.RecentCount()
	if err != nil {
		errCount = 0
	}

	return reloadDoneMsg{states: states, errlogCount: errCount}
}

// scheduleRedrawTick returns a Cmd that fires redrawTickMsg after 60 seconds.
// It follows the same pattern as scheduleMirrorTick in mirror.go.
func scheduleRedrawTick() tea.Cmd {
	return tea.Tick(60*time.Second, func(time.Time) tea.Msg { return redrawTickMsg{} })
}

// Init satisfies tea.Model.
func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("tmux-cc-monitor"),
		scheduleRedrawTick(),
	)
}

// Update satisfies tea.Model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.mode == modeMirror && m.mirror != nil {
			// Recapture with updated dimensions.
			return m, cmdCapture(m.mirror.paneID, m.captureLines())
		}
		return m, nil

	case reloadDoneMsg:
		m.sections = groupByStatus(msg.states)
		m.flatItems = makeFlatItems(m.sections)
		m.errlogCount = msg.errlogCount
		// Clamp cursor to valid range.
		if m.cursor >= len(m.flatItems) && len(m.flatItems) > 0 {
			m.cursor = len(m.flatItems) - 1
		}
		return m, nil

	case redrawTickMsg:
		// No state change; re-rendering the View re-evaluates time.Now() so
		// elapsed-time columns are refreshed. Schedule the next tick.
		return m, scheduleRedrawTick()

	case captureResultMsg:
		if m.mode == modeMirror && m.mirror != nil {
			if msg.err != nil {
				// Capture failed; check if pane is still alive.
				return m, cmdPaneAlive(m.mirror.paneID)
			}
			m.mirror.content = msg.content
			m.mirror.warnMsg = ""
		}
		return m, nil

	case mirrorTickMsg:
		if m.mode != modeMirror || m.mirror == nil {
			return m, nil
		}
		// Schedule next tick AND capture.
		return m, tea.Batch(
			scheduleMirrorTick(),
			cmdCapture(m.mirror.paneID, m.captureLines()),
		)

	case paneAliveResultMsg:
		if m.mode != modeMirror || m.mirror == nil {
			return m, nil
		}
		if !msg.alive {
			m.mirror.deadBanner = true
			return m, scheduleReturnToList()
		}
		return m, nil

	case returnToListMsg:
		m.mode = modeList
		m.mirror = nil
		return m, nil

	case sendErrMsg:
		if m.mode == modeMirror && m.mirror != nil {
			m.mirror.sendErr = string(msg)
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeMirror:
			return m.updateMirror(msg)
		}
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "q":
			return m, tea.Quit
		case "k":
			m.moveCursor(-1)
		case "j":
			m.moveCursor(1)
		case "r":
			// Async reload: fire off reloadStates as a Cmd.
			return m, func() tea.Msg { return reloadStates() }
		}
	case tea.KeyUp:
		m.moveCursor(-1)
	case tea.KeyDown:
		m.moveCursor(1)
	case tea.KeyEnter:
		if len(m.flatItems) > 0 && m.cursor < len(m.flatItems) {
			selected := m.flatItems[m.cursor]
			m.mirror = &mirrorState{paneID: selected.PaneID}
			m.mode = modeMirror
			// Initial capture + start tick.
			return m, tea.Batch(
				cmdCapture(selected.PaneID, m.captureLines()),
				scheduleMirrorTick(),
			)
		}
	}
	return m, nil
}

func (m *model) moveCursor(delta int) {
	if len(m.flatItems) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.flatItems) {
		m.cursor = len(m.flatItems) - 1
	}
}

// captureLines returns the number of lines to capture from the pane, computed
// from the terminal height minus chrome rows (header + footer).
func (m model) captureLines() int {
	const chromeRows = 3 // 1 header + 1 footer + 1 separator
	lines := m.height - chromeRows
	if lines < 1 {
		lines = 1
	}
	return lines
}

// View satisfies tea.Model.
func (m model) View() string {
	switch m.mode {
	case modeList:
		return m.viewList()
	case modeMirror:
		return m.viewMirror()
	}
	return ""
}

func (m model) viewList() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("tmux-cc-monitor — Claude Code sessions"))
	b.WriteString("\n\n")

	now := time.Now()
	msgWidth := max(m.width-46, 10)

	// Track the flat-item index as we iterate sections.
	flatIdx := 0
	for i, sec := range m.sections {
		// Section header
		header := fmt.Sprintf("── %s ──", sec.title)
		switch i {
		case 0:
			b.WriteString(styleSectionAction.Render(header))
		case 1:
			b.WriteString(styleSectionWaiting.Render(header))
		case 2:
			b.WriteString(styleSectionRunning.Render(header))
		default:
			b.WriteString(styleSectionNeutral.Render(header))
		}
		b.WriteString("\n")

		if len(sec.items) == 0 {
			b.WriteString("  ")
			b.WriteString(styleNone.Render("(none)"))
			b.WriteString("\n")
		} else {
			for _, item := range sec.items {
				project := filepath.Base(item.CWD)
				elapsed := humanizeDuration(now.Sub(item.UpdatedAt))
				msg := truncateMessage(strings.ReplaceAll(item.LastMessage, "\n", " "), msgWidth)

				// Build a tmux target like "session:window.pane [window_name]"
				// so the user can identify the pane from the popup. The bare
				// pane_id ("%42") was not legible.
				target := fmt.Sprintf("%s:%d.%d", item.Session, item.WindowIndex, item.PaneIndex)
				if item.WindowName != "" {
					target += " [" + item.WindowName + "]"
				}

				line := fmt.Sprintf("  %-26s %-12s %5s — %s",
					truncateMessage(target, 26),
					truncateMessage(project, 12),
					elapsed,
					msg,
				)
				// Pad to full visible width for reverse-video highlight.
				// lipgloss.Width strips ANSI escapes so any styling
				// does not inflate len() and shrink the padding.
				visible := lipgloss.Width(line)
				if visible < m.width {
					line += strings.Repeat(" ", m.width-visible)
				}

				if flatIdx == m.cursor {
					b.WriteString(styleSelected.Render(line))
				} else {
					b.WriteString(line)
				}
				b.WriteString("\n")
				flatIdx++
			}
		}
		b.WriteString("\n")
	}

	// Footer
	footerLeft := fmt.Sprintf("errlog: %d", m.errlogCount)
	footerRight := "q quit · ↑↓/jk select · r reload · enter → mirror"
	footer := footerLeft + "  ·  " + footerRight
	b.WriteString(styleFooter.Render(footer))

	return b.String()
}

// Ensure model satisfies the tea.Model interface at compile time.
var _ tea.Model = model{}
