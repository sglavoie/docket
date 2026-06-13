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
}

// FilterText returns searchable text for parent filtering.
func (p ParentPickerItem) FilterText() string {
	return fmt.Sprintf("#%d %s %s", p.Task.ShortID, p.Task.Type.HeadingPrefix(), p.Task.Title)
}

// DisplayText returns the task title for display.
func (p ParentPickerItem) DisplayText() string {
	return p.Task.Title
}

// CountValue returns 0 as parents don't have a count.
func (p ParentPickerItem) CountValue() int {
	return 0
}

// ItemID returns the task ID as its unique identifier.
func (p ParentPickerItem) ItemID() string {
	return p.Task.ID
}

// ParentSelectorModel provides fuzzy-find selection of parent tasks.
type ParentSelectorModel struct {
	*BasePicker[ParentPickerItem]
	allItems       []ParentPickerItem // All valid parent candidates
	allowNone      bool               // Whether "None" is a valid option
	currentID      *string            // Currently selected parent ID (for pre-selection)
	source         ViewType           // Which view opened this selector
	reparentTaskID string             // When non-empty, forwarded back in ParentSelectedMsg
	alwaysSearch   *alwaysSearchSinglePicker[ParentPickerItem]
}

// NewParentSelectorModel creates a new parent selector.
func NewParentSelectorModel(validParents []*models.Task, currentID *string, allowNone bool, source ViewType) *ParentSelectorModel {
	items := make([]ParentPickerItem, len(validParents))
	for i, t := range validParents {
		items[i] = ParentPickerItem{Task: t}
	}

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Select Parent",
		EmptyMessage:   "No valid parents available.",
		NoMatchMessage: "No matching parents.",
		MultiSelect:    false,
		ShowCounts:     false,
	})

	m := &ParentSelectorModel{
		BasePicker: picker,
		allItems:   items,
		allowNone:  allowNone,
		currentID:  currentID,
		source:     source,
	}

	// Pre-select current parent if set
	if currentID != nil {
		for i, item := range items {
			if item.Task.ID == *currentID {
				if allowNone {
					m.Cursor = i + 1 // +1 to account for "None" option
				} else {
					m.Cursor = i
				}
				break
			}
		}
	}

	m.alwaysSearch = newAlwaysSearchSinglePicker(m.BasePicker, alwaysSearchSinglePickerConfig[ParentPickerItem]{
		AllItems: items,
		MaxCursor: func(filtered []ParentPickerItem) int {
			if m.allowNone {
				return len(filtered)
			}
			return max(len(filtered)-1, 0)
		},
		CancelMsg: func() tea.Msg {
			return CloseParentSelectorMsg{}
		},
		SelectMsg: func(cursor int, filtered []ParentPickerItem) tea.Msg {
			var parentID *string

			if m.allowNone {
				if cursor == 0 {
					parentID = nil
				} else if cursor-1 < len(filtered) {
					parentID = &filtered[cursor-1].Task.ID
				}
			} else if cursor < len(filtered) {
				parentID = &filtered[cursor].Task.ID
			}

			return ParentSelectedMsg{
				ParentID:       parentID,
				Source:         m.source,
				ReparentTaskID: m.reparentTaskID,
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
			ParentID:       nil,
			Source:         m.source,
			ReparentTaskID: m.reparentTaskID,
		}
	}
}

func (m *ParentSelectorModel) View() tea.View {
	var b strings.Builder

	title := "Select Parent"
	subtitle := fmt.Sprintf("%d parent%s available", len(m.allItems), pluralize(len(m.allItems), "", "s"))
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
	right := MutedStyle.Render(fmt.Sprintf("%d items", len(m.Filtered)))
	b.WriteString(renderStatusBarFooter(left, right, m.Width()))

	return tea.NewView(b.String())
}

// renderList renders the filtered parent list.
func (m *ParentSelectorModel) renderList() string {
	var b strings.Builder

	maxVisible := max(
		// Account for title, input, footer
		m.Height()-10, 5)

	// Calculate total items (including "None" if allowed)
	totalItems := len(m.Filtered)
	if m.allowNone {
		totalItems++
	}

	if totalItems == 0 {
		return MutedStyle.Render("No matching parents.")
	}

	scrollOffset := calcScrollOffset(m.Cursor, maxVisible)

	itemIndex := 0

	// Render "None" option if allowed and in view
	if m.allowNone {
		if itemIndex >= scrollOffset && itemIndex < scrollOffset+maxVisible {
			cursor := "  "
			if m.Cursor == 0 {
				cursor = "  "
			}
			line := fmt.Sprintf("%s%s", cursor, parentSelectorNoParentLabel)
			if m.Cursor == 0 {
				line = SelectedLine(line, m.Width())
			} else {
				line = MutedStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		itemIndex++
	}

	// Render parent tasks
	for i, item := range m.Filtered {
		cursorIdx := i
		if m.allowNone {
			cursorIdx = i + 1
		}

		if itemIndex >= scrollOffset && itemIndex < scrollOffset+maxVisible {
			cursor := "  "
			if m.Cursor == cursorIdx {
				cursor = "  "
			}

			// Type icon with badge style
			typeIcon := GetTypeBadge(string(item.Task.Type), item.Task.Type.Icon())

			// Short ID
			shortID := MutedStyle.Render(fmt.Sprintf("#%-4d", item.Task.ShortID))

			prefix := fmt.Sprintf("%s%s %s ", cursor, typeIcon, shortID)
			maxTitleLen := max(m.Width()-lipgloss.Width(prefix), 1)
			title := truncateToVisualWidth(item.Task.Title, maxTitleLen)
			titleRendered := TaskTitleStyleFor(item.Task, TaskTitleStyleOptions{
				CursorSelected: m.Cursor == cursorIdx,
			}).Render(title)
			line := prefix + titleRendered

			if m.Cursor == cursorIdx {
				line = SelectedLine(line, m.Width())
			}

			b.WriteString(line)
			b.WriteString("\n")
		}
		itemIndex++
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
