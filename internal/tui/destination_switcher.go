package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const destinationSwitcherOverlayKey = "global-destination-switcher"

type ShowDestinationSwitcherMsg struct{}

// GlobalDestination distinguishes the two TaskWorkspace destinations (Tasks
// and Project) while keeping the final RouteID inventory at exactly seven.
type GlobalDestination uint8

const (
	DestinationHome GlobalDestination = iota
	DestinationPlanner
	DestinationTasks
	DestinationProject
	DestinationReview
	DestinationManage
)

type DestinationEntry struct {
	ID          GlobalDestination
	Name        string
	Description string
	Route       Route
	Enabled     bool
}

// GlobalDestinationEntries is the single six-entry Ctrl+G contract. Project
// is disabled until a project is selected; the other five destinations always
// resolve to canonical route parameters.
func GlobalDestinationEntries(projectID string) []DestinationEntry {
	mustRoute := func(id RouteID, params RouteParams) Route {
		route, err := NewRoute(id, params)
		if err != nil {
			panic(err)
		}
		return route
	}
	projectRoute := mustRoute(TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()})
	projectEnabled := projectID != ""
	if projectEnabled {
		projectRoute = mustRoute(TaskWorkspace, TaskWorkspaceParams{Scope: ProjectEntityScope(projectID)})
	}
	return []DestinationEntry{
		{ID: DestinationHome, Name: "Home", Description: "Projects and attention", Route: mustRoute(Home, HomeParams{}), Enabled: true},
		{ID: DestinationPlanner, Name: "Planner", Description: "Today, Next, and Backlog", Route: mustRoute(Planner, PlannerParams{Tab: PlannerToday}), Enabled: true},
		{ID: DestinationTasks, Name: "Tasks", Description: "All active tasks", Route: mustRoute(TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()}), Enabled: true},
		{ID: DestinationProject, Name: "Project", Description: "Current project tasks", Route: projectRoute, Enabled: projectEnabled},
		{ID: DestinationReview, Name: "Review", Description: "Reviews and reports", Route: mustRoute(ReviewHub, ReviewParams{Module: ReviewLanding}), Enabled: true},
		{ID: DestinationManage, Name: "Manage", Description: "Collections and settings", Route: mustRoute(ManageHub, ManageParams{Module: ManageLanding, Scope: AppEntityScope()}), Enabled: true},
	}
}

type DestinationSwitcher struct {
	entries []DestinationEntry
	cursor  int
}

func NewDestinationSwitcher(projectID string) *DestinationSwitcher {
	return &DestinationSwitcher{entries: GlobalDestinationEntries(projectID)}
}

func (m *AppModel) toggleDestinationSwitcher() {
	if m.destinationSwitcher != nil {
		m.closeDestinationSwitcher()
		return
	}
	projectID := ""
	if m.currentProject != nil {
		projectID = m.currentProject.ID
	}
	m.destinationSwitcher = NewDestinationSwitcher(projectID)
	m.pushTypedOverlay(OverlayPicker, destinationSwitcherOverlayKey)
}

func (m *AppModel) closeDestinationSwitcher() {
	m.destinationSwitcher = nil
	m.closeTypedOverlay(OverlayPicker, destinationSwitcherOverlayKey)
}

func (m *AppModel) handleDestinationSwitcherKey(msg tea.KeyPressMsg) tea.Cmd {
	switch key := msg.String(); key {
	case "esc", "ctrl+g":
		m.closeDestinationSwitcher()
		return nil
	case "j", "down":
		if m.destinationSwitcher.cursor < len(m.destinationSwitcher.entries)-1 {
			m.destinationSwitcher.cursor++
		}
	case "k", "up":
		if m.destinationSwitcher.cursor > 0 {
			m.destinationSwitcher.cursor--
		}
	case "enter":
		return m.selectGlobalDestination(m.destinationSwitcher.cursor)
	case "1", "2", "3", "4", "5", "6":
		return m.selectGlobalDestination(int(key[0] - '1'))
	}
	return nil
}

func (m *AppModel) selectGlobalDestination(index int) tea.Cmd {
	if m.destinationSwitcher == nil || index < 0 || index >= len(m.destinationSwitcher.entries) {
		m.closeDestinationSwitcher()
		return nil
	}
	entry := m.destinationSwitcher.entries[index]
	if !entry.Enabled {
		m.closeDestinationSwitcher()
		return func() tea.Msg { return ShowProjectPickerMsg{} }
	}

	m.closeDestinationSwitcher()
	m.saveProjectViewState()
	m.saveOutlinerFilterState()
	m.jumpToDashboard()

	switch entry.ID {
	case DestinationHome:
		return nil
	case DestinationPlanner:
		return func() tea.Msg { return ShowFocusBoardMsg{Tab: boardTabToday, SetTab: true} }
	case DestinationTasks:
		return tea.Batch(m.activateTaskWorkspaceRoute(entry.Route), m.prepareCrossProjectOutliner())
	case DestinationProject:
		m.prepareProjectOutliner(m.currentProject, nil)
		return m.activateTaskWorkspaceRoute(entry.Route)
	case DestinationReview:
		return m.activateReviewHub(ReviewLanding, nil)
	case DestinationManage:
		return m.activateManageHub(ManageParams{Module: ManageLanding, Scope: AppEntityScope()})
	}
	return nil
}

func (m *AppModel) renderDestinationSwitcher() string {
	if m.destinationSwitcher == nil {
		return ""
	}
	lines := make([]string, 0, len(m.destinationSwitcher.entries))
	for i, entry := range m.destinationSwitcher.entries {
		marker := "  "
		style := lipgloss.NewStyle()
		if i == m.destinationSwitcher.cursor {
			marker = "› "
			style = style.Foreground(ColorPrimary).Bold(true)
		} else if !entry.Enabled {
			style = style.Foreground(ColorMuted)
		}
		line := fmt.Sprintf("%s%d  %-9s %s", marker, i+1, entry.Name, entry.Description)
		lines = append(lines, style.Render(line))
	}
	footer := HelpStyle.Render("↑/↓ choose · enter open · esc close")
	return renderTitledBox(strings.Join(lines, "\n"), 52, "Go to", footer, lipgloss.NewStyle().Foreground(ColorPrimary))
}
