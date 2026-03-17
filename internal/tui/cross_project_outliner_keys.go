package tui

import tea "github.com/charmbracelet/bubbletea"

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
