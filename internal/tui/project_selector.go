package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// ProjectPickerItem wraps a Project for selection with fuzzy scoring.
type ProjectPickerItem struct {
	Project *models.Project
}

// FilterText returns searchable text for project filtering.
func (p ProjectPickerItem) FilterText() string {
	return fmt.Sprintf("%s %s", p.Project.Name, p.Project.Context)
}

// DisplayText returns the project name for display.
func (p ProjectPickerItem) DisplayText() string {
	return p.Project.Name
}

// CountValue returns the task count.
func (p ProjectPickerItem) CountValue() int {
	return p.Project.CountTasks()
}

// ItemID returns the project ID as its unique identifier.
func (p ProjectPickerItem) ItemID() string {
	return p.Project.ID
}

// ProjectSelectorModel provides fuzzy-find selection of projects for task move.
type ProjectSelectorModel struct {
	*BasePicker[ProjectPickerItem]
	allItems      []ProjectPickerItem // All valid project candidates
	task          *models.Task        // Task being moved
	sourceProject *models.Project     // Project task is moving from
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

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Move to Project",
		EmptyMessage:   "No other projects available",
		NoMatchMessage: "No matching projects",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	m := &ProjectSelectorModel{
		BasePicker:    picker,
		allItems:      items,
		task:          task,
		sourceProject: sourceProject,
	}

	// Activate search immediately so typing works without Init() being called
	m.SetSearchActive(true)
	m.searchInput.Focus()

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

func (m *ProjectSelectorModel) Init() tea.Cmd {
	return m.ActivateSearch()
}

func (m *ProjectSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.IsSearchMode() && m.SearchQuery() != "" {
				m.ClearSearch()
				m.fuzzyFilter("")
				return m, nil
			}
			return m, func() tea.Msg {
				return CloseProjectSelectorMsg{}
			}

		case "enter":
			return m, m.selectCurrent()

		case "up", "ctrl+k":
			m.moveCursor(-1)
			return m, nil

		case "down", "ctrl+j":
			m.moveCursor(1)
			return m, nil

		case "ctrl+u":
			m.moveCursor(-10)
			return m, nil

		case "ctrl+d":
			m.moveCursor(10)
			return m, nil

		default:
			// Handle search input
			handled, cmd, valueChanged := m.HandleSearchInput(msg)
			if valueChanged {
				m.fuzzyFilter(m.SearchQuery())
			}
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

// moveCursor moves the cursor by delta.
func (m *ProjectSelectorModel) moveCursor(delta int) {
	maxCursor := len(m.Filtered) - 1
	if maxCursor < 0 {
		maxCursor = 0
	}

	m.Cursor += delta
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	if m.Cursor > maxCursor {
		m.Cursor = maxCursor
	}
}

// fuzzyFilter filters projects using fuzzy scoring.
func (m *ProjectSelectorModel) fuzzyFilter(query string) {
	query = strings.TrimSpace(query)

	if query == "" {
		m.Filtered = m.allItems
		m.Cursor = 0
		return
	}

	// Score each project and collect matches
	type scoredItem struct {
		item  ProjectPickerItem
		score int
	}
	var scored []scoredItem

	for _, item := range m.allItems {
		score := fuzzy.ScoreTerms(item.FilterText(), query)
		if score > 0 {
			scored = append(scored, scoredItem{item: item, score: score})
		}
	}

	// Sort by score descending (best matches first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract sorted results
	m.Filtered = make([]ProjectPickerItem, len(scored))
	for i, s := range scored {
		m.Filtered[i] = s.item
	}

	// Reset cursor if out of bounds
	maxCursor := len(m.Filtered) - 1
	if maxCursor < 0 {
		maxCursor = 0
	}
	if m.Cursor > maxCursor {
		m.Cursor = maxCursor
	}
}

// selectCurrent returns a command with the selected project.
func (m *ProjectSelectorModel) selectCurrent() tea.Cmd {
	return func() tea.Msg {
		if m.Cursor >= len(m.Filtered) || len(m.Filtered) == 0 {
			return CloseProjectSelectorMsg{}
		}

		selected := m.Filtered[m.Cursor]
		return MoveTaskToProjectMsg{
			Task:          m.task,
			SourceProject: m.sourceProject,
			DestProjectID: selected.Project.ID,
		}
	}
}

func (m *ProjectSelectorModel) View() string {
	var b strings.Builder

	// Title with task info
	taskInfo := fmt.Sprintf("Moving: #%d %s", m.task.ShortID, m.task.Title)
	if len(taskInfo) > m.Width()-4 {
		taskInfo = util.Truncate(taskInfo, m.Width()-4)
	}
	b.WriteString(renderFullscreenTitle("Move to Project", taskInfo))
	b.WriteString("\n\n")

	// Search input
	b.WriteString(m.SearchInputView())
	b.WriteString("\n\n")

	// Render the list
	b.WriteString(m.renderList())
	b.WriteString("\n")

	// Footer
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("↑/↓ navigate • Enter select • Esc cancel"), "", m.Width()))

	return b.String()
}

// renderList renders the filtered project list.
func (m *ProjectSelectorModel) renderList() string {
	var b strings.Builder

	maxVisible := m.Height() - 12 // Account for title, task info, input, footer
	if maxVisible < 5 {
		maxVisible = 5
	}

	totalItems := len(m.Filtered)

	if totalItems == 0 {
		return MutedStyle.Render("No matching projects.")
	}

	scrollOffset := calcScrollOffset(m.Cursor, maxVisible)

	// Render projects
	for i, item := range m.Filtered {
		if i < scrollOffset || i >= scrollOffset+maxVisible {
			continue
		}

		cursor := "  "
		if m.Cursor == i {
			cursor = "> "
		}

		// Context badge
		contextBadge := MutedStyle.Render(fmt.Sprintf("[%s]", item.Project.Context))

		// Task count
		taskCount := fmt.Sprintf("%d tasks", item.Project.CountTasks())

		// Project name (truncated)
		maxNameLen := m.Width() - 30
		if maxNameLen < 10 {
			maxNameLen = 10
		}
		name := util.Truncate(item.Project.Name, maxNameLen)

		// Build line
		line := fmt.Sprintf("%s%-*s %s %s", cursor, maxNameLen, name, contextBadge, MutedStyle.Render(taskCount))

		if m.Cursor == i {
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
