package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/fuzzy"
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
	allItems []ProjectPickerItem
}

// NewProjectPickerModel creates a new project picker for switching projects.
func NewProjectPickerModel(projects []*models.Project) *ProjectPickerModel {
	// Sort projects alphabetically
	sorted := make([]*models.Project, len(projects))
	copy(sorted, projects)
	sort.Slice(sorted, func(i, j int) bool {
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
		allItems:   items,
	}

	m.SetSearchActive(true)
	m.searchInput.Focus()

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
	return m.ActivateSearch()
}

func (m *ProjectPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.IsSearchMode() && m.SearchQuery() != "" {
				m.ClearSearch()
				m.fuzzyFilter("")
				return m, nil
			}
			return m, func() tea.Msg {
				return ProjectPickerDoneMsg{Canceled: true}
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

func (m *ProjectPickerModel) moveCursor(delta int) {
	maxCursor := len(m.Filtered) - 1
	if maxCursor < 0 {
		maxCursor = 0
	}

	m.Cursor += delta
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	if m.Cursor > maxCursor {
		m.Cursor = maxCursor
	}
}

func (m *ProjectPickerModel) fuzzyFilter(query string) {
	query = strings.TrimSpace(query)

	if query == "" {
		m.Filtered = m.allItems
		m.Cursor = 0
		return
	}

	type scoredItem struct {
		item  ProjectPickerItem
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

	m.Filtered = make([]ProjectPickerItem, len(scored))
	for i, s := range scored {
		m.Filtered[i] = s.item
	}

	maxCursor := len(m.Filtered) - 1
	if maxCursor < 0 {
		maxCursor = 0
	}
	if m.Cursor > maxCursor {
		m.Cursor = maxCursor
	}
}

func (m *ProjectPickerModel) selectCurrent() tea.Cmd {
	return func() tea.Msg {
		if m.Cursor >= len(m.Filtered) || len(m.Filtered) == 0 {
			return ProjectPickerDoneMsg{Canceled: true}
		}

		selected := m.Filtered[m.Cursor]
		return ProjectPickerDoneMsg{
			Project:  selected.Project,
			Canceled: false,
		}
	}
}
