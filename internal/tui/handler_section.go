package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleSectionMessages handles section-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleSectionMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowProjectFilterSelectorMsg:
		m.showView(ViewProjectFilterSelector, NewProjectFilterSelectorModel(msg.Projects, msg.Selected))
		return true, nil

	case ProjectFilterSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		if m.crossOutliner != nil {
			selected := make(map[string]bool, len(msg.SelectedProjectIDs))
			for _, id := range msg.SelectedProjectIDs {
				selected[id] = true
			}
			m.crossOutliner.filterProjects = selected
			m.crossOutliner.crossForceRebuild = true
		}
		return true, nil

	case ShowSectionFilterSelectorMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewSectionFilterSelector, NewSectionFilterSelectorModel(msg.ProjectTree))
		} else if msg.Project != nil {
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
		if m.projectTree != nil && m.currentView() == ViewProjectTree {
			m.projectTree.filter.Sections = sections
			m.projectTree.updateVisibility()
			m.projectTree.clampCursor()
		} else if m.outliner != nil && m.currentView() == ViewOutliner {
			m.outliner.filter.Sections = sections
			m.outliner.rebuildRows()
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

		// Check if this is a bulk operation from palette
		if len(m.pendingBulkTasks) > 0 {
			refs := validTaskProjectRefs(m.pendingBulkTasks)
			count, _, err := m.applyBulkTaskRefsWithWorkflow(refs, "Set Priority", func(project *models.Project, taskID string) bool {
				return project.MutateTask(taskID, func(t *models.Task) {
					t.Priority = msg.Priority
				})
			})

			m.pendingBulkTasks = nil
			if err != nil {
				return true, errorCmd("save failed: %w", err)
			}
			m.clearBulkTaskSelectionsForRefs(refs)
			successMsg := fmt.Sprintf("Set priority %d for %d tasks", msg.Priority, count)
			m.setSuccessStatus(successMsg)
			cmds := m.refreshTaskViewsAfterSave()
			cmds = append(cmds, defaultStatusTimeout())
			return true, tea.Batch(cmds...)
		}

		// Single task operation
		if msg.Task != nil && msg.Project != nil {
			undoOpts := m.taskMutationSaveOptions(msg.Task, "Set Priority")
			if !msg.Project.MutateTask(msg.Task.ID, func(t *models.Task) {
				t.Priority = msg.Priority
			}) {
				return true, errorCmd("task no longer exists in project")
			}

			successMsg := fmt.Sprintf("Set priority %d for task #%d", msg.Priority, msg.Task.ShortID)
			return true, tea.Batch(
				startLoading("Saving..."),
				m.appPersister().SaveWithBackupOptions(msg.Project, successMsg, undoOpts),
			)
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
			var changedIDs []string
			for _, task := range m.pendingBulkSectionTasks {
				if task != nil {
					if project.MutateTask(task.ID, func(t *models.Task) {
						assignSection(t, msg.Section)
					}) {
						count++
						changedIDs = append(changedIDs, task.ID)
					}
				}
			}

			if msg.Section != "" {
				project.EnsureSection(msg.Section)
				if globalColor := m.config.GetGlobalSectionColor(msg.Section); globalColor != "gray" {
					project.SetSectionColor(msg.Section, globalColor)
				}
			}
			m.config.ApplyGlobalColorsToProject(project)

			refs := taskProjectRefsFromTasks(m.pendingBulkSectionTasks, project)
			m.pendingBulkSectionTasks = nil
			m.pendingBulkSectionProject = nil

			var successMsg string
			switch msg.Section {
			case "":
				successMsg = fmt.Sprintf("Removed %d tasks from GTD section", count)
			default:
				successMsg = fmt.Sprintf("Moved %d tasks to %s", count, msg.Section)
			}
			opts := BuildWorkflowOpts(
				m.appPersister(),
				project,
				m.config,
				m.configManager,
				nil,
				"Move to Section",
				fmt.Sprintf("%d tasks", count),
				changedIDs,
			)
			if err := persistWithWorkflow(opts); err != nil {
				return true, errorCmd("save failed: %w", err)
			}
			m.clearBulkTaskSelectionsForRefs(refs)
			m.setSuccessStatus(successMsg)
			cmds := m.refreshTaskViewsAfterSave()
			cmds = append(cmds, defaultStatusTimeout())
			return true, tea.Batch(cmds...)
		}

		// Form-based section pick (no task yet, e.g. TaskFormModel in create mode)
		if msg.Task == nil && m.currentView() == ViewTaskForm {
			cmd := m.views.Update(m.currentView(), msg)
			return true, cmd
		}

		// Single task operation
		if msg.Task != nil && msg.Project != nil {
			undoOpts := m.taskMutationSaveOptions(msg.Task, "Move to Section")
			if !msg.Project.MutateTask(msg.Task.ID, func(t *models.Task) {
				assignSection(t, msg.Section)
			}) {
				return true, errorCmd("task no longer exists in project")
			}

			if msg.Section != "" {
				msg.Project.EnsureSection(msg.Section)
				if globalColor := m.config.GetGlobalSectionColor(msg.Section); globalColor != "gray" {
					msg.Project.SetSectionColor(msg.Section, globalColor)
				}
			}

			m.config.ApplyGlobalColorsToProject(msg.Project)

			var successMsg string
			switch msg.Section {
			case "":
				successMsg = "Removed from GTD section"
			default:
				successMsg = fmt.Sprintf("Moved to %s", msg.Section)
			}
			return true, tea.Batch(
				startLoading("Saving..."),
				m.appPersister().SaveWithBackupOptions(msg.Project, successMsg, undoOpts),
			)
		}
		return true, nil

	case ShowSectionManagerMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewSectionManager, NewSectionManagerModel(msg.ProjectTree, nil, m.config, m.configManager, msg.StartAddMode))
		} else if msg.Project != nil {
			m.showView(ViewSectionManager, NewSectionManagerModel(nil, msg.Project, m.config, m.configManager, msg.StartAddMode))
		}
		return true, nil

	case CloseSectionManagerMsg:
		popCmd := m.popView()
		m.reloadAndRefreshProjectTree()
		return true, popCmd

	case GoToSectionMsg:
		m.popView()
		if m.projectTree != nil {
			m.projectTree.pinnedTagTab = 0
			m.projectTree.filter.Sections = map[string]bool{msg.SectionName: true}
			m.projectTree.updateVisibility()
			m.projectTree.clampCursor()
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
		m.switchToProject(msg.Project, false) // Don't restore state - apply specific section filter
		m.projectTree.filter.Sections = map[string]bool{msg.Section: true}
		m.projectTree.updateVisibility()
		m.showView(ViewProjectTree, m.projectTree)
		return true, nil
	}
	return false, nil
}

// assignSection routes section mutations through the task's invariant-safe helper.
func assignSection(task *models.Task, section string) {
	task.SetSection(section)
}
