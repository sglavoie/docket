package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// SectionInfo holds section name and task count for display.
type SectionInfo struct {
	Name      string
	TaskCount int
	ColorName string // Optional color name from the section color palette
	IsCustom  bool   // True if this is a user-defined section (not GTD)
}

// SectionFilterSelectorModel allows selecting multiple sections to filter by.
type SectionFilterSelectorModel struct {
	*BasePicker[SectionInfo]
	projectTree *ProjectTreeModel
}

// ShowSectionFilterSelectorMsg is sent to open the section filter selector.
type ShowSectionFilterSelectorMsg struct {
	ProjectTree *ProjectTreeModel
	Project     *models.Project // Used when ProjectTree is nil (e.g., from outliner)
}

// SectionFilterSelectorDoneMsg is sent when the section filter selector is done.
type SectionFilterSelectorDoneMsg struct {
	SelectedSections []string
	Canceled         bool
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

	for section := range pt.filter.Sections {
		picker.PreSelectByID(section)
	}

	return &SectionFilterSelectorModel{
		BasePicker:  picker,
		projectTree: pt,
	}
}

// NewSectionFilterSelectorModelForProject creates a section filter selector
// backed by a standalone project (no pre-selected filters).
func NewSectionFilterSelectorModelForProject(project *models.Project) *SectionFilterSelectorModel {
	sections := buildSectionList(project)

	picker := NewBasePicker(sections, PickerConfig{
		Title:          "Filter by Sections",
		EmptyMessage:   "No sections in project",
		NoMatchMessage: "No matching sections",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	return &SectionFilterSelectorModel{
		BasePicker: picker,
	}
}

// buildSectionList scans all tasks and builds a sorted list of sections with counts.
func buildSectionList(project *models.Project) []SectionInfo {
	sectionCounts := make(map[string]int)

	for _, task := range project.Tasks {
		if task.Section != "" {
			sectionCounts[task.Section]++
		}
	}

	for _, section := range project.GetSections() {
		if _, exists := sectionCounts[section]; !exists {
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
	cmd := filterSelectorUpdate(m.BasePicker, msg,
		func() tea.Msg { return SectionFilterSelectorDoneMsg{Canceled: true} },
		func(selected []string) tea.Msg {
			return SectionFilterSelectorDoneMsg{SelectedSections: selected, Canceled: false}
		},
	)
	return m, cmd
}

// View renders the section filter selector.
func (m *SectionFilterSelectorModel) View() string {
	return filterSelectorView(m.BasePicker, "section")
}
