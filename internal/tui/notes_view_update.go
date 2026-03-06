package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *NotesViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyMsg:
		key := msg.String()

		// Handle discard confirmation for rename mode
		if m.confirmDiscard {
			switch key {
			case "y", "Y":
				m.confirmDiscard = false
				m.renaming = false
				m.statusMessage = "Rename canceled"
				return m, nil
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}

		// Handle rename input mode
		if m.renaming {
			return m.handleRenameKey(msg)
		}

		// Handle delete confirmation mode
		if m.confirm.IsActive() {
			action := m.confirm.Action()
			if confirmed, handled := m.handleConfirmKey(key); handled {
				if confirmed {
					switch action {
					case "delete":
						if m.pendingDeleteIdx < len(m.notes) {
							return m, m.performDelete()
						}
						m.statusMessage = "Delete failed: note no longer exists"
					case "bulk-delete":
						return m, m.performBulkDelete()
					}
				}
				return m, nil
			}
		}

		m.statusMessage = ""
		m.statusIsError = false

		// Handle pending 'g' prefix key — always consume the second keystroke
		if m.pendingPrefix == "g" {
			m.pendingPrefix = ""
			if key == "X" && m.multiSelectMode {
				m.deselectAllNotes()
				return m, nil
			}
			_, cmd := handleGotoTopPrefix(key, &m.cursor, &m.cursorFlash, func() { m.detailsPane.ResetScroll() })
			return m, cmd
		}

		// Tab toggles focus between list and details pane
		if key == "tab" {
			m.detailsPane.Focused = !m.detailsPane.Focused
			if m.detailsPane.Focused {
				m.detailsPane.ResetScroll()
			}
			return m, nil
		}

		// Z toggles fullscreen (no-op during rename/delete/multiselect)
		if key == "Z" && !m.renaming && !m.confirm.IsActive() && !m.multiSelectMode {
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
		if m.detailsPane.Focused {
			return m.handleDetailsPaneKey(key)
		}

		switch key {
		case "[", "h", "]", "l":
			if handleTabCycleKey(key, &m.tabs, true, m.loadNotes) {
				return m, nil
			}
		case "q", "esc":
			if m.isGlobal {
				return m, func() tea.Msg { return CloseGlobalNotesMsg{} }
			}
			return m, func() tea.Msg { return CloseNotesMsg{} }
		case "j", "down", "k", "up", "G", "ctrl+u", "pgup", "ctrl+d", "pgdown":
			prev := m.cursor
			_, navCmd := handleListNavigation(key, &m.cursor, len(m.notes), m.height, &m.cursorFlash)
			if m.cursor != prev {
				m.detailsPane.ResetScroll()
			}
			if navCmd != nil {
				return m, navCmd
			}
		case "g":
			m.pendingPrefix = "g"
			return m, nil
		case "V":
			m.toggleNoteMultiSelectMode()
			return m, nil
		case " ":
			if m.multiSelectMode {
				m.toggleNoteSelection()
				// Move cursor down after toggle
				if m.cursor < len(m.notes)-1 {
					m.cursor++
					m.detailsPane.ResetScroll()
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
				m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d notes?", m.selectedNoteCount()))
				return m, nil
			}
			return m, m.deleteCurrentNote()
		case "L":
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
			return m, m.togglePin()
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
			tags := buildNoteFilterTagList(m.project, m.notesDir)
			filterTags := m.filterTags
			return m, func() tea.Msg {
				return ShowTagFilterMsg{Tags: tags, Selected: filterTags, Source: ViewNotes, Title: "Filter Notes by Tags"}
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
			return m, helpCmd(sourceView, viewName, m.GetHelpItems())
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
	return m.confirm.IsActive() || m.detailsPane.Focused || m.multiSelectMode || m.renaming || m.fullscreenPane || m.confirmDiscard
}

// HasTextInput returns false since search is now a separate view.
func (m *NotesViewModel) HasTextInput() bool {
	return m.renaming
}

func (m *NotesViewModel) handleDetailsPaneKey(key string) (tea.Model, tea.Cmd) {
	// Scroll keys handled by shared state
	if m.detailsPane.HandleScrollKey(key) {
		return m, nil
	}

	switch key {
	case "esc":
		m.detailsPane.Focused = false
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
		if m.cursor < len(m.notes) && strings.TrimSpace(m.renameInput.Value()) != m.notes[m.cursor].Title {
			m.confirmDiscard = true
			return m, nil
		}
		m.renaming = false
		m.statusMessage = "Rename canceled"
		return m, nil
	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// buildNoteFilterTagList scans all notes and builds a sorted list of tags with counts.
func buildNoteFilterTagList(project *models.Project, notesDir string) []TagInfo {
	tagCounts := make(map[string]int)
	var notes []*models.Note
	if project != nil {
		notes, _ = storage.ListNotes(project)
	} else if notesDir != "" {
		notes, _ = storage.ListNotesFrom(notesDir)
	}
	for _, n := range notes {
		for _, tag := range n.Tags {
			tagCounts[tag]++
		}
	}

	tags := make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		tags = append(tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
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
