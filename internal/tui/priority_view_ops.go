package tui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// taskOpsView implementation for PriorityViewModel.

func (m *PriorityViewModel) getTaskRef() (*models.Task, *models.Project) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return nil, nil
	}
	return pt.Task, m.project
}

func (m *PriorityViewModel) getPersister() Persister { return m.projectService }
func (m *PriorityViewModel) getDataDir() string      { return m.config.DataDir }
func (m *PriorityViewModel) reloadData()             { m.loadActionableTasks() }
func (m *PriorityViewModel) setErr(err error)        { m.SetError(err) }
func (m *PriorityViewModel) setStatus(msg string)    { m.statusMessage = msg }

func (m *PriorityViewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	return taskOpsChangeStatus(m, newStatus)
}

func (m *PriorityViewModel) changeTaskPriority(newPriority int) (tea.Model, tea.Cmd) {
	return taskOpsChangePriority(m, strconv.Itoa(newPriority))
}

func (m *PriorityViewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	return taskOpsSetQuickDueDate(m, daysFromNow)
}

func (m *PriorityViewModel) clearDueDate() (tea.Model, tea.Cmd) {
	return taskOpsClearDueDate(m)
}

func (m *PriorityViewModel) sectionMove(section string) (tea.Model, tea.Cmd) {
	return taskOpsSectionMove(m, section)
}

func (m *PriorityViewModel) deleteTask() (tea.Model, tea.Cmd) {
	return taskOpsDeleteTask(m)
}
