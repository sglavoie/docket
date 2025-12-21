package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m *AppModel) View() string {
	if m.quitting {
		return ""
	}

	var content string

	switch m.currentView() {
	case ViewDashboard:
		if m.dashboard != nil {
			content = m.dashboard.View()
		}
	case ViewProjectList:
		if m.projectList != nil {
			content = m.projectList.View()
		}
	case ViewProjectTree:
		if m.projectTree != nil {
			content = m.projectTree.View()
		}
	case ViewTaskDetail:
		if m.taskDetail != nil {
			content = m.taskDetail.View()
		}
	case ViewProjectCreator:
		if m.projectCreator != nil {
			content = m.projectCreator.View()
		}
	case ViewProjectEditor:
		if m.projectEditor != nil {
			content = m.projectEditor.View()
		}
	case ViewTaskCreator:
		if m.taskCreator != nil {
			content = m.taskCreator.View()
		}
	case ViewTaskEditor:
		if m.taskEditor != nil {
			content = m.taskEditor.View()
		}
	case ViewSettings:
		if m.settings != nil {
			content = m.settings.View()
		}
	case ViewHelp:
		if m.helpView != nil {
			content = m.helpView.View()
		}
	case ViewDateInput:
		if m.dateInput != nil {
			content = m.dateInput.View()
		}
	case ViewDependencySelector:
		if m.dependencySelector != nil {
			content = m.dependencySelector.View()
		}
	case ViewFilter:
		if m.filterView != nil {
			content = m.filterView.View()
		}
	case ViewTaskJumper:
		if m.taskJumper != nil {
			content = m.taskJumper.View()
		}
	case ViewProjectPreview:
		if m.projectPreview != nil {
			content = m.projectPreview.View()
		}
	case ViewDueDateOverview:
		if m.dueDateOverview != nil {
			content = m.dueDateOverview.View()
		}
	case ViewTagManager:
		if m.tagManager != nil {
			content = m.tagManager.View()
		}
	case ViewGlobalTagManager:
		if m.globalTagManager != nil {
			content = m.globalTagManager.View()
		}
	case ViewTagFilterSelector:
		if m.tagFilterSelector != nil {
			content = m.tagFilterSelector.View()
		}
	case ViewContextPicker:
		if m.contextPicker != nil {
			content = m.contextPicker.View()
		}
	default:
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
		} else {
			status = "Project"
		}
	case ViewTaskDetail:
		status = "Task Details"
	case ViewProjectCreator:
		status = "New Project"
	case ViewProjectEditor:
		status = "Edit Project"
	case ViewTaskCreator:
		status = "New Task"
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
	case ViewContextPicker:
		status = "Filter by Context"
	}

	help := "Press ? for help"

	left := StatusBarStyle.Render(status)
	right := HelpStyle.Render(help)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	spacer := lipgloss.NewStyle().Width(gap).Render("")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}
