package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// handleProjectCycleMessages handles project cycling messages.
// Returns true if the message was handled.
func (m *AppModel) handleProjectCycleMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowProjectCycleManagerMsg:
		m.showProjectCycle = !m.showProjectCycle
		m.projectCycleCursor = 0
		m.projectCycleScroll = 0
		if m.showProjectCycle {
			m.resetProjectCycleSearch()
			m.refreshProjectCycleItems()
		} else {
			m.resetProjectCycleSearch()
			m.invalidateCyclePreview()
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
	contextKey := m.cycleContextKey()
	cycleProjects := m.runtime.CycleProjectsForContext(contextKey)

	if len(cycleProjects) == 0 {
		m.setInfoStatus(m.noCycleProjectsStatus(contextKey))
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
	case "esc", "`":
		m.showProjectCyclePicker = false
		return m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		machine := m.runtime.Machine()
		contextKey := machine.FilterContext
		if machine.FilterCustomContext != "" {
			contextKey = "+" + machine.FilterCustomContext
		}
		cycleProjects := m.runtime.CycleProjectsForContext(contextKey)
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
	contextKey := m.cycleContextKey()
	cycleProjects := m.runtime.CycleProjectsForContext(contextKey)

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
	contextKey := m.cycleContextKey()
	cycleProjects := m.runtime.CycleProjectsForContext(contextKey)

	if len(cycleProjects) == 0 {
		m.setInfoStatus(m.noCycleProjectsStatus(contextKey))
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

func (m *AppModel) cycleContextKey() string {
	machine := m.runtime.Machine()
	return getContextKey(machine.FilterContext, machine.FilterCustomContext)
}

func (m *AppModel) noCycleProjectsStatus(contextKey string) string {
	machine := m.runtime.Machine()
	hiddenCount := m.runtime.ArchivedCycleProjectCountForContext(contextKey)
	return cycleNoProjectsStatus(machine.FilterContext, machine.FilterCustomContext, hiddenCount)
}

func cycleNoProjectsStatus(filterContext, filterCustomContext string, hiddenCount int) string {
	scope := "in cycle"
	if filterCustomContext != "" {
		scope = fmt.Sprintf("in cycle for +%s", filterCustomContext)
	} else if filterContext != "" {
		scope = fmt.Sprintf("in cycle for context: %s", filterContext)
	}

	if hiddenCount > 0 {
		return fmt.Sprintf(
			"No active projects %s; %s. Unarchive from Dashboard archived mode or press @ to configure.",
			scope,
			cycleArchivedHiddenText(hiddenCount),
		)
	}
	return fmt.Sprintf("No projects %s. Press @ to configure.", scope)
}

func cycleArchivedHiddenText(hiddenCount int) string {
	return fmt.Sprintf("%d archived %s hidden", hiddenCount, pluralize(hiddenCount, "project", "projects"))
}

// switchToCycleProject switches to a project from the cycle list.
func (m *AppModel) switchToCycleProject(project *models.Project) (bool, tea.Cmd) {
	cmd, _ := m.maybePromptPark("", func() tea.Cmd {
		_, cmd := m.switchToCycleProjectNow(project)
		return cmd
	})
	return true, cmd
}

func (m *AppModel) switchToCycleProjectNow(project *models.Project) (bool, tea.Cmd) {
	m.saveCurrentProjectView()
	m.saveProjectTreeState()
	m.saveOutlinerToggleState()
	m.saveOutlinerFilterState()
	m.currentProject = project
	if err := m.commitMachineMutation(func(machine *config.MachineState) {
		machine.CurrentProject = project.ID
	}); err != nil {
		return true, errorCmd("failed to save machine state: %w", err)
	}

	_ = m.persister.Load(project)
	m.runtime.Settings().ApplyGlobalColorsToProject(project)
	m.projectTree = NewProjectTreeModel(project, m.configManager, m.runtime)
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
		m.outliner = NewOutlinerModel(project, m.configManager, m.runtime)
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
