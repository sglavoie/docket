package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// TaskJumperModel provides fuzzy-find navigation to tasks.
type TaskJumperModel struct {
	project  *models.Project
	tasks    []*models.Task // Flat list of all tasks
	filtered []*models.Task // Tasks matching current query
	cursor   int            // Currently selected task index

	// Input for filtering
	searchInput textinput.Model

	// Dimensions
	width        int
	height       int
	listWidth    int
	previewWidth int
}

// NewTaskJumperModel creates a new task jumper model.
func NewTaskJumperModel(project *models.Project) *TaskJumperModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Type to filter tasks..."
	searchInput.CharLimit = 100
	searchInput.Width = 40
	searchInput.Focus()

	m := &TaskJumperModel{
		project:     project,
		tasks:       project.Tasks,
		searchInput: searchInput,
		cursor:      0,
	}
	m.filtered = m.tasks
	return m
}

// HasActiveSearch returns true (search input is always active in jumper).
func (m *TaskJumperModel) HasActiveSearch() bool {
	return true
}

// SetSize sets the view dimensions.
func (m *TaskJumperModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Calculate split pane widths: 50/50
	m.listWidth = width * 50 / 100
	if m.listWidth < 30 {
		m.listWidth = 30
	}
	if m.listWidth > width-30 {
		m.listWidth = width - 30
	}
	m.previewWidth = width - m.listWidth - 1 // -1 for border

	m.searchInput.Width = width - 4
}

func (m *TaskJumperModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *TaskJumperModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return CloseTaskJumperMsg{}
			}
		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				return m, func() tea.Msg {
					return TaskJumperSelectMsg{Task: m.filtered[m.cursor]}
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
func (m *TaskJumperModel) filterTasks() {
	query := strings.TrimSpace(m.searchInput.Value())

	if query == "" {
		m.filtered = m.tasks
		m.cursor = 0
		return
	}

	// Score each task and collect matches
	type scoredTask struct {
		task  *models.Task
		score int
	}
	var scored []scoredTask

	for _, task := range m.tasks {
		score := m.scoreTask(task, query)
		if score > 0 {
			scored = append(scored, scoredTask{task: task, score: score})
		}
	}

	// Sort by score descending (best matches first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract sorted results
	m.filtered = make([]*models.Task, len(scored))
	for i, s := range scored {
		m.filtered[i] = s.task
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// scoreTask returns a fuzzy match score for a task against the query.
// Returns 0 if no match, higher scores indicate better matches.
func (m *TaskJumperModel) scoreTask(task *models.Task, query string) int {
	// Build searchable text combining multiple fields
	searchable := fmt.Sprintf("#%d %s %s %s",
		task.ShortID,
		task.Title,
		task.Description[:min(len(task.Description), 200)],
		strings.Join(task.Tags, " "),
	)

	return fuzzy.ScoreTerms(searchable, query)
}

func (m *TaskJumperModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(TaskJumperTitleStyle.Render("Jump to Task"))
	b.WriteString("\n")

	// Search input
	b.WriteString(m.searchInput.View())
	b.WriteString("\n\n")

	// Handle edge cases
	if len(m.tasks) == 0 {
		b.WriteString(MutedStyle.Render("No tasks in this project."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press Esc or q to close"))
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
	footer := HelpStyle.Render("↑/↓ navigate • Enter select • Esc close")
	b.WriteString(footer)

	return b.String()
}

// renderTaskList renders the filtered task list.
func (m *TaskJumperModel) renderTaskList() string {
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
		task := m.filtered[i]

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

		// Short ID
		shortID := MutedStyle.Render(fmt.Sprintf("#%-4d", task.ShortID))

		// Title (truncated)
		// Account for: cursor(2) + icons(4) + spaces(3) + ID(5) = 14
		maxTitleLen := m.listWidth - 14
		if maxTitleLen < 10 {
			maxTitleLen = 10
		}
		title := truncate(task.Title, maxTitleLen)

		// Build line
		line := fmt.Sprintf("%s%s %s %s %s", cursor, typeIcon, statusIcon, shortID, title)

		if i == m.cursor {
			line = TaskJumperSelectedStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// renderPreview renders the preview pane for the selected task.
func (m *TaskJumperModel) renderPreview() string {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return MutedStyle.Render("No task selected.")
	}

	task := m.filtered[m.cursor]
	var b strings.Builder

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
		maxLines := m.height - 18
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

// CloseTaskJumperMsg is sent when the task jumper should close without selection.
type CloseTaskJumperMsg struct{}

// TaskJumperSelectMsg is sent when a task is selected from the jumper.
type TaskJumperSelectMsg struct {
	Task *models.Task
}

// ShowTaskJumperMsg is sent to open the task jumper.
type ShowTaskJumperMsg struct {
	Project *models.Project
}
