package tui

import (
	"sort"

	tea "charm.land/bubbletea/v2"

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
}

// ShowSectionFilterSelectorMsg is sent to open the section filter selector.
// Projects, when set, builds the section list across the given projects (the
// global cross-project Outliner); otherwise Project builds it for one project.
type ShowSectionFilterSelectorMsg struct {
	Project  *models.Project
	Projects []*models.Project
}

// SectionFilterSelectorDoneMsg is sent when the section filter selector is done.
type SectionFilterSelectorDoneMsg struct {
	SelectedSections []string
	Canceled         bool
}

// NewSectionFilterSelectorModelForProject creates a section filter selector
// backed by a standalone project (no pre-selected filters).
func NewSectionFilterSelectorModelForProject(project *models.Project) *SectionFilterSelectorModel {
	return newSectionFilterSelectorModel(buildSectionList(project))
}

// NewSectionFilterSelectorModelForProjects creates a section filter selector
// whose section list merges GTD sections and task counts across several
// projects, so the cross-project Outliner can filter by section in global scope.
func NewSectionFilterSelectorModelForProjects(projects []*models.Project) *SectionFilterSelectorModel {
	return newSectionFilterSelectorModel(buildSectionListForProjects(projects))
}

func newSectionFilterSelectorModel(sections []SectionInfo) *SectionFilterSelectorModel {
	picker := NewBasePicker(sections, PickerConfig{
		Title:          "Filter by Sections",
		EmptyMessage:   "No sections in project.",
		NoMatchMessage: "No matching sections.",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	return &SectionFilterSelectorModel{
		BasePicker: picker,
	}
}

// buildSectionList scans all tasks and builds a sorted list of sections with counts.
func buildSectionList(project *models.Project) []SectionInfo {
	return buildSectionListForProjects([]*models.Project{project})
}

// buildSectionListForProjects merges GTD section names and task counts across
// every given project into one sorted list.
func buildSectionListForProjects(projects []*models.Project) []SectionInfo {
	sectionCounts := make(map[string]int)

	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if bucket := taskSectionBucket(task); bucket != "" && models.IsGTDSection(bucket) {
				sectionCounts[bucket]++
			}
		}

		for _, section := range project.GetSections() {
			if !models.IsGTDSection(section) {
				continue
			}
			if _, exists := sectionCounts[section]; !exists {
				if section == models.SectionNext {
					continue
				}
				sectionCounts[section] = 0
			}
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
func (m *SectionFilterSelectorModel) View() tea.View {
	return tea.NewView(filterSelectorView(m.BasePicker, "section"))
}

func (m *SectionFilterSelectorModel) StatusBarLabel() string { return "Filter by Sections" }
