package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	task         *models.Task
	project      *models.Project
	alwaysSearch *alwaysSearchSinglePicker[SectionInfo]
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
		task:       task,
		project:    project,
	}
	m.alwaysSearch = newAlwaysSearchSinglePicker(m.BasePicker, alwaysSearchSinglePickerConfig[SectionInfo]{
		AllItems: items,
		CancelMsg: func() tea.Msg {
			return CustomSectionPickerDoneMsg{
				Task:     m.task,
				Project:  m.project,
				Canceled: true,
			}
		},
		SelectMsg: func(cursor int, filtered []SectionInfo) tea.Msg {
			if cursor >= len(filtered) || len(filtered) == 0 {
				return CustomSectionPickerDoneMsg{
					Task:     m.task,
					Project:  m.project,
					Canceled: true,
				}
			}

			selected := filtered[cursor]
			return CustomSectionPickerDoneMsg{
				Task:    m.task,
				Project: m.project,
				Section: selected.Name,
			}
		},
	})

	// Activate search immediately
	m.PrimeAlwaysSearch()

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
	return m.alwaysSearch.Init()
}

func (m *CustomSectionPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := m.alwaysSearch.Update(msg); handled {
		return m, cmd
	}
	return m, nil
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
