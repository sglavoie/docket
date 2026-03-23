package tui

import tea "charm.land/bubbletea/v2"

// handleCommonUpdateMsg handles messages shared across secondary views:
// WindowSizeMsg, TagSelectorDoneMsg, SnoozeDateChangedMsg,
// TaskPropertyChangedMsg, TaskUpdatedMsg.
//
// Returns (handled, cmd). When handled is true, the caller should return
// (model, cmd) immediately.
func handleCommonUpdateMsg(
	msg tea.Msg,
	setSize func(int, int),
	reload func(),
	clampCursor func(),
	setStatus func(string),
) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		setSize(msg.Width, msg.Height)
		return true, nil

	case TagSelectorDoneMsg:
		if !msg.Canceled {
			reload()
			clampCursor()
			setStatus("Tags updated")
		}
		return true, nil

	case SnoozeDateChangedMsg:
		reload()
		clampCursor()
		setStatus("Snooze date updated")
		return true, nil

	case TaskPropertyChangedMsg:
		reload()
		clampCursor()
		return true, nil

	case TaskUpdatedMsg:
		reload()
		clampCursor()
		return true, nil
	}

	return false, nil
}
