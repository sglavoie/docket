package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// ParentSelectorModel provides fuzzy-find selection of parent tasks.
type ParentSelectorModel struct {
	validParents []*models.Task // All valid parent candidates
	filtered     []*models.Task // Tasks matching current query
	cursor       int            // Currently selected index (0 = None if allowNone)
	allowNone    bool           // Whether "None" is a valid option
	currentID    *string        // Currently selected parent ID (for pre-selection)
	source       ViewType       // Which view opened this selector

	// Input for filtering
	searchInput textinput.Model

	// Dimensions
	width  int
	height int
}

// NewParentSelectorModel creates a new parent selector.
func NewParentSelectorModel(validParents []*models.Task, currentID *string, allowNone bool, source ViewType) *ParentSelectorModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Type to filter parents..."
	searchInput.CharLimit = 100
	searchInput.Width = 40
	searchInput.Focus()

	m := &ParentSelectorModel{
		validParents: validParents,
		allowNone:    allowNone,
		currentID:    currentID,
		source:       source,
		searchInput:  searchInput,
		cursor:       0,
	}
	m.filtered = validParents

	// Pre-select current parent if set
	if currentID != nil {
		for i, p := range validParents {
			if p.ID == *currentID {
				if allowNone {
					m.cursor = i + 1 // +1 to account for "None" option
				} else {
					m.cursor = i
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

// SetSize sets the view dimensions.
func (m *ParentSelectorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.searchInput.Width = width - 4
}

func (m *ParentSelectorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ParentSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return CloseParentSelectorMsg{}
			}
		case "enter":
			return m, m.selectCurrent()
		case "up", "ctrl+k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+j":
			maxCursor := len(m.filtered)
			if !m.allowNone {
				maxCursor--
			}
			if m.cursor < maxCursor {
				m.cursor++
			}
			return m, nil
		case "ctrl+u":
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			return m, nil
		case "ctrl+d":
			maxCursor := len(m.filtered)
			if !m.allowNone {
				maxCursor--
			}
			m.cursor += 10
			if m.cursor > maxCursor {
				m.cursor = maxCursor
			}
			return m, nil
		default:
			// Update search input
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.filterParents()
			return m, cmd
		}
	}

	return m, nil
}

// selectCurrent returns a command with the selected parent.
func (m *ParentSelectorModel) selectCurrent() tea.Cmd {
	return func() tea.Msg {
		var parentID *string

		if m.allowNone {
			if m.cursor == 0 {
				// "None" selected
				parentID = nil
			} else if m.cursor-1 < len(m.filtered) {
				parentID = &m.filtered[m.cursor-1].ID
			}
		} else {
			if m.cursor < len(m.filtered) {
				parentID = &m.filtered[m.cursor].ID
			}
		}

		return ParentSelectedMsg{
			ParentID: parentID,
			Source:   m.source,
		}
	}
}

// filterParents filters parents based on the current search query.
func (m *ParentSelectorModel) filterParents() {
	query := strings.TrimSpace(m.searchInput.Value())

	if query == "" {
		m.filtered = m.validParents
		m.cursor = 0
		return
	}

	// Score each parent and collect matches
	type scoredTask struct {
		task  *models.Task
		score int
	}
	var scored []scoredTask

	for _, task := range m.validParents {
		score := m.scoreParent(task, query)
		if score > 0 {
			scored = append(scored, scoredTask{task: task, score: score})
		}
	}

	// Sort by score descending (best matches first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract sorted results
	m.filtered = make([]*models.Task, len(scored))
	for i, s := range scored {
		m.filtered[i] = s.task
	}

	// Reset cursor if out of bounds
	maxCursor := len(m.filtered)
	if m.allowNone {
		// cursor 0 is "None", so max is len(filtered)
	} else {
		maxCursor--
	}
	if m.cursor > maxCursor {
		m.cursor = maxCursor
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// scoreParent returns a fuzzy match score for a parent task.
func (m *ParentSelectorModel) scoreParent(task *models.Task, query string) int {
	searchable := fmt.Sprintf("#%d %s %s",
		task.ShortID,
		task.Type.HeadingPrefix(),
		task.Title,
	)
	return fuzzy.ScoreTerms(searchable, query)
}

func (m *ParentSelectorModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render("Select Parent"))
	b.WriteString("\n\n")

	// Search input
	b.WriteString(m.searchInput.View())
	b.WriteString("\n\n")

	// Render the list
	b.WriteString(m.renderList())
	b.WriteString("\n")

	// Footer
	footer := HelpStyle.Render("↑/↓ navigate • Enter select • Esc cancel")
	b.WriteString(footer)

	return b.String()
}

// renderList renders the filtered parent list.
func (m *ParentSelectorModel) renderList() string {
	var b strings.Builder

	maxVisible := m.height - 10 // Account for title, input, footer
	if maxVisible < 5 {
		maxVisible = 5
	}

	// Calculate total items (including "None" if allowed)
	totalItems := len(m.filtered)
	if m.allowNone {
		totalItems++
	}

	if totalItems == 0 {
		return MutedStyle.Render("No matching parents.")
	}

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if m.cursor >= maxVisible {
		scrollOffset = m.cursor - maxVisible + 1
	}

	itemIndex := 0

	// Render "None" option if allowed and in view
	if m.allowNone {
		if itemIndex >= scrollOffset && itemIndex < scrollOffset+maxVisible {
			cursor := "  "
			if m.cursor == 0 {
				cursor = "> "
			}
			line := fmt.Sprintf("%s%s", cursor, "(None)")
			if m.cursor == 0 {
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
	for i, task := range m.filtered {
		cursorIdx := i
		if m.allowNone {
			cursorIdx = i + 1
		}

		if itemIndex >= scrollOffset && itemIndex < scrollOffset+maxVisible {
			cursor := "  "
			if m.cursor == cursorIdx {
				cursor = "> "
			}

			// Type icon
			typeStyle := GetTypeStyle(string(task.Type))
			typeIcon := typeStyle.Render(task.Type.Icon())

			// Short ID
			shortID := MutedStyle.Render(fmt.Sprintf("#%-4d", task.ShortID))

			// Title (truncated)
			maxTitleLen := m.width - 20
			if maxTitleLen < 10 {
				maxTitleLen = 10
			}
			title := util.Truncate(task.Title, maxTitleLen)

			// Build line
			line := fmt.Sprintf("%s%s %s %s", cursor, typeIcon, shortID, title)

			if m.cursor == cursorIdx {
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
