package tui

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// resolveCockpitActionForKey centralizes Today's context-sensitive action
// grammar so render hints and key handlers do not infer behavior separately.
func (m *FocusBoardModel) resolveCockpitActionForKey(key string) CockpitAction {
	switch key {
	case "a":
		return actionQuickCapture()
	case "/":
		return actionSearchFilter()
	case "enter":
		return actionOpenTask()
	case "x":
		if m.shouldDismissSelectedTask() {
			return actionDismissTask()
		}
		return actionCompleteTask()
	case "p":
		if row := m.getRowAtCursor(); row != nil && row.isTask() {
			return actionPrioritizeTask()
		}
		rec := m.resolveCockpitRecommendation()
		switch rec.PrimaryAction.Command {
		case CockpitCommandNewProject,
			CockpitCommandPlanToday,
			CockpitCommandPlanMore,
			CockpitCommandTriageInbox:
			return rec.PrimaryAction
		default:
			return actionPlanToday()
		}
	case "u":
		if m.canUnblockSelectedTask() {
			return actionUnblockTask()
		}
	}
	return CockpitAction{}
}

func (m *FocusBoardModel) resolveCockpitRecommendation() CockpitRecommendation {
	return BuildCockpitRecommendation(m.computeNextActionRecommendation(), CockpitContext{
		HasProjects: m.hasAnyProjectsForRecommendation(),
		OpenLoops:   m.openLoops,
	})
}

func (m *FocusBoardModel) executeCockpitAction(action CockpitAction) (tea.Model, tea.Cmd) {
	switch action.Command {
	case CockpitCommandQuickCapture:
		m.addMode = true
		m.addInput.SetValue("")
		m.addInput.Focus()
		return m, textinput.Blink
	case CockpitCommandSearchFilter:
		m.searchInput.SetValue(m.searchQuery)
		return m, m.ActivateSearch()
	case CockpitCommandOpenTask:
		return m.handleEnter()
	case CockpitCommandCompleteTask:
		return m.markDone()
	case CockpitCommandDismissTask:
		return m.dismiss()
	case CockpitCommandPrioritizeTask:
		row := m.getRowAtCursor()
		if row == nil || row.Task == nil {
			return m, nil
		}
		return m, func() tea.Msg { return ShowPriorityPickerMsg{Task: row.Task, Project: row.Project} }
	case CockpitCommandUnblockTask:
		return m.unblockSelectedTask()
	case CockpitCommandPlanToday, CockpitCommandPlanMore:
		return m, func() tea.Msg { return ShowFocusViewMsg{} }
	case CockpitCommandTriageInbox:
		return m, func() tea.Msg { return ShowGlobalInboxProcessorMsg{} }
	case CockpitCommandNewProject:
		return m, func() tea.Msg { return CreateProjectMsg{} }
	case CockpitCommandSwitchTab:
		m.tabs.Cycle(1)
		m.applyFilter()
		m.clampCursor()
		return m, nil
	case CockpitCommandOpenPipelineHub:
		return m, func() tea.Msg { return ShowPipelineHubMsg{} }
	case CockpitCommandOpenCommandPalette:
		return m, func() tea.Msg { return ShowCommandPaletteMsg{} }
	case CockpitCommandShowHelp:
		return m.toggleHelpAction()
	case CockpitCommandCloseView:
		return m, func() tea.Msg { return BackMsg{} }
	default:
		return m, nil
	}
}

func (m *FocusBoardModel) canUnblockSelectedTask() bool {
	row := m.getRowAtCursor()
	if row == nil || row.Task == nil || row.Project == nil {
		return false
	}
	return row.Task.Status == models.StatusBlocked && !row.Project.HasIncompleteDependencies(row.Task)
}

func (m *FocusBoardModel) unblockSelectedTask() (tea.Model, tea.Cmd) {
	if !m.canUnblockSelectedTask() {
		if row := m.getRowAtCursor(); row != nil && row.Task != nil && row.Project != nil && row.Project.HasIncompleteDependencies(row.Task) {
			m.setStatus("Complete blockers before unblocking")
		}
		return m, nil
	}
	cmd := taskOpsApply(m, "Unblock Task", applyFocusBoardUnblock)
	m.setMutationFlash(statusMutationFlashBg(models.StatusTodo))
	return m, tea.Batch(cmd, mutationFlashTimeout())
}

func applyFocusBoardUnblock(pf persistFn, project *models.Project, taskID string) TaskOpResult {
	if project == nil {
		return TaskOpResult{Err: errNilProject}
	}
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}
	if task.Status != models.StatusBlocked {
		return TaskOpResult{StatusMessage: "Task is not blocked"}
	}

	result := pf(project, taskID, func(t *models.Task) {
		t.ApplyTransition(models.StatusTodo)
		t.Section = models.SectionNext
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	result.StatusMessage = fmt.Sprintf("Task #%d unblocked (u to undo)", task.ShortID)
	result.Cmd = func() tea.Msg { return TaskStatusChangedMsg{Task: task} }
	return result
}

func (m *FocusBoardModel) shouldDismissSelectedTask() bool {
	row := m.getRowAtCursor()
	if row == nil || row.Task == nil {
		return false
	}
	return !m.selectedTaskCanComplete(row) && row.Task.CanTransitionTo(models.StatusCanceled) == ""
}

func (m *FocusBoardModel) selectedTaskCanComplete(row *boardRow) bool {
	if row == nil || row.Task == nil {
		return false
	}
	if row.Task.CanTransitionTo(models.StatusDone) != "" {
		return false
	}
	if row.Project != nil && row.Project.HasIncompleteDependencies(row.Task) {
		return false
	}
	for _, child := range row.Task.Children {
		if !child.Status.IsComplete() {
			return false
		}
	}
	return true
}
