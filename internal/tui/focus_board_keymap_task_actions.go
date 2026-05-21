package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

func (m *FocusBoardModel) buildTaskKeymapDeps(hasItem func() bool, stages []string) TaskKeymapDeps {
	return TaskKeymapDeps{
		View:           m,
		HasItem:        hasItem,
		Stages:         stages,
		ToggleHelp:     m.toggleHelpAction,
		DeleteTask:     m.deleteTaskAction,
		StatusChange:   m.statusChangeAction,
		PriorityChange: m.priorityChangeAction,
		DueDateSet:     m.bulkSetDueDateAction,
		DueDateClear:   m.bulkClearDueDateAction,
		SnoozeDate:     m.snoozeDateAction,
		SectionMove:    m.sectionMoveAction,
		StageChange:    m.stageChangeAction,
		OpenURL:        m.openURLAction,
		MoveToProject:  m.moveToProjectAction,
		QuickNote:      m.quickNoteAction,
	}
}

func (m *FocusBoardModel) toggleHelpAction() (tea.Model, tea.Cmd) {
	return m, helpCmd(m.viewID(), m.viewName(), m.GetHelpItems())
}

func (m *FocusBoardModel) deleteTaskAction() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task != nil {
		m.showDeleteConfirm(task.ShortID)
	}
	return m, nil
}

func (m *FocusBoardModel) bulkSetDueDateAction(days int) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			dueDate := util.StartOfDay(time.Now().AddDate(0, 0, days))
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, "Set Due Date",
				func(proj *models.Project, taskID string) bool {
					proj.MutateTask(taskID, func(t *models.Task) {
						d := dueDate
						t.DueDate = &d
						t.HasTime = false
					})
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else {
				m.setStatus(fmt.Sprintf("Due date set on %d tasks", count))
			}
			return m, nil
		}
		return taskOpsSetQuickDueDate(m, days)
	}
}

func (m *FocusBoardModel) bulkClearDueDateAction() (tea.Model, tea.Cmd) {
	if m.multiSelectMode && m.SelectedCount() > 0 {
		items := m.getSelectedBulkItems()
		count, err := taskOpsApplyBulk(m, items, "Clear Due Date",
			func(proj *models.Project, taskID string) bool {
				proj.MutateTask(taskID, func(t *models.Task) {
					t.DueDate = nil
					t.HasTime = false
				})
				return true
			})
		m.ExitMultiSelect()
		if err != nil {
			m.setStatus(fmt.Sprintf("Error: %v", err))
		} else {
			m.setStatus(fmt.Sprintf("Cleared due date on %d tasks", count))
		}
		return m, nil
	}
	return taskOpsClearDueDate(m)
}

func (m *FocusBoardModel) snoozeDateAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		snoozeDate, _, ok := resolveQuickSnoozeDate(key)
		if !ok {
			return m, nil
		}
		if m.multiSelectMode && m.SelectedCount() > 0 {
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, "Snooze",
				func(proj *models.Project, taskID string) bool {
					proj.MutateTask(taskID, func(t *models.Task) { t.SnoozedUntil = snoozeDate })
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else {
				m.setStatus(fmt.Sprintf("Snoozed %d tasks", count))
			}
			return m, nil
		}
		return taskOpsQuickSnooze(m, snoozeDate)
	}
}

func (m *FocusBoardModel) statusChangeAction(status string) keymap.Action {
	s := models.TaskStatus(status)
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, StatusVerb(s)+" Task",
				func(proj *models.Project, taskID string) bool {
					t := proj.GetTaskByID(taskID)
					if t == nil || t.CanTransitionTo(s) != "" {
						return false
					}
					proj.MutateTask(taskID, func(t *models.Task) { t.ApplyTransition(s) })
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else if count == 0 {
				m.setStatus("No tasks were changed")
			} else {
				m.setStatus(fmt.Sprintf("Changed %d tasks to %s", count, s))
			}
			return m, nil
		}
		model, cmd := taskOpsChangeStatus(m, s)
		if s == models.StatusDone {
			m.setCompletionFlash()
			return model, tea.Batch(cmd, completionFlashScheduler())
		}
		m.setMutationFlash(statusMutationFlashBg(s))
		return model, tea.Batch(cmd, mutationFlashTimeout())
	}
}

func (m *FocusBoardModel) priorityChangeAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			p, _ := PriorityForPrefixKey(key)
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, "Set Priority",
				func(proj *models.Project, taskID string) bool {
					t := proj.GetTaskByID(taskID)
					if t == nil || t.Priority == p {
						return false
					}
					proj.MutateTask(taskID, func(t *models.Task) { t.Priority = p })
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else if count == 0 {
				m.setStatus("No tasks were changed")
			} else {
				m.setStatus(fmt.Sprintf("Set priority %d on %d tasks", p, count))
			}
			return m, nil
		}
		p, _ := PriorityForPrefixKey(key)
		model, cmd := taskOpsChangePriority(m, key)
		m.setMutationFlash(priorityMutationFlashBg(p))
		return model, tea.Batch(cmd, mutationFlashTimeout())
	}
}

func (m *FocusBoardModel) sectionMoveAction(section string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, "Move to Section",
				func(proj *models.Project, taskID string) bool {
					t := proj.GetTaskByID(taskID)
					if t == nil || t.InSection(section) {
						return false
					}
					proj.MutateTask(taskID, func(t *models.Task) { t.SetSection(section) })
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else if count == 0 {
				m.setStatus("No tasks were moved")
			} else {
				m.setStatus(fmt.Sprintf("Moved %d tasks to %s", count, section))
			}
			return m, nil
		}
		return taskOpsSectionMove(m, section)
	}
}

func (m *FocusBoardModel) stageChangeAction(stage string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, "Set Stage",
				func(proj *models.Project, taskID string) bool {
					t := proj.GetTaskByID(taskID)
					if t == nil || t.Stage == stage {
						return false
					}
					proj.MutateTask(taskID, func(t *models.Task) { t.Stage = stage })
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else if count == 0 {
				m.setStatus("No tasks were changed")
			} else {
				m.setStatus(fmt.Sprintf("Set stage on %d tasks", count))
			}
			return m, nil
		}
		return taskOpsChangeStage(m, stage)
	}
}

func (m *FocusBoardModel) openURLAction() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil || task.URL == "" {
		return m, nil
	}
	return m, openBrowserCmd(task.URL)
}

func (m *FocusBoardModel) moveToProjectAction() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
}

func (m *FocusBoardModel) quickNoteAction() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: project} }
}
