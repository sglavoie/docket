package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/storage"
)

// handleTagMessages handles tag-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleTagMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTagManagerMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewTagManager, NewTagManagerModel(msg.ProjectTree))
		}
		return true, nil

	case CloseTagManagerMsg:
		m.popView()
		return true, nil

	case ShowTagSelectorMsg:
		selector := NewTagSelectorModel(msg.Task, msg.Project, msg.CurrentTags)
		m.showView(ViewTagSelector, selector)
		return true, nil

	case ShowBulkTagSelectorMsg:
		selector := NewBulkTagSelectorModel(msg.Tasks, msg.Project)
		m.showView(ViewBulkTagSelector, selector)
		return true, nil

	case TagSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		// Check if we came from project tree - apply tags directly
		if m.currentView() == ViewProjectTree && msg.Task != nil {
			return true, m.applyTagsToTask(msg.Task, msg.Project, msg.SelectedTags)
		}
		// Otherwise forward to current view (inbox processor, task editor, etc.)
		cmd := m.views.Update(m.currentView(), msg)
		if v := m.views.Get(ViewInboxProcessor); v != nil {
			m.inboxProcessor = v.(*InboxProcessorModel)
		}
		return true, cmd

	case BulkTagSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		return true, m.applyTagsToTasks(msg.Tasks, msg.Project, msg.SelectedTags)

	case ShowTagFilterSelectorMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewTagFilterSelector, NewTagFilterSelectorModel(msg.ProjectTree))
		}
		return true, nil

	case TagFilterSelectorDoneMsg:
		m.popView()
		if !msg.Canceled && m.projectTree != nil {
			m.projectTree.filter.Tags = make(map[string]bool)
			for _, tag := range msg.SelectedTags {
				m.projectTree.filter.Tags[tag] = true
			}
			m.projectTree.updateVisibility()
			m.projectTree.clampCursor()
		}
		return true, nil

	case ShowKanbanTagFilterSelectorMsg:
		if m.kanban != nil {
			m.showView(ViewKanbanTagFilterSelector, NewKanbanTagFilterSelectorModel(m.kanban))
		}
		return true, nil

	case KanbanTagFilterSelectorDoneMsg:
		m.popView()
		if !msg.Canceled && m.kanban != nil {
			m.kanban.filter.Tags = make(map[string]bool)
			for _, tag := range msg.SelectedTags {
				m.kanban.filter.Tags[tag] = true
			}
			m.kanban.loadTasks()
		}
		return true, nil

	case TagRenamedMsg:
		m.popView()
		m.reloadAndRefreshProjectTree()
		return true, nil

	case ShowGlobalTagManagerMsg:
		m.showView(ViewGlobalTagManager, NewGlobalTagManagerModel(m.config))
		return true, nil

	case CloseGlobalTagManagerMsg:
		m.popView()
		return true, nil

	case GlobalTagRenamedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ShowNoteTagFilterSelectorMsg:
		if msg.Project != nil {
			var currentFilterTags map[string]bool
			if v := m.views.Get(ViewNotes); v != nil {
				if notesView, ok := v.(*NotesViewModel); ok {
					currentFilterTags = notesView.filterTags
				}
			}
			if currentFilterTags == nil {
				currentFilterTags = make(map[string]bool)
			}
			m.showView(ViewNoteTagFilterSelector, NewNoteTagFilterSelectorModel(msg.Project, currentFilterTags))
		}
		return true, nil

	case NoteTagFilterSelectorDoneMsg:
		m.popView()
		if !msg.Canceled {
			if v := m.views.Get(ViewNotes); v != nil {
				if notesView, ok := v.(*NotesViewModel); ok {
					notesView.filterTags = make(map[string]bool)
					for _, tag := range msg.SelectedTags {
						notesView.filterTags[tag] = true
					}
					notesView.loadNotes()
				}
			}
		}
		return true, nil

	case BulkNoteTagMsg:
		if msg.Project != nil && len(msg.Notes) > 0 {
			// Use first note's tags as current; open the standard note tag selector
			selector := NewNoteTagSelectorModel(msg.Notes[0], msg.Project, msg.Notes[0].Tags)
			m.pendingBulkNotes = msg.Notes
			m.showView(ViewNoteTagSelector, selector)
		}
		return true, nil

	case BulkNoteTagDoneMsg:
		if !msg.Canceled {
			// Snapshot for undo
			var undoEntries []noteTagUndoEntry
			for _, note := range msg.Notes {
				oldTags := make([]string, len(note.Tags))
				copy(oldTags, note.Tags)
				undoEntries = append(undoEntries, noteTagUndoEntry{Note: note, OldTags: oldTags})
			}
			var failed int
			for _, note := range msg.Notes {
				note.Tags = msg.SelectedTags
				note.ModifiedAt = time.Now()
				if err := storage.SaveNote(msg.Project, note); err != nil {
					failed++
				}
			}
			total := len(msg.Notes)
			if failed > 0 {
				m.statusMessage = fmt.Sprintf("Tagged %d notes (%d failed)", total, failed)
			} else {
				m.statusMessage = fmt.Sprintf("Tagged %d notes", total)
			}
			// Store undo state on notes view
			if v := m.views.Get(ViewNotes); v != nil {
				if nv, ok := v.(*NotesViewModel); ok {
					nv.lastTagUndo = undoEntries
					nv.lastLinkUndo = nil
				}
			}
		}
		m.pendingBulkNotes = nil
		return true, func() tea.Msg { return NotesRefreshMsg{} }

	case ShowNoteTagSelectorMsg:
		selector := NewNoteTagSelectorModel(msg.Note, msg.Project, msg.CurrentTags)
		m.showView(ViewNoteTagSelector, selector)
		return true, nil

	case NoteTagSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			m.pendingBulkNotes = nil
			return true, nil
		}
		// Bulk mode: apply tags to all pending notes
		if len(m.pendingBulkNotes) > 0 {
			// Snapshot for undo
			var undoEntries []noteTagUndoEntry
			for _, note := range m.pendingBulkNotes {
				oldTags := make([]string, len(note.Tags))
				copy(oldTags, note.Tags)
				undoEntries = append(undoEntries, noteTagUndoEntry{Note: note, OldTags: oldTags})
			}
			var failed int
			for _, note := range m.pendingBulkNotes {
				note.Tags = msg.SelectedTags
				note.ModifiedAt = time.Now()
				if err := storage.SaveNote(msg.Project, note); err != nil {
					failed++
				}
			}
			total := len(m.pendingBulkNotes)
			if failed > 0 {
				m.statusMessage = fmt.Sprintf("Tagged %d notes (%d failed)", total, failed)
			} else {
				m.statusMessage = fmt.Sprintf("Tagged %d notes", total)
			}
			// Store undo state on notes view
			if v := m.views.Get(ViewNotes); v != nil {
				if nv, ok := v.(*NotesViewModel); ok {
					nv.lastTagUndo = undoEntries
					nv.lastLinkUndo = nil
				}
			}
			m.pendingBulkNotes = nil
			return true, func() tea.Msg { return NotesRefreshMsg{} }
		}
		// Single note mode
		msg.Note.Tags = msg.SelectedTags
		msg.Note.ModifiedAt = time.Now()
		if err := storage.SaveNote(msg.Project, msg.Note); err != nil {
			return true, errorCmd("failed to save note tags: %w", err)
		}
		return true, func() tea.Msg { return NotesRefreshMsg{} }

	case GlobalTagNavigateMsg:
		m.popView()
		m.switchToProject(msg.Project, false) // Don't restore state - apply specific tag filter
		m.projectTree.filter.Tags = map[string]bool{msg.Tag: true}
		m.projectTree.updateVisibility()
		m.showView(ViewProjectTree, m.projectTree)
		return true, nil
	}
	return false, nil
}
