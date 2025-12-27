package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m *AppModel) View() string {
	if m.quitting {
		return ""
	}

	content := m.views.View(m.currentView())
	if content == "" {
		content = "Unknown view"
	}

	// Add status bar
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, content, statusBar)
}

func (m *AppModel) renderStatusBar() string {
	// Show temporary status message if present
	if m.statusMessage != "" {
		msg := SuccessStyle.Render("✓ " + m.statusMessage)
		gap := m.width - lipgloss.Width(msg)
		if gap < 0 {
			gap = 0
		}
		spacer := lipgloss.NewStyle().Width(gap).Render("")
		return lipgloss.JoinHorizontal(lipgloss.Top, msg, spacer)
	}

	var status string

	switch m.currentView() {
	case ViewDashboard:
		status = "Dashboard"
	case ViewProjectList:
		status = "Projects"
	case ViewProjectTree:
		if m.currentProject != nil {
			status = fmt.Sprintf("Project: %s", m.currentProject.Name)
			// Add cycle position indicator if project is in cycle list
			cycleProjects := m.config.GetCycleProjects()
			if len(cycleProjects) > 0 {
				for i, p := range cycleProjects {
					if p.ID == m.currentProject.ID {
						status += fmt.Sprintf(" [%d/%d]", i+1, len(cycleProjects))
						break
					}
				}
			}
			// Add sidebar focus indicator if sidebar is focused
			if m.projectTree != nil && m.projectTree.sidebarFocused {
				status += " " + SidebarFocusBadgeStyle.Render("[SIDEBAR]")
			}
		} else {
			status = "Project"
		}
	case ViewProjectCreator:
		status = "New Project"
	case ViewProjectEditor:
		status = "Edit Project"
	case ViewTaskCreator:
		status = "New Task"
	case ViewNoteCreator:
		status = "New Note"
	case ViewTaskEditor:
		status = "Edit Task"
	case ViewSettings:
		status = "Settings"
	case ViewHelp:
		status = "Help"
	case ViewDateInput:
		status = "Set Due Date"
	case ViewFilter:
		status = "Filters"
	case ViewTaskJumper:
		status = "Jump to Task"
	case ViewProjectPreview:
		status = "Project Preview"
	case ViewDueDateOverview:
		status = "Due Dates"
	case ViewTagManager:
		status = "Tag Manager"
	case ViewGlobalTagManager:
		status = "Global Tags"
	case ViewTagFilterSelector:
		status = "Filter by Tags"
	case ViewSectionFilterSelector:
		status = "Filter by Sections"
	case ViewSectionPicker:
		status = "Assign Section"
	case ViewSectionManager:
		status = "Section Manager"
	case ViewContextPicker:
		status = "Filter by Context"
	case ViewGlobalSearch:
		status = "Global Search"
	}

	// Hide help text in editor/creator/settings/search views where ? doesn't apply
	currentView := m.currentView()
	showHelp := currentView != ViewTaskCreator &&
		currentView != ViewNoteCreator &&
		currentView != ViewTaskEditor &&
		currentView != ViewProjectCreator &&
		currentView != ViewProjectEditor &&
		currentView != ViewSettings &&
		currentView != ViewTaskJumper &&
		currentView != ViewGlobalSearch &&
		currentView != ViewDependencyViewer

	help := ""
	if showHelp {
		help = "Press ? for help"
	}

	left := StatusBarStyle.Render(status)
	right := HelpStyle.Render(help)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	spacer := lipgloss.NewStyle().Width(gap).Render("")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}
