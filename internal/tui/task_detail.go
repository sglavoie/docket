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

// TaskDetailModel represents the task detail view.
type TaskDetailModel struct {
	task          *models.Task
	project       *models.Project
	config        *config.Config
	configManager *config.Manager
	width         int
	height        int
	scrollY       int
	lineCount     int

	// Search state
	searchMode   bool
	searchInput  textinput.Model
	searchQuery  string
	matchCount   int
	currentMatch int
}

// NewTaskDetailModel creates a new task detail model.
func NewTaskDetailModel(task *models.Task, project *models.Project, cfg *config.Config, manager *config.Manager) *TaskDetailModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search content..."
	searchInput.CharLimit = 100
	searchInput.Width = 40

	return &TaskDetailModel{
		task:          task,
		project:       project,
		config:        cfg,
		configManager: manager,
		scrollY:       0,
		searchMode:    false,
		searchInput:   searchInput,
		searchQuery:   "",
		matchCount:    0,
		currentMatch:  0,
	}
}

// SetSize sets the view dimensions.
func (m *TaskDetailModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *TaskDetailModel) HasActiveSearch() bool {
	return m.searchMode || m.searchQuery != ""
}

// countMatches counts occurrences of query in the searchable content.
func (m *TaskDetailModel) countMatches() int {
	if m.searchQuery == "" {
		return 0
	}
	query := strings.ToLower(m.searchQuery)
	count := 0

	// Count in title
	count += strings.Count(strings.ToLower(m.task.Title), query)

	// Count in description
	count += strings.Count(strings.ToLower(m.task.Description), query)

	return count
}

// highlightText wraps matching substrings with the search highlight style.
func (m *TaskDetailModel) highlightText(text string) string {
	if m.searchQuery == "" {
		return text
	}

	query := strings.ToLower(m.searchQuery)
	lower := strings.ToLower(text)
	var result strings.Builder
	lastEnd := 0

	for {
		idx := strings.Index(lower[lastEnd:], query)
		if idx == -1 {
			break
		}
		idx += lastEnd

		// Add text before match
		result.WriteString(text[lastEnd:idx])

		// Add highlighted match (preserve original case)
		matchEnd := idx + len(m.searchQuery)
		result.WriteString(SearchHighlightStyle.Render(text[idx:matchEnd]))

		lastEnd = matchEnd
	}

	// Add remaining text
	result.WriteString(text[lastEnd:])

	return result.String()
}

func (m *TaskDetailModel) Init() tea.Cmd {
	return nil
}

func (m *TaskDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle search mode
		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				m.searchQuery = ""
				m.matchCount = 0
				m.currentMatch = 0
				return m, nil
			case "enter":
				m.searchMode = false
				m.searchInput.Blur()
				m.searchQuery = m.searchInput.Value()
				m.matchCount = m.countMatches()
				if m.matchCount > 0 {
					m.currentMatch = 1
				} else {
					m.currentMatch = 0
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.searchQuery = m.searchInput.Value()
				m.matchCount = m.countMatches()
				if m.matchCount > 0 && m.currentMatch == 0 {
					m.currentMatch = 1
				} else if m.matchCount == 0 {
					m.currentMatch = 0
				}
				return m, cmd
			}
		}

		switch msg.String() {
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink
		case "esc":
			// Clear search filter if active
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.matchCount = 0
				m.currentMatch = 0
				return m, nil
			}
		case "n":
			// Next match
			if m.matchCount > 0 {
				m.currentMatch++
				if m.currentMatch > m.matchCount {
					m.currentMatch = 1
				}
			}
		case "N":
			// Previous match
			if m.matchCount > 0 {
				m.currentMatch--
				if m.currentMatch < 1 {
					m.currentMatch = m.matchCount
				}
			}
		case "up", "k":
			if m.scrollY > 0 {
				m.scrollY--
			}
		case "down", "j":
			// Allow scrolling if content exceeds view height
			maxScroll := max(m.lineCount-m.height+10, 0)
			if m.scrollY < maxScroll {
				m.scrollY++
			}
		case "g":
			m.scrollY = 0
		case "G":
			maxScroll := max(m.lineCount-m.height+10, 0)
			m.scrollY = maxScroll
		case "?":
			return m, func() tea.Msg {
				return ShowHelpMsg{
					SourceView: ViewTaskDetail,
					ViewName:   "Task Detail",
					Items:      m.GetHelpItems(),
				}
			}
		case "R":
			// View task with glow pager
			return m, func() tea.Msg {
				return GlowViewMsg{Task: m.task}
			}
		case "y":
			// Yank (copy) task title to clipboard
			return m, func() tea.Msg {
				return CopyTaskTitleMsg{Task: m.task}
			}
		case "Y":
			// Yank (copy) full task details to clipboard
			return m, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: m.task}
			}
		case "ctrl+o":
			// Edit title and description in external editor
			return m, func() tea.Msg {
				return EditTaskContentMsg{Task: m.task, Project: m.project}
			}

		// Edit task
		case "e":
			return m, func() tea.Msg {
				return EditTaskMsg{Task: m.task}
			}

		// Status changes
		case "s":
			// Start task (in_progress)
			if m.task.Status == models.StatusTodo || m.task.Status == models.StatusBlocked {
				m.task.Start()
				return m, m.saveAndRefresh()
			}
		case "m":
			// Mark complete (done)
			if m.task.Status.IsActive() {
				// Block completion if task has incomplete dependencies
				if incompleteDeps := m.project.GetIncompleteDependencies(m.task); len(incompleteDeps) > 0 {
					depNames := make([]string, len(incompleteDeps))
					for i, dep := range incompleteDeps {
						depNames[i] = fmt.Sprintf("#%d", dep.ShortID)
					}
					return m, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("cannot complete: %d incomplete dependencies (%s)",
							len(incompleteDeps), strings.Join(depNames, ", "))}
					}
				}
				m.task.Complete()
				return m, m.saveAndRefresh()
			}
		case "b":
			// Block task
			if m.task.Status.IsActive() && m.task.Status != models.StatusBlocked {
				m.task.Block()
				return m, m.saveAndRefresh()
			}
		case "x":
			// Cancel task
			if m.task.Status.IsActive() {
				m.task.Cancel()
				return m, m.saveAndRefresh()
			}
		case "o":
			// Reopen task (back to todo)
			if m.task.Status != models.StatusTodo {
				m.task.Reopen()
				return m, m.saveAndRefresh()
			}
		case "`":
			// Cycle status forward: todo -> in_progress -> done -> todo
			// Skips blocked and cancelled (require explicit intent)
			switch m.task.Status {
			case models.StatusTodo:
				m.task.Start()
				return m, m.saveAndRefresh()
			case models.StatusInProgress:
				if incompleteDeps := m.project.GetIncompleteDependencies(m.task); len(incompleteDeps) > 0 {
					depNames := make([]string, len(incompleteDeps))
					for i, dep := range incompleteDeps {
						depNames[i] = fmt.Sprintf("#%d", dep.ShortID)
					}
					return m, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("cannot complete: %d incomplete dependencies (%s)",
							len(incompleteDeps), strings.Join(depNames, ", "))}
					}
				}
				m.task.Complete()
				return m, m.saveAndRefresh()
			case models.StatusDone:
				m.task.Reopen()
				return m, m.saveAndRefresh()
			}
		case "~":
			// Cycle status backward: todo -> done -> in_progress -> todo
			// Skips blocked and cancelled (require explicit intent)
			switch m.task.Status {
			case models.StatusTodo:
				m.task.Complete()
				return m, m.saveAndRefresh()
			case models.StatusInProgress:
				m.task.Reopen()
				return m, m.saveAndRefresh()
			case models.StatusDone:
				m.task.Start()
				return m, m.saveAndRefresh()
			}
		case "z":
			// Undo last operation
			return m, m.performUndo()
		}
	}

	return m, nil
}

// saveAndRefresh saves the project and returns a status changed message.
func (m *TaskDetailModel) saveAndRefresh() tea.Cmd {
	return func() tea.Msg {
		if m.project == nil {
			return nil
		}
		// Create backup before saving for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return TaskStatusChangedMsg{Task: m.task}
	}
}

// performUndo restores the project from backup.
func (m *TaskDetailModel) performUndo() tea.Cmd {
	return func() tea.Msg {
		if m.project == nil {
			return ErrorMsg{Err: fmt.Errorf("no project context for undo")}
		}
		if !storage.BackupExists(m.project) {
			return ErrorMsg{Err: fmt.Errorf("no backup available to undo")}
		}

		if err := storage.RestoreFromBackup(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to undo: %w", err)}
		}

		if err := storage.LoadProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reload after undo: %w", err)}
		}

		return UndoCompletedMsg{}
	}
}

func (m *TaskDetailModel) View() string {
	var b strings.Builder

	// Search bar
	if m.searchMode {
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.searchInput.View())
		b.WriteString("\n\n")
	} else if m.searchQuery != "" {
		searchInfo := fmt.Sprintf("Search: \"%s\" ", m.searchQuery)
		if m.matchCount > 0 {
			searchInfo += fmt.Sprintf("(%d/%d) ", m.currentMatch, m.matchCount)
		} else {
			searchInfo += "(no matches) "
		}
		b.WriteString(MutedStyle.Render(searchInfo))
		b.WriteString(HelpStyle.Render("(n/N navigate, Esc clear)"))
		b.WriteString("\n\n")
	}

	b.WriteString(m.renderFormattedView())

	// Help hint
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Press ? for help"))

	// Count lines for scrolling
	content := b.String()
	m.lineCount = strings.Count(content, "\n")

	return content
}

func (m *TaskDetailModel) renderFormattedView() string {
	var b strings.Builder
	task := m.task

	// Header with type and title (apply highlighting)
	typeStyle := GetTypeStyle(string(task.Type))
	title := task.Title
	if m.searchQuery != "" {
		title = m.highlightText(title)
	}
	header := fmt.Sprintf("%s #%d: %s",
		typeStyle.Render(task.Type.HeadingPrefix()),
		task.ShortID,
		title)
	b.WriteString(TitleStyle.Render(header))
	b.WriteString("\n\n")

	// Metadata section
	b.WriteString(m.renderMetadata())
	b.WriteString("\n")

	// Description (apply highlighting)
	if task.Description != "" {
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		description := task.Description
		if m.searchQuery != "" {
			description = m.highlightText(description)
		}
		b.WriteString(description)
		b.WriteString("\n\n")
	}

	// Children
	if len(task.Children) > 0 {
		b.WriteString(m.renderChildren())
	}

	return b.String()
}

func (m *TaskDetailModel) renderMetadata() string {
	var b strings.Builder
	task := m.task

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
	typeStyle := GetTypeStyle(string(task.Type))
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

	// Dates
	b.WriteString(fmt.Sprintf("%s %s\n",
		MutedStyle.Render("Created:"),
		task.CreatedAt.Format("2006-01-02 15:04")))

	b.WriteString(fmt.Sprintf("%s %s\n",
		MutedStyle.Render("Updated:"),
		storage.FormatRelativeTime(task.UpdatedAt)))

	if task.CompletedAt != nil {
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Completed:"),
			task.CompletedAt.Format("2006-01-02 15:04")))
	}

	if task.DueDate != nil {
		dueStyle := NormalStyle
		// Could add overdue highlighting here
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Due:"),
			dueStyle.Render(task.DueDate.Format("2006-01-02"))))
	}

	// Parent reference
	if task.ParentID != nil && task.Parent != nil {
		parentRef := fmt.Sprintf("#%d: %s", task.Parent.ShortID, task.Parent.Title)
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Parent:"),
			MutedStyle.Render(parentRef)))
	}

	// Dependencies
	if len(task.DependsOn) > 0 && m.project != nil {
		blockers := m.project.GetBlockingTasks(task)
		if len(blockers) > 0 {
			b.WriteString(fmt.Sprintf("%s\n", MutedStyle.Render("Depends on:")))
			for _, blocker := range blockers {
				statusStyle := GetStatusStyle(string(blocker.Status))
				b.WriteString(fmt.Sprintf("  %s #%d: %s\n",
					statusStyle.Render(blocker.Status.Icon()),
					blocker.ShortID,
					blocker.Title))
			}
		}
	}

	// Tasks that depend on this one
	if m.project != nil {
		blocked := m.project.GetBlockedByTasks(task)
		if len(blocked) > 0 {
			b.WriteString(fmt.Sprintf("%s\n", MutedStyle.Render("Blocks:")))
			for _, t := range blocked {
				statusStyle := GetStatusStyle(string(t.Status))
				b.WriteString(fmt.Sprintf("  %s #%d: %s\n",
					statusStyle.Render(t.Status.Icon()),
					t.ShortID,
					t.Title))
			}
		}
	}

	return b.String()
}

func (m *TaskDetailModel) renderChildren() string {
	var b strings.Builder
	task := m.task

	// Progress
	completed := 0
	for _, child := range task.Children {
		if child.IsComplete() {
			completed++
		}
	}
	progress := task.GetProgress()

	b.WriteString(SubtitleStyle.Render("Subtasks"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%s %d/%d completed (%.0f%%)\n",
		MutedStyle.Render("Progress:"),
		completed, len(task.Children),
		progress))

	// Progress bar
	b.WriteString(RenderProgressBar(progress, 30))
	b.WriteString("\n\n")

	// List children
	for _, child := range task.Children {
		checkbox := "○"
		style := NormalStyle
		if child.IsComplete() {
			checkbox = "●"
			style = DoneStyle
		} else if child.Status == models.StatusInProgress {
			checkbox = "◐"
			style = InProgressStyle
		} else if child.Status == models.StatusBlocked {
			checkbox = "◈"
			style = BlockedStyle
		}

		typeStyle := GetTypeStyle(string(child.Type))
		line := fmt.Sprintf("  %s %s #%d: %s",
			style.Render(checkbox),
			typeStyle.Render(child.Type.Icon()),
			child.ShortID,
			style.Render(truncate(child.Title, m.width-20)))

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// GetHelpItems returns the help items for the task detail view.
func (m *TaskDetailModel) GetHelpItems() []HelpItem {
	return []HelpItem{
		// Navigation
		{Key: "j/k", Description: "Scroll", Category: "Navigation"},
		{Key: "g", Description: "Go to top", Category: "Navigation"},
		{Key: "G", Description: "Go to bottom", Category: "Navigation"},

		// Status Changes
		{Key: "s", Description: "Start (in progress)", Category: "Status Changes"},
		{Key: "m", Description: "Mark complete", Category: "Status Changes"},
		{Key: "b", Description: "Block task", Category: "Status Changes"},
		{Key: "x", Description: "Cancel task", Category: "Status Changes"},
		{Key: "o", Description: "Reopen (todo)", Category: "Status Changes"},

		// Actions
		{Key: "e", Description: "Edit task", Category: "Actions"},
		{Key: "ctrl+O", Description: "Edit in external editor", Category: "Actions"},

		// Search
		{Key: "/", Description: "Search content", Category: "Search"},
		{Key: "n", Description: "Next match", Category: "Search"},
		{Key: "N", Description: "Previous match", Category: "Search"},

		// View
		{Key: "R", Description: "View with glow", Category: "View"},
		{Key: "?", Description: "Toggle help", Category: "View"},

		// Clipboard
		{Key: "y", Description: "Copy title", Category: "Clipboard"},
		{Key: "Y", Description: "Copy full details", Category: "Clipboard"},

		// Other
		{Key: "z", Description: "Undo last change", Category: "Other"},
	}
}
