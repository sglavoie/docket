package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// DependencySelectorModel allows toggling dependencies for a task.
type DependencySelectorModel struct {
	*ListSurfaceAdapter[TaskPickerItem]
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

	picker := NewListSurfaceAdapter(candidates, PickerConfig{
		Title: fmt.Sprintf("Dependencies for #%d: %s",
			task.ShortID,
			TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(task.Title)),
		EmptyMessage:   "No tasks available as dependencies.",
		NoMatchMessage: "No matching tasks.",
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
		ListSurfaceAdapter: picker,
		task:               task,
		project:            project,
		originalDepsOn:     originalDepsOn,
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

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return DependencySelectorDoneMsg{
					Task:       m.task,
					Project:    m.project,
					HasChanges: false,
				}
			}

		case "enter":
			newDeps := m.SelectedIDs()
			hasChanges := !mapsEqual(m.originalDepsOn, m.SelectionMap())
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
func (m *DependencySelectorModel) View() tea.View {
	if m.task == nil {
		return tea.NewView("No task selected")
	}

	var b strings.Builder

	title := "Dependencies"
	subtitlePrefix := fmt.Sprintf("Task #%d: ", m.task.ShortID)
	availableTitleWidth := max(m.Width()-lipgloss.Width(title)-lipgloss.Width(subtitlePrefix)-4, 1)
	taskTitle := truncateToVisualWidth(m.task.Title, availableTitleWidth)
	titleContent := fullscreenTitleStyle().Render(title) + "  " +
		MutedStyle.Render(subtitlePrefix) +
		TaskTitleStyleFor(m.task, TaskTitleStyleOptions{}).Render(taskTitle)
	b.WriteString(renderFullscreenTitleContent(titleContent, ""))
	b.WriteString("\n\n")

	// Search bar
	b.WriteString(m.RenderSearchBar())

	// Current dependencies count
	depCount := len(m.SelectionMap())
	if depCount == 0 {
		b.WriteString(MutedStyle.Render("No dependencies selected"))
	} else {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("%d dependenc%s selected", depCount, pluralize(depCount, "y", "ies"))))
	}
	b.WriteString("\n\n")

	// Custom list rendering with task status icons
	b.WriteString(m.renderTaskList())

	// Footer
	left := HelpStyle.Render("Space: toggle · /: search · Enter: confirm · Esc: cancel")
	right := MutedStyle.Render(fmt.Sprintf("%d items", len(m.FilteredItems())))
	b.WriteString(renderStatusBarFooter(left, right, m.Width()))

	return tea.NewView(b.String())
}

// renderTaskList renders the task list with status icons.
func (m *DependencySelectorModel) renderTaskList() string {
	var b strings.Builder

	if len(m.FilteredItems()) == 0 {
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
	if m.CursorIndex() >= maxVisible {
		start = m.CursorIndex() - maxVisible + 1
	}
	end := min(start+maxVisible, len(m.FilteredItems()))

	for i := start; i < end; i++ {
		item := m.FilteredItems()[i]
		t := item.Task

		// Checkbox
		checkbox := "[ ]"
		if m.SelectionMap()[item.ItemID()] {
			checkbox = "[x]"
		}

		// Cursor indicator
		cursor := "  "
		if i == m.CursorIndex() {
			cursor = "  "
		}

		// Status icon with badge style
		statusIcon := GetStateBadge(t.State())

		// Type icon with badge style
		typeIcon := GetTypeBadge(string(t.Type), t.Type.Icon())

		// Task info
		idStr := MutedStyle.Render(fmt.Sprintf("#%d", t.ShortID))
		prefix := fmt.Sprintf("%s%s %s %s %s ", cursor, checkbox, typeIcon, statusIcon, idStr)
		title := truncateToVisualWidth(t.Title, max(m.Width()-lipgloss.Width(prefix), 1))
		titleRendered := TaskTitleStyleFor(t, TaskTitleStyleOptions{
			CursorSelected: i == m.CursorIndex(),
		}).Render(title)

		line := prefix + titleRendered
		if i == m.CursorIndex() {
			line = BoldStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(m.FilteredItems()) > maxVisible {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", len(m.FilteredItems())-maxVisible)))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *DependencySelectorModel) StatusBarLabel() string { return "Dependencies" }
