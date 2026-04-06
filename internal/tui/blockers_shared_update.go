package tui

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

// blockerUpdateShared handles the Update logic shared between both blocker views.
func blockerUpdateShared(s *blockerViewState, ds blockerDataSource, msg tea.Msg) (tea.Model, tea.Cmd, bool) { //nolint:gocyclo
	var cmd tea.Cmd

	if s.expandedView {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "esc", "enter", "q":
				s.expandedView = false
			}
		}
		return nil, nil, true
	}

	if s.confirm.IsActive() {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			if confirmed, handled := s.confirm.HandleKey(keyMsg.String()); handled {
				if confirmed {
					return nil, blockerExecuteDelete(s, ds), true
				}
				s.err = nil
				return nil, nil, true
			}
		}
		return nil, nil, true
	}

	if s.editMode {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "esc":
				s.editMode = false
				s.editTextInput.Reset()
				return nil, nil, true
			case "ctrl+s":
				return nil, blockerSaveEdited(s, ds), true
			case "ctrl+e":
				return nil, blockerEditInExternalEditor(s, ds, true), true
			}
		}
		s.editTextInput, cmd = s.editTextInput.Update(msg)
		return nil, cmd, true
	}

	if s.addMode {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "esc":
				s.addMode = false
				s.textInput.Reset()
				return nil, nil, true
			case "ctrl+s":
				return nil, blockerSaveNew(s, ds), true
			case "ctrl+e":
				return nil, blockerAddInExternalEditor(s, ds), true
			}
		}
		s.textInput, cmd = s.textInput.Update(msg)
		return nil, cmd, true
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		visible := ds.getVisibleItems(s.tabs.Active)

		switch keyMsg.String() {
		case "q", "esc":
			return nil, func() tea.Msg { return ds.closeMsg() }, true
		case "j", "down":
			if len(visible) > 0 && s.cursor < len(visible)-1 {
				s.cursor++
			}
			return nil, nil, true
		case "k", "up":
			if s.cursor > 0 {
				s.cursor--
			}
			return nil, nil, true
		case "g":
			if len(visible) > 0 && s.cursor < len(visible) {
				return nil, func() tea.Msg {
					return BlockerGoToProjectMsg{Project: visible[s.cursor].Project}
				}, true
			}
			return nil, nil, true
		case "G":
			if len(visible) > 0 {
				s.cursor = len(visible) - 1
			}
			return nil, nil, true
		case "a", "n":
			s.addMode = true
			s.textInput.Focus()
			return nil, textarea.Blink, true
		case "e":
			if len(visible) > 0 && s.cursor < len(visible) {
				return nil, blockerStartEdit(s, ds, visible[s.cursor]), true
			}
			return nil, nil, true
		case "E":
			if len(visible) > 0 && s.cursor < len(visible) {
				s.editIndex = ds.findItemIndex(visible[s.cursor].Blocker)
				return nil, blockerEditInExternalEditor(s, ds, false), true
			}
			return nil, nil, true
		case "R":
			if len(visible) > 0 && s.cursor < len(visible) {
				return nil, blockerToggleResolve(s, ds, visible[s.cursor]), true
			}
			return nil, nil, true
		case "d", "x":
			if len(visible) > 0 && s.cursor < len(visible) {
				s.confirm.Show("delete", "Delete blocker?")
			}
			return nil, nil, true
		case "[", "h", "]", "l":
			if handleTabCycleKey(keyMsg.String(), &s.tabs, true, func() {
				blockerClampCursor(s, ds)
			}) {
				return nil, nil, true
			}
		case "enter":
			if len(visible) > 0 && s.cursor < len(visible) {
				s.expandedView = true
			}
			return nil, nil, true
		}
	}

	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		s.width = sizeMsg.Width
		s.height = sizeMsg.Height
		return nil, nil, true
	}

	return nil, nil, false
}

func blockerClampCursor(s *blockerViewState, ds blockerDataSource) {
	visible := ds.getVisibleItems(s.tabs.Active)
	if s.cursor >= len(visible) {
		s.cursor = max(len(visible)-1, 0)
	}
}
