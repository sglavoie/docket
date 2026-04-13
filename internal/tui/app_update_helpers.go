package tui

import (
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

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

// batchWithFocusRefresh batches a command with refresh messages for both focus views.
func (m *AppModel) batchWithFocusRefresh(cmd tea.Cmd) tea.Cmd {
	return tea.Batch(
		cmd,
		func() tea.Msg { return FocusViewRefreshMsg{} },
		func() tea.Msg { return FocusBoardRefreshMsg{} },
	)
}

// hasActiveTextInput returns true if the current view has an active text input
// that should receive keyboard input (like search boxes, text fields).
// This is used to determine whether global keyboard shortcuts should be intercepted.
func (m *AppModel) hasActiveTextInput() bool {
	if m.quickNoteOverlay != nil {
		return true
	}
	if m.quickCaptureProjectPicker != nil {
		return true
	}
	if caps := m.currentViewCapabilities(); caps != nil {
		return caps.HasTextInput()
	}
	return false
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

// hasActiveMultiSelect returns true if any primary view has multi-select mode active.
func (m *AppModel) hasActiveMultiSelect() bool {
	if m.projectTree != nil && m.projectTree.multiSelectMode {
		return true
	}
	if m.outliner != nil && m.outliner.multiSelectMode {
		return true
	}
	if m.crossOutliner != nil && m.crossOutliner.multiSelectMode {
		return true
	}
	// Check temporary outliner (not tracked in m.outliner)
	if v := m.views.Get(ViewTemporaryOutliner); v != nil {
		if o, ok := v.(*OutlinerModel); ok && o.multiSelectMode {
			return true
		}
	}
	return false
}
