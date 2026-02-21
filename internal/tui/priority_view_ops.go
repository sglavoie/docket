package tui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
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

func (m *PriorityViewModel) getPersister() Persister           { return m.projectService }
func (m *PriorityViewModel) getConfig() *config.Config         { return m.config }
func (m *PriorityViewModel) getConfigManager() *config.Manager { return m.configManager }
func (m *PriorityViewModel) getDataDir() string                { return m.config.DataDir }
func (m *PriorityViewModel) reloadData() {
	_ = m.projectService.Load(m.project)
	m.dismissedIDs = make(map[string]bool)
	m.loadActionableTasks()
}
func (m *PriorityViewModel) setErr(err error)     { m.SetError(err) }
func (m *PriorityViewModel) setStatus(msg string) { m.statusMessage = msg }

// commonActionView methods for PriorityViewModel.

func (m *PriorityViewModel) viewID() ViewType { return ViewPriorityView }
func (m *PriorityViewModel) viewName() string { return "Priority View" }

func (m *PriorityViewModel) changeTaskPriority(newPriority int) (tea.Model, tea.Cmd) {
	return taskOpsChangePriority(m, strconv.Itoa(newPriority))
}

func (m *PriorityViewModel) deleteTask() (tea.Model, tea.Cmd) {
	return taskOpsDeleteTask(m)
}
