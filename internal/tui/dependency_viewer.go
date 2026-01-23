package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
)

// DependencyViewerModel displays dependencies for a task in a navigable read-only view.
type DependencyViewerModel struct {
	task      *models.Task
	project   *models.Project
	dependsOn []*models.Task // Tasks that this task depends on (→)
	blockedBy []*models.Task // Tasks that depend on this task (←)

	// Navigation state
	section int // 0 = dependsOn, 1 = blockedBy
	cursor  int // Position within current section

	// Dimensions
	width  int
	height int
}

// NewDependencyViewerModel creates a new dependency viewer.
func NewDependencyViewerModel(task *models.Task, project *models.Project) *DependencyViewerModel {
	dependsOn := project.GetBlockingTasks(task)
	blockedBy := project.GetBlockedByTasks(task)

	return &DependencyViewerModel{
		task:      task,
		project:   project,
		dependsOn: dependsOn,
		blockedBy: blockedBy,
		section:   0,
		cursor:    0,
	}
}

func (m *DependencyViewerModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *DependencyViewerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *DependencyViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return CloseDependencyViewerMsg{} }

		case "j", "down":
			m.moveDown()

		case "k", "up":
			m.moveUp()

		case "tab":
			m.switchSection()

		case "enter":
			// Navigate to selected task in project tree
			if task := m.getSelectedTask(); task != nil {
				return m, func() tea.Msg {
					return DependencyViewerSelectMsg{Task: task}
				}
			}

		case "d":
			// View dependencies of selected task
			if task := m.getSelectedTask(); task != nil {
				return m, func() tea.Msg {
					return DependencyViewerViewDepsMsg{Task: task, Project: m.project}
				}
			}
		}
	}

	return m, nil
}

// currentSectionItems returns the slice for the active section.
func (m *DependencyViewerModel) currentSectionItems() []*models.Task {
	if m.section == 0 {
		return m.dependsOn
	}
	return m.blockedBy
}

// getSelectedTask returns the currently selected task based on section and cursor.
func (m *DependencyViewerModel) getSelectedTask() *models.Task {
	items := m.currentSectionItems()
	if m.cursor >= 0 && m.cursor < len(items) {
		return items[m.cursor]
	}
	return nil
}

// moveDown moves cursor down within the current section.
func (m *DependencyViewerModel) moveDown() {
	items := m.currentSectionItems()
	if len(items) > 0 && m.cursor < len(items)-1 {
		m.cursor++
	}
}

// moveUp moves cursor up within the current section.
func (m *DependencyViewerModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// switchSection toggles between sections and resets cursor if needed.
func (m *DependencyViewerModel) switchSection() {
	if m.section == 0 {
		m.section = 1
	} else {
		m.section = 0
	}

	// Reset cursor to valid position for new section
	items := m.currentSectionItems()
	if m.cursor >= len(items) {
		if len(items) > 0 {
			m.cursor = len(items) - 1
		} else {
			m.cursor = 0
		}
	}
}

func (m *DependencyViewerModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render(fmt.Sprintf("Dependencies for #%d: %s", m.task.ShortID, m.task.Title)))
	b.WriteString("\n\n")

	// Calculate available height for each section
	headerLines := 4    // title + 2 newlines + help line
	sectionHeaders := 4 // 2 headers with newlines
	availableHeight := m.height - headerLines - sectionHeaders
	maxPerSection := availableHeight / 2
	if maxPerSection < 3 {
		maxPerSection = 3
	}

	// Depends On section (→)
	b.WriteString(m.renderSection(0, "Depends On", "→", m.dependsOn, maxPerSection))
	b.WriteString("\n")

	// Blocked By section (←)
	b.WriteString(m.renderSection(1, "Blocked By", "←", m.blockedBy, maxPerSection))
	b.WriteString("\n")

	// Help footer
	b.WriteString(MutedStyle.Render("Tab: switch section | j/k: navigate | Enter: go to task | d: view deps | Esc: close"))

	return b.String()
}

func (m *DependencyViewerModel) renderSection(sectionIdx int, title, arrow string, tasks []*models.Task, maxVisible int) string {
	var b strings.Builder

	// Section header with styling based on whether it's active
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary)
	if m.section == sectionIdx {
		headerStyle = headerStyle.Foreground(ColorPrimary).Underline(true)
	}

	countStr := fmt.Sprintf("(%d)", len(tasks))
	b.WriteString(headerStyle.Render(fmt.Sprintf("--- %s %s %s ---", title, arrow, countStr)))
	b.WriteString("\n")

	// Empty section message
	if len(tasks) == 0 {
		if sectionIdx == 0 {
			b.WriteString(MutedStyle.Render("  No dependencies"))
		} else {
			b.WriteString(MutedStyle.Render("  No tasks depend on this task"))
		}
		b.WriteString("\n")
		return b.String()
	}

	// Calculate scroll offset for this section if it's active
	scrollOffset := 0
	if m.section == sectionIdx && m.cursor >= maxVisible {
		scrollOffset = m.cursor - maxVisible + 1
	}

	// Render visible tasks
	for i := scrollOffset; i < len(tasks) && i < scrollOffset+maxVisible; i++ {
		task := tasks[i]

		// Cursor indicator (only for active section)
		cursor := "  "
		if m.section == sectionIdx && i == m.cursor {
			cursor = "> "
		}

		// Type icon with badge style
		typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

		// Status icon with badge style
		statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

		// Task ID
		idStr := MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID))

		// Title (truncated)
		title := task.Title
		maxTitleLen := m.width - 20
		if maxTitleLen < 20 {
			maxTitleLen = 20
		}
		if len(title) > maxTitleLen {
			title = title[:maxTitleLen-3] + "..."
		}

		// Build line
		line := fmt.Sprintf("%s%s %s %s %s", cursor, typeIcon, statusIcon, idStr, title)

		// Highlight selected item
		if m.section == sectionIdx && i == m.cursor {
			line = BoldStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(tasks) > maxVisible {
		remaining := len(tasks) - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}

	return b.String()
}
