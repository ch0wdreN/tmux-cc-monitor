package ui

import "github.com/charmbracelet/lipgloss"

var (
	// styleHeader is used for the top-level application header.
	styleHeader = lipgloss.NewStyle().Bold(true)

	// styleSectionAction is the section header for waiting_action.
	styleSectionAction = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")) // bright cyan

	// styleSectionRunning is the section header for running status.
	styleSectionRunning = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")) // bright green

	// styleSectionNeutral is the section header for waiting_other and idle.
	styleSectionNeutral = lipgloss.NewStyle().Bold(true).Faint(true)

	// styleNone is the "(none)" placeholder when a section has no items.
	styleNone = lipgloss.NewStyle().Faint(true)

	// styleSelected is the highlight style for the currently selected list row.
	styleSelected = lipgloss.NewStyle().Reverse(true)

	// styleFooter is used for the bottom help/status line.
	styleFooter = lipgloss.NewStyle().Faint(true)

	// styleErrorBanner is used for the send-failure banner in input mode.
	styleErrorBanner = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	// --- mirror mode styles ---

	// styleMirrorHeader is the header line shown at the top of mirror mode.
	styleMirrorHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")) // bright blue

	// styleMirrorFooter is the footer line shown at the bottom of mirror mode.
	styleMirrorFooter = lipgloss.NewStyle().Faint(true)

	// styleMirrorWarn is used for drop-key WARN messages in the mirror footer.
	styleMirrorWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true) // bright yellow

	// styleMirrorErrorBanner is used for pane-gone and send-failure banners in mirror mode.
	styleMirrorErrorBanner = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true) // bright red
)
