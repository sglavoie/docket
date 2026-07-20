package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// handleTagMessages handles tag-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleTagMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowTagManagerMsg:
		if msg.Project != nil {
			m.showView(ViewTagManager, NewTagManagerModelForProject(msg.Project, m.secondaryFeatures.Tag))
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
		if (m.currentView() == ViewOutliner || m.currentView() == ViewCrossProjectOutliner) && msg.Task != nil {
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

	case TagManagerFilterMsg:
		m.popView()
		if msg.Tag != "" {
			m.applyTagFilterToView(ViewOutliner, map[string]bool{msg.Tag: true})
		}
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
		m.showView(ViewGlobalTagManager, NewGlobalTagManagerModel(m.runtime.ActiveProjects(), m.runtime, m.secondaryFeatures.Tag))
		return true, nil

	case GlobalTagRenamedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh()
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
		if len(msg.Sources) > 0 {
			// Use first note's tags as current; open the standard note tag selector
			selector := NewNoteTagSelectorModel(msg.Sources[0], msg.AvailableTags, msg.Sources[0].Note.Tags)
			m.pendingBulkNotes = append([]noteMutationRef(nil), msg.Sources...)
			m.showView(ViewNoteTagSelector, selector)
		}
		return true, nil

	case ShowNoteTagSelectorMsg:
		selector := NewNoteTagSelectorModel(msg.Source, msg.AvailableTags, msg.CurrentTags)
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
			location := m.pendingBulkNotes[0].Location
			return true, m.applyNoteTags(location, noteRefsToCopies(m.pendingBulkNotes), msg.SelectedTags, true)
		}
		// Single note mode
		return true, m.applyNoteTags(msg.Source.Location, []*models.Note{msg.Source.noteCopy()}, msg.SelectedTags, false)

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

func (m *AppModel) applyProjectNoteTags(projectID string, notes []*models.Note, tags []string, bulk bool) tea.Cmd {
	return m.applyNoteTags(app.ProjectNoteLocation(projectID), notes, tags, bulk)
}

func (m *AppModel) applyNoteTags(location app.NoteLocation, notes []*models.Note, tags []string, bulk bool) tea.Cmd {
	operations := make([]app.NoteMutationRequest, 0, len(notes))
	for _, note := range notes {
		updated := cloneNoteForCommand(note)
		if updated == nil {
			continue
		}
		updated.SetTags(tags)
		operations = append(operations, app.NoteMutationRequest{Kind: app.NoteUpdate, Location: location, Note: updated})
	}
	label := "Updated note tags"
	if bulk {
		label = fmt.Sprintf("Tagged %d notes", len(operations))
	}
	return noteMutationCmd(m.appService, location, app.NoteBatchRequest{
		Operations:  operations,
		UndoAction:  "Tag Notes",
		UndoContext: fmt.Sprintf("%d notes", len(operations)),
	}, label, bulk)
}

func (m *AppModel) showProjectWithTagFilter(project *models.Project, tag string) {
	if project == nil || tag == "" {
		return
	}

	m.switchToProject(project, false) // Don't restore state - apply specific tag filter.
	tags := map[string]bool{tag: true}

	m.saveOutlinerToggleState()
	m.outliner = NewOutlinerModel(project, m.configManager, m.runtime)
	m.outliner.queueIDs = m.focusQueueIDs
	m.outliner.dailyPlanIDs = m.dailyPlanIDs
	m.restoreOutlinerToggleState()
	m.outliner.filter.Tags = tags
	m.outliner.rebuildRows()
	m.showView(ViewOutliner, m.outliner)
}

func (m *AppModel) refreshAfterProjectTagChange(oldName, newName string) {
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
		ViewFocusBoard, ViewProjectFocusView,
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
	machine := m.runtime.Machine()

	// mutate captures the machine-local persistence for the affected view; the
	// view-model updates happen regardless so the change is visible even when the
	// runtime is not writable (isolated tests).
	var mutate func(*config.MachineState)

	switch source {
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
		priorityMode := resolvePriorityMode(targetProjectID, machine, "outliner")
		target.columnConfig = NewColumnConfig(toggles, machine.OutlinerDefaultColumns, DefaultOutlinerColumns, priorityMode)
		target.rebuildRows()
		if targetProjectID != "" {
			mutate = func(mach *config.MachineState) {
				state, _ := mach.GetProjectViewState(targetProjectID)
				state.OutlinerColumnToggles = toggles
				mach.SetProjectViewState(targetProjectID, state)
			}
		}
	case ViewCrossProjectOutliner:
		priorityMode := machine.OutlinerPriorityMode
		if m.crossOutliner != nil {
			m.crossOutliner.columnConfig = NewColumnConfig(toggles, machine.OutlinerDefaultColumns, DefaultOutlinerColumns, priorityMode)
			m.crossOutliner.rebuildRows()
		}
		mutate = func(mach *config.MachineState) {
			mach.CrossOutlinerColumnToggles = toggles
		}
	default:
	}

	if mutate == nil {
		return
	}
	if err := m.commitMachineMutation(mutate); err != nil {
		logging.Warn("failed to save config: %v", err)
	}
}
