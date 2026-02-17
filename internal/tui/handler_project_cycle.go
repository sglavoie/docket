package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
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
		// Refresh dashboard to reflect any pin changes
		var dash *DashboardModel
		if v := m.views.Get(ViewDashboard); v != nil {
			dash, _ = v.(*DashboardModel)
		}
		if dash == nil {
			dash = m.dashboard
		}
		if dash != nil {
			dash.Refresh(m.config)
		}
		if msg.Changed {
			contextKey := m.config.FilterContext
			if m.config.FilterCustomContext != "" {
				contextKey = "+" + m.config.FilterCustomContext
			}
			count := len(m.config.GetCycleProjectsForContext(contextKey))
			if count == 0 {
				m.statusMessage = "Project cycle cleared"
			} else {
				contextInfo := ""
				if m.config.FilterCustomContext != "" {
					contextInfo = fmt.Sprintf(" for +%s", m.config.FilterCustomContext)
				} else if m.config.FilterContext != "" {
					contextInfo = fmt.Sprintf(" for %s", m.config.FilterContext)
				}
				m.statusMessage = fmt.Sprintf("Project cycle updated%s: %d projects", contextInfo, count)
			}
			return true, defaultStatusTimeout()
		}
		return true, nil

	case CycleProjectMsg:
		return m.handleCycleProject(msg.Forward)
	}
	return false, nil
}

// handleCycleProject cycles through configured projects.
func (m *AppModel) handleCycleProject(forward bool) (bool, tea.Cmd) {
	// Use context-specific cycle list when a context filter is active
	// Custom context takes precedence over regular context
	contextKey := m.config.FilterContext
	if m.config.FilterCustomContext != "" {
		contextKey = "+" + m.config.FilterCustomContext
	}
	cycleProjects := m.config.GetCycleProjectsForContext(contextKey)

	if len(cycleProjects) == 0 {
		if m.config.FilterCustomContext != "" {
			m.statusMessage = fmt.Sprintf("No projects in cycle for +%s. Press @ to configure.", m.config.FilterCustomContext)
		} else if m.config.FilterContext != "" {
			m.statusMessage = fmt.Sprintf("No projects in cycle for context: %s. Press @ to configure.", m.config.FilterContext)
		} else {
			m.statusMessage = "No projects in cycle. Press @ to configure."
		}
		return true, defaultStatusTimeout()
	}

	if len(cycleProjects) == 1 {
		// Single project in cycle - switch to it if not current
		if m.currentProject == nil || m.currentProject.ID != cycleProjects[0].ID {
			return m.switchToCycleProject(cycleProjects[0], len(cycleProjects))
		}
		m.statusMessage = "Only one project in cycle"
		return true, defaultStatusTimeout()
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

	return m.switchToCycleProject(cycleProjects[nextIdx], len(cycleProjects))
}

// switchToCycleProject switches to a project from the cycle list.
// cycleTotal is the number of projects in the filtered cycle list.
func (m *AppModel) switchToCycleProject(project *models.Project, cycleTotal int) (bool, tea.Cmd) {
	m.saveProjectTreeState()
	m.currentProject = project
	m.config.CurrentProject = project.ID
	if err := m.configManager.Save(m.config); err != nil {
		return true, errorCmd("failed to save config: %w", err)
	}

	_ = m.persister.Load(project)
	m.config.ApplyGlobalColorsToProject(project)
	m.projectTree = NewProjectTreeModel(project, m.config, m.configManager)
	m.projectTree.SetSize(m.width, m.height-2)
	m.restoreProjectTreeState()
	m.views.Register(ViewProjectTree, m.projectTree)

	// Determine which project view to show: preserve current view if it's a
	// project-level view, otherwise use the last project view the user was on.
	targetView := m.currentView()
	if targetView != ViewProjectTree && targetView != ViewKanban && targetView != ViewOutliner {
		targetView = m.lastProjectView
	}

	switch targetView {
	case ViewKanban:
		m.kanban = NewKanbanViewModel(project, m.config, m.configManager)
		m.kanban.projectTree = m.projectTree
		m.restoreKanbanState()
		m.kanban.SetSize(m.width, m.height-2)
		m.views.Register(ViewKanban, m.kanban)
		if m.currentView() != ViewKanban {
			m.pushView(ViewKanban)
		}
	case ViewOutliner:
		m.outliner = NewOutlinerModel(project, m.config, m.configManager)
		m.outliner.SetSize(m.width, m.height-2)
		m.views.Register(ViewOutliner, m.outliner)
		if m.currentView() != ViewOutliner {
			m.pushView(ViewOutliner)
		}
	default:
		// ProjectTree or no prior project view — navigate to project tree
		if m.currentView() != ViewProjectTree {
			m.pushView(ViewProjectTree)
		}
	}

	// Show cycle position in status (using filtered count)
	m.statusMessage = fmt.Sprintf("Switched to: %s [%d projects in cycle]", project.Name, cycleTotal)
	return true, defaultStatusTimeout()
}
