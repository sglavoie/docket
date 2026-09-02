package tui

// simplifiedInlineControlOwnsInput is the Outliner's half of the root grammar's
// dispatch guard in app_simplified_input.go: the shell asks the mounted surface
// whether an inline control already owns the keystroke before resolving it
// against the OperationRegistry.
func (m *OutlinerModel) simplifiedInlineControlOwnsInput() bool {
	if m == nil {
		return false
	}
	if m.IsSearchMode() ||
		m.quickEdit.IsActive() ||
		m.confirm.IsActive() ||
		m.queryReadoutExpanded ||
		// An open menu claims the key first even though most keys end up running
		// their root operation anyway: it has to close before the accelerator
		// fires, or the act would run under a dropdown still covering the rows it
		// changed. handleMenuBarKey closes and re-emits, so the grammar sees the
		// same key one update later with nothing owning input.
		m.menuBarOpen {
		return true
	}
	return false
}
