package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleTagMessages handles tag-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleTagMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowTagManagerMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewTagManager, NewTagManagerModel(msg.ProjectTree))
		} else if msg.Project != nil {
			m.showView(ViewTagManager, NewTagManagerModelForProject(msg.Project))
		}
		return true, nil

	case ShowTagSelectorMsg:
		selector := NewTagSelectorModel(msg.Task, msg.Project, msg.CurrentTags)
		m.showView(ViewTagSelector, selector)
		return true, nil

	case ShowBulkTagSelectorMsg:
		if refs := validTaskProjectRefs(msg.TaskRefs); len(refs) > 0 {
			selector := NewBulkTagSelectorModel(refs)
			m.showView(ViewBulkTagSelector, selector)
		}
		return true, nil

	case TagSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		// Check if we came from project tree, outliner, or cross-project outliner - apply tags directly
		if (m.currentView() == ViewProjectTree || m.currentView() == ViewOutliner || m.currentView() == ViewCrossProjectOutliner) && msg.Task != nil {
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
		return true, m.applyTagsToTaskRefs(msg.TaskRefs, msg.SelectedTags)

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
		m.applyTagFilterToView(msg.Source, tags)
		return true, nil

	case TagRenamedMsg:
		if m.currentView() == ViewTagManager {
			cmd := m.views.Update(ViewTagManager, msg)
			m.refreshAfterProjectTagChange(msg.OldName, msg.NewName)
			return true, cmd
		}
		m.popView()
		m.reloadAndRefreshProjectTree()
		return true, nil

	case TagAddedMsg:
		if m.currentView() == ViewTagManager {
			cmd := m.views.Update(ViewTagManager, msg)
			m.refreshAfterProjectTagChange("", "")
			return true, cmd
		}
		m.reloadAndRefreshProjectTree()
		return true, nil

	case TagRemovedMsg:
		if m.currentView() == ViewTagManager {
			cmd := m.views.Update(ViewTagManager, msg)
			m.refreshAfterProjectTagChange(msg.Name, "")
			return true, cmd
		}
		m.reloadAndRefreshProjectTree()
		return true, nil

	case TagPinnedMsg:
		if m.currentView() == ViewTagManager {
			cmd := m.views.Update(ViewTagManager, msg)
			m.refreshAfterProjectTagChange("", "")
			return true, cmd
		}
		m.reloadAndRefreshProjectTree()
		return true, nil

	case TagManagerErrorMsg:
		if m.currentView() == ViewTagManager {
			return true, m.views.Update(ViewTagManager, msg)
		}
		if msg.Err != nil {
			return true, errorCmd("%v", msg.Err)
		}
		return true, nil

	case ShowGlobalTagManagerMsg:
		m.showView(ViewGlobalTagManager, NewGlobalTagManagerModel(m.config, m.configManager))
		return true, nil

	case GlobalTagRenamedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case GlobalTagPinnedMsg:
		if m.currentView() == ViewGlobalTagManager {
			cmd := m.views.Update(ViewGlobalTagManager, msg)
			m.refreshAfterGlobalTagPinnedChange()
			return true, cmd
		}
		m.refreshAfterGlobalTagPinnedChange()
		return true, nil

	case GlobalTagManagerErrorMsg:
		if m.currentView() == ViewGlobalTagManager {
			return true, m.views.Update(ViewGlobalTagManager, msg)
		}
		if msg.Err != nil {
			return true, errorCmd("%v", msg.Err)
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
				m.setSuccessStatus(fmt.Sprintf("Tagged %d notes (%d failed)", total, failed))
			} else {
				m.setSuccessStatus(fmt.Sprintf("Tagged %d notes", total))
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
				m.setSuccessStatus(fmt.Sprintf("Tagged %d notes (%d failed)", total, failed))
			} else {
				m.setSuccessStatus(fmt.Sprintf("Tagged %d notes", total))
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
		m.showProjectWithTagFilter(msg.Project, msg.Tag)
		return true, nil

	case ShowColumnPickerMsg:
		m.showView(ViewColumnPicker, NewColumnPickerModel(msg))
		return true, nil

	case ColumnPickerDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		m.applyColumnToggles(msg.Source, msg.Toggles)
		return true, nil
	}
	return false, nil
}

func (m *AppModel) showProjectWithTagFilter(project *models.Project, tag string) {
	if project == nil || tag == "" {
		return
	}

	m.switchToProject(project, false) // Don't restore state - apply specific tag filter.
	tags := map[string]bool{tag: true}

	switch defaultProjectViewType(m.config) {
	case ViewProjectTree:
		m.projectTree.filter.Tags = tags
		m.projectTree.updateVisibility()
		m.showView(ViewProjectTree, m.projectTree)
	default:
		m.saveOutlinerToggleState()
		m.outliner = NewOutlinerModel(project, m.config, m.configManager)
		m.outliner.queueIDs = m.focusQueueIDs
		m.outliner.dailyPlanIDs = m.dailyPlanIDs
		m.restoreOutlinerToggleState()
		m.outliner.filter.Tags = tags
		m.outliner.rebuildRows()
		m.showView(ViewOutliner, m.outliner)
	}
}

func (m *AppModel) refreshAfterProjectTagChange(oldName, newName string) {
	if m.projectTree != nil {
		updateTagFilterAfterCatalogChange(m.projectTree.filter.Tags, oldName, newName)
		m.projectTree.buildTree()
		m.projectTree.updateVisibility()
		m.projectTree.clampCursor()
	}
	if m.outliner != nil {
		updateTagFilterAfterCatalogChange(m.outliner.filter.Tags, oldName, newName)
		m.outliner.rebuildRows()
	}
	if m.crossOutliner != nil {
		updateTagFilterAfterCatalogChange(m.crossOutliner.filter.Tags, oldName, newName)
		m.crossOutliner.rebuildRows()
	}
}

func (m *AppModel) refreshAfterGlobalTagPinnedChange() {
	if m.crossOutliner != nil {
		m.crossOutliner.crossForceRebuild = true
		m.crossOutliner.rebuildRows()
	}
}

func updateTagFilterAfterCatalogChange(tags map[string]bool, oldName, newName string) {
	if tags == nil || oldName == "" || !tags[oldName] {
		return
	}
	delete(tags, oldName)
	if newName != "" {
		tags[newName] = true
	}
}

// applyTagFilterToView dispatches a tag filter selection to the originating view.
func (m *AppModel) applyTagFilterToView(source ViewType, tags map[string]bool) {
	// Views that implement commonFilterView can be handled generically.
	standardViews := []ViewType{
		ViewFocusView, ViewFocusBoard, ViewProjectFocusView, ViewDueDateOverview,
		ViewDailyReview, ViewRecentActivity, ViewPriorityView,
		ViewAllTasks, ViewKanban, ViewCrossProjectKanban,
	}
	for _, vt := range standardViews {
		if source != vt {
			continue
		}
		if v := m.views.Get(vt); v != nil {
			if cfv, ok := v.(tagFilterApplicable); ok {
				cfv.getFilter().Tags = tags
				cfv.refreshAndClamp()
			}
		}
		return
	}

	switch source {
	case ViewProjectTree:
		if m.projectTree != nil {
			m.projectTree.filter.Tags = tags
			m.projectTree.updateVisibility()
			m.projectTree.clampCursor()
		}
	case ViewOutliner:
		target := m.outliner
		if target == nil {
			if v := m.views.Get(ViewTemporaryOutliner); v != nil {
				if o, ok := v.(*OutlinerModel); ok {
					target = o
				}
			}
		}
		if target != nil {
			target.filter.Tags = tags
			target.rebuildRows()
			target.clampCursor()
		}
	case ViewCrossProjectOutliner:
		if m.crossOutliner != nil {
			m.crossOutliner.filter.Tags = tags
			m.crossOutliner.rebuildRows()
			m.crossOutliner.clampCursor()
		}
	case ViewNotes:
		if v := m.views.Get(ViewNotes); v != nil {
			if notesView, ok := v.(*NotesViewModel); ok {
				notesView.filterTags = tags
				notesView.loadNotes()
			}
		}
	default:
	}
}

// applyColumnToggles applies the column toggle changes from the picker to the
// appropriate view and persists them to config.
func (m *AppModel) applyColumnToggles(source ViewType, toggles map[string]bool) {
	projectID := ""
	if m.currentProject != nil {
		projectID = m.currentProject.ID
	}

	switch source {
	case ViewProjectTree:
		priorityMode := resolvePriorityMode(projectID, m.config, "tree")
		if m.projectTree != nil {
			m.projectTree.columnConfig = NewColumnConfig(toggles, m.config.TreeDefaultColumns, DefaultTreeColumns, priorityMode)
		}
		if projectID != "" {
			state, _ := m.config.GetProjectViewState(projectID)
			state.TreeColumnToggles = toggles
			m.config.SetProjectViewState(projectID, state)
		}
	case ViewOutliner:
		target := m.outliner
		if m.currentView() == ViewTemporaryOutliner {
			if v := m.views.Get(ViewTemporaryOutliner); v != nil {
				if o, ok := v.(*OutlinerModel); ok {
					target = o
				}
			}
		}
		if target == nil {
			break
		}
		targetProjectID := ""
		if target.project != nil {
			targetProjectID = target.project.ID
		}
		priorityMode := resolvePriorityMode(targetProjectID, m.config, "outliner")
		target.columnConfig = NewColumnConfig(toggles, m.config.OutlinerDefaultColumns, DefaultOutlinerColumns, priorityMode)
		target.rebuildRows()
		if targetProjectID != "" {
			state, _ := m.config.GetProjectViewState(targetProjectID)
			state.OutlinerColumnToggles = toggles
			m.config.SetProjectViewState(targetProjectID, state)
		}
	case ViewCrossProjectOutliner:
		priorityMode := ""
		if m.config != nil {
			priorityMode = m.config.OutlinerPriorityMode
		}
		if m.crossOutliner != nil {
			m.crossOutliner.columnConfig = NewColumnConfig(toggles, m.config.OutlinerDefaultColumns, DefaultOutlinerColumns, priorityMode)
			m.crossOutliner.rebuildRows()
		}
		m.config.CrossOutlinerColumnToggles = toggles
	default:
	}

	if err := m.configManager.Save(m.config); err != nil {
		logging.Warn("failed to save config: %v", err)
	}
}
