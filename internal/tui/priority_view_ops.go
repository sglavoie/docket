package tui

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

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

func (m *PriorityViewModel) getRuntime() *config.RuntimeState  { return m.runtime }
func (m *PriorityViewModel) getConfigManager() *config.Manager { return m.configManager }
func (m *PriorityViewModel) reloadData() {
	_ = m.projectService.Load(m.project)
	m.dismissedIDs = make(map[string]bool)
	m.loadActionableTasks()
}
func (m *PriorityViewModel) setStatus(msg string) { m.statusMessage = msg }

// commonActionView methods for PriorityViewModel.

func (m *PriorityViewModel) viewID() ViewType { return ViewPriorityView }
func (m *PriorityViewModel) viewName() string { return "Priority View" }

func (m *PriorityViewModel) changeTaskPriority(newPriority int) (tea.Model, tea.Cmd) {
	return m, wfChangePriority(m, strconv.Itoa(newPriority))
}

func (m *PriorityViewModel) deleteTask() (tea.Model, tea.Cmd) {
	return m, wfDeleteTask(m, m.runtime, m.configManager)
}
