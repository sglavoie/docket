package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// searchResult holds a task and its parent project for display.
type searchResult struct {
	Task    *models.Task
	Project *models.Project
}

// GlobalSearchModel provides global search across all projects.
type GlobalSearchModel struct {
	config   *config.Config
	projects []*models.Project
	allTasks []searchResult // flattened list of all tasks with project refs
	filtered []searchResult // tasks matching current query
	cursor   int

	// Input for filtering
	searchInput textinput.Model

	// Dimensions
	width        int
	height       int
	listWidth    int
	previewWidth int
}

// NewGlobalSearchModel creates a new global search model.
func NewGlobalSearchModel(cfg *config.Config) *GlobalSearchModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search all tasks..."
	searchInput.CharLimit = 100
	searchInput.Width = 40
	searchInput.Focus()

	projects := cfg.GetActiveProjects()

	// Load all tasks from all projects
	var allTasks []searchResult
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}
		for _, task := range project.Tasks {
			allTasks = append(allTasks, searchResult{
				Task:    task,
				Project: project,
			})
		}
	}

	m := &GlobalSearchModel{
		config:      cfg,
		projects:    projects,
		allTasks:    allTasks,
		searchInput: searchInput,
		cursor:      0,
	}
	// Start with no results (empty query shows nothing)
	m.filtered = nil
	return m
}

// HasActiveSearch returns true (search input is always active).
func (m *GlobalSearchModel) HasActiveSearch() bool {
	return true
}

// SetSize sets the view dimensions.
func (m *GlobalSearchModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Calculate split pane widths: ~40% for list, ~60% for preview
	m.listWidth = width * 40 / 100
	if m.listWidth < 30 {
		m.listWidth = 30
	}
	if m.listWidth > width-30 {
		m.listWidth = width - 30
	}
	m.previewWidth = width - m.listWidth - 1 // -1 for border

	m.searchInput.Width = width - 4
}

func (m *GlobalSearchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *GlobalSearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return CloseGlobalSearchMsg{}
			}
		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				result := m.filtered[m.cursor]
				return m, func() tea.Msg {
					return GlobalSearchSelectMsg{
						Task:    result.Task,
						Project: result.Project,
					}
				}
			}
			return m, nil
		case "up", "ctrl+k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "ctrl+u":
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			return m, nil
		case "ctrl+d":
			m.cursor += 10
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			return m, nil
		default:
			// Update search input
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.filterTasks()
			return m, cmd
		}
	}

	return m, nil
}

// filterTasks filters tasks based on the current search query.
func (m *GlobalSearchModel) filterTasks() {
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))

	// Empty query shows no results
	if query == "" {
		m.filtered = nil
		m.cursor = 0
		return
	}

	var filtered []searchResult
	for _, result := range m.allTasks {
		if m.matchesQuery(result.Task, query) {
			filtered = append(filtered, result)
		}
	}
	m.filtered = filtered

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// matchesQuery checks if a task matches the search query.
func (m *GlobalSearchModel) matchesQuery(task *models.Task, query string) bool {
	if strings.Contains(strings.ToLower(task.Title), query) {
		return true
	}
	if strings.Contains(strings.ToLower(task.Description), query) {
		return true
	}
	return false
}

func (m *GlobalSearchModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(TaskJumperTitleStyle.Render("Global Search"))
	b.WriteString("\n")

	// Search input
	b.WriteString(m.searchInput.View())
	b.WriteString("\n\n")

	// Handle edge cases
	if len(m.allTasks) == 0 {
		b.WriteString(MutedStyle.Render("No tasks found in any project."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press Esc to close"))
		return b.String()
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		b.WriteString(MutedStyle.Render("Type to search across all tasks..."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press Esc to close"))
		return b.String()
	}

	if len(m.filtered) == 0 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("No tasks match \"%s\".", query)))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press Esc to close"))
		return b.String()
	}

	// Split pane: list on left, preview on right
	listContent := m.renderTaskList()
	previewContent := m.renderPreview()

	// Apply styles and join horizontally
	listPane := TaskJumperListStyle.
		Width(m.listWidth).
		Height(m.height - 6). // Account for title, input, footer
		Render(listContent)

	previewPane := TaskJumperPreviewStyle.
		Width(m.previewWidth).
		Height(m.height - 6).
		Render(previewContent)

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listPane, previewPane))
	b.WriteString("\n")

	// Footer
	footer := HelpStyle.Render(fmt.Sprintf("↑/↓ navigate • Enter select • Esc close • %d result(s)", len(m.filtered)))
	b.WriteString(footer)

	return b.String()
}

// renderTaskList renders the filtered task list.
func (m *GlobalSearchModel) renderTaskList() string {
	if len(m.filtered) == 0 {
		return MutedStyle.Render("No matching tasks.")
	}

	var b strings.Builder
	maxVisible := m.height - 8 // Account for chrome

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if m.cursor >= maxVisible {
		scrollOffset = m.cursor - maxVisible + 1
	}

	for i := scrollOffset; i < len(m.filtered) && i < scrollOffset+maxVisible; i++ {
		result := m.filtered[i]
		task := result.Task

		// Cursor indicator
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		// Type icon
		typeStyle := GetTypeStyle(string(task.Type))
		typeIcon := typeStyle.Render(task.Type.Icon())

		// Status icon
		statusStyle := GetStatusStyle(string(task.Status))
		statusIcon := statusStyle.Render(task.Status.Icon())

		// Project name (truncated)
		projectName := MutedStyle.Render(fmt.Sprintf("[%s]", truncate(result.Project.Slug, 10)))

		// Short ID
		shortID := MutedStyle.Render(fmt.Sprintf("#%-4d", task.ShortID))

		// Title (truncated)
		maxTitleLen := m.listWidth - 28 // Account for cursor, icons, ID, project
		if maxTitleLen < 10 {
			maxTitleLen = 10
		}
		title := truncate(task.Title, maxTitleLen)

		// Build line
		line := fmt.Sprintf("%s%s %s %s %s %s", cursor, typeIcon, statusIcon, shortID, projectName, title)

		if i == m.cursor {
			line = TaskJumperSelectedStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// renderPreview renders the preview pane for the selected task.
func (m *GlobalSearchModel) renderPreview() string {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return MutedStyle.Render("No task selected.")
	}

	result := m.filtered[m.cursor]
	task := result.Task
	project := result.Project
	var b strings.Builder

	// Project header
	b.WriteString(MutedStyle.Render("Project: "))
	b.WriteString(BoldStyle.Render(project.Name))
	b.WriteString("\n\n")

	// Header with type and title
	typeStyle := GetTypeStyle(string(task.Type))
	header := fmt.Sprintf("%s #%d: %s",
		typeStyle.Render(task.Type.HeadingPrefix()),
		task.ShortID,
		task.Title)
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(header))
	b.WriteString("\n\n")

	// Status
	statusStyle := GetStatusStyle(string(task.Status))
	b.WriteString(fmt.Sprintf("%s %s %s\n",
		MutedStyle.Render("Status:"),
		statusStyle.Render(task.Status.Icon()),
		statusStyle.Render(string(task.Status))))

	// Priority
	priorityStyle := NormalStyle
	if task.Priority <= 2 {
		priorityStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	}
	b.WriteString(fmt.Sprintf("%s %s\n",
		MutedStyle.Render("Priority:"),
		priorityStyle.Render(fmt.Sprintf("%d", task.Priority))))

	// Type
	b.WriteString(fmt.Sprintf("%s %s\n",
		MutedStyle.Render("Type:"),
		typeStyle.Render(task.Type.HeadingPrefix())))

	// Tags
	if len(task.Tags) > 0 {
		tagStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
		tags := make([]string, len(task.Tags))
		for i, tag := range task.Tags {
			tags[i] = tagStyle.Render(tag)
		}
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, ", ")))
	}

	// Due date
	if task.DueDate != nil {
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Due:"),
			task.DueDate.Format("2006-01-02")))
	}

	// Parent reference
	if task.Parent != nil {
		parentRef := fmt.Sprintf("#%d: %s", task.Parent.ShortID, task.Parent.Title)
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Parent:"),
			MutedStyle.Render(truncate(parentRef, m.previewWidth-10))))
	}

	// Description (truncated)
	if task.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")

		// Truncate description to fit preview
		maxLines := m.height - 20
		if maxLines < 3 {
			maxLines = 3
		}
		lines := strings.Split(task.Description, "\n")
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines = append(lines, "...")
		}
		b.WriteString(strings.Join(lines, "\n"))
	}

	// Children summary
	if len(task.Children) > 0 {
		b.WriteString("\n\n")
		completed := 0
		for _, child := range task.Children {
			if child.IsComplete() {
				completed++
			}
		}
		b.WriteString(fmt.Sprintf("%s %d/%d completed\n",
			MutedStyle.Render("Subtasks:"),
			completed, len(task.Children)))
	}

	return b.String()
}

// CloseGlobalSearchMsg is sent when the global search should close.
type CloseGlobalSearchMsg struct{}

// GlobalSearchSelectMsg is sent when a task is selected from global search.
type GlobalSearchSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// ShowGlobalSearchMsg is sent to open the global search.
type ShowGlobalSearchMsg struct{}
