package tui

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// ShowProjectPickerMsg opens the global project picker overlay.
type ShowProjectPickerMsg struct{}

// ProjectPickerDoneMsg returns the result from the project picker.
type ProjectPickerDoneMsg struct {
	Project  *models.Project
	Canceled bool
}

// ProjectPickerModel provides fuzzy-find selection of projects for quick switching.
type ProjectPickerModel struct {
	*BasePicker[ProjectPickerItem]
	alwaysSearch *alwaysSearchSinglePicker[ProjectPickerItem]
}

// NewProjectPickerModel creates a new project picker for switching projects.
func NewProjectPickerModel(projects []*models.Project) *ProjectPickerModel {
	// Sort projects alphabetically
	sorted := make([]*models.Project, len(projects))
	copy(sorted, projects)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Archived != sorted[j].Archived {
			return !sorted[i].Archived
		}
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})

	items := make([]ProjectPickerItem, len(sorted))
	for i, p := range sorted {
		items[i] = ProjectPickerItem{Project: p}
	}

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Switch Project",
		EmptyMessage:   "No projects available.",
		NoMatchMessage: "No matching projects.",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	m := &ProjectPickerModel{
		BasePicker: picker,
	}

	m.alwaysSearch = newAlwaysSearchSinglePicker(m.BasePicker, alwaysSearchSinglePickerConfig[ProjectPickerItem]{
		AllItems: items,
		UpKeys:   []string{"up", "ctrl+k", "ctrl+p"},
		DownKeys: []string{"down", "ctrl+j", "ctrl+n"},
		CancelMsg: func() tea.Msg {
			return ProjectPickerDoneMsg{Canceled: true}
		},
		SelectMsg: func(cursor int, filtered []ProjectPickerItem) tea.Msg {
			if cursor >= len(filtered) || len(filtered) == 0 {
				return ProjectPickerDoneMsg{Canceled: true}
			}

			selected := filtered[cursor]
			return ProjectPickerDoneMsg{Project: selected.Project}
		},
	})
	m.PrimeAlwaysSearch()

	return m
}

// HasActiveSearch returns true (search input is always active).
func (m *ProjectPickerModel) HasActiveSearch() bool {
	return true
}

// HasTextInput returns true because the picker always has input focus.
func (m *ProjectPickerModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns true so esc is handled by the picker, not the global handler.
func (m *ProjectPickerModel) ShouldCaptureEsc() bool {
	return true
}

func (m *ProjectPickerModel) Init() tea.Cmd {
	return m.alwaysSearch.Init()
}

func (m *ProjectPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := m.alwaysSearch.Update(msg); handled {
		return m, cmd
	}
	return m, nil
}

func (m *ProjectPickerModel) StatusBarLabel() string { return "Switch Project" }
