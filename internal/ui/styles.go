package ui

import "github.com/charmbracelet/lipgloss"

var (
	// styleHeader is used for the top-level application header.
	styleHeader = lipgloss.NewStyle().Bold(true)

	// styleSectionPermission is the section header for waiting_permission.
	styleSectionPermission = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")) // bright red

	// styleSectionWaitingOther is the section header for waiting_other.
	styleSectionWaitingOther = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")) // bright yellow

	// styleSectionNeutral is the section header for running and idle.
	styleSectionNeutral = lipgloss.NewStyle().Bold(true).Faint(true)

	// styleNone is the "(none)" placeholder when a section has no items.
	styleNone = lipgloss.NewStyle().Faint(true)

	// styleSelected is the highlight style for the currently selected list row.
	styleSelected = lipgloss.NewStyle().Reverse(true)

	// styleFooter is used for the bottom help/status line.
	styleFooter = lipgloss.NewStyle().Faint(true)

	// styleErrorBanner is used for the send-failure banner in input mode.
	styleErrorBanner = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	// styleBadgePermission is the badge for waiting_permission status.
	styleBadgePermission = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	// styleBadgeWaiting is the badge for waiting_other status.
	styleBadgeWaiting = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	// styleBadgeRunning is the badge for running status.
	styleBadgeRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // bright green

	// styleBadgeIdle is the badge for idle status.
	styleBadgeIdle = lipgloss.NewStyle().Faint(true)
)
