package tui

import tea "github.com/charmbracelet/bubbletea"

// crossHandleContextPickerKey handles the C key in cross-project mode,
// emitting ShowContextPickerMsg{} to open the context picker. It is called
// early in the key dispatch so that C works even when m.rows is empty (the
// empty-state view rendered when no projects exist in the current context).
//
// Returns (model, cmd, true) in cross-project mode (m.project == nil),
// (nil, nil, false) otherwise.
func (m *OutlinerModel) crossHandleContextPickerKey(key string) (tea.Model, tea.Cmd, bool) {
	if m.project != nil || key != "C" {
		return nil, nil, false
	}
	return m, func() tea.Msg {
		return ShowContextPickerMsg{}
	}, true
}

// crossInterceptCreationKeys intercepts 'a' (normal mode only), 'i', and 'I'
// in the cross-project outliner and shows a status message directing the user
// to use 'go' to open the single-project outliner for task creation.
//
// Returns (model, cmd, true) when the key is consumed, (nil, nil, false)
// otherwise. The caller must place this check after the multi-select block so
// that 'a' in multi-select mode still triggers select-all.
func (m *OutlinerModel) crossInterceptCreationKeys(key string) (tea.Model, tea.Cmd, bool) {
	if m.project != nil {
		return nil, nil, false
	}
	switch key {
	case "a", "i", "I":
		m.statusMessage = "use go to open project outliner for task creation"
		return m, nil, true
	}
	return nil, nil, false
}

// crossHandlePerspectiveCycle handles h/l perspective cycling for the
// cross-project outliner. It changes activePerspective, sets forceRebuild via
// rebuildRows, rebuilds rows, then skips past any separator the cursor may
// have landed on after the row list is reconstructed.
func (m *OutlinerModel) crossHandlePerspectiveCycle(key string) (tea.Model, tea.Cmd) {
	perspCount := len(PerspectivePresets())
	prevCursor := m.cursor
	switch key {
	case "h":
		m.activePerspective = (m.activePerspective + perspCount - 1) % perspCount
	case "l":
		m.activePerspective = (m.activePerspective + 1) % perspCount
	}
	m.rebuildRows()
	return m.crossSkipSeparators(prevCursor, nil)
}

// crossHandlePerspectiveJump handles P0-P9 direct perspective jumps for the
// cross-project outliner. It sets activePerspective, forces a rebuild via
// rebuildRows, then skips past any separator the cursor may have landed on.
func (m *OutlinerModel) crossHandlePerspectiveJump(perspKey int) (tea.Model, tea.Cmd) {
	prevCursor := m.cursor
	m.activePerspective = perspKey
	m.rebuildRows()
	return m.crossSkipSeparators(prevCursor, nil)
}

// crossSkipSeparators advances the cursor past any contiguous IsSeparator rows
// after a cursor movement, ensuring the cursor never lands on a separator.
//
// It uses prevCursor to determine the direction of travel:
//   - Moving up (cursor < prevCursor): walks backward until a non-separator or
//     index 0 is reached, handling separators at the top.
//   - Moving down (cursor >= prevCursor): walks forward until a non-separator
//     is found, then clamps to the last row if the end is overshot, handling
//     separators at the bottom.
//
// Multiple consecutive separators are handled because each iteration of the
// loop advances one row, so the loop continues until a non-separator is found
// or a boundary is hit.
//
// cmd is the Cmd returned by handleListNavigation and is passed through
// unchanged so the caller can return it directly.
func (m *OutlinerModel) crossSkipSeparators(prevCursor int, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if len(m.rows) == 0 {
		return m, cmd
	}

	if m.cursor < prevCursor {
		// Moving upward: skip backward past any separators.
		for m.cursor > 0 && m.rows[m.cursor].IsSeparator {
			m.cursor--
		}
		// If we stopped at index 0 and it is still a separator, recover
		// by moving forward to the first non-separator row.
		if m.rows[m.cursor].IsSeparator {
			for m.cursor < len(m.rows)-1 && m.rows[m.cursor].IsSeparator {
				m.cursor++
			}
		}
	} else {
		// Moving downward (or no change): skip forward past any separators.
		for m.cursor < len(m.rows) && m.rows[m.cursor].IsSeparator {
			m.cursor++
		}
		// Clamp to the last row when separators run to the end of the list.
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
		// If the clamped row is itself a separator, walk backward to the
		// nearest non-separator (mirrors the upward-boundary recovery).
		if m.rows[m.cursor].IsSeparator {
			for m.cursor > 0 && m.rows[m.cursor].IsSeparator {
				m.cursor--
			}
		}
	}

	return m, cmd
}
