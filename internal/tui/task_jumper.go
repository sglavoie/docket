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
	"github.com/sglavoie/tt/internal/util"
)

// filteredTask holds a task with its match positions for highlighting.
type filteredTask struct {
	task           *models.Task
	titlePositions []int // Match positions within the title for highlighting
}

// TaskJumperModel provides fuzzy-find navigation to tasks.
type TaskJumperModel struct {
	project  *models.Project
	tasks    []*models.Task // Flat list of all tasks
	filtered []filteredTask // Tasks matching current query with highlight positions
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
	// Initialize filtered with all tasks (no highlighting when no query)
	m.filtered = make([]filteredTask, len(m.tasks))
	for i, task := range m.tasks {
		m.filtered[i] = filteredTask{task: task}
	}
	return m
}

// HasActiveSearch returns true (search input is always active in jumper).
func (m *TaskJumperModel) HasActiveSearch() bool {
	return true
}

// HasTextInput returns true because the jumper always uses the search input.
func (m *TaskJumperModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns true to allow the jumper to handle esc itself.
func (m *TaskJumperModel) ShouldCaptureEsc() bool {
	return true
}

// SetSize sets the view dimensions.
func (m *TaskJumperModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Calculate inner content width for split-pane overlay:
	// cardWidth = viewWidth - 4 (margin)
	// lipgloss Width = cardWidth - 2 (border adds 2 to total)
	// usable text area = Width - 4 (padding inside) = viewWidth - 10
	innerWidth := width - 10
	if innerWidth < 60 {
		innerWidth = 60
	}

	// Calculate split pane widths: 50/50 based on inner width
	m.listWidth = innerWidth * 50 / 100
	if m.listWidth < 30 {
		m.listWidth = 30
	}
	m.previewWidth = innerWidth - m.listWidth - 1 // -1 for separator

	m.searchInput.Width = innerWidth - 4
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
				selectedTask := m.filtered[m.cursor].task
				return m, func() tea.Msg {
					return TaskJumperSelectMsg{Task: selectedTask}
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
			jumpCursor(&m.cursor, len(m.filtered), -10)
			return m, nil
		case "ctrl+d":
			jumpCursor(&m.cursor, len(m.filtered), 10)
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
		// Reset to all tasks with no highlighting
		m.filtered = make([]filteredTask, len(m.tasks))
		for i, task := range m.tasks {
			m.filtered[i] = filteredTask{task: task}
		}
		m.cursor = 0
		return
	}

	// Score each task and collect matches with title positions
	type scoredTask struct {
		task           *models.Task
		score          int
		titlePositions []int
	}
	var scored []scoredTask

	for _, task := range m.tasks {
		score, titlePositions := m.scoreTask(task, query)
		if score > 0 {
			scored = append(scored, scoredTask{
				task:           task,
				score:          score,
				titlePositions: titlePositions,
			})
		}
	}

	// Sort by score descending (best matches first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract sorted results with positions
	m.filtered = make([]filteredTask, len(scored))
	for i, s := range scored {
		m.filtered[i] = filteredTask{
			task:           s.task,
			titlePositions: s.titlePositions,
		}
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// scoreTask returns a fuzzy match score for a task against the query,
// along with match positions within the title for highlighting.
// Returns 0 and nil if no match, higher scores indicate better matches.
func (m *TaskJumperModel) scoreTask(task *models.Task, query string) (int, []int) {
	// Build searchable text combining multiple fields
	searchable := fmt.Sprintf("#%d %s %s %s",
		task.ShortID,
		task.Title,
		task.Description[:min(len(task.Description), 200)],
		strings.Join(task.Tags, " "),
	)

	score := fuzzy.ScoreTerms(searchable, query)
	if score == 0 {
		return 0, nil
	}

	// Get match positions specifically for the title (for highlighting)
	_, titlePositions := fuzzy.ScoreTermsWithPositions(task.Title, query)
	return score, titlePositions
}

func (m *TaskJumperModel) View() string {
	var out strings.Builder

	// Title
	subtitle := fmt.Sprintf("%d tasks", len(m.tasks))
	if len(m.filtered) != len(m.tasks) {
		subtitle = fmt.Sprintf("%d/%d matching", len(m.filtered), len(m.tasks))
	}
	out.WriteString(renderFullscreenTitle("Jump to Task", subtitle))
	out.WriteString("\n")

	// Search input
	out.WriteString(m.searchInput.View())
	out.WriteString("\n\n")

	// Handle edge cases
	if len(m.tasks) == 0 {
		out.WriteString(MutedStyle.Render("No tasks in this project."))
		out.WriteString("\n\n")
		out.WriteString(HelpStyle.Render("Press Esc to close"))
		return out.String()
	}

	// Calculate pane height for split view
	paneHeight := m.height - 8 // title + input + footer
	if paneHeight < 5 {
		paneHeight = 5
	}

	// Split pane: list on left, preview on right
	listContent := m.renderTaskList()
	previewContent := m.renderPreview()

	splitPane := renderOverlaySplitPane(listContent, previewContent, m.listWidth, m.previewWidth, paneHeight)
	out.WriteString(splitPane)
	out.WriteString("\n")

	// Footer
	out.WriteString(HelpStyle.Render("↑↓ nav · Enter · Esc"))

	return out.String()
}

// renderTaskList renders the filtered task list.
func (m *TaskJumperModel) renderTaskList() string {
	if len(m.filtered) == 0 {
		return MutedStyle.Render("No matching tasks.")
	}

	var b strings.Builder
	maxVisible := m.height - 8 // Account for chrome

	scrollOffset := calcScrollOffset(m.cursor, maxVisible)

	for i := scrollOffset; i < len(m.filtered) && i < scrollOffset+maxVisible; i++ {
		ft := m.filtered[i]
		task := ft.task

		// Cursor indicator
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		// Type icon with badge style
		typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

		// Status icon with badge style
		statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

		// Short ID
		shortID := MutedStyle.Render(fmt.Sprintf("#%-4d", task.ShortID))

		// Title (truncated) with highlighting
		// Account for: cursor(2) + icons(4) + spaces(3) + ID(5) = 14
		maxTitleLen := m.listWidth - 14
		if maxTitleLen < 10 {
			maxTitleLen = 10
		}
		title := util.Truncate(task.Title, maxTitleLen)

		// Adjust positions for truncated title and apply highlighting
		var titleRendered string
		if len(ft.titlePositions) > 0 {
			// Filter positions to only include those within the truncated title
			titleRunes := []rune(title)
			var validPositions []int
			for _, p := range ft.titlePositions {
				if p < len(titleRunes) {
					validPositions = append(validPositions, p)
				}
			}
			titleRendered = HighlightText(title, validPositions, NormalStyle)
		} else {
			titleRendered = title
		}

		// Build line
		line := fmt.Sprintf("%s%s %s %s %s", cursor, typeIcon, statusIcon, shortID, titleRendered)

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

	task := m.filtered[m.cursor].task
	var b strings.Builder

	// Header with type and title (truncate to fit preview width)
	typeStyle := GetTypeStyle(string(task.Type))
	titlePrefix := fmt.Sprintf("%s #%d: ", task.Type.HeadingPrefix(), task.ShortID)
	maxTitleLen := m.previewWidth - len(titlePrefix) - 4 // -4 for padding/margin
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	header := fmt.Sprintf("%s #%d: %s",
		typeStyle.Render(task.Type.HeadingPrefix()),
		task.ShortID,
		util.Truncate(task.Title, maxTitleLen))
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
			task.DueDate.Format(util.DateFormat)))
	}

	// Parent reference
	if task.Parent != nil {
		parentRef := fmt.Sprintf("#%d: %s", task.Parent.ShortID, task.Parent.Title)
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Parent:"),
			MutedStyle.Render(util.Truncate(parentRef, m.previewWidth-10))))
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
		// Truncate each line to fit preview width
		for i, line := range lines {
			lines[i] = util.Truncate(line, m.previewWidth-2)
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
