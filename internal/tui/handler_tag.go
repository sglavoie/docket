package tui

import (
	"fmt"

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

	// Unified tag filter selector: single Show handler for all views
	case ShowTagFilterMsg:
		m.showView(ViewTagFilterSelector, NewTagFilterSelectorModel(msg))
		return true, nil

	// Unified tag filter done: dispatch by Source to the originating view
	case TagFilterDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		tags := make(map[string]bool)
		for _, tag := range msg.SelectedTags {
			tags[tag] = true
		}
		switch msg.Source {
		case ViewProjectTree:
			if m.projectTree != nil {
				m.projectTree.filter.Tags = tags
				m.projectTree.updateVisibility()
				m.projectTree.clampCursor()
			}
		case ViewKanban:
			if m.kanban != nil {
				m.kanban.filter.Tags = tags
				m.kanban.loadTasks()
			}
		case ViewFocusView:
			if v := m.views.Get(ViewFocusView); v != nil {
				if fv, ok := v.(*FocusViewModel); ok {
					fv.filter.Tags = tags
					fv.applyFilter()
					fv.clampCursor()
				}
			}
		case ViewDueDateOverview:
			if v := m.views.Get(ViewDueDateOverview); v != nil {
				if ddo, ok := v.(*DueDateOverviewModel); ok {
					ddo.filter.Tags = tags
					ddo.loadTasksWithDueDates()
					ddo.clampCursor()
				}
			}
		case ViewDailyReview:
			if v := m.views.Get(ViewDailyReview); v != nil {
				if dr, ok := v.(*DailyReviewModel); ok {
					dr.filter.Tags = tags
					dr.loadTasks()
					dr.clampCursor()
				}
			}
		case ViewRecentActivity:
			if v := m.views.Get(ViewRecentActivity); v != nil {
				if ra, ok := v.(*RecentActivityModel); ok {
					ra.filter.Tags = tags
					ra.applyFilter()
					ra.clampCursor()
				}
			}
		case ViewPriorityView:
			if v := m.views.Get(ViewPriorityView); v != nil {
				if pv, ok := v.(*PriorityViewModel); ok {
					pv.filter.Tags = tags
					pv.loadActionableTasks()
					pv.clampCursor()
				}
			}
		case ViewNotes:
			if v := m.views.Get(ViewNotes); v != nil {
				if notesView, ok := v.(*NotesViewModel); ok {
					notesView.filterTags = tags
					notesView.loadNotes()
				}
			}
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

	case BulkNoteTagMsg:
		if msg.Project != nil && len(msg.Notes) > 0 {
			// Use first note's tags as current; open the standard note tag selector
			selector := NewNoteTagSelectorModel(msg.Notes[0], msg.Project, msg.NotesDir, msg.IsGlobal, msg.Notes[0].Tags)
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
				note.SetTags(msg.SelectedTags)
				var saveErr error
				if msg.IsGlobal {
					saveErr = storage.SaveNoteTo(msg.NotesDir, note)
				} else {
					saveErr = storage.SaveNote(msg.Project, note)
				}
				if saveErr != nil {
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
			if nv := m.getActiveNotesView(); nv != nil {
				nv.lastTagUndo = undoEntries
				nv.lastLinkUndo = nil
			}
		}
		m.pendingBulkNotes = nil
		return true, func() tea.Msg { return NotesRefreshMsg{} }

	case ShowNoteTagSelectorMsg:
		selector := NewNoteTagSelectorModel(msg.Note, msg.Project, msg.NotesDir, msg.IsGlobal, msg.CurrentTags)
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
				note.SetTags(msg.SelectedTags)
				var saveErr error
				if msg.IsGlobal {
					saveErr = storage.SaveNoteTo(msg.NotesDir, note)
				} else {
					saveErr = storage.SaveNote(msg.Project, note)
				}
				if saveErr != nil {
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
			if nv := m.getActiveNotesView(); nv != nil {
				nv.lastTagUndo = undoEntries
				nv.lastLinkUndo = nil
			}
			m.pendingBulkNotes = nil
			return true, func() tea.Msg { return NotesRefreshMsg{} }
		}
		// Single note mode
		msg.Note.SetTags(msg.SelectedTags)
		var saveErr error
		if msg.IsGlobal {
			saveErr = storage.SaveNoteTo(msg.NotesDir, msg.Note)
		} else {
			saveErr = storage.SaveNote(msg.Project, msg.Note)
		}
		if saveErr != nil {
			return true, errorCmd("failed to save note tags: %w", saveErr)
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
