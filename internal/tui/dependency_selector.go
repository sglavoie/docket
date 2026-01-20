package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
)

// DependencySelectorModel allows toggling dependencies for a task.
type DependencySelectorModel struct {
	*BasePicker[TaskPickerItem]
	task           *models.Task
	project        *models.Project
	originalDepsOn map[string]bool
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
	candidates := buildDependencyCandidates(task, project)

	picker := NewBasePicker(candidates, PickerConfig{
		Title:          fmt.Sprintf("Dependencies for #%d: %s", task.ShortID, task.Title),
		EmptyMessage:   "No tasks available as dependencies",
		NoMatchMessage: "No matching tasks",
		MultiSelect:    true,
		ShowCounts:     false,
	})

	// Track original dependencies and pre-select them
	originalDepsOn := make(map[string]bool)
	for _, depID := range task.DependsOn {
		originalDepsOn[depID] = true
		picker.PreSelectByID(depID)
	}

	return &DependencySelectorModel{
		BasePicker:     picker,
		task:           task,
		project:        project,
		originalDepsOn: originalDepsOn,
	}
}

// buildDependencyCandidates builds a list of tasks that can be dependencies.
func buildDependencyCandidates(task *models.Task, project *models.Project) []TaskPickerItem {
	// Build set of descendants to exclude
	descendants := make(map[string]bool)
	var markDescendants func(t *models.Task)
	markDescendants = func(t *models.Task) {
		descendants[t.ID] = true
		for _, child := range t.Children {
			markDescendants(child)
		}
	}
	markDescendants(task)

	var candidates []TaskPickerItem
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
		candidates = append(candidates, TaskPickerItem{Task: t})
	}

	return candidates
}

// Init initializes the model.
func (m *DependencySelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *DependencySelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return DependencySelectorDoneMsg{
					Task:       m.task,
					Project:    m.project,
					HasChanges: false,
				}
			}

		case "enter":
			newDeps := m.SelectedIDs()
			hasChanges := !mapsEqual(m.originalDepsOn, m.Selected)
			return m, func() tea.Msg {
				return DependencySelectorDoneMsg{
					Task:       m.task,
					Project:    m.project,
					NewDepsOn:  newDeps,
					HasChanges: hasChanges,
				}
			}

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

// mapsEqual checks if two string-bool maps are equal.
func mapsEqual(a, b map[string]bool) bool {
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

// View renders the dependency selector.
func (m *DependencySelectorModel) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render(m.Config.Title))
	b.WriteString("\n\n")

	// Search bar
	b.WriteString(m.RenderSearchBar())

	// Current dependencies count
	depCount := len(m.Selected)
	if depCount == 0 {
		b.WriteString(MutedStyle.Render("No dependencies selected"))
	} else {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("%d dependenc%s selected", depCount, pluralize(depCount, "y", "ies"))))
	}
	b.WriteString("\n\n")

	// Custom list rendering with task status icons
	b.WriteString(m.renderTaskList())

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Space: toggle | /: search | Enter: confirm | Esc: cancel"))

	return b.String()
}

// renderTaskList renders the task list with status icons.
func (m *DependencySelectorModel) renderTaskList() string {
	var b strings.Builder

	if len(m.Filtered) == 0 {
		b.WriteString(MutedStyle.Render(m.Config.EmptyMessage))
		b.WriteString("\n")
		return b.String()
	}

	maxVisible := 15
	if m.Height() > 0 {
		maxVisible = m.Height() - 12
	}
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := 0
	if m.Cursor >= maxVisible {
		start = m.Cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.Filtered) {
		end = len(m.Filtered)
	}

	for i := start; i < end; i++ {
		item := m.Filtered[i]
		t := item.Task

		// Checkbox
		checkbox := "[ ]"
		if m.Selected[item.ItemID()] {
			checkbox = "[x]"
		}

		// Cursor indicator
		cursor := "  "
		if i == m.Cursor {
			cursor = "> "
		}

		// Status icon with badge style
		statusIcon := GetStatusBadge(string(t.Status), t.Status.Icon())

		// Task info
		idStr := MutedStyle.Render(fmt.Sprintf("#%d", t.ShortID))
		title := t.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}

		line := fmt.Sprintf("%s%s %s %s %s", cursor, checkbox, statusIcon, idStr, title)
		if i == m.Cursor {
			line = BoldStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(m.Filtered) > maxVisible {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", len(m.Filtered)-maxVisible)))
		b.WriteString("\n")
	}

	return b.String()
}
