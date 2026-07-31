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
	view := m.activeView()
	if view == nil {
		return nil
	}
	if caps, ok := view.(ViewCapabilities); ok {
		return caps
	}
	return nil
}

// hasActiveTextInput returns true if the current view has an active text input
// that should receive keyboard input (like search boxes, text fields).
// This is used to determine whether global keyboard shortcuts should be intercepted.
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

// hasActiveConfirmDialog returns true if the current view has an active confirm dialog.
// Used to prevent the global clipboard handler from intercepting y/Y during delete confirmations.
