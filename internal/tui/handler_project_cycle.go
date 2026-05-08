package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleProjectCycleMessages handles project cycling messages.
// Returns true if the message was handled.
func (m *AppModel) handleProjectCycleMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowProjectCycleManagerMsg:
		m.showProjectCycle = !m.showProjectCycle
		m.projectCycleCursor = 0
		if m.showProjectCycle {
			m.refreshProjectCycleItems()
			m.sortCycleItemsSelectedFirst()
		}
		return true, nil

	case ShowProjectCyclePickerMsg:
		return m.handleShowProjectCyclePicker()

	case CycleProjectMsg:
		return m.handleCycleProject(msg.Forward)
	}
	return false, nil
}

// handleShowProjectCyclePicker opens the numbered project picker popup.
func (m *AppModel) handleShowProjectCyclePicker() (bool, tea.Cmd) {
	contextKey := m.config.FilterContext
	if m.config.FilterCustomContext != "" {
		contextKey = "+" + m.config.FilterCustomContext
	}
	cycleProjects := m.config.GetCycleProjectsForContext(contextKey)

	if len(cycleProjects) == 0 {
		if m.config.FilterCustomContext != "" {
			m.setInfoStatus(fmt.Sprintf("No projects in cycle for +%s. Press @ to configure.", m.config.FilterCustomContext))
		} else if m.config.FilterContext != "" {
			m.setInfoStatus(fmt.Sprintf("No projects in cycle for context: %s. Press @ to configure.", m.config.FilterContext))
		} else {
			m.setInfoStatus("No projects in cycle. Press @ to configure.")
		}
		return true, defaultStatusTimeout()
	}

	if len(cycleProjects) == 1 {
		if m.currentProject == nil || m.currentProject.ID != cycleProjects[0].ID {
			return m.switchToCycleProject(cycleProjects[0])
		}
		m.setInfoStatus("Only one project in cycle")
		return true, defaultStatusTimeout()
	}

	m.showProjectCyclePicker = true
	return true, nil
}

// handleProjectCyclePickerKey handles key events when the project cycle picker is open.
func (m *AppModel) handleProjectCyclePickerKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	if isModifierOnly(msg) {
		return m, nil
	}
	switch msg.String() {
	case "escape", "`":
		m.showProjectCyclePicker = false
		return m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		contextKey := m.config.FilterContext
		if m.config.FilterCustomContext != "" {
			contextKey = "+" + m.config.FilterCustomContext
		}
		cycleProjects := m.config.GetCycleProjectsForContext(contextKey)
		idx := int(msg.String()[0]-'0') - 1
		if idx < len(cycleProjects) {
			m.showProjectCyclePicker = false
			_, cmd := m.switchToCycleProject(cycleProjects[idx])
			return m, cmd
		}
		return m, nil
	}
	return m, nil
}

// buildProjectCyclePickerHints builds the hints for the project cycle picker popup.
func (m *AppModel) buildProjectCyclePickerHints() PrefixHints {
	contextKey := m.config.FilterContext
	if m.config.FilterCustomContext != "" {
		contextKey = "+" + m.config.FilterCustomContext
	}
	cycleProjects := m.config.GetCycleProjectsForContext(contextKey)

	var hints []PrefixHint
	for i, p := range cycleProjects {
		if i >= 9 {
			break
		}
		hint := PrefixHint{
			Key:         fmt.Sprintf("%d", i+1),
			Description: p.Name,
			Active:      m.currentProject != nil && m.currentProject.ID == p.ID,
		}
		hints = append(hints, hint)
	}
	return PrefixHints{"`": hints}
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
			m.setInfoStatus(fmt.Sprintf("No projects in cycle for +%s. Press @ to configure.", m.config.FilterCustomContext))
		} else if m.config.FilterContext != "" {
			m.setInfoStatus(fmt.Sprintf("No projects in cycle for context: %s. Press @ to configure.", m.config.FilterContext))
		} else {
			m.setInfoStatus("No projects in cycle. Press @ to configure.")
		}
		return true, defaultStatusTimeout()
	}

	if len(cycleProjects) == 1 {
		// Single project in cycle - switch to it if not current
		if m.currentProject == nil || m.currentProject.ID != cycleProjects[0].ID {
			return m.switchToCycleProject(cycleProjects[0])
		}
		m.setInfoStatus("Only one project in cycle")
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

	return m.switchToCycleProject(cycleProjects[nextIdx])
}

// switchToCycleProject switches to a project from the cycle list.
func (m *AppModel) switchToCycleProject(project *models.Project) (bool, tea.Cmd) {
	m.saveCurrentProjectView()
	m.saveProjectTreeState()
	m.saveOutlinerToggleState()
	m.saveOutlinerFilterState()
	m.currentProject = project
	m.config.CurrentProject = project.ID
	if err := m.configManager.Save(m.config); err != nil {
		return true, errorCmd("failed to save config: %w", err)
	}

	_ = m.persister.Load(project)
	m.config.ApplyGlobalColorsToProject(project)
	m.projectTree = NewProjectTreeModel(project, m.config, m.configManager)
	m.projectTree.queueIDs = m.focusQueueIDs
	m.projectTree.dailyPlanIDs = m.dailyPlanIDs
	m.projectTree.SetSize(m.width, m.height-2)
	m.restoreProjectTreeState()
	m.views.Register(ViewProjectTree, m.projectTree)

	// Determine which project view to show: preserve current view if it's a
	// project-level view, otherwise use the last project view the user was on.
	targetView := m.currentView()
	if targetView != ViewProjectTree && targetView != ViewOutliner {
		targetView = m.lastProjectView
	}

	switch targetView {
	case ViewOutliner:
		m.saveOutlinerToggleState()
		m.saveOutlinerFilterState()
		m.outliner = NewOutlinerModel(project, m.config, m.configManager)
		m.outliner.queueIDs = m.focusQueueIDs
		m.outliner.dailyPlanIDs = m.dailyPlanIDs
		m.restoreOutlinerToggleState()
		m.restoreOutlinerFilterState()
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

	return true, nil
}
