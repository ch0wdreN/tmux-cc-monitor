package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ch0wdreN/tmux-cc-monitor/internal/tmuxutil"
)

// mirrorState holds the state for mirror mode.
type mirrorState struct {
	paneID     string // target pane, e.g. "%42"
	content    string // last captured output (ANSI-preserved)
	sendErr    string // non-empty when last send-keys call failed
	warnMsg    string // non-empty when a key was dropped (no tmux name); clears on next key/tick
	deadBanner bool   // true when pane-gone banner is showing
}

// --- message types used only by mirror ---

// mirrorTickMsg is sent by the 500ms background tick.
type mirrorTickMsg struct{}

// captureResultMsg carries the result of an async capture-pane call.
type captureResultMsg struct {
	content string
	err     error
}

// paneAliveResultMsg carries the result of an async PaneAlive check.
type paneAliveResultMsg struct {
	alive bool
}

// returnToListMsg triggers the transition back to list mode.
type returnToListMsg struct{}

// sendErrMsg carries a send-keys error message to display in mirror footer.
type sendErrMsg string

// --- cmd helpers ---

// scheduleMirrorTick returns a Cmd that fires mirrorTickMsg after 500ms.
func scheduleMirrorTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return mirrorTickMsg{}
	})
}

// cmdCapture returns a Cmd that calls CapturePane and returns a captureResultMsg.
func cmdCapture(paneID string, lines int) tea.Cmd {
	return func() tea.Msg {
		content, err := tmuxutil.CapturePane(paneID, lines)
		return captureResultMsg{content: content, err: err}
	}
}

// cmdPaneAlive returns a Cmd that checks whether paneID still exists.
func cmdPaneAlive(paneID string) tea.Cmd {
	return func() tea.Msg {
		alive, _ := tmuxutil.PaneAlive(paneID)
		return paneAliveResultMsg{alive: alive}
	}
}

// scheduleReturnToList returns a Cmd that fires returnToListMsg after 1 second.
func scheduleReturnToList() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return returnToListMsg{}
	})
}

// cmdSendLiteral sends literal text and then triggers a capture.
func cmdSendLiteral(paneID, text string, lines int) tea.Cmd {
	return func() tea.Msg {
		if err := tmuxutil.SendLiteral(paneID, text); err != nil {
			return sendErrMsg(fmt.Sprintf("send failed: %v", err))
		}
		// Immediate re-capture after successful send.
		content, err := tmuxutil.CapturePane(paneID, lines)
		return captureResultMsg{content: content, err: err}
	}
}

// cmdSendKeyName sends a tmux key name and then triggers a capture.
func cmdSendKeyName(paneID, name string, lines int) tea.Cmd {
	return func() tea.Msg {
		if err := tmuxutil.SendKeyName(paneID, name); err != nil {
			return sendErrMsg(fmt.Sprintf("send failed: %v", err))
		}
		// Immediate re-capture after successful send.
		content, err := tmuxutil.CapturePane(paneID, lines)
		return captureResultMsg{content: content, err: err}
	}
}

// --- key mapping ---

// keyMapResult describes what to do with a bubbletea KeyMsg in mirror mode.
type keyMapResult struct {
	action  keyAction
	literal string // for actionSendLiteral
	keyName string // for actionSendKeyName
	warnMsg string // for actionDrop
}

type keyAction int

const (
	// actionQuit exits mirror mode (back to list).
	actionQuit keyAction = iota
	// actionSendLiteral sends msg.Runes as literal text.
	actionSendLiteral
	// actionSendKeyName sends a tmux key name.
	actionSendKeyName
	// actionDrop silently drops the key and shows warnMsg in the footer.
	actionDrop
	// actionReserved drops the key silently (popup-reserved, no warning).
	actionReserved
)

// mapKey converts a bubbletea KeyMsg to a mirror keyMapResult.
//
// IMPORTANT: switch cases for KeyEnter / KeyTab / KeyBackspace / KeyDelete
// MUST appear before KeyCtrl* cases because bubbletea reuses C0 values:
//
//	KeyCtrlM == KeyEnter (13)
//	KeyCtrlI == KeyTab (9)
//	KeyCtrlH == KeyBackspace (8) — note: KeyBackspace == keyDEL (127), not keyBS
//	KeyCtrlQuestionMark == KeyBackspace (127)
//
// Placing the semantic names first ensures the correct tmux key name is sent.
func mapKey(msg tea.KeyMsg) keyMapResult {
	// Paste: send runes as a literal block regardless of key type.
	if msg.Paste {
		return keyMapResult{action: actionSendLiteral, literal: string(msg.Runes)}
	}

	// Alt modifier: wrap the key name with "M-" prefix, handled after base mapping.
	// We resolve the base key first, then prefix "M-" if Alt is set.

	switch msg.Type {
	// --- popup-reserved keys (not forwarded) ---
	// KeyEsc == KeyEscape == KeyCtrlOpenBracket (all equal 27).
	// Quit mirror mode; do not forward Esc to the target pane.
	case tea.KeyEsc:
		return keyMapResult{action: actionQuit}

	// --- F-key range ---
	case tea.KeyF1:
		// TODO: popup help overlay reserved; not yet implemented.
		return keyMapResult{action: actionReserved}

	case tea.KeyF2:
		return altWrap(msg, "F2")
	case tea.KeyF3:
		return altWrap(msg, "F3")
	case tea.KeyF4:
		return altWrap(msg, "F4")
	case tea.KeyF5:
		return altWrap(msg, "F5")
	case tea.KeyF6:
		return altWrap(msg, "F6")
	case tea.KeyF7:
		return altWrap(msg, "F7")
	case tea.KeyF8:
		return altWrap(msg, "F8")
	case tea.KeyF9:
		return altWrap(msg, "F9")
	case tea.KeyF10:
		return altWrap(msg, "F10")
	case tea.KeyF11:
		return altWrap(msg, "F11")
	case tea.KeyF12:
		return altWrap(msg, "F12")

	// F13–F20 have no tmux key names; drop with WARN.
	case tea.KeyF13, tea.KeyF14, tea.KeyF15, tea.KeyF16,
		tea.KeyF17, tea.KeyF18, tea.KeyF19, tea.KeyF20:
		return keyMapResult{
			action:  actionDrop,
			warnMsg: fmt.Sprintf("warn: %s has no tmux key name; ignored", msg.Type.String()),
		}

	// KeyCtrlPgUp / KeyCtrlPgDown have no tmux key names; drop with WARN.
	case tea.KeyCtrlPgUp, tea.KeyCtrlPgDown:
		return keyMapResult{
			action:  actionDrop,
			warnMsg: fmt.Sprintf("warn: %s has no tmux key name; ignored", msg.Type.String()),
		}

	// --- semantic keys (MUST appear before KeyCtrl* due to value aliasing) ---

	// KeyEnter == KeyCtrlM (13). "Enter" first.
	case tea.KeyEnter:
		return altWrap(msg, "Enter")

	// KeyTab == KeyCtrlI (9). "Tab" first.
	case tea.KeyTab:
		return altWrap(msg, "Tab")

	// KeyBackspace == keyDEL (127) == KeyCtrlQuestionMark. "BSpace" first.
	case tea.KeyBackspace:
		return altWrap(msg, "BSpace")

	// KeyDelete is a separate escape sequence (distinct from Backspace).
	case tea.KeyDelete:
		return altWrap(msg, "DC")

	// --- Ctrl keys (only reached when NOT aliased to Enter/Tab/Backspace) ---
	case tea.KeyCtrlAt:
		return altWrap(msg, "C-@")
	case tea.KeyCtrlA:
		return altWrap(msg, "C-a")
	case tea.KeyCtrlB:
		return altWrap(msg, "C-b")
	case tea.KeyCtrlC:
		return altWrap(msg, "C-c")
	case tea.KeyCtrlD:
		return altWrap(msg, "C-d")
	case tea.KeyCtrlE:
		return altWrap(msg, "C-e")
	case tea.KeyCtrlF:
		return altWrap(msg, "C-f")
	case tea.KeyCtrlG:
		return altWrap(msg, "C-g")
	case tea.KeyCtrlH:
		// Not reached for literal Backspace (keyDEL=127), but reached for
		// terminals that send BS (8) as backspace. Map to BSpace for consistency.
		return altWrap(msg, "BSpace")
	// KeyCtrlI == KeyTab: handled above in KeyTab case.
	// KeyCtrlM == KeyEnter: handled above in KeyEnter case.
	case tea.KeyCtrlJ:
		return altWrap(msg, "C-j")
	case tea.KeyCtrlK:
		return altWrap(msg, "C-k")
	case tea.KeyCtrlL:
		return altWrap(msg, "C-l")
	case tea.KeyCtrlN:
		return altWrap(msg, "C-n")
	case tea.KeyCtrlO:
		return altWrap(msg, "C-o")
	case tea.KeyCtrlP:
		return altWrap(msg, "C-p")
	case tea.KeyCtrlQ:
		return altWrap(msg, "C-q")
	case tea.KeyCtrlR:
		return altWrap(msg, "C-r")
	case tea.KeyCtrlS:
		return altWrap(msg, "C-s")
	case tea.KeyCtrlT:
		return altWrap(msg, "C-t")
	case tea.KeyCtrlU:
		return altWrap(msg, "C-u")
	case tea.KeyCtrlV:
		return altWrap(msg, "C-v")
	case tea.KeyCtrlW:
		return altWrap(msg, "C-w")
	case tea.KeyCtrlX:
		return altWrap(msg, "C-x")
	case tea.KeyCtrlY:
		return altWrap(msg, "C-y")
	case tea.KeyCtrlZ:
		return altWrap(msg, "C-z")
	// NOTE: tea.KeyCtrlOpenBracket == tea.KeyEsc == 27, already handled above.
	// No separate case needed; including it would be a duplicate case compile error.
	case tea.KeyCtrlBackslash:
		return altWrap(msg, "C-\\")
	case tea.KeyCtrlCloseBracket:
		return altWrap(msg, "C-]")
	case tea.KeyCtrlCaret:
		return altWrap(msg, "C-^")
	case tea.KeyCtrlUnderscore:
		return altWrap(msg, "C-_")

	// --- navigation keys ---
	case tea.KeyUp:
		return altWrap(msg, "Up")
	case tea.KeyDown:
		return altWrap(msg, "Down")
	case tea.KeyLeft:
		return altWrap(msg, "Left")
	case tea.KeyRight:
		return altWrap(msg, "Right")
	case tea.KeyHome:
		return altWrap(msg, "Home")
	case tea.KeyEnd:
		return altWrap(msg, "End")
	case tea.KeyPgUp:
		return altWrap(msg, "PPage")
	case tea.KeyPgDown:
		return altWrap(msg, "NPage")
	case tea.KeyInsert:
		return altWrap(msg, "IC")
	case tea.KeyShiftTab:
		return altWrap(msg, "BTab")

	case tea.KeyCtrlUp:
		return altWrap(msg, "C-Up")
	case tea.KeyCtrlDown:
		return altWrap(msg, "C-Down")
	case tea.KeyCtrlLeft:
		return altWrap(msg, "C-Left")
	case tea.KeyCtrlRight:
		return altWrap(msg, "C-Right")
	case tea.KeyCtrlHome:
		return altWrap(msg, "C-Home")
	case tea.KeyCtrlEnd:
		return altWrap(msg, "C-End")

	case tea.KeyShiftUp:
		return altWrap(msg, "S-Up")
	case tea.KeyShiftDown:
		return altWrap(msg, "S-Down")
	case tea.KeyShiftLeft:
		return altWrap(msg, "S-Left")
	case tea.KeyShiftRight:
		return altWrap(msg, "S-Right")
	case tea.KeyShiftHome:
		return altWrap(msg, "S-Home")
	case tea.KeyShiftEnd:
		return altWrap(msg, "S-End")

	case tea.KeyCtrlShiftUp:
		return altWrap(msg, "C-S-Up")
	case tea.KeyCtrlShiftDown:
		return altWrap(msg, "C-S-Down")
	case tea.KeyCtrlShiftLeft:
		return altWrap(msg, "C-S-Left")
	case tea.KeyCtrlShiftRight:
		return altWrap(msg, "C-S-Right")
	case tea.KeyCtrlShiftHome:
		return altWrap(msg, "C-S-Home")
	case tea.KeyCtrlShiftEnd:
		return altWrap(msg, "C-S-End")

	case tea.KeySpace:
		// Send as literal space.
		return keyMapResult{action: actionSendLiteral, literal: " "}

	case tea.KeyRunes:
		if len(msg.Runes) == 0 {
			return keyMapResult{action: actionReserved}
		}
		text := string(msg.Runes)
		// 'q' is reserved as quit in mirror mode.
		if !msg.Alt && text == "q" {
			return keyMapResult{action: actionQuit}
		}
		if msg.Alt && len(msg.Runes) == 1 {
			// Alt+rune: send as key name "M-<rune>".
			return keyMapResult{action: actionSendKeyName, keyName: "M-" + string(msg.Runes[0])}
		}
		// Printable literal(s).
		return keyMapResult{action: actionSendLiteral, literal: text}
	}

	// Unknown key type: drop silently.
	return keyMapResult{action: actionReserved}
}

// altWrap wraps a key name with "M-" if msg.Alt is set.
func altWrap(msg tea.KeyMsg, name string) keyMapResult {
	if msg.Alt {
		return keyMapResult{action: actionSendKeyName, keyName: "M-" + name}
	}
	return keyMapResult{action: actionSendKeyName, keyName: name}
}

// --- mirror Update ---

func (m model) updateMirror(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mirror == nil {
		m.mode = modeList
		return m, nil
	}

	// Clear transient messages on any keypress.
	m.mirror.sendErr = ""
	m.mirror.warnMsg = ""

	result := mapKey(msg)
	paneID := m.mirror.paneID
	lines := m.captureLines()

	switch result.action {
	case actionQuit:
		m.mode = modeList
		m.mirror = nil
		return m, func() tea.Msg { return reloadStates() }

	case actionSendLiteral:
		return m, cmdSendLiteral(paneID, result.literal, lines)

	case actionSendKeyName:
		return m, cmdSendKeyName(paneID, result.keyName, lines)

	case actionDrop:
		m.mirror.warnMsg = result.warnMsg
		return m, nil

	case actionReserved:
		// No action.
		return m, nil
	}

	return m, nil
}

// --- mirror View ---

func (m model) viewMirror() string {
	if m.mirror == nil {
		return ""
	}

	var b strings.Builder

	// Header line.
	header := fmt.Sprintf("mirror: %s  (q/Esc → list)", m.mirror.paneID)
	b.WriteString(styleMirrorHeader.Render(header))
	b.WriteString("\n")

	// Content area: raw ANSI output from capture-pane, not wrapped in lipgloss.
	// Wrapping in lipgloss would cause double-interpretation of ANSI sequences.
	if m.mirror.deadBanner {
		b.WriteString(styleMirrorErrorBanner.Render("target pane no longer alive"))
		b.WriteString("\n")
	} else if m.mirror.content != "" {
		b.WriteString(m.mirror.content)
		b.WriteString("\n")
	}

	// Footer line: errors and warnings.
	var footerText string
	switch {
	case m.mirror.sendErr != "":
		footerText = m.mirror.sendErr
		b.WriteString(styleMirrorErrorBanner.Render(footerText))
	case m.mirror.warnMsg != "":
		footerText = m.mirror.warnMsg
		b.WriteString(styleMirrorWarn.Render(footerText))
	default:
		footerText = "q/Esc → list · keys forwarded to target pane"
		b.WriteString(styleMirrorFooter.Render(footerText))
	}

	return b.String()
}
