package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleProjectCycleMessages handles project cycling messages.
// Returns true if the message was handled.
func (m *AppModel) handleProjectCycleMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowProjectCycleManagerMsg:
		m.showView(ViewProjectCycleManager, NewProjectCycleManagerModel(m.config, m.configManager))
		return true, nil

	case CloseProjectCycleManagerMsg:
		m.popView()
		// Rebuild project tree since buildProjectInfoList() reloads all projects,
		// which replaces task objects and invalidates the tree's task pointers
		if m.projectTree != nil {
			m.projectTree.buildTree()
		}
		if msg.Changed {
			count := len(m.config.CycleProjects)
			if count == 0 {
				m.statusMessage = "Project cycle cleared"
			} else {
				m.statusMessage = fmt.Sprintf("Project cycle updated: %d projects", count)
			}
			return true, statusTimeout(2 * time.Second)
		}
		return true, nil

	case CycleProjectMsg:
		return m.handleCycleProject(msg.Forward)
	}
	return false, nil
}

// handleCycleProject cycles through configured projects.
func (m *AppModel) handleCycleProject(forward bool) (bool, tea.Cmd) {
	cycleProjects := m.config.GetCycleProjects()

	if len(cycleProjects) == 0 {
		m.statusMessage = "No projects in cycle. Press C to configure."
		return true, statusTimeout(2 * time.Second)
	}

	if len(cycleProjects) == 1 {
		// Single project in cycle - switch to it if not current
		if m.currentProject == nil || m.currentProject.ID != cycleProjects[0].ID {
			return m.switchToCycleProject(cycleProjects[0])
		}
		m.statusMessage = "Only one project in cycle"
		return true, statusTimeout(2 * time.Second)
	}

	// Find current project's index in cycle list
	currentIdx := -1
	if m.currentProject != nil {
		for i, p := range cycleProjects {
			if p.ID == m.currentProject.ID {
				currentIdx = i
				break
			}
		}
	}

	// Calculate next index
	var nextIdx int
	if currentIdx < 0 {
		// Current project not in cycle list, jump to first
		nextIdx = 0
	} else if forward {
		nextIdx = (currentIdx + 1) % len(cycleProjects)
	} else {
		nextIdx = (currentIdx - 1 + len(cycleProjects)) % len(cycleProjects)
	}

	return m.switchToCycleProject(cycleProjects[nextIdx])
}

// switchToCycleProject switches to a project from the cycle list.
func (m *AppModel) switchToCycleProject(project *models.Project) (bool, tea.Cmd) {
	m.saveProjectTreeState()
	m.currentProject = project
	m.config.CurrentProject = project.ID
	if err := m.configManager.Save(m.config); err != nil {
		return true, errorCmd("failed to save config: %w", err)
	}

	storage.LoadProject(project)
	m.config.ApplyGlobalColorsToProject(project)
	m.projectTree = NewProjectTreeModel(project, m.config, m.configManager)
	m.projectTree.SetSize(m.width, m.height-2)
	m.restoreProjectTreeState()
	m.views.Register(ViewProjectTree, m.projectTree)

	// Navigate to project tree if not already there
	if m.currentView() != ViewProjectTree {
		m.pushView(ViewProjectTree)
	}

	// Show cycle position in status
	cycleProjects := m.config.GetCycleProjects()
	idx := m.config.GetCycleProjectIndex(project.ID)
	m.statusMessage = fmt.Sprintf("Switched to: %s [%d/%d]", project.Name, idx+1, len(cycleProjects))
	return true, statusTimeout(2 * time.Second)
}
