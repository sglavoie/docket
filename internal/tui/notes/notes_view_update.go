package notes

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"
)

//nolint:gocyclo // Centralized dispatch keeps Notes interaction and Esc precedence auditable.
func (m *NotesViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case status.CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case NotesLocationPickerResultMsg:
		return m.handleLocationPickerResult(msg)
	case tea.PasteMsg:
		if m.LocationPicker != nil {
			if handled, cmd := m.LocationPicker.Update(msg); handled {
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
		if m.LocationPicker != nil {
			if handled, cmd := m.LocationPicker.Update(msg); handled {
				return m, cmd
			}
		}
		if m.confirmDiscard {
			switch key {
			case "y", "Y":
				m.confirmDiscard = false
				m.renaming = false
				m.SetStatus("Rename canceled")
			case "n", "N", "esc":
				m.confirmDiscard = false
			}
			return m, nil
		}
		if m.renaming {
			return m.handleRenameKey(msg)
		}
		if m.Confirm.IsActive() {
			if confirmed, handled := m.handleConfirmKey(key); handled {
				if confirmed && m.rowByIdentity(m.PendingDelete) != nil {
					return m, m.PerformDelete()
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
			// Paging is claimed ahead of the text input, so refining a query and
			// reading the note it lands on are the same interaction. Neither press
			// accepts or cancels the draft, and neither reaches the input as a
			// control character.
			case "ctrl+f":
				m.pageNotesPreview(1)
				return m, nil
			case "ctrl+b":
				m.pageNotesPreview(-1)
				return m, nil
			case "up", "ctrl+k", "down", "ctrl+j":
				navKey := key
				switch key {
				case "ctrl+k":
					navKey = "up"
				case "ctrl+j":
					navKey = "down"
				}
				_, cmd := listsurface.HandleListNavigation(
					navKey,
					&m.Cursor,
					len(m.VisibleRows),
					m.Height(),
					&m.cursorFlash,
				)
				return m, cmd
			default:
				return m, m.updateSearchDraft(msg)
			}
		}

		m.ClearStatus()
		if m.completeNotesTopChord(key) {
			return m, nil
		}
		switch key {
		case "j", "down", "k", "up", "G", "end", "ctrl+u", "pgup", "ctrl+d", "pgdown":
			_, cmd := listsurface.HandleListNavigation(key, &m.Cursor, len(m.VisibleRows), m.Height(), &m.cursorFlash)
			return m, cmd
		case "g":
			m.pendingTopChord = true
		case "home":
			m.moveCursorToTop()
		case "ctrl+f":
			m.pageNotesPreview(1)
		case "ctrl+b":
			m.pageNotesPreview(-1)
		case "enter", "a", "e", "%", "/", "p", "y", "Y":
			return m, m.dispatchNotesKey(key)
		case " ", "space":
			return m, m.dispatchNotesKey("space")
		case "u":
			return m, m.UndoDelete()
		case "ctrl+r":
			return m, m.RedoNoteMutation()
		case "?":
			return m, help.HelpCmd("Notes", m.GetHelpItems())
		case "esc":
			if m.SearchApplied != "" {
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
	m.Cursor = 0
	m.resetScrollOffset()
}

// ShouldCaptureEsc returns true while Notes owns an internal interaction layer.
func (m *NotesViewModel) ShouldCaptureEsc() bool {
	return m.Confirm.IsActive() || m.renaming || m.confirmDiscard ||
		m.LocationPicker != nil || m.searchEditing || m.SearchApplied != ""
}

func (m *NotesViewModel) HasTextInput() bool {
	return m.renaming || m.LocationPicker != nil || m.searchEditing
}

func (m *NotesViewModel) handleLocationPickerResult(
	msg NotesLocationPickerResultMsg,
) (tea.Model, tea.Cmd) {
	if m.LocationPicker == nil || m.LocationPicker.Purpose != msg.Purpose {
		return m, nil
	}
	if msg.Selected && !m.LocationPicker.contains(msg.Item) {
		m.LocationPicker = nil
		m.moveSource = nil
		m.SetErrorStatus("Location selection failed: the location is no longer available")
		return m, nil
	}
	m.LocationPicker = nil
	moveSource := m.moveSource
	m.moveSource = nil
	if !msg.Selected {
		switch msg.Purpose {
		case NotesLocationPickerCreation:
			m.SetStatus("Note creation canceled")
		case NotesLocationPickerMove:
			m.SetStatus("Note move canceled")
		case NotesLocationPickerScope:
			m.SetStatus("Scope selection canceled")
		}
		return m, nil
	}

	switch msg.Purpose {
	case NotesLocationPickerScope:
		scope := msg.Item.Scope
		return m, func() tea.Msg {
			return route.ManageHubModuleSelectedMsg{
				Params: route.ManageParams{Module: route.ManageNotes, NotesScope: scope},
			}
		}
	case NotesLocationPickerCreation:
		return m, m.createNoteAt(msg.Item.Location)
	case NotesLocationPickerMove:
		if moveSource == nil || !moveSource.Valid() {
			m.SetErrorStatus("Move failed: note is no longer available")
			return m, nil
		}
		if msg.Item.Location == moveSource.Location {
			m.SetStatus("Move canceled: destination is the current location")
			return m, nil
		}
		sourceLabel := m.LocationLabel(moveSource.Location)
		destinationLabel := m.LocationLabel(msg.Item.Location)
		return m, NoteMutationCmd(m.NoteService, moveSource.Location, app.NoteBatchRequest{
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
			m.SetStatus("Rename canceled: empty title")
			m.renaming = false
			return m, nil
		}
		if row == nil {
			m.renaming = false
			m.SetErrorStatus("Rename failed: note no longer exists")
			return m, nil
		}
		oldTitle := row.Note.Title
		if newTitle == oldTitle {
			m.renaming = false
			m.SetStatus("Rename canceled: same title")
			return m, nil
		}
		m.renaming = false
		return m, NoteMutationCmd(m.NoteService, row.Location, app.NoteBatchRequest{
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
		m.SetStatus("Rename canceled")
		return m, nil
	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// StartRename begins inline rename mode for the current located row.
func (m *NotesViewModel) StartRename() {
	row := m.CurrentRow()
	if row == nil {
		return
	}
	if row.Note.ParseError != "" {
		m.SetErrorStatus("Cannot rename: note has a parse error; press Enter to repair it")
		return
	}
	ti := textinput.New()
	ti.SetValue(row.Note.Title)
	ti.CharLimit = 100
	ti.SetWidth(50)
	ti.Focus()
	theme.ApplyThemedTextInput(&ti)
	m.renameInput = ti
	m.renameTarget = row.Identity()
	m.renaming = true
}
