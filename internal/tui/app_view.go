package tui

import (
	"fmt"
	"strings"

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

	// Hide status bar for command palette, tutor, and help (they render their own footer)
	if m.currentView() == ViewCommandPalette || m.currentView() == ViewTutor || m.currentView() == ViewHelp {
		return content
	}

	// Add status bar
	statusBar := m.renderStatusBar()
	statusBarHeight := lipgloss.Height(statusBar)

	// Pad content so the status bar is always at the bottom of the terminal
	contentHeight := lipgloss.Height(content)
	targetContentHeight := m.height - statusBarHeight
	if contentHeight < targetContentHeight {
		content += strings.Repeat("\n", targetContentHeight-contentHeight)
	}

	return content + "\n" + statusBar
}

func (m *AppModel) renderStatusBar() string {
	// Top border separator for visual distinction
	separator := MutedStyle.Render(strings.Repeat("─", m.width))

	// Show loading spinner if active
	if m.isLoading {
		spinnerView := m.spinner.View()
		msg := LoadingStyle.Render(spinnerView + " " + m.loadingMessage)
		gap := m.width - lipgloss.Width(msg)
		if gap < 0 {
			gap = 0
		}
		spacer := lipgloss.NewStyle().Width(gap).Render("")
		return separator + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, msg, spacer)
	}

	// Show error message if present
	if m.err != nil {
		msg := ErrorStatusStyle.Render("✗ " + m.err.Error())
		gap := m.width - lipgloss.Width(msg)
		if gap < 0 {
			gap = 0
		}
		spacer := lipgloss.NewStyle().Width(gap).Render("")
		return separator + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, msg, spacer)
	}

	// Show temporary status message if present
	if m.statusMessage != "" {
		msg := SuccessStyle.Render("✓ " + m.statusMessage)
		gap := m.width - lipgloss.Width(msg)
		if gap < 0 {
			gap = 0
		}
		spacer := lipgloss.NewStyle().Width(gap).Render("")
		return separator + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, msg, spacer)
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
			if m.projectTree != nil && m.projectTree.sidebar.IsFocused() {
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
	case ViewTaskEditor:
		status = "Edit Task"
	case ViewSettings:
		status = "Settings"
	case ViewHelp:
		status = "Help" // fallback; help view renders its own status bar
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
	case ViewKanbanTagFilterSelector:
		status = "Kanban: Filter by Tags"
	case ViewSectionFilterSelector:
		status = "Filter by Sections"
	case ViewSectionPicker:
		status = "Assign Section"
	case ViewPriorityPicker:
		status = "Set Priority"
	case ViewSectionManager:
		status = "Custom Section Manager"
	case ViewContextPicker:
		status = "Filter by Context"
	case ViewCustomContextEditor:
		status = "Custom Context Editor"
	case ViewGlobalSearch:
		status = "Global Search"
	case ViewWorkflowRules:
		status = "Workflow Rules"
	case ViewKanban:
		if m.currentProject != nil {
			status = fmt.Sprintf("Kanban: %s", m.currentProject.Name)
		} else {
			status = "Kanban"
		}
	case ViewNotes:
		status = "Notes"
	case ViewGlobalNotes:
		status = "Global Notes"
	case ViewNotesSearch:
		status = "Notes Search"
	case ViewNoteTagFilterSelector:
		status = "Filter Notes by Tags"
	}

	// Hide help text in editor/creator/settings/search views where ? doesn't apply
	currentView := m.currentView()
	showHelp := currentView != ViewTaskCreator &&
		currentView != ViewTaskEditor &&
		currentView != ViewProjectCreator &&
		currentView != ViewProjectEditor &&
		currentView != ViewSettings &&
		currentView != ViewTaskJumper &&
		currentView != ViewGlobalSearch &&
		currentView != ViewDependencyViewer &&
		currentView != ViewCustomContextEditor &&
		currentView != ViewTaskActivity

	help := ""
	if showHelp && m.isNotesFullscreen() {
		showHelp = false
	}
	if showHelp {
		help = "Press : for commands, ? for help"
		// Add Kanban-specific help when in Kanban view
		if currentView == ViewKanban && m.kanban != nil {
			if m.kanban.detailsPaneFocused {
				help = "j/k: scroll | Enter: glow | Esc/q: back | " + help
			} else {
				help = "Tab: details | " + help
			}
		}
	}

	var left string
	if m.currentView() == ViewDashboard && !m.config.GetTutorDismissed() {
		blue := lipgloss.NewStyle().Foreground(ColorPrimary)
		left = StatusBarStyle.Render("Dashboard") + ": Run " + blue.Render("tutor") + " command to start. Dismiss with " + blue.Render("|")
	} else {
		left = StatusBarStyle.Render(status)
	}
	right := HelpStyle.Render(help)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	spacer := lipgloss.NewStyle().Width(gap).Render("")

	return separator + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}
