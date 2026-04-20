package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// ShowCustomSectionPickerMsg is sent to open the custom section picker.
type ShowCustomSectionPickerMsg struct {
	Task     *models.Task
	Project  *models.Project
	Projects []*models.Project // All projects to collect sections from
}

// CustomSectionPickerDoneMsg is sent when the custom section picker completes.
type CustomSectionPickerDoneMsg struct {
	Task     *models.Task
	Project  *models.Project
	Section  string // "" means unset (remove custom section)
	Canceled bool
}

// CustomSectionPickerModel allows selecting a custom section to assign to a task.
type CustomSectionPickerModel struct {
	*BasePicker[SectionInfo]
	allItems []SectionInfo
	task     *models.Task
	project  *models.Project
}

// NewCustomSectionPickerModel creates a picker showing only custom sections
// gathered from all provided projects.
func NewCustomSectionPickerModel(task *models.Task, project *models.Project, projects []*models.Project) *CustomSectionPickerModel {
	items := buildCustomSectionList(project, projects)

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Move to Custom Section",
		EmptyMessage:   "No custom sections in project.",
		NoMatchMessage: "No matching sections.",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	m := &CustomSectionPickerModel{
		BasePicker: picker,
		allItems:   items,
		task:       task,
		project:    project,
	}

	// Activate search immediately
	m.SetSearchActive(true)
	m.searchInput.Focus()

	// Pre-select current custom section
	if task.CustomSection != "" {
		m.SetCursorToID(task.CustomSection)
	}

	return m
}

// buildCustomSectionList builds a list of custom sections gathered from all
// provided projects, with task counts from all projects combined.
func buildCustomSectionList(taskProject *models.Project, projects []*models.Project) []SectionInfo {
	customCounts := make(map[string]int)

	// Count custom sections across ALL projects
	for _, p := range projects {
		for _, task := range p.Tasks {
			if task.CustomSection != "" {
				customCounts[task.CustomSection]++
			}
		}
	}

	// Include all project-defined custom sections, even if empty
	for _, p := range projects {
		for _, section := range p.GetCustomSections() {
			if _, exists := customCounts[section]; !exists {
				customCounts[section] = 0
			}
		}
	}

	items := make([]SectionInfo, 0, len(customCounts)+1)

	// Add "Unset" option first (removes custom section)
	unsetCount := 0
	for _, task := range taskProject.Tasks {
		if task.CustomSection == "" {
			unsetCount++
		}
	}
	items = append(items, SectionInfo{Name: "", TaskCount: unsetCount})

	// Add custom sections sorted alphabetically
	var names []string
	for name := range customCounts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		colorName := taskProject.GetSectionColor(name)
		items = append(items, SectionInfo{
			Name:      name,
			TaskCount: customCounts[name],
			ColorName: colorName,
			IsCustom:  true,
		})
	}

	return items
}

// HasActiveSearch returns true (search input is always active).
func (m *CustomSectionPickerModel) HasActiveSearch() bool {
	return true
}

// HasTextInput returns true because the picker always has input focus.
func (m *CustomSectionPickerModel) HasTextInput() bool {
	return true
}

func (m *CustomSectionPickerModel) Init() tea.Cmd {
	return m.ActivateSearch()
}

func (m *CustomSectionPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if m.IsSearchMode() && m.SearchQuery() != "" {
				m.ClearSearch()
				m.fuzzyFilter("")
				return m, nil
			}
			return m, func() tea.Msg {
				return CustomSectionPickerDoneMsg{
					Task:     m.task,
					Project:  m.project,
					Canceled: true,
				}
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
func (m *CustomSectionPickerModel) moveCursor(delta int) {
	maxCursor := max(len(m.Filtered)-1, 0)
	m.Cursor += delta
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	if m.Cursor > maxCursor {
		m.Cursor = maxCursor
	}
}

// fuzzyFilter filters sections using fuzzy scoring.
func (m *CustomSectionPickerModel) fuzzyFilter(query string) {
	query = strings.TrimSpace(query)

	if query == "" {
		m.Filtered = m.allItems
		m.Cursor = 0
		return
	}

	type scoredItem struct {
		item  SectionInfo
		score int
	}
	var scored []scoredItem

	for _, item := range m.allItems {
		score := fuzzy.ScoreTerms(item.FilterText(), query)
		if score > 0 {
			scored = append(scored, scoredItem{item: item, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	m.Filtered = make([]SectionInfo, len(scored))
	for i, s := range scored {
		m.Filtered[i] = s.item
	}

	maxCursor := max(len(m.Filtered)-1, 0)
	if m.Cursor > maxCursor {
		m.Cursor = maxCursor
	}
}

// selectCurrent returns a command with the selected section.
func (m *CustomSectionPickerModel) selectCurrent() tea.Cmd {
	return func() tea.Msg {
		if m.Cursor >= len(m.Filtered) || len(m.Filtered) == 0 {
			return CustomSectionPickerDoneMsg{
				Task:     m.task,
				Project:  m.project,
				Canceled: true,
			}
		}

		selected := m.Filtered[m.Cursor]
		return CustomSectionPickerDoneMsg{
			Task:    m.task,
			Project: m.project,
			Section: selected.Name,
		}
	}
}

func (m *CustomSectionPickerModel) View() tea.View {
	var b strings.Builder

	title := "Move to Custom Section"
	taskInfo := fmt.Sprintf("Task #%d: %s", m.task.ShortID, m.task.Title)
	availableSubtitleWidth := max(m.Width()-lipgloss.Width(title)-2, 1)
	if lipgloss.Width(taskInfo) > availableSubtitleWidth {
		taskInfo = truncateToVisualWidth(taskInfo, availableSubtitleWidth)
	}
	b.WriteString(renderFullscreenTitle(title, taskInfo))
	b.WriteString("\n\n")

	// Search input
	b.WriteString(m.SearchInputView())
	b.WriteString("\n\n")

	// Current section
	current := "(no custom section)"
	if m.task.CustomSection != "" {
		current = m.task.CustomSection
	}
	b.WriteString(MutedStyle.Render(fmt.Sprintf("Current: %s", current)))
	b.WriteString("\n\n")

	// List
	b.WriteString(m.renderList())
	b.WriteString("\n")

	// Footer
	b.WriteString(renderStatusBarFooter(
		HelpStyle.Render("↑/↓ navigate · Enter select · Esc cancel"),
		MutedStyle.Render(fmt.Sprintf("%d items", len(m.Filtered))),
		m.Width(),
	))

	return tea.NewView(b.String())
}

// renderList renders the filtered section list.
func (m *CustomSectionPickerModel) renderList() string {
	var b strings.Builder

	maxVisible := max(m.Height()-12, 5)
	totalItems := len(m.Filtered)

	if totalItems == 0 {
		return MutedStyle.Render("No matching sections.")
	}

	scrollOffset := calcScrollOffset(m.Cursor, maxVisible)
	rowWidth := max(m.Width()-4, 30)

	for i, item := range m.Filtered {
		if i < scrollOffset || i >= scrollOffset+maxVisible {
			continue
		}

		// Display text
		displayText := item.DisplayText()
		if item.Name == "" {
			displayText = "(unset)"
		}

		// Count
		countStr := TagManagerCountStyle.Render(fmt.Sprintf(" (%d)", item.CountValue()))

		content := "  " + displayText + countStr
		if m.Cursor == i {
			b.WriteString(SelectedLine(content, rowWidth))
		} else {
			b.WriteString(content)
		}
		b.WriteString("\n")
	}

	if totalItems > maxVisible {
		remaining := totalItems - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... %d more", remaining)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// ShouldCaptureEsc returns true when search has a query, so first Esc clears it.
func (m *CustomSectionPickerModel) ShouldCaptureEsc() bool {
	return m.IsSearchMode() && m.SearchQuery() != ""
}

func (m *CustomSectionPickerModel) StatusBarLabel() string { return "Custom Section" }
