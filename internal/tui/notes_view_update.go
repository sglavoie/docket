package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/storage"
)

func (m *NotesViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Handle rename input mode
		if m.renaming {
			return m.handleRenameKey(msg)
		}

		// Handle delete confirmation mode
		if m.confirmDelete {
			m.confirmDelete = false
			if key == "d" {
				if m.multiSelectMode && m.selectedNoteCount() > 0 {
					return m, m.performBulkDelete()
				}
				if m.pendingDeleteIdx < len(m.notes) {
					return m, m.performDelete()
				}
				m.statusMessage = "Delete failed: note no longer exists"
				return m, nil
			}
			m.statusMessage = "Delete canceled"
			return m, nil
		}

		m.statusMessage = ""
		m.statusIsError = false

		// Handle pending 'g' prefix key — always consume the second keystroke
		if m.pendingGKey {
			m.pendingGKey = false
			if key == "X" && m.multiSelectMode {
				m.deselectAllNotes()
				return m, nil
			}
			// Any other key after 'g' means jump to top
			m.cursor = 0
			m.detailsScroll = 0
			return m, nil
		}

		// Tab toggles focus between list and details pane
		if key == "tab" {
			m.detailsPaneFocused = !m.detailsPaneFocused
			if m.detailsPaneFocused {
				m.detailsScroll = 0
			}
			return m, nil
		}

		// Z toggles fullscreen (no-op during rename/delete/multiselect)
		if key == "Z" && !m.renaming && !m.confirmDelete && !m.multiSelectMode {
			m.fullscreenPane = !m.fullscreenPane
			return m, nil
		}

		// Esc exits fullscreen before anything else
		if key == "esc" && m.fullscreenPane {
			m.fullscreenPane = false
			return m, nil
		}

		// Multi-select exit takes priority over details pane routing
		if m.multiSelectMode && (key == "esc" || key == "q") {
			m.multiSelectMode = false
			m.selectedNotes = make(map[string]bool)
			m.statusMessage = ""
			return m, nil
		}

		// Route keys when details pane is focused
		if m.detailsPaneFocused {
			return m.handleDetailsPaneKey(key)
		}

		switch key {
		case "[", "h":
			if m.activeTab > 0 {
				m.activeTab--
			} else {
				m.activeTab = len(noteTabNames) - 1
			}
			m.loadNotes()
			return m, nil
		case "]":
			if m.activeTab < len(noteTabNames)-1 {
				m.activeTab++
			} else {
				m.activeTab = 0
			}
			m.loadNotes()
			return m, nil
		case "q", "esc":
			if m.isGlobal {
				return m, func() tea.Msg { return CloseGlobalNotesMsg{} }
			}
			return m, func() tea.Msg { return CloseNotesMsg{} }
		case "j", "down":
			if m.cursor < len(m.notes)-1 {
				m.cursor++
				m.detailsScroll = 0
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.detailsScroll = 0
			}
		case "g":
			m.pendingGKey = true
			return m, nil
		case "G":
			if len(m.notes) > 0 {
				m.cursor = len(m.notes) - 1
				m.detailsScroll = 0
			}
		case "V":
			m.toggleNoteMultiSelectMode()
			return m, nil
		case " ":
			if m.multiSelectMode {
				m.toggleNoteSelection()
				// Move cursor down after toggle
				if m.cursor < len(m.notes)-1 {
					m.cursor++
					m.detailsScroll = 0
				}
				return m, nil
			}
		case "a":
			if m.multiSelectMode {
				m.selectAllNotes()
				return m, nil
			}
		case "enter":
			return m, m.viewCurrentNote()
		case "e":
			return m, m.editCurrentNote()
		case "n":
			return m, m.createNote()
		case "d":
			if m.multiSelectMode && m.selectedNoteCount() > 0 {
				m.confirmDelete = true
				m.statusMessage = fmt.Sprintf("Press d again to delete %d notes", m.selectedNoteCount())
				return m, nil
			}
			return m, m.deleteCurrentNote()
		case "l":
			if m.isGlobal && !m.multiSelectMode {
				// In global notes, l cycles tabs forward (no task linking available)
				if m.activeTab < len(noteTabNames)-1 {
					m.activeTab++
				} else {
					m.activeTab = 0
				}
				m.loadNotes()
				return m, nil
			}
			if m.isGlobal {
				return m, nil // no task linking in global notes
			}
			if m.multiSelectMode && m.selectedNoteCount() > 0 {
				return m, m.bulkLinkNotes()
			}
			return m, m.linkCurrentNote()
		case "m":
			if m.isGlobal {
				return m, m.moveNoteToProject()
			}
		case "u":
			return m, m.undoDelete()
		case "p":
			m.togglePin()
		case "s":
			m.cycleSortMode()
			return m, nil
		case "y":
			return m, m.copyNoteTitle()
		case "Y":
			return m, m.copyNoteBody()
		case "ctrl+y":
			return m, m.copyNoteTitleAndBody()
		case "r":
			m.startRename()
			return m, nil
		case "t":
			if m.multiSelectMode && m.selectedNoteCount() > 0 {
				return m, m.bulkTagNotes()
			}
			return m, m.showTagSelector()
		case "T":
			return m, func() tea.Msg {
				return ShowNoteTagFilterSelectorMsg{Project: m.project, NotesDir: m.notesDir}
			}
		case "C":
			if len(m.filterTags) > 0 {
				m.filterTags = make(map[string]bool)
				m.loadNotes()
				m.statusMessage = "Filter cleared"
				return m, nil
			}
		case "/":
			if m.isGlobal {
				return m, nil // search not yet available for global notes
			}
			return m, func() tea.Msg {
				return ShowNotesSearchMsg{Project: m.project}
			}
		case "?":
			sourceView := ViewNotes
			viewName := "Notes"
			if m.isGlobal {
				sourceView = ViewGlobalNotes
				viewName = "Global Notes"
			}
			return m, func() tea.Msg {
				return ShowHelpMsg{
					Items:      m.GetHelpItems(),
					SourceView: sourceView,
					ViewName:   viewName,
				}
			}
		}
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	case NotesRefreshMsg:
		m.tasksLoaded = false
		m.loadNotes()
	case NoteTaskLinkedMsg:
		m.loadNotes()
		m.statusMessage = "Linked note to task"
	}
	return m, nil
}

// ShouldCaptureEsc returns true when delete confirmation or multi-select is active.
func (m *NotesViewModel) ShouldCaptureEsc() bool {
	return m.confirmDelete || m.detailsPaneFocused || m.multiSelectMode || m.renaming || m.fullscreenPane
}

// HasTextInput returns false since search is now a separate view.
func (m *NotesViewModel) HasTextInput() bool {
	return m.renaming
}

func (m *NotesViewModel) handleDetailsPaneKey(key string) (tea.Model, tea.Cmd) {
	halfPage := m.detailsHeight / 2
	if halfPage < 1 {
		halfPage = 1
	}
	switch key {
	case "j", "down":
		m.detailsScroll++
	case "k", "up":
		if m.detailsScroll > 0 {
			m.detailsScroll--
		}
	case "ctrl+d":
		m.detailsScroll += halfPage
	case "ctrl+u":
		m.detailsScroll -= halfPage
		if m.detailsScroll < 0 {
			m.detailsScroll = 0
		}
	case "g":
		m.detailsScroll = 0
	case "G":
		// Scroll to bottom; renderDetailsPane will clamp to maxScroll
		m.detailsScroll = 99999
	case "esc":
		m.detailsPaneFocused = false
	case "q":
		if m.isGlobal {
			return m, func() tea.Msg { return CloseGlobalNotesMsg{} }
		}
		return m, func() tea.Msg { return CloseNotesMsg{} }
	case "e":
		return m, m.editCurrentNote()
	case "enter":
		return m, m.viewCurrentNote()
	}
	return m, nil
}

func (m *NotesViewModel) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		newTitle := strings.TrimSpace(m.renameInput.Value())
		if newTitle == "" {
			m.statusMessage = "Rename canceled: empty title"
			m.renaming = false
			return m, nil
		}
		if m.cursor >= len(m.notes) {
			m.renaming = false
			return m, nil
		}
		note := m.notes[m.cursor]
		oldTitle := note.Title
		if newTitle == oldTitle {
			m.renaming = false
			m.statusMessage = "Rename canceled: same title"
			return m, nil
		}
		_, err := storage.RenameNoteIn(m.effectiveDir(), note.Filename, newTitle)
		if err != nil {
			m.statusMessage = "Rename failed: " + err.Error()
			m.renaming = false
			return m, nil
		}
		m.statusMessage = fmt.Sprintf("Renamed: %s → %s", oldTitle, newTitle)
		m.loadNotes()
		m.renaming = false
		return m, func() tea.Msg { return NotesRefreshMsg{} }
	case "esc":
		m.renaming = false
		m.statusMessage = "Rename canceled"
		return m, nil
	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// startRename begins inline rename mode for the current note.
func (m *NotesViewModel) startRename() {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return
	}
	ti := textinput.New()
	ti.SetValue(m.notes[m.cursor].Title)
	ti.CharLimit = 100
	ti.Width = 50
	ti.Focus()
	m.renameInput = ti
	m.renaming = true
}
