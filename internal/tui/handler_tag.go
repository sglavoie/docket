package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// handleTagMessages handles tag-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleTagMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowTagManagerMsg:
		if msg.Project == nil {
			return true, m.activateManageProjectPicker(ManageTags)
		}
		return true, m.activateManageHub(ManageParams{
			Module: ManageTags,
			Scope:  ProjectEntityScope(msg.Project.ID),
		})

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
		if m.currentView() == ViewCrossProjectOutliner && msg.Task != nil {
			return true, m.applyTagsToTask(msg.Task, msg.Project, msg.SelectedTags)
		}
		// Otherwise forward to the current view.
		cmd := m.views.Update(m.currentView(), msg)
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
			// Apply to whichever Outliner is the visible project surface — the
			// canonical cross Outliner at the sole root (including a project scope
			// frame), or the legacy single-project instance when it is still the
			// base view — so the mutated model is always the one on screen.
			m.applyTagFilterToView(m.baseProjectView(), map[string]bool{msg.Tag: true})
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
		return true, m.activateManageHub(ManageParams{
			Module:     ManageTags,
			Scope:      AppEntityScope(),
			ProjectSet: msg.ProjectSet,
		})

	case GlobalTagRenamedMsg:
		m.popView()
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
		return true, m.showProjectWithTagFilter(msg.Project, msg.Tag)

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

// showProjectWithTagFilter enters the target project as a scope frame on the
// single authoritative (cross-project) Outliner and applies the tag filter,
// rather than constructing a separate AppModel.outliner root. The visible model
// and the model targeted by later palette actions are therefore the same
// instance.
func (m *AppModel) showProjectWithTagFilter(project *models.Project, tag string) tea.Cmd {
	if project == nil || tag == "" {
		return nil
	}

	cmd := m.enterProjectScopeOutliner(project, nil, OutlinerOpenOptions{})
	if m.crossOutliner != nil {
		m.crossOutliner.filter.Tags = map[string]bool{tag: true}
		m.crossOutliner.crossForceRebuild = true
		m.crossOutliner.rebuildRows()
		m.crossOutliner.clampCursor()
	}
	return cmd
}

func (m *AppModel) refreshAfterProjectTagChange(oldName, newName string) {
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
func (m *AppModel) applyTagFilterToView(source PresentationID, tags map[string]bool) {
	// Views that implement commonFilterView can be handled generically.
	standardViews := []PresentationID{
		ViewDailyReview, ViewWeeklyReview, ViewRecentActivity,
	}
	for _, vt := range standardViews {
		if source != vt {
			continue
		}
		if v := m.views.Get(vt); v != nil {
			if cfv, ok := v.(commonFilterView); ok {
				cfv.getFilter().Tags = tags
				cfv.refreshAndClamp()
			}
		}
		return
	}

	switch source {
	case ViewCrossProjectOutliner:
		if m.crossOutliner != nil {
			m.crossOutliner.filter.Tags = tags
			m.crossOutliner.crossForceRebuild = true
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
func (m *AppModel) applyColumnToggles(source PresentationID, toggles map[string]bool) {
	switch source {
	case ViewCrossProjectOutliner:
		if m.crossOutliner != nil {
			priorityMode := m.crossOutliner.columnConfig.PriorityMode()
			m.crossOutliner.columnConfig = NewColumnConfig(toggles, nil, DefaultOutlinerColumns, priorityMode)
			m.crossOutliner.rebuildRows()
		}
	default:
	}
}
