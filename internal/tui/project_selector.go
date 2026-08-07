package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// ProjectPickerItem wraps a Project for selection with fuzzy scoring.
type ProjectPickerItem struct {
	Project *models.Project
}

// FilterText returns searchable text for project filtering.
func (p ProjectPickerItem) FilterText() string {
	if p.Project == nil {
		return ""
	}
	if p.Project.Archived {
		return fmt.Sprintf("%s %s", p.Project.Name, archivedProjectMarker)
	}
	return p.Project.Name
}

// DisplayText returns the project name for display.
func (p ProjectPickerItem) DisplayText() string {
	return projectPickerDisplayText(p.Project)
}

// CountValue returns the task count.
func (p ProjectPickerItem) CountValue() int {
	if p.Project == nil {
		return 0
	}
	return p.Project.CountTasks()
}

// CountDisplayText marks an unloaded destination's task count as unknown so a
// metadata-only project does not read as an empty one.
func (p ProjectPickerItem) CountDisplayText() string {
	return "(" + projectTaskCountLabel(p.Project) + ")"
}

// ItemID returns the project ID as its unique identifier.
func (p ProjectPickerItem) ItemID() string {
	if p.Project == nil {
		return ""
	}
	return p.Project.ID
}

// ProjectSelectorModel provides fuzzy-find selection of projects for task move.
type ProjectSelectorModel struct {
	*ListSurfaceAdapter[ProjectPickerItem]
	task          *models.Task
	sourceProject *models.Project
	alwaysSearch  *alwaysSearchSinglePicker[ProjectPickerItem]
}

// NewProjectSelectorModel creates a new project selector for moving a task.
func NewProjectSelectorModel(projects []*models.Project, task *models.Task, sourceProject *models.Project) *ProjectSelectorModel {
	// Filter out the source project
	var filteredProjects []*models.Project
	for _, p := range projects {
		if p.ID != sourceProject.ID && !p.Archived {
			filteredProjects = append(filteredProjects, p)
		}
	}

	items := make([]ProjectPickerItem, len(filteredProjects))
	for i, p := range filteredProjects {
		items[i] = ProjectPickerItem{Project: p}
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{
		Title:          "Move to Project",
		EmptyMessage:   "No other projects available.",
		NoMatchMessage: "No matching projects.",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	m := &ProjectSelectorModel{
		ListSurfaceAdapter: picker,
		task:               task,
		sourceProject:      sourceProject,
	}
	m.alwaysSearch = newAlwaysSearchSinglePicker(m.ListSurfaceAdapter, alwaysSearchSinglePickerConfig[ProjectPickerItem]{
		CancelMsg: func() tea.Msg {
			return CloseProjectSelectorMsg{}
		},
		SelectMsg: func(cursor int, filtered []ProjectPickerItem) tea.Msg {
			if cursor >= len(filtered) || len(filtered) == 0 {
				return CloseProjectSelectorMsg{}
			}

			selected := filtered[cursor]
			return MoveTaskToProjectMsg{
				Task:          m.task,
				SourceProject: m.sourceProject,
				DestProjectID: selected.Project.ID,
			}
		},
	})

	// Activate search immediately so typing works without Init() being called
	m.PrimeAlwaysSearch()

	return m
}

// HasActiveSearch returns true (search input is always active).
func (m *ProjectSelectorModel) HasActiveSearch() bool {
	return true
}

// HasTextInput returns true because the project selector always has input focus.
func (m *ProjectSelectorModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc lets the selector clear an active query before closing.
func (m *ProjectSelectorModel) ShouldCaptureEsc() bool {
	return true
}

func (m *ProjectSelectorModel) Init() tea.Cmd {
	return m.alwaysSearch.Init()
}

func (m *ProjectSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := m.alwaysSearch.Update(msg); handled {
		return m, cmd
	}
	return m, nil
}

func (m *ProjectSelectorModel) View() tea.View {
	var b strings.Builder

	// Title with task info
	title := "Move to Project"
	taskInfo := fmt.Sprintf("Moving: #%d %s", m.task.ShortID, m.task.Title)
	availableSubtitleWidth := max(m.Width()-lipgloss.Width(title)-2, 1)
	if lipgloss.Width(taskInfo) > availableSubtitleWidth {
		taskInfo = truncateToVisualWidth(taskInfo, availableSubtitleWidth)
	}
	b.WriteString(renderFullscreenTitle(title, taskInfo))
	b.WriteString("\n\n")

	// Search input
	b.WriteString(m.SearchInputView())
	b.WriteString("\n\n")

	// Render the list
	b.WriteString(m.renderList())
	b.WriteString("\n")

	guidance, _ := m.alwaysSearchGuidanceSpec("select")
	b.WriteString(renderGuidance(guidance, m.Width()))

	return tea.NewView(b.String())
}

// renderList renders the filtered project list.
func (m *ProjectSelectorModel) renderList() string {
	var b strings.Builder

	maxVisible := max(
		// Account for title, task info, input, footer
		m.Height()-12, 5)

	totalItems := len(m.FilteredItems())

	if totalItems == 0 {
		return MutedStyle.Render("No matching projects.")
	}

	scrollOffset := calcScrollOffset(m.CursorIndex(), maxVisible)

	// Render projects
	for i, item := range m.FilteredItems() {
		if i < scrollOffset || i >= scrollOffset+maxVisible {
			continue
		}

		cursor := "  "
		if m.CursorIndex() == i {
			cursor = "  "
		}

		// Task count
		taskCount := projectTaskCountLabel(item.Project) + " tasks"

		// Project name (truncated)
		maxNameLen := max(m.Width()-30, 10)
		name := truncateToVisualWidth(item.Project.Name, maxNameLen)

		// Build line
		line := fmt.Sprintf("%s%s %s", cursor, truncateAndPad(name, maxNameLen), MutedStyle.Render(taskCount))

		if m.CursorIndex() == i {
			line = SelectedItemTextStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Show scroll indicator if there are more items
	if totalItems > maxVisible {
		remaining := totalItems - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... %d more", remaining)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *ProjectSelectorModel) StatusBarLabel() string { return "Select Project" }
