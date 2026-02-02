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
	allItems  []ParentPickerItem // All valid parent candidates
	allowNone bool               // Whether "None" is a valid option
	currentID *string            // Currently selected parent ID (for pre-selection)
	source    ViewType           // Which view opened this selector
}

// NewParentSelectorModel creates a new parent selector.
func NewParentSelectorModel(validParents []*models.Task, currentID *string, allowNone bool, source ViewType) *ParentSelectorModel {
	items := make([]ParentPickerItem, len(validParents))
	for i, t := range validParents {
		items[i] = ParentPickerItem{Task: t}
	}

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Select Parent",
		EmptyMessage:   "No valid parents available",
		NoMatchMessage: "No matching parents",
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

func (m *ParentSelectorModel) Init() tea.Cmd {
	return m.ActivateSearch()
}

func (m *ParentSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				return CloseParentSelectorMsg{}
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
			// Handle search input
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

// moveCursor moves the cursor by delta, accounting for the "None" option.
func (m *ParentSelectorModel) moveCursor(delta int) {
	maxCursor := len(m.Filtered)
	if !m.allowNone {
		maxCursor--
	}
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

// fuzzyFilter filters parents using fuzzy scoring instead of substring matching.
func (m *ParentSelectorModel) fuzzyFilter(query string) {
	query = strings.TrimSpace(query)

	if query == "" {
		m.Filtered = m.allItems
		m.Cursor = 0
		return
	}

	// Score each parent and collect matches
	type scoredItem struct {
		item  ParentPickerItem
		score int
	}
	var scored []scoredItem

	for _, item := range m.allItems {
		score := fuzzy.ScoreTerms(item.FilterText(), query)
		if score > 0 {
			scored = append(scored, scoredItem{item: item, score: score})
		}
	}

	// Sort by score descending (best matches first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract sorted results
	m.Filtered = make([]ParentPickerItem, len(scored))
	for i, s := range scored {
		m.Filtered[i] = s.item
	}

	// Reset cursor if out of bounds
	maxCursor := len(m.Filtered)
	if m.allowNone {
		// cursor 0 is "None", so max is len(filtered)
	} else if maxCursor > 0 {
		maxCursor--
	}
	if m.Cursor > maxCursor {
		m.Cursor = maxCursor
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
}

// selectCurrent returns a command with the selected parent.
func (m *ParentSelectorModel) selectCurrent() tea.Cmd {
	return func() tea.Msg {
		var parentID *string

		if m.allowNone {
			if m.Cursor == 0 {
				// "None" selected
				parentID = nil
			} else if m.Cursor-1 < len(m.Filtered) {
				parentID = &m.Filtered[m.Cursor-1].Task.ID
			}
		} else {
			if m.Cursor < len(m.Filtered) {
				parentID = &m.Filtered[m.Cursor].Task.ID
			}
		}

		return ParentSelectedMsg{
			ParentID: parentID,
			Source:   m.source,
		}
	}
}

func (m *ParentSelectorModel) View() string {
	var b strings.Builder

	// Search input
	b.WriteString(m.SearchInputView())
	b.WriteString("\n\n")

	// Render the list
	b.WriteString(m.renderList())
	b.WriteString("\n")

	// Footer
	footer := HelpStyle.Render("↑/↓ navigate • Enter select • Esc cancel")
	b.WriteString(footer)

	boxWidth := 80
	subtitle := fmt.Sprintf("%d parent%s available", len(m.allItems), pluralize(len(m.allItems), "", "s"))

	return renderOverlayCard(b.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    "Select Parent",
		Subtitle: subtitle,
		Width:    boxWidth,
		CenterH:  true,
	})
}

// renderList renders the filtered parent list.
func (m *ParentSelectorModel) renderList() string {
	var b strings.Builder

	maxVisible := m.Height() - 10 // Account for title, input, footer
	if maxVisible < 5 {
		maxVisible = 5
	}

	// Calculate total items (including "None" if allowed)
	totalItems := len(m.Filtered)
	if m.allowNone {
		totalItems++
	}

	if totalItems == 0 {
		return MutedStyle.Render("No matching parents.")
	}

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if m.Cursor >= maxVisible {
		scrollOffset = m.Cursor - maxVisible + 1
	}

	itemIndex := 0

	// Render "None" option if allowed and in view
	if m.allowNone {
		if itemIndex >= scrollOffset && itemIndex < scrollOffset+maxVisible {
			cursor := "  "
			if m.Cursor == 0 {
				cursor = "> "
			}
			line := fmt.Sprintf("%s%s", cursor, "(None)")
			if m.Cursor == 0 {
				line = SelectorOptionSelectedStyle.Render(line)
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
				cursor = "> "
			}

			// Type icon with badge style
			typeIcon := GetTypeBadge(string(item.Task.Type), item.Task.Type.Icon())

			// Short ID
			shortID := MutedStyle.Render(fmt.Sprintf("#%-4d", item.Task.ShortID))

			// Title (truncated)
			maxTitleLen := m.Width() - 20
			if maxTitleLen < 10 {
				maxTitleLen = 10
			}
			title := util.Truncate(item.Task.Title, maxTitleLen)

			// Build line
			line := fmt.Sprintf("%s%s %s %s", cursor, typeIcon, shortID, title)

			if m.Cursor == cursorIdx {
				line = BoldStyle.Foreground(ColorPrimary).Render(line)
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
