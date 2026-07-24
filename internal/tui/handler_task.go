package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

// handleTaskMessages handles task-related messages (create, edit, delete, status changes).
// Returns true if the message was handled.
func (m *AppModel) handleTaskMessages(msg tea.Msg) (bool, tea.Cmd) {
	// Delegate to specialized handlers by category

	// Dependency messages (already delegated)
	if handled, cmd := m.handleDependencyMessages(msg); handled {
		return handled, cmd
	}

	// CRUD operations
	if handled, cmd := m.handleTaskCRUDMessages(msg); handled {
		return handled, cmd
	}

	// Archive operations
	if handled, cmd := m.handleTaskArchiveMessages(msg); handled {
		return handled, cmd
	}

	// Date-related messages (due dates, snooze)
	if handled, cmd := m.handleTaskDateMessages(msg); handled {
		return handled, cmd
	}

	// Parking notes
	if handled, cmd := m.handleTaskParkMessages(msg); handled {
		return handled, cmd
	}

	// Undo/redo operations
	if handled, cmd := m.handleTaskUndoMessages(msg); handled {
		return handled, cmd
	}

	// Parent selector messages
	if handled, cmd := m.handleTaskParentMessages(msg); handled {
		return handled, cmd
	}

	// View-opening messages (detail, comments, checklist, activity)
	if handled, cmd := m.handleTaskViewMessages(msg); handled {
		return handled, cmd
	}

	// Checklist item messages
	if handled, cmd := m.handleTaskChecklistMessages(msg); handled {
		return handled, cmd
	}

	// Comment messages
	if handled, cmd := m.handleTaskCommentMessages(msg); handled {
		return handled, cmd
	}

	// Decision messages
	if handled, cmd := m.handleTaskDecisionMessages(msg); handled {
		return handled, cmd
	}

	// Link-related messages (linked tasks, cross-project links)
	if handled, cmd := m.handleTaskLinkMessages(msg); handled {
		return handled, cmd
	}

	// Status and property change messages
	if handled, cmd := m.handleTaskStatusPropertyMessages(msg); handled {
		return handled, cmd
	}

	// Command palette task actions
	if handled, cmd := m.handleTaskPaletteMessages(msg); handled {
		return handled, cmd
	}

	// Content editing messages
	if handled, cmd := m.handleTaskContentMessages(msg); handled {
		return handled, cmd
	}

	// Move operations (already in separate file)
	if handled, cmd := m.handleTaskMoveMessages(msg); handled {
		return handled, cmd
	}

	return false, nil
}

// handleTaskArchiveMessages handles task archive messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskArchiveMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ArchiveCompletedTasksMsg:
		return m.handleArchiveCompletedTasks(msg)

	case ArchiveAllCompletedTasksMsg:
		return m.handleArchiveAllCompletedTasks()

	case TasksArchivedMsg:
		if len(msg.ArchivedTaskLocators) > 0 {
			domains := cloneRuntimeDomains(m.runtime)
			if domains == nil {
				m.err = fmt.Errorf("runtime state is unavailable")
				return true, nil
			}
			locators := configTaskLocatorsFromApp(msg.ArchivedTaskLocators)
			domains.Planning.RemoveDurableTaskLocators(locators)
			domains.Machine.RemoveMachineLocalTaskLocators(locators)
			result, err := m.commitRuntimeDomains(domains)
			if err != nil {
				m.err = err
				return true, nil
			}
			if result.MachineErr != nil {
				repairCmd := m.queueMachineDomainRepair(machineRepairForArchivedTasks(locators))
				m.pruneDeletedTaskRefs()
				statusMsg := fmt.Sprintf("Archived %d task(s) to completed.md", msg.Count)
				if msg.Projects > 0 {
					statusMsg = fmt.Sprintf("Archived %d task(s) across %d project(s)", msg.Count, msg.Projects)
				}
				refreshCmd := m.handleWithRefresh(false, false, "")
				m.setWarningStatus(machineRepairWarning(statusMsg, result.MachineErr))
				return true, tea.Batch(refreshCmd, repairCmd, defaultStatusTimeout())
			}
		}
		m.pruneDeletedTaskRefs()
		statusMsg := fmt.Sprintf("Archived %d task(s) to completed.md", msg.Count)
		if msg.Projects > 0 {
			statusMsg = fmt.Sprintf("Archived %d task(s) across %d project(s)", msg.Count, msg.Projects)
		}
		return true, m.handleWithRefresh(false, false, statusMsg)
	}
	return false, nil
}

// handleTaskStatusPropertyMessages handles task status and property change messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskStatusPropertyMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case TaskStatusChangedMsg:
		if msg.Task != nil && msg.Task.State() == models.StateDone {
			m.sessionStats.tasksCompleted++
			m.updateStreak()
			m.advanceOnboarding(func(progress *config.OnboardingProgress) { progress.TaskCompleted = true })
			m.handleWithRefresh(false, false, "")
			m.setSuccessStatus(fmt.Sprintf("Done! (%d today)", m.sessionStats.tasksCompleted))
			cmd := defaultStatusTimeout()
			if bannerCmd := m.maybeFireTodayCleared(); bannerCmd != nil {
				cmd = tea.Batch(cmd, bannerCmd)
			}
			return true, cmd
		}
		return true, m.handleWithRefresh(false, false, "")

	case TaskPropertyChangedMsg:
		m.handleWithRefresh(false, false, "")
		cmd := m.views.Update(m.currentView(), msg)
		m.syncInboxProcessor()
		if msg.NeedsSave && m.currentProject != nil {
			// The palette/command handlers already applied the property change to the
			// task(s) in m.currentProject in memory (toggle active/pin, set priority,
			// set due date). Commit that mutated project through the application layer
			// so the durable write flows through the transactional repository and
			// workspace ledger instead of the legacy async save wrapper. No workflow
			// rules run (empty rules), matching the legacy no-cascade save behavior;
			// the shell reloads the touched project via the BatchTransactionResultMsg.
			cmd = tea.Batch(cmd, m.secondaryFeatures.TaskItem.CommitProjectWorkflow(m.currentProject, projectWorkflowOptsFromTaskPropertyMsg(msg), ""))
		}
		return true, cmd
	}
	return false, nil
}

// projectWorkflowOptsFromTaskPropertyMsg builds the project-scoped commit options
// for an in-place task property change. Rules are intentionally empty so the
// commit runs no workflow cascade — preserving the legacy plain-save behavior of
// the command-palette property paths — while the undo action/context label the
// ledger entry.
func projectWorkflowOptsFromTaskPropertyMsg(msg TaskPropertyChangedMsg) app.ProjectWorkflowOptions {
	return app.ProjectWorkflowOptions{
		UndoAction:  msg.UndoAction,
		UndoContext: msg.UndoContext,
	}
}

// handleTaskPaletteMessages handles command palette task action messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskPaletteMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case CommandPaletteTaskActionMsg:
		return m.handleCommandPaletteTaskAction(msg)

	case CommandPaletteBulkActionMsg:
		return m.handleCommandPaletteBulkAction(msg)

	case InlineTaskCreateMsg:
		return m.handleInlineTaskCreate(msg)
	}
	return false, nil
}

// handleTaskContentMessages handles task content editing messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskContentMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case EditTaskContentMsg:
		return m.handleEditTaskContent(msg)

	case TaskContentEditedMsg:
		return m.handleTaskContentEdited(msg)

	case DescriptionEditorDoneMsg:
		return m.handleDescriptionEditorDone(msg)
	}
	return false, nil
}

// handleTaskMoveMessages handles task move messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskMoveMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowProjectSelectorForMoveMsg:
		if msg.Task != nil && msg.SourceProject != nil {
			projects := m.activeProjects()
			m.showView(ViewProjectSelector, NewProjectSelectorModel(projects, msg.Task, msg.SourceProject))
		}
		return true, nil

	case MoveTaskToProjectMsg:
		return m.handleMoveTaskToProject(msg)

	case TaskMovedToProjectMsg:
		return m.handleTaskMovedToProject(msg)
	}
	return false, nil
}

// updateStreak bumps or resets the consecutive-day completion streak on task
// done. The streak is planning-domain state, read from the cached runtime
// snapshot and committed to planning.json alone rather than through a whole-config
// save.
func (m *AppModel) updateStreak() {
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	planning := m.runtime.Planning()
	if planning.GetStreakLastDate() == today {
		return
	}
	newCount := 1
	if planning.GetStreakLastDate() == yesterday {
		newCount = planning.GetStreakCount() + 1
	}
	if err := m.commitPlanningMutation(func(p *config.PlanningState) {
		p.SetStreakCount(newCount)
		p.SetStreakLastDate(today)
	}); err != nil {
		m.err = err
	}
}

// maybeFireTodayCleared evaluates whether this status mutation cleared the
// Today plan for the first time today. When it did, it sets the Today-cleared
// banner (with the current streak count) and returns the banner-timeout cmd.
// Returns nil when the banner should not fire (Today empty, not yet complete,
// or already fired today).
func (m *AppModel) maybeFireTodayCleared() tea.Cmd {
	if m.runtime == nil {
		return nil
	}
	loops := m.openLoops
	if loops.TodayTotal == 0 || loops.TodayDone != loops.TodayTotal {
		return nil
	}
	today := time.Now().Format("2006-01-02")
	if m.runtime.Planning().GetTodayClearedAt() == today {
		return nil
	}
	if err := m.commitPlanningMutation(func(p *config.PlanningState) {
		p.SetTodayClearedAt(today)
	}); err != nil {
		m.err = err
	}

	streak := m.runtime.Planning().GetStreakCount()
	unit := "day"
	if streak != 1 {
		unit = "days"
	}
	m.todayClearedBanner = fmt.Sprintf("%s Today cleared. Streak: %d %s.", icons.Get("streak"), streak, unit)
	return todayClearedBannerTimeout()
}
