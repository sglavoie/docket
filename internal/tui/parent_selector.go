package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

const parentSelectorNoParentLabel = "No parent (top-level)"

// ParentPickerItem wraps a Task for parent selection with fuzzy scoring.
type ParentPickerItem struct {
	Task *models.Task
	None bool
}

// FilterText returns searchable text for parent filtering.
func (p ParentPickerItem) FilterText() string {
	if p.None {
		return parentSelectorNoParentLabel
	}
	return fmt.Sprintf("#%d %s", p.Task.ShortID, p.Task.Title)
}

// DisplayText returns the task title for display.
func (p ParentPickerItem) DisplayText() string {
	if p.None {
		return parentSelectorNoParentLabel
	}
	return p.Task.Title
}

// CountValue returns 0 as parents don't have a count.
func (p ParentPickerItem) CountValue() int {
	return 0
}

// ItemID returns the task ID as its unique identifier.
func (p ParentPickerItem) ItemID() string {
	if p.None {
		return "parent:none"
	}
	return p.Task.ID
}

// ParentSelectorModel provides fuzzy-find selection of parent tasks.
type ParentSelectorModel struct {
	*ListSurfaceAdapter[ParentPickerItem]
	allItems     []ParentPickerItem // All valid parent candidates
	allowNone    bool               // Whether "None" is a valid option
	currentID    *string            // Currently selected parent ID (for pre-selection)
	source       PresentationID     // Which view opened this selector
	reparentTask TaskIdentity       // When valid, forwarded back in ParentSelectedMsg
	alwaysSearch *alwaysSearchSinglePicker[ParentPickerItem]
}

// NewParentSelectorModel creates a new parent selector.
func NewParentSelectorModel(validParents []*models.Task, currentID *string, allowNone bool, source PresentationID) *ParentSelectorModel {
	items := make([]ParentPickerItem, 0, len(validParents)+1)
	if allowNone {
		items = append(items, ParentPickerItem{None: true})
	}
	for _, task := range validParents {
		items = append(items, ParentPickerItem{Task: task})
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{
		Title:          "Select Parent",
		EmptyMessage:   "No valid parents available.",
		NoMatchMessage: "No matching parents.",
		MultiSelect:    false,
		ShowCounts:     false,
	})

	m := &ParentSelectorModel{
		ListSurfaceAdapter: picker,
		allItems:           items,
		allowNone:          allowNone,
		currentID:          currentID,
		source:             source,
	}

	// Pre-select current parent if set
	if currentID != nil {
		m.SetCursorToID(*currentID)
	}

	m.alwaysSearch = newAlwaysSearchSinglePicker(m.ListSurfaceAdapter, alwaysSearchSinglePickerConfig[ParentPickerItem]{
		AllItems: items,
		CancelMsg: func() tea.Msg {
			return CloseParentSelectorMsg{}
		},
		SelectMsg: func(cursor int, filtered []ParentPickerItem) tea.Msg {
			if cursor >= len(filtered) {
				return CloseParentSelectorMsg{}
			}
			var parentID *string
			if !filtered[cursor].None {
				parentID = &filtered[cursor].Task.ID
			}

			return ParentSelectedMsg{
				ParentID:     parentID,
				Source:       m.source,
				ReparentTask: m.reparentTask,
			}
		},
	})

	return m
}

// HasActiveSearch returns true (search input is always active).
func (m *ParentSelectorModel) HasActiveSearch() bool {
	return true
}

// HasTextInput returns true because the parent selector always has input focus.
func (m *ParentSelectorModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc lets the selector clear an active query before closing.
func (m *ParentSelectorModel) ShouldCaptureEsc() bool {
	return true
}

func (m *ParentSelectorModel) Init() tea.Cmd {
	return m.alwaysSearch.Init()
}

func (m *ParentSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && m.allowNone && key.String() == "ctrl+x" {
		return m, m.selectNoParent()
	}
	if handled, cmd := m.alwaysSearch.Update(msg); handled {
		return m, cmd
	}
	return m, nil
}

func (m *ParentSelectorModel) selectNoParent() tea.Cmd {
	return func() tea.Msg {
		return ParentSelectedMsg{
			ParentID:     nil,
			Source:       m.source,
			ReparentTask: m.reparentTask,
		}
	}
}

func (m *ParentSelectorModel) View() tea.View {
	var b strings.Builder

	title := "Select Parent"
	parentCount := len(m.allItems)
	if m.allowNone {
		parentCount--
	}
	subtitle := fmt.Sprintf("%d parent%s available", parentCount, pluralize(parentCount, "", "s"))
	availableSubtitleWidth := max(m.Width()-lipgloss.Width(title)-2, 1)
	subtitle = truncateToVisualWidth(subtitle, availableSubtitleWidth)
	b.WriteString(renderFullscreenTitle(title, subtitle))
	b.WriteString("\n\n")

	// Search input
	b.WriteString(m.SearchInputView())
	b.WriteString("\n\n")

	// Render the list
	b.WriteString(m.renderList())

	// Footer
	footer := "↑/↓ navigate · Enter select · Esc cancel"
	if m.allowNone {
		footer = "↑/↓ navigate · Enter select · Ctrl+X no parent · Esc cancel"
	}
	left := HelpStyle.Render(footer)
	right := MutedStyle.Render(fmt.Sprintf("%d items", len(m.FilteredItems())))
	b.WriteString(renderStatusBarFooter(left, right, m.Width()))

	return tea.NewView(b.String())
}

// renderList renders the filtered parent list.
func (m *ParentSelectorModel) renderList() string {
	var b strings.Builder

	maxVisible := max(
		// Account for title, input, footer
		m.Height()-10, 5)

	totalItems := len(m.FilteredItems())
	if totalItems == 0 {
		return MutedStyle.Render("No matching parents.")
	}

	scrollOffset := calcScrollOffset(m.CursorIndex(), maxVisible)

	for i, item := range m.FilteredItems() {
		if i >= scrollOffset && i < scrollOffset+maxVisible {
			if item.None {
				line := MutedStyle.Render(parentSelectorNoParentLabel)
				if m.CursorIndex() == i {
					line = SelectedLine(parentSelectorNoParentLabel, m.Width())
				}
				b.WriteString(line)
				b.WriteString("\n")
				continue
			}

			// Short ID
			shortID := MutedStyle.Render(fmt.Sprintf("#%-4d", item.Task.ShortID))

			prefix := fmt.Sprintf("  %s ", shortID)
			maxTitleLen := max(m.Width()-lipgloss.Width(prefix), 1)
			title := truncateToVisualWidth(item.Task.Title, maxTitleLen)
			titleRendered := TaskTitleStyleFor(item.Task, TaskTitleStyleOptions{
				CursorSelected: m.CursorIndex() == i,
			}).Render(title)
			line := prefix + titleRendered

			if m.CursorIndex() == i {
				line = SelectedLine(line, m.Width())
			}

			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Show scroll indicator if there are more items
	if totalItems > maxVisible {
		remaining := totalItems - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... %d more", remaining)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *ParentSelectorModel) StatusBarLabel() string { return "Select Parent" }
