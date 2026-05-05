// Package ui implements the bubbletea TUI for tmux-cc-monitor.
// It renders a grouped list of Claude Code panes, lets the user select one,
// type a message, and send it via tmux send-keys.
package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ch0wdreN/tmux-cc-monitor/internal/state"
	"github.com/ch0wdreN/tmux-cc-monitor/internal/tmuxutil"
)

// Run launches the bubbletea TUI.
//
// states are the pre-loaded, cleanup-filtered pane states.
// errlogCount is the number of recent hook errors, shown in the footer.
//
// Run returns nil on clean quit (q/esc/ctrl+c in list mode, or after a
// successful send). It returns an error only if the bubbletea runtime itself
// fails to start or run.
func Run(states []*state.State, errlogCount int) error {
	m := newModel(states, errlogCount)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// --- interaction modes ---

type mode int

const (
	modeList  mode = iota // browsing the grouped pane list
	modeInput             // typing a message to send to a selected pane
)

// --- list data structures ---

// section groups panes by a single status value.
type section struct {
	title  string
	status state.Status
	items  []*state.State
}

// groupByStatus returns the 4 sections in the fixed display order:
// waiting_permission → waiting_other → running → idle.
// Each section's items are sorted by UpdatedAt descending (most recent first).
func groupByStatus(states []*state.State) [4]section {
	sections := [4]section{
		{title: "Permission Waiting", status: state.StatusWaitingPermission},
		{title: "Waiting (other)", status: state.StatusWaitingOther},
		{title: "Running", status: state.StatusRunning},
		{title: "Idle", status: state.StatusIdle},
	}
	for _, s := range states {
		switch s.Status {
		case state.StatusWaitingPermission:
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
// Rules: <60s → "Ns", <60m → "Nm", <24h → "Nh", else "Nd".
func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
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

// statusBadge returns a short display badge for a status.
func statusBadge(status state.Status) string {
	switch status {
	case state.StatusWaitingPermission:
		return styleBadgePermission.Render("[PERM]")
	case state.StatusWaitingOther:
		return styleBadgeWaiting.Render("[WAIT]")
	case state.StatusRunning:
		return styleBadgeRunning.Render("[RUN] ")
	case state.StatusIdle:
		return styleBadgeIdle.Render("[IDLE]")
	}
	return "[????]"
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

	// input-mode state
	selectedState *state.State
	ta            textarea.Model
	sendErr       string // non-empty when last send failed

	// terminal size
	width  int
	height int
}

func newModel(states []*state.State, errlogCount int) model {
	sections := groupByStatus(states)
	flat := makeFlatItems(sections)

	ta := textarea.New()
	ta.SetWidth(80)
	ta.SetHeight(5)
	ta.Placeholder = "Type your message here..."
	ta.ShowLineNumbers = false
	// Remap InsertNewline to alt+enter so plain enter can be intercepted for
	// send. alt+enter is reliably detectable cross-terminal (unlike shift+enter).
	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("alt+enter"),
		key.WithHelp("alt+enter", "insert newline"),
	)
	ta.CharLimit = 0 // unlimited

	return model{
		mode:        modeList,
		sections:    sections,
		errlogCount: errlogCount,
		flatItems:   flat,
		ta:          ta,
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

// Init satisfies tea.Model.
func (m model) Init() tea.Cmd {
	return tea.SetWindowTitle("tmux-cc-monitor")
}

// Update satisfies tea.Model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ta.SetWidth(m.width - 4) // leave a small margin
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeInput:
			return m.updateInput(msg)
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
		}
	case tea.KeyUp:
		m.moveCursor(-1)
	case tea.KeyDown:
		m.moveCursor(1)
	case tea.KeyEnter:
		if len(m.flatItems) > 0 && m.cursor < len(m.flatItems) {
			m.selectedState = m.flatItems[m.cursor]
			m.ta.Reset()
			m.ta.Focus()
			m.sendErr = ""
			m.mode = modeInput
		}
	}
	return m, nil
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear error banner on any keypress.
	if m.sendErr != "" {
		m.sendErr = ""
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		// Cancel: return to list mode, clear textarea.
		m.ta.Reset()
		m.mode = modeList
		return m, nil
	case tea.KeyEnter:
		if !msg.Alt {
			// Plain enter: send the message.
			text := m.ta.Value()
			if strings.TrimSpace(text) == "" {
				// Don't deliver a stray Enter to the target pane.
				return m, nil
			}
			if err := tmuxutil.SendKeys(m.selectedState.PaneID, text); err != nil {
				m.sendErr = fmt.Sprintf("send failed: %v", err)
				// Remain in input mode; let the textarea keep its value.
				return m, nil
			}
			// Success: quit the program (popup will close).
			return m, tea.Quit
		}
		// alt+enter falls through to textarea for InsertNewline.
	}

	// Forward to textarea.
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	return m, cmd
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

// View satisfies tea.Model.
func (m model) View() string {
	switch m.mode {
	case modeList:
		return m.viewList()
	case modeInput:
		return m.viewInput()
	}
	return ""
}

func (m model) viewList() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("tmux-cc-monitor — Claude Code sessions"))
	b.WriteString("\n\n")

	now := time.Now()
	msgWidth := max(m.width-52, 10)

	// Track the flat-item index as we iterate sections.
	flatIdx := 0
	for i, sec := range m.sections {
		// Section header
		header := fmt.Sprintf("── %s ──", sec.title)
		switch i {
		case 0:
			b.WriteString(styleSectionPermission.Render(header))
		case 1:
			b.WriteString(styleSectionWaitingOther.Render(header))
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
				badge := statusBadge(item.Status)
				elapsed := humanizeDuration(now.Sub(item.UpdatedAt))
				msg := truncateMessage(strings.ReplaceAll(item.LastMessage, "\n", " "), msgWidth)

				// Build a tmux target like "session:window.pane [window_name]"
				// so the user can identify the pane from the popup. The bare
				// pane_id ("%42") was not legible.
				target := fmt.Sprintf("%s:%d.%d", item.Session, item.WindowIndex, item.PaneIndex)
				if item.WindowName != "" {
					target += " [" + item.WindowName + "]"
				}

				line := fmt.Sprintf("  %-26s %-12s %s %5s — %s",
					truncateMessage(target, 26),
					truncateMessage(project, 12),
					badge,
					elapsed,
					msg,
				)
				// Pad to full visible width for reverse-video highlight.
				// lipgloss.Width strips ANSI escapes so the badge's styling
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
	footerRight := "q quit · ↑↓/jk select · enter → send"
	footer := footerLeft + "  ·  " + footerRight
	b.WriteString(styleFooter.Render(footer))

	return b.String()
}

func (m model) viewInput() string {
	var b strings.Builder

	if m.selectedState != nil {
		project := filepath.Base(m.selectedState.CWD)
		target := fmt.Sprintf("%s:%d.%d", m.selectedState.Session, m.selectedState.WindowIndex, m.selectedState.PaneIndex)
		if m.selectedState.WindowName != "" {
			target += " [" + m.selectedState.WindowName + "]"
		}
		header := fmt.Sprintf("Send to %s (%s)", target, project)
		b.WriteString(styleHeader.Render(header))
		b.WriteString("\n\n")
	}

	if m.sendErr != "" {
		b.WriteString(styleErrorBanner.Render(m.sendErr))
		b.WriteString("\n")
	}

	b.WriteString(m.ta.View())
	b.WriteString("\n\n")

	footer := "enter → send · alt+enter → newline · esc → cancel"
	b.WriteString(styleFooter.Render(footer))

	return b.String()
}

// Ensure model satisfies the tea.Model interface at compile time.
var _ tea.Model = model{}
