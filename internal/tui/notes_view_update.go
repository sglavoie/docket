package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
)

//nolint:gocyclo // Centralized dispatch keeps Notes interaction and Esc precedence auditable.
func (m *NotesViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case notesLocationPickerResultMsg:
		return m.handleLocationPickerResult(msg)
	case tea.PasteMsg:
		if m.locationPicker != nil {
			if handled, cmd := m.locationPicker.Update(msg); handled {
				return m, cmd
			}
		}
		if m.renaming {
			var cmd tea.Cmd
			m.renameInput, cmd = m.renameInput.Update(msg)
			return m, cmd
		}
		if m.searchEditing {
			return m, m.updateSearchDraft(msg)
		}
	case tea.KeyPressMsg:
		key := msg.String()
		if m.locationPicker != nil {
			if handled, cmd := m.locationPicker.Update(msg); handled {
				return m, cmd
			}
		}
		if m.confirmDiscard {
			switch key {
			case "y", "Y":
				m.confirmDiscard = false
				m.renaming = false
				m.setStatus("Rename canceled")
			case "n", "N", "esc":
				m.confirmDiscard = false
			}
			return m, nil
		}
		if m.renaming {
			return m.handleRenameKey(msg)
		}
		if m.confirm.IsActive() {
			if confirmed, handled := m.handleConfirmKey(key); handled {
				if confirmed && m.rowByIdentity(m.pendingDelete) != nil {
					return m, m.performDelete()
				}
				return m, nil
			}
		}
		if m.searchEditing {
			switch key {
			case "enter":
				m.acceptSearch()
				return m, nil
			case "esc":
				m.cancelSearchEdit()
				return m, nil
			case "up", "ctrl+k", "down", "ctrl+j":
				navKey := key
				switch key {
				case "ctrl+k":
					navKey = "up"
				case "ctrl+j":
					navKey = "down"
				}
				_, cmd := handleListNavigation(
					navKey,
					&m.cursor,
					len(m.visibleRows),
					m.height,
					&m.cursorFlash,
				)
				return m, cmd
			default:
				return m, m.updateSearchDraft(msg)
			}
		}

		m.clearStatus()
		if m.completeNotesTopChord(key) {
			return m, nil
		}
		switch key {
		case "j", "down", "k", "up", "G", "end", "ctrl+u", "pgup", "ctrl+d", "pgdown":
			_, cmd := handleListNavigation(key, &m.cursor, len(m.visibleRows), m.height, &m.cursorFlash)
			return m, cmd
		case "g":
			m.pendingTopChord = true
		case "home":
			m.moveCursorToTop()
		case "enter", "a", "e", "%", "/", "p", "y", "Y":
			return m, m.dispatchNotesKey(key)
		case " ", "space":
			return m, m.dispatchNotesKey("space")
		case "u":
			return m, m.undoDelete()
		case "ctrl+r":
			return m, m.redoNoteMutation()
		case "?":
			return m, helpCmd(ViewNotes, "Notes", m.GetHelpItems())
		case "esc":
			if m.searchApplied != "" {
				m.clearSearch()
				return m, nil
			}
			return m, func() tea.Msg { return CloseNotesManagerMsg{} }
		}
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	}
	return m, nil
}

// completeNotesTopChord answers the second g of gg and reports the press as
// consumed. Any other key abandons the prefix silently and is then handled
// normally, so a stray g never swallows the key after it.
func (m *NotesViewModel) completeNotesTopChord(key string) bool {
	if !m.pendingTopChord {
		return false
	}
	m.pendingTopChord = false
	if key != "g" {
		return false
	}
	m.moveCursorToTop()
	return true
}

// moveCursorToTop is the one act Home and gg share, so the window follows the
// cursor to the first row under either spelling.
func (m *NotesViewModel) moveCursorToTop() {
	m.cursor = 0
	m.resetScrollOffset()
}

// ShouldCaptureEsc returns true while Notes owns an internal interaction layer.
func (m *NotesViewModel) ShouldCaptureEsc() bool {
	return m.confirm.IsActive() || m.renaming || m.confirmDiscard ||
		m.locationPicker != nil || m.searchEditing || m.searchApplied != ""
}

func (m *NotesViewModel) HasTextInput() bool {
	return m.renaming || m.locationPicker != nil || m.searchEditing
}

func (m *NotesViewModel) handleLocationPickerResult(
	msg notesLocationPickerResultMsg,
) (tea.Model, tea.Cmd) {
	if m.locationPicker == nil || m.locationPicker.purpose != msg.Purpose {
		return m, nil
	}
	if msg.Selected && !m.locationPicker.contains(msg.Item) {
		m.locationPicker = nil
		m.moveSource = nil
		m.setErrorStatus("Location selection failed: the location is no longer available")
		return m, nil
	}
	m.locationPicker = nil
	moveSource := m.moveSource
	m.moveSource = nil
	if !msg.Selected {
		switch msg.Purpose {
		case notesLocationPickerCreation:
			m.setStatus("Note creation canceled")
		case notesLocationPickerMove:
			m.setStatus("Note move canceled")
		case notesLocationPickerScope:
			m.setStatus("Scope selection canceled")
		}
		return m, nil
	}

	switch msg.Purpose {
	case notesLocationPickerScope:
		scope := msg.Item.Scope
		return m, func() tea.Msg {
			return ManageHubModuleSelectedMsg{
				Params: ManageParams{Module: ManageNotes, NotesScope: scope},
			}
		}
	case notesLocationPickerCreation:
		return m, m.createNoteAt(msg.Item.Location)
	case notesLocationPickerMove:
		if moveSource == nil || !moveSource.valid() {
			m.setErrorStatus("Move failed: note is no longer available")
			return m, nil
		}
		if msg.Item.Location == moveSource.Location {
			m.setStatus("Move canceled: destination is the current location")
			return m, nil
		}
		sourceLabel := m.locationLabel(moveSource.Location)
		destinationLabel := m.locationLabel(msg.Item.Location)
		return m, noteMutationCmd(m.noteService, moveSource.Location, app.NoteBatchRequest{
			Operations: []app.NoteMutationRequest{{
				Kind:        app.NoteMove,
				Location:    moveSource.Location,
				Destination: msg.Item.Location,
				Note:        moveSource.noteCopy(),
			}},
			UndoAction:  "Move Note",
			UndoContext: moveSource.Note.Title,
		}, fmt.Sprintf(
			"Moved %s: %s → %s",
			moveSource.Note.Title,
			sourceLabel,
			destinationLabel,
		))
	default:
		return m, nil
	}
}

func (m *NotesViewModel) handleRenameKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	row := m.rowByIdentity(m.renameTarget)
	switch msg.String() {
	case "enter":
		newTitle := strings.TrimSpace(m.renameInput.Value())
		if newTitle == "" {
			m.setStatus("Rename canceled: empty title")
			m.renaming = false
			return m, nil
		}
		if row == nil {
			m.renaming = false
			m.setErrorStatus("Rename failed: note no longer exists")
			return m, nil
		}
		oldTitle := row.Note.Title
		if newTitle == oldTitle {
			m.renaming = false
			m.setStatus("Rename canceled: same title")
			return m, nil
		}
		m.renaming = false
		return m, noteMutationCmd(m.noteService, row.Location, app.NoteBatchRequest{
			Operations: []app.NoteMutationRequest{{
				Kind:     app.NoteRename,
				Location: row.Location,
				Note:     row.Note,
				NewTitle: newTitle,
			}},
			UndoAction:  "Rename Note",
			UndoContext: oldTitle,
		}, fmt.Sprintf("Renamed: %s → %s", oldTitle, newTitle))
	case "esc":
		if row != nil && strings.TrimSpace(m.renameInput.Value()) != row.Note.Title {
			m.confirmDiscard = true
			return m, nil
		}
		m.renaming = false
		m.setStatus("Rename canceled")
		return m, nil
	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// startRename begins inline rename mode for the current located row.
func (m *NotesViewModel) startRename() {
	row := m.currentRow()
	if row == nil {
		return
	}
	if row.Note.ParseError != "" {
		m.setErrorStatus("cannot rename — note has a parse error; press Enter to repair it")
		return
	}
	ti := textinput.New()
	ti.SetValue(row.Note.Title)
	ti.CharLimit = 100
	ti.SetWidth(50)
	ti.Focus()
	applyThemedTextInput(&ti)
	m.renameInput = ti
	m.renameTarget = row.Identity()
	m.renaming = true
}
