package outliner

import "github.com/sglavoie/tt/internal/tui/chrome"

func (m *OutlinerModel) DesktopTextBuffer() *chrome.DesktopTextBuffer {
	if m.MenuBar.Open || m.QueryReadout.Expanded || m.RelationPickerOpen() {
		return nil
	}
	if m.QuickEdit != nil && m.QuickEdit.IsActive() && !m.QuickEdit.Pending {
		buffer := chrome.DesktopInput(&m.QuickEdit.Input, m.QuickEdit.RefreshDraft)
		// The row editor reuses one input for different tasks.
		buffer.Key = [2]any{buffer.Key, m.QuickEdit.GetTask()}
		return buffer
	}
	if m.IsSearchMode() && m.Search.historyPicker == nil {
		return &chrome.DesktopTextBuffer{Key: &m.BaseSearchableView, Text: m.SearchInputValue(), Column: m.SearchInputCursor(), Restore: func(text string, _, column int) {
			m.SetSearchInputValue(text)
			m.SetSearchInputCursor(column)
			m.Search.history.resetRecall()
			RebuildOutlinerPreservingCursor(m)
			m.RefreshSearchAutocomplete()
		}}
	}
	return nil
}
