package tui

import (
	"fmt"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// The boundary filter is a presentation narrowing and nothing else: choosing
// one changes which rows the Project Hub lists, never a project's stored
// boundary. It is deliberately synchronous and embedded, mirroring the
// contextual action sheet, because the Hub drops every message that is not a
// keypress and so could never observe a command-driven picker.

const (
	// projectHubFilterAll admits every project.
	projectHubFilterAll = ""

	// projectHubFilterArchived selects the archived tail. It is not a boundary
	// ID: archived projects retain their boundary, but they are grouped and
	// filtered by lifecycle so the archived tail stays one place.
	projectHubFilterArchived = "project-hub:filter:archived"

	projectHubFilterItemPrefix = "project-hub-filter:"
)

// projectHubFilterOption is one boundary the Hub can be narrowed to.
type projectHubFilterOption struct {
	ID    string
	Label string
	Count int
}

// projectHubFilterPicker is the searchable boundary chooser. Typing filters
// immediately, so the picker never needs a search mode of its own. Chrome is
// supplied by the caller because the same shell answers two different
// questions: which boundary to list, and which boundary receives the members of
// one being deleted.
type projectHubFilterPicker struct {
	surface *ListSurfaceModel
	title   string
	footer  string
}

func newProjectHubFilterPicker(
	title string,
	footer string,
	options []projectHubFilterOption,
	selected string,
) (*projectHubFilterPicker, error) {
	items := make([]ListItem, 0, len(options))
	for _, option := range options {
		label := option.Label
		if option.ID == selected {
			label += " (current)"
		}
		items = append(items, ListItem{
			ID:            ListItemID(projectHubFilterItemPrefix + projectHubFilterKey(option.ID)),
			PrimaryText:   label,
			SecondaryText: projectHubFilterCount(option.Count),
			Aliases:       []string{option.Label},
			Data:          option,
		})
	}
	surface, err := NewListSurfaceModel(items, nil, ListSurfaceConfig{
		SelectionMode: ListSelectionSingle,
		EmptyState:    ListEmptyState{Title: "No boundaries are defined."},
		NoMatchState: ListEmptyState{
			Title:  "No boundaries match this search.",
			Detail: "Clear or change the search.",
		},
	})
	if err != nil {
		return nil, err
	}
	surface.SetCursorToID(ListItemID(projectHubFilterItemPrefix + projectHubFilterKey(selected)))
	return &projectHubFilterPicker{surface: surface, title: title, footer: footer}, nil
}

func projectHubFilterKey(id string) string {
	if id == projectHubFilterAll {
		return "all"
	}
	return id
}

func projectHubFilterCount(count int) string {
	if count == 1 {
		return "1 project"
	}
	return fmt.Sprintf("%d projects", count)
}

// handleKey answers one keypress in place, reporting the chosen option and
// whether the picker is finished.
func (picker *projectHubFilterPicker) handleKey(
	msg tea.KeyPressMsg,
) (projectHubFilterOption, bool, bool) {
	switch msg.String() {
	case "esc":
		if picker.surface.Query().Raw != "" {
			picker.surface.SetQuery("")
			return projectHubFilterOption{}, false, false
		}
		return projectHubFilterOption{}, false, true
	case "backspace":
		query := picker.surface.Query().Raw
		if query != "" {
			_, size := utf8.DecodeLastRuneInString(query)
			picker.surface.SetQuery(query[:len(query)-size])
		}
		return projectHubFilterOption{}, false, false
	case "up", "ctrl+k":
		picker.surface.MoveCursor(-1)
		return projectHubFilterOption{}, false, false
	case "down", "ctrl+j":
		picker.surface.MoveCursor(1)
		return projectHubFilterOption{}, false, false
	case "home":
		picker.surface.MoveCursorToStart()
		return projectHubFilterOption{}, false, false
	case "end":
		picker.surface.MoveCursorToEnd()
		return projectHubFilterOption{}, false, false
	case "enter":
		item, ok := picker.surface.CurrentItem()
		if !ok {
			return projectHubFilterOption{}, false, false
		}
		option, ok := item.Data.(projectHubFilterOption)
		return option, ok, ok
	default:
		if msg.Text != "" {
			picker.surface.SetQuery(picker.surface.Query().Raw + msg.Text)
		}
		return projectHubFilterOption{}, false, false
	}
}

func (picker *projectHubFilterPicker) render(width int) string {
	innerWidth := min(max(width*3/5, 40), max(width-6, 24))
	listHeight := min(max(len(picker.surface.Rows()), 5), 14)
	query := picker.surface.Query().Raw
	content := MutedStyle.Render("Filter: "+query+"▏") + "\n\n" +
		RenderCompactListSurface(picker.surface, innerWidth-4, listHeight)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(picker.title)
	footer := HelpStyle.Render(picker.footer)
	return renderOverlayBox(content, innerWidth, title, footer, lipgloss.NewStyle().Foreground(ColorPrimary))
}
