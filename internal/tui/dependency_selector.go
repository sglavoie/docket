package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
)

// DependencySelectorModel allows toggling dependencies for a task.
type DependencySelectorModel struct {
	task           *models.Task
	project        *models.Project
	candidates     []*models.Task
	filtered       []*models.Task
	cursor         int
	done           bool
	cancelled      bool
	searchMode     bool
	searchInput    textinput.Model
	width          int
	height         int
	originalDepsOn map[string]bool // Original dependencies
	currentDepsOn  map[string]bool // Current selection
}

// DependencySelectorDoneMsg is sent when the dependency selector is done.
type DependencySelectorDoneMsg struct {
	Task       *models.Task
	Project    *models.Project
	NewDepsOn  []string
	HasChanges bool
}

// NewDependencySelectorModel creates a new dependency selector.
func NewDependencySelectorModel(task *models.Task, project *models.Project) *DependencySelectorModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Type to filter..."
	searchInput.CharLimit = 50
	searchInput.Width = 40

	// Build candidate list (all tasks except self and descendants)
	descendants := make(map[string]bool)
	var markDescendants func(t *models.Task)
	markDescendants = func(t *models.Task) {
		descendants[t.ID] = true
		for _, child := range t.Children {
			markDescendants(child)
		}
	}
	markDescendants(task)

	var candidates []*models.Task
	for _, t := range project.Tasks {
		if t.ID == task.ID {
			continue
		}
		if descendants[t.ID] {
			continue
		}
		// Skip tasks that would create circular dependencies
		if project.HasCircularDependency(task.ID, t.ID) {
			continue
		}
		// Skip completed tasks unless they're already a dependency
		if t.IsComplete() && !task.HasDependency(t.ID) {
			continue
		}
		candidates = append(candidates, t)
	}

	// Track original and current dependencies
	originalDepsOn := make(map[string]bool)
	currentDepsOn := make(map[string]bool)
	for _, depID := range task.DependsOn {
		originalDepsOn[depID] = true
		currentDepsOn[depID] = true
	}

	return &DependencySelectorModel{
		task:           task,
		project:        project,
		candidates:     candidates,
		filtered:       candidates,
		cursor:         0,
		searchInput:    searchInput,
		originalDepsOn: originalDepsOn,
		currentDepsOn:  currentDepsOn,
	}
}

func (m *DependencySelectorModel) Init() tea.Cmd {
	return textinput.Blink
}

// SetSize updates the view dimensions.
func (m *DependencySelectorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *DependencySelectorModel) filterCandidates(query string) {
	if query == "" {
		m.filtered = m.candidates
	} else {
		var filtered []*models.Task
		queryLower := strings.ToLower(query)
		for _, t := range m.candidates {
			if strings.Contains(strings.ToLower(t.Title), queryLower) ||
				strings.Contains(fmt.Sprintf("#%d", t.ShortID), queryLower) {
				filtered = append(filtered, t)
			}
		}
		m.filtered = filtered
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
}

func (m *DependencySelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				m.filterCandidates("")
				return m, nil
			case "enter":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			default:
				oldVal := m.searchInput.Value()
				m.searchInput, cmd = m.searchInput.Update(msg)
				if oldVal != m.searchInput.Value() {
					m.filterCandidates(m.searchInput.Value())
				}
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "esc":
			m.done = true
			m.cancelled = true
			return m, func() tea.Msg {
				return DependencySelectorDoneMsg{
					Task:       m.task,
					Project:    m.project,
					HasChanges: false,
				}
			}
		case "enter":
			// Confirm and apply changes
			m.done = true
			var newDeps []string
			for depID := range m.currentDepsOn {
				newDeps = append(newDeps, depID)
			}
			hasChanges := !m.mapsEqual(m.originalDepsOn, m.currentDepsOn)
			return m, func() tea.Msg {
				return DependencySelectorDoneMsg{
					Task:       m.task,
					Project:    m.project,
					NewDepsOn:  newDeps,
					HasChanges: hasChanges,
				}
			}
		case "j", "down":
			if len(m.filtered) > 0 {
				m.cursor = (m.cursor + 1) % len(m.filtered)
			}
		case "k", "up":
			if len(m.filtered) > 0 {
				m.cursor = (m.cursor - 1 + len(m.filtered)) % len(m.filtered)
			}
		case " ":
			// Toggle dependency
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				t := m.filtered[m.cursor]
				if m.currentDepsOn[t.ID] {
					delete(m.currentDepsOn, t.ID)
				} else {
					m.currentDepsOn[t.ID] = true
				}
			}
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink
		}
	}

	return m, nil
}

func (m *DependencySelectorModel) mapsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func (m *DependencySelectorModel) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	b.WriteString(titleStyle.Render(fmt.Sprintf("Dependencies for #%d: %s", m.task.ShortID, m.task.Title)))
	b.WriteString("\n\n")

	if m.searchMode {
		b.WriteString("Filter: ")
		b.WriteString(m.searchInput.View())
		b.WriteString("\n\n")
	}

	// Current dependencies count
	depCount := len(m.currentDepsOn)
	if depCount == 0 {
		b.WriteString(MutedStyle.Render("No dependencies selected"))
	} else {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("%d dependenc%s selected", depCount, pluralize(depCount, "y", "ies"))))
	}
	b.WriteString("\n\n")

	if len(m.filtered) == 0 {
		b.WriteString(MutedStyle.Render("No tasks available as dependencies"))
	} else {
		// Show list with checkboxes
		maxVisible := 15
		if m.height > 0 {
			maxVisible = m.height - 12
		}
		if maxVisible < 5 {
			maxVisible = 5
		}

		start := 0
		if m.cursor >= maxVisible {
			start = m.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := start; i < end; i++ {
			t := m.filtered[i]

			// Checkbox
			checkbox := "[ ]"
			if m.currentDepsOn[t.ID] {
				checkbox = "[x]"
			}

			// Cursor indicator
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			// Status icon
			statusStyle := GetStatusStyle(string(t.Status))
			statusIcon := statusStyle.Render(t.Status.Icon())

			// Task info
			idStr := MutedStyle.Render(fmt.Sprintf("#%d", t.ShortID))
			title := t.Title
			if len(title) > 50 {
				title = title[:47] + "..."
			}

			line := fmt.Sprintf("%s%s %s %s %s", cursor, checkbox, statusIcon, idStr, title)
			if i == m.cursor {
				line = BoldStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}

		if len(m.filtered) > maxVisible {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", len(m.filtered)-maxVisible)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	b.WriteString(helpStyle.Render("Space: toggle | /: search | Enter: confirm | Esc: cancel"))

	return b.String()
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
