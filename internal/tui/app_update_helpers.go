package tui

import (
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// keyInterceptor is implemented by overlay widgets that intercept key presses
// and return (inputCmd, dismissed, resultMsg).
type keyInterceptor interface {
	handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg)
}

// dispatchOverlayKey forwards a key to an overlay and assembles the resulting
// tea.Cmd. The caller is responsible for nil-checking the overlay before
// calling and for niling the field when dismiss is true.
func dispatchOverlayKey(overlay keyInterceptor, msg tea.KeyPressMsg) (dismiss bool, cmd tea.Cmd) {
	inputCmd, dismissed, resultMsg := overlay.handleKey(msg)
	var cmds []tea.Cmd
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	if resultMsg != nil {
		captured := resultMsg
		cmds = append(cmds, func() tea.Msg { return captured })
	}
	return dismissed, tea.Batch(cmds...)
}

func (m *AppModel) currentViewCapabilities() ViewCapabilities {
	view := m.views.Get(m.currentView())
	if view == nil {
		return nil
	}
	if caps, ok := view.(ViewCapabilities); ok {
		return caps
	}
	return nil
}

// batchWithFocusRefresh batches a command with a refresh message for the Planner.
func (m *AppModel) batchWithFocusRefresh(cmd tea.Cmd) tea.Cmd {
	return tea.Batch(
		cmd,
		func() tea.Msg { return FocusBoardRefreshMsg{} },
	)
}

// hasActiveTextInput returns true if the current view has an active text input
// that should receive keyboard input (like search boxes, text fields).
// This is used to determine whether global keyboard shortcuts should be intercepted.
func (m *AppModel) hasActiveTextInput() bool {
	if m.Overlays().HasActiveTextInput() {
		return true
	}
	if caps := m.currentViewCapabilities(); caps != nil {
		return caps.HasTextInput()
	}
	return false
}

// resolveQuickCaptureProject enters the shared capture flow. The system Inbox
// is the default destination and %project syntax makes routing elsewhere
// optional inside the same input.
func (m *AppModel) resolveQuickCaptureProject() tea.Msg {
	return ShowQuickCaptureMsg{}
}

func (m *AppModel) resolveQuickCaptureCmd() tea.Cmd {
	return func() tea.Msg { return m.resolveQuickCaptureProject() }
}

// isModifierOnly returns true when the key event is a bare modifier press
// (Shift, Ctrl, Alt, etc.) with no accompanying character.
func isModifierOnly(msg tea.KeyPressMsg) bool {
	switch msg.Code {
	case tea.KeyLeftShift, tea.KeyRightShift,
		tea.KeyLeftCtrl, tea.KeyRightCtrl,
		tea.KeyLeftAlt, tea.KeyRightAlt,
		tea.KeyLeftSuper, tea.KeyRightSuper,
		tea.KeyLeftHyper, tea.KeyRightHyper,
		tea.KeyLeftMeta, tea.KeyRightMeta:
		return true
	}
	return false
}

// isCtrlEscKey reports whether the key event is Ctrl+Esc. The string check
// covers Bubble Tea's normalized representation; the code/modifier check keeps
// this stable for tests and terminal protocols that preserve the modifier.
func isCtrlEscKey(msg tea.KeyPressMsg) bool {
	if msg.String() == "ctrl+esc" {
		return true
	}
	return (msg.Code == tea.KeyEsc || msg.Code == tea.KeyEscape) && msg.Mod.Contains(tea.ModCtrl)
}

// upgradeKittyKeyboard adds the KittyReportAllKeysAsEscapeCodes flag to the
// terminal's Kitty keyboard protocol. Bubble Tea v2 only requests mode 1
// (disambiguate escape codes) which doesn't report shift modifiers for
// printable keys like space. This upgrade makes shift+space distinguishable
// from plain space.
func upgradeKittyKeyboard() tea.Cmd {
	return func() tea.Msg {
		f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
		if err != nil {
			return nil
		}
		defer f.Close() //nolint:errcheck
		_, _ = f.WriteString(ansi.KittyKeyboard(ansi.KittyReportAllKeysAsEscapeCodes|ansi.KittyReportAlternateKeys|ansi.KittyReportAssociatedKeys, 2))
		return nil
	}
}

// hasActivePendingPrefix returns true if the current view is in the middle of
// a multi-key prefix sequence. Used to suppress global single-character
// shortcuts (e.g. #, q, w, u) that would otherwise hijack prefix sub-keys.
func (m *AppModel) hasActivePendingPrefix() bool {
	view := m.views.Get(m.currentView())
	if view == nil {
		return false
	}
	if p, ok := view.(PendingPrefixView); ok {
		return p.HasPendingPrefix()
	}
	return false
}

// hasActiveConfirmDialog returns true if the current view has an active confirm dialog.
// Used to prevent the global clipboard handler from intercepting y/Y during delete confirmations.
func (m *AppModel) hasActiveConfirmDialog() bool {
	if m.projectArchiveConfirm.IsActive() {
		return true
	}
	view := m.views.Get(m.currentView())
	if view == nil {
		return false
	}
	if c, ok := view.(interface{ IsConfirmActive() bool }); ok {
		return c.IsConfirmActive()
	}
	return false
}

// hasActiveMultiSelect returns true if the current view has multi-select mode active.
func (m *AppModel) hasActiveMultiSelect() bool {
	type multiSelectStateful interface {
		IsMultiSelectActive() bool
	}
	if v, ok := m.currentViewAsModel().(multiSelectStateful); ok {
		return v.IsMultiSelectActive()
	}
	return false
}
