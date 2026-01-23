package tui

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// SectionInfo holds section name and task count for display.
type SectionInfo struct {
	Name      string
	TaskCount int
}

// SectionFilterSelectorModel allows selecting multiple sections to filter by.
type SectionFilterSelectorModel struct {
	*BasePicker[SectionInfo]
	projectTree *ProjectTreeModel
}

// ShowSectionFilterSelectorMsg is sent to open the section filter selector.
type ShowSectionFilterSelectorMsg struct {
	ProjectTree *ProjectTreeModel
}

// SectionFilterSelectorDoneMsg is sent when the section filter selector is done.
type SectionFilterSelectorDoneMsg struct {
	SelectedSections []string
	Cancelled        bool
}

// NewSectionFilterSelectorModel creates a new section filter selector.
func NewSectionFilterSelectorModel(pt *ProjectTreeModel) *SectionFilterSelectorModel {
	sections := buildSectionList(pt.project)

	picker := NewBasePicker(sections, PickerConfig{
		Title:          "Filter by Sections",
		EmptyMessage:   "No sections in project",
		NoMatchMessage: "No matching sections",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	// Pre-select currently filtered sections
	for section := range pt.filter.Sections {
		picker.PreSelectByID(section)
	}

	return &SectionFilterSelectorModel{
		BasePicker:  picker,
		projectTree: pt,
	}
}

// buildSectionList scans all tasks and builds a sorted list of sections with counts.
func buildSectionList(project *models.Project) []SectionInfo {
	sectionCounts := make(map[string]int)

	// Count tasks per section
	for _, task := range project.Tasks {
		if task.Section != "" {
			sectionCounts[task.Section]++
		}
	}

	// Include all project-defined sections, even if empty
	for _, section := range project.GetSections() {
		if _, exists := sectionCounts[section]; !exists {
			// Skip empty Next
			if section == models.SectionNext {
				continue
			}
			sectionCounts[section] = 0
		}
	}

	sections := make([]SectionInfo, 0, len(sectionCounts))
	for name, count := range sectionCounts {
		sections = append(sections, SectionInfo{Name: name, TaskCount: count})
	}

	// Sort: Next first (if present), then alphabetically
	sort.Slice(sections, func(i, j int) bool {
		if sections[i].Name == models.SectionNext {
			return true
		}
		if sections[j].Name == models.SectionNext {
			return false
		}
		return sections[i].Name < sections[j].Name
	})

	return sections
}

// Init initializes the model.
func (m *SectionFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *SectionFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return SectionFilterSelectorDoneMsg{Cancelled: true}
			}

		case "enter":
			selectedSections := m.SelectedIDs()
			sort.Strings(selectedSections)
			return m, func() tea.Msg {
				return SectionFilterSelectorDoneMsg{
					SelectedSections: selectedSections,
					Cancelled:        false,
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

// View renders the section filter selector.
func (m *SectionFilterSelectorModel) View() string {
	// Selection count header
	selectedCount := len(m.Selected)
	var headerContent string
	if selectedCount == 0 {
		headerContent = MutedStyle.Render("No sections selected (showing all)")
	} else {
		headerContent = MutedStyle.Render(fmt.Sprintf("%d section%s selected", selectedCount, pluralize(selectedCount, "", "s")))
	}

	return m.RenderInBox(m.Config.Title, headerContent, 0)
}
