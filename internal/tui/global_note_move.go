package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// GlobalNoteMoveDoneMsg indicates the user selected a project (or canceled).
type GlobalNoteMoveDoneMsg struct {
	Project *models.Project // nil if canceled
}

// GlobalNoteMoveProjectSelectorModel provides fuzzy-find selection of projects
// for moving a global note into a project.
type GlobalNoteMoveProjectSelectorModel struct {
	*BasePicker[ProjectPickerItem]
	alwaysSearch *alwaysSearchSinglePicker[ProjectPickerItem]
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
	}
	m.alwaysSearch = newAlwaysSearchSinglePicker(m.BasePicker, alwaysSearchSinglePickerConfig[ProjectPickerItem]{
		AllItems: items,
		CancelMsg: func() tea.Msg {
			return GlobalNoteMoveDoneMsg{}
		},
		SelectMsg: func(cursor int, filtered []ProjectPickerItem) tea.Msg {
			if cursor >= len(filtered) || len(filtered) == 0 {
				return GlobalNoteMoveDoneMsg{}
			}
			return GlobalNoteMoveDoneMsg{Project: filtered[cursor].Project}
		},
	})
	m.PrimeAlwaysSearch()
	return m
}

func (m *GlobalNoteMoveProjectSelectorModel) HasActiveSearch() bool  { return true }
func (m *GlobalNoteMoveProjectSelectorModel) HasTextInput() bool     { return true }
func (m *GlobalNoteMoveProjectSelectorModel) ShouldCaptureEsc() bool { return true }

func (m *GlobalNoteMoveProjectSelectorModel) Init() tea.Cmd {
	return m.alwaysSearch.Init()
}

func (m *GlobalNoteMoveProjectSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := m.alwaysSearch.Update(msg); handled {
		return m, cmd
	}
	return m, nil
}

func (m *GlobalNoteMoveProjectSelectorModel) View() tea.View {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("Move Note to Project"))
	b.WriteString("\n\n")

	b.WriteString(m.SearchInputView())
	b.WriteString("\n\n")

	b.WriteString(m.renderList())
	b.WriteString("\n")

	b.WriteString(HelpStyle.Render(renderWrappingHelpLine("↑/↓ navigate · Enter select · Esc cancel", m.Width())))

	return tea.NewView(b.String())
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
			cursor = "  "
		}

		contextBadge := MutedStyle.Render(fmt.Sprintf("[%s]", item.Project.Context))
		maxNameLen := max(m.Width()-lipgloss.Width(cursor)-lipgloss.Width(contextBadge)-1, 1)
		name := truncateToVisualWidth(item.Project.Name, maxNameLen)
		line := fmt.Sprintf("%s%s %s", cursor, truncateAndPad(name, maxNameLen), contextBadge)

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
