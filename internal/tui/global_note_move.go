package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// GlobalNoteMoveDoneMsg indicates the user selected a project (or canceled).
type GlobalNoteMoveDoneMsg struct {
	Project *models.Project // nil if canceled
}

// GlobalNoteMoveProjectSelectorModel provides fuzzy-find selection of projects
// for moving a global note into a project.
type GlobalNoteMoveProjectSelectorModel struct {
	*BasePicker[ProjectPickerItem]
	allItems []ProjectPickerItem
}

// NewGlobalNoteMoveProjectSelectorModel creates a project selector for moving a global note.
func NewGlobalNoteMoveProjectSelectorModel(projects []*models.Project) *GlobalNoteMoveProjectSelectorModel {
	var filtered []*models.Project
	for _, p := range projects {
		if !p.Archived {
			filtered = append(filtered, p)
		}
	}

	items := make([]ProjectPickerItem, len(filtered))
	for i, p := range filtered {
		items[i] = ProjectPickerItem{Project: p}
	}

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Move Note to Project",
		EmptyMessage:   "No projects available.",
		NoMatchMessage: "No matching projects.",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	m := &GlobalNoteMoveProjectSelectorModel{
		BasePicker: picker,
		allItems:   items,
	}
	m.SetSearchActive(true)
	m.searchInput.Focus()
	return m
}

func (m *GlobalNoteMoveProjectSelectorModel) HasActiveSearch() bool  { return true }
func (m *GlobalNoteMoveProjectSelectorModel) HasTextInput() bool     { return true }
func (m *GlobalNoteMoveProjectSelectorModel) ShouldCaptureEsc() bool { return true }

func (m *GlobalNoteMoveProjectSelectorModel) Init() tea.Cmd {
	return m.ActivateSearch()
}

func (m *GlobalNoteMoveProjectSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			return m, func() tea.Msg { return GlobalNoteMoveDoneMsg{} }
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

func (m *GlobalNoteMoveProjectSelectorModel) moveCursor(delta int) {
	maxCursor := max(len(m.Filtered)-1, 0)
	m.Cursor += delta
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	if m.Cursor > maxCursor {
		m.Cursor = maxCursor
	}
}

func (m *GlobalNoteMoveProjectSelectorModel) fuzzyFilter(query string) {
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
	maxCursor := max(len(m.Filtered)-1, 0)
	if m.Cursor > maxCursor {
		m.Cursor = maxCursor
	}
}

func (m *GlobalNoteMoveProjectSelectorModel) selectCurrent() tea.Cmd {
	return func() tea.Msg {
		if m.Cursor >= len(m.Filtered) || len(m.Filtered) == 0 {
			return GlobalNoteMoveDoneMsg{}
		}
		return GlobalNoteMoveDoneMsg{Project: m.Filtered[m.Cursor].Project}
	}
}

func (m *GlobalNoteMoveProjectSelectorModel) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("Move Note to Project"))
	b.WriteString("\n\n")

	b.WriteString(m.SearchInputView())
	b.WriteString("\n\n")

	b.WriteString(m.renderList())
	b.WriteString("\n")

	b.WriteString(HelpStyle.Render("↑/↓ navigate · Enter select · Esc cancel"))

	return b.String()
}

func (m *GlobalNoteMoveProjectSelectorModel) renderList() string {
	var b strings.Builder

	maxVisible := max(m.Height()-10, 5)

	totalItems := len(m.Filtered)
	if totalItems == 0 {
		return MutedStyle.Render("No matching projects.")
	}

	scrollOffset := calcScrollOffset(m.Cursor, maxVisible)

	for i, item := range m.Filtered {
		if i < scrollOffset || i >= scrollOffset+maxVisible {
			continue
		}

		cursor := "  "
		if m.Cursor == i {
			cursor = "> "
		}

		contextBadge := MutedStyle.Render(fmt.Sprintf("[%s]", item.Project.Context))
		maxNameLen := max(m.Width()-20, 10)
		name := util.Truncate(item.Project.Name, maxNameLen)
		line := fmt.Sprintf("%s%-*s %s", cursor, maxNameLen, name, contextBadge)

		if m.Cursor == i {
			line = SelectedItemTextStyle.Render(line)
		}

		b.WriteString(line)
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

func (m *GlobalNoteMoveProjectSelectorModel) StatusBarLabel() string { return "Move Note" }
