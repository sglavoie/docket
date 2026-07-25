package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// handleSectionMessages handles section-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleSectionMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowSectionFilterSelectorMsg:
		switch {
		case len(msg.Projects) > 0:
			m.showView(ViewSectionFilterSelector, NewSectionFilterSelectorModelForProjects(msg.Projects))
		case msg.Project != nil:
			m.showView(ViewSectionFilterSelector, NewSectionFilterSelectorModelForProject(msg.Project))
		}
		return true, nil

	case SectionFilterSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		sections := make(map[string]bool)
		for _, section := range msg.SelectedSections {
			sections[section] = true
		}
		if out := m.visibleOutliner(); out != nil {
			out.filter.Sections = sections
			out.crossForceRebuild = true
			out.rebuildRows()
		}
		return true, nil

	case ShowBulkSectionPickerMsg:
		if len(msg.Tasks) > 0 && msg.Project != nil {
			m.pendingBulkSectionTasks = msg.Tasks
			m.pendingBulkSectionProject = msg.Project
			m.showView(ViewSectionPicker, NewSectionPickerModel(msg.Tasks[0], msg.Project))
		}
		return true, nil

	case ShowSectionPickerMsg:
		if msg.Project != nil {
			m.showView(ViewSectionPicker, NewSectionPickerModel(msg.Task, msg.Project))
		}
		return true, nil

	case ShowPriorityPickerMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewPriorityPicker, NewPriorityPickerModel(msg.Task, msg.Project))
		}
		return true, nil

	case PriorityPickerDoneMsg:
		m.popView()
		if msg.Canceled {
			m.pendingBulkTasks = nil
			return true, nil
		}

		// Bulk operation from palette (PaletteBulkPriorityMsg): commit every selected
		// task's priority as one asynchronous cross-project transaction through the
		// BulkCoordinator. Priority is an attribute mutation, so no workflow rules run;
		// the shell owns reload and selection-clear via the batch result.
		if len(m.pendingBulkTasks) > 0 {
			refs := validTaskProjectRefs(m.pendingBulkTasks)
			locators := taskLocatorsFromTaskRefs(refs)
			m.pendingBulkTasks = nil
			cmd := m.secondaryFeatures.Bulk.SetPriorityLocators(locators, msg.Priority)
			return true, finalizeLocatedBatchResult(cmd, locators, fmt.Sprintf("Set priority %d for %d tasks", msg.Priority, len(locators)))
		}

		// Single task operation: commit through the application-layer priority
		// mutation (one ledger-undoable ChangeSet), then reload and refresh.
		if msg.Task != nil && msg.Project != nil {
			res, err := m.appService.SetPriorityByLocator(taskIdentityFor(msg.Task, msg.Project).Locator(), msg.Priority)
			if err != nil {
				return true, errorCmd("%s", err)
			}
			m.reloadProjectByID(res.ProjectID)
			return true, m.handleSaveComplete(SaveCompleteMsg{
				SuccessMessage: fmt.Sprintf("Set priority %d for task #%d", msg.Priority, res.ShortID),
			})
		}
		return true, nil

	case SectionPickerDoneMsg:
		m.popView()
		if msg.Canceled {
			m.pendingBulkSectionTasks = nil
			m.pendingBulkSectionProject = nil
			return true, nil
		}

		// Check if this is a bulk operation
		if len(m.pendingBulkSectionTasks) > 0 && m.pendingBulkSectionProject != nil {
			project := m.pendingBulkSectionProject
			count := 0
			var clearIDs []string
			var clearLocators []app.TaskLocator
			for _, task := range m.pendingBulkSectionTasks {
				if task == nil {
					continue
				}
				clearIDs = append(clearIDs, task.ID)
				clearLocators = append(clearLocators, app.TaskLocator{ProjectID: project.ID, TaskID: task.ID})
				if project.MutateTask(task.ID, func(t *models.Task) {
					assignSection(t, msg.Section)
				}) {
					count++
				}
			}

			if msg.Section != "" {
				project.EnsureSection(msg.Section)
				if globalColor := m.runtime.Settings().GetGlobalSectionColor(msg.Section); globalColor != "gray" {
					project.SetSectionColor(msg.Section, globalColor)
				}
			}
			m.runtime.Settings().ApplyGlobalColorsToProject(project)

			m.pendingBulkSectionTasks = nil
			m.pendingBulkSectionProject = nil

			var successMsg string
			switch msg.Section {
			case "":
				successMsg = fmt.Sprintf("Removed %d tasks from GTD section", count)
			default:
				successMsg = fmt.Sprintf("Moved %d tasks to %s", count, msg.Section)
			}
			// Commit the already-mutated project through the workflow-aware
			// coordinator: grouped ledger undo plus the legacy rule cascade
			// (stateless, matching the former nil-PrevStates save). The shell owns
			// reload, selection-clear, and the status line via the async batch
			// result, mirroring the migrated ProjectTree/Outliner section moves.
			opts := app.ProjectWorkflowOptions{
				Rules:       m.runtime.Automation().GetEnabledWorkflowRules(),
				UndoAction:  "Move to Section",
				UndoContext: fmt.Sprintf("%d tasks", count),
				// The section move changed each task's derived workflow state, so
				// reconcile Today/Next exclusivity for exactly those tasks in the same
				// transaction (enter Next drops Today, leave Next drops the stale order).
				PlanningTaskIDs: clearIDs,
			}
			cmd := m.secondaryFeatures.TaskItem.CommitProjectWorkflow(project, opts, successMsg)
			// Populate ClearSelectionTaskLocators so shell cleanup cannot clear a
			// same-UUID selection owned by another project.
			return true, finalizeLocatedBatchResult(cmd, clearLocators, "")
		}

		// Form-based section pick (no task yet, e.g. TaskFormModel in create mode)
		if msg.Task == nil && m.currentView() == ViewTaskForm {
			cmd := m.views.Update(m.currentView(), msg)
			return true, cmd
		}

		// Single task operation: route through the canonical application-layer
		// SetSection seam (one ledger-undoable ChangeSet). Matching the finalized
		// retired-section decision (see handleGlobalSectionChanged), this no longer
		// ensures the section on the project or applies global section colors — the
		// section field is a retired legacy artifact resynced only into the derived
		// workflow state.
		if msg.Task != nil && msg.Project != nil {
			res, err := m.appService.SetSectionByLocator(taskIdentityFor(msg.Task, msg.Project).Locator(), msg.Section)
			if err != nil {
				return true, errorCmd("%s", err)
			}
			m.reloadProjectByID(res.ProjectID)
			var successMsg string
			switch msg.Section {
			case "":
				successMsg = "Removed from GTD section"
			default:
				successMsg = fmt.Sprintf("Moved to %s", msg.Section)
			}
			return true, m.handleSaveComplete(SaveCompleteMsg{SuccessMessage: successMsg})
		}
		return true, nil

	case ShowSectionManagerMsg:
		if msg.Project != nil {
			m.showView(ViewSectionManager, NewSectionManagerModel(msg.Project, m.runtime, msg.StartAddMode))
		}
		return true, nil

	case CloseSectionManagerMsg:
		popCmd := m.popView()
		m.reloadAndRefreshProjectTree()
		return true, popCmd

	case GoToSectionMsg:
		m.popView()
		if out := m.visibleOutliner(); out != nil {
			out.filter.Sections = map[string]bool{msg.SectionName: true}
			out.crossForceRebuild = true
			out.rebuildRows()
		}
		return true, nil

	case SectionColorChangedMsg:
		// The Settings commit completed before this message was emitted. Apply the
		// committed value on the single-threaded update path so every loaded
		// project observes the global color together and a failed commit cannot
		// mutate any project cache.
		colorName := msg.ColorName
		if m.runtime != nil {
			colorName = m.runtime.Settings().GetGlobalSectionColor(msg.SectionName)
		}
		m.applySectionColorToLoadedProjects(msg.SectionName, colorName)
		if m.currentView() == ViewSectionManager {
			// Let the section manager rebuild its rows after the cache update.
			return false, nil
		}
		return true, nil

	case SectionRenamedMsg, SectionAddedMsg, SectionRemovedMsg:
		if m.currentView() == ViewSectionManager {
			// Let the message fall through to delegateToCurrentView
			// so the section manager stays open and rebuilds its list.
			// Project tree reload happens on CloseSectionManagerMsg.
			return false, nil
		}
		m.popView()
		m.reloadAndRefreshProjectTree()
		return true, nil

	case CommandPaletteSectionSelectMsg:
		m.popView()
		// Enter the project as a scope frame on the canonical cross Outliner and
		// apply the section filter there, rather than constructing a separate
		// AppModel.outliner root that would diverge from the palette's target.
		cmd := m.enterProjectScopeOutliner(msg.Project, nil, OutlinerOpenOptions{})
		if m.crossOutliner != nil {
			m.crossOutliner.filter.Sections = map[string]bool{msg.Section: true}
			m.crossOutliner.crossForceRebuild = true
			m.crossOutliner.rebuildRows()
			m.crossOutliner.clampCursor()
		}
		return true, cmd
	}
	return false, nil
}

// applySectionColorToLoadedProjects updates every live project pointer owned by
// the shell. Cross-project outliners intentionally maintain disk-loaded copies
// outside the central project cache, so update those copies too. Deduplicate by
// pointer rather than project ID because two caches can hold distinct models for
// the same durable project and both must observe the committed global color.
func (m *AppModel) applySectionColorToLoadedProjects(sectionName, colorName string) {
	seen := make(map[*models.Project]struct{})
	apply := func(project *models.Project) {
		if project == nil {
			return
		}
		if _, ok := seen[project]; ok {
			return
		}
		seen[project] = struct{}{}
		project.SetSectionColor(sectionName, colorName)
	}
	for _, project := range m.projects {
		apply(project)
	}
	if m.outliner != nil {
		apply(m.outliner.project)
		for _, project := range m.outliner.loadedProjects {
			apply(project)
		}
	}
	if m.crossOutliner != nil {
		apply(m.crossOutliner.project)
		for _, project := range m.crossOutliner.loadedProjects {
			apply(project)
		}
	}
}

// assignSection routes section mutations through the task's invariant-safe helper.
func assignSection(task *models.Task, section string) {
	task.SetSection(section)
}
