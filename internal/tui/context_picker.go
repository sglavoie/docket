package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ContextInfo holds a context name and its project count.
type ContextInfo struct {
	Name         string
	ProjectCount int
}

// ContextPickerModel allows selecting a single context to filter by.
type ContextPickerModel struct {
	dashboard   *DashboardModel
	contexts    []ContextInfo // All contexts (first entry is "All Contexts")
	filtered    []ContextInfo
	cursor      int
	width       int
	height      int
	searchMode  bool
	searchInput textinput.Model
}

// ShowContextPickerMsg is sent to open the context picker.
type ShowContextPickerMsg struct {
	Dashboard *DashboardModel
}

// ContextPickerDoneMsg is sent when the context picker is done.
type ContextPickerDoneMsg struct {
	SelectedContext string
	Cancelled       bool
}

// NewContextPickerModel creates a new context picker.
func NewContextPickerModel(dashboard *DashboardModel) *ContextPickerModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Type to filter..."
	searchInput.CharLimit = 50
	searchInput.Width = 30

	m := &ContextPickerModel{
		dashboard:   dashboard,
		cursor:      0,
		searchInput: searchInput,
	}
	m.buildContextList()

	// Position cursor on currently filtered context
	if dashboard.filterContext != "" {
		for i, ctx := range m.contexts {
			if ctx.Name == dashboard.filterContext {
				m.cursor = i
				break
			}
		}
	}

	return m
}

// SetSize sets the view dimensions.
func (m *ContextPickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// buildContextList gathers unique contexts from projects with counts.
func (m *ContextPickerModel) buildContextList() {
	contextCounts := make(map[string]int)
	for _, p := range m.dashboard.projects {
		if p.Context != "" {
			contextCounts[p.Context]++
		}
	}

	// Build sorted list
	m.contexts = make([]ContextInfo, 0, len(contextCounts)+1)

	// Add "All Contexts" option first (empty string value)
	m.contexts = append(m.contexts, ContextInfo{
		Name:         "",
		ProjectCount: len(m.dashboard.projects),
	})

	// Add actual contexts sorted alphabetically
	names := make([]string, 0, len(contextCounts))
	for name := range contextCounts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		m.contexts = append(m.contexts, ContextInfo{
			Name:         name,
			ProjectCount: contextCounts[name],
		})
	}

	m.filtered = m.contexts
}

// filterContextList filters contexts by search query.
func (m *ContextPickerModel) filterContextList(query string) {
	if query == "" {
		m.filtered = m.contexts
	} else {
		var filtered []ContextInfo
		queryLower := strings.ToLower(query)
		for _, ctx := range m.contexts {
			// Always include "All Contexts" option, or match by name
			displayName := ctx.Name
			if displayName == "" {
				displayName = "all contexts"
			}
			if strings.Contains(strings.ToLower(displayName), queryLower) {
				filtered = append(filtered, ctx)
			}
		}
		m.filtered = filtered
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
}

// Init initializes the model.
func (m *ContextPickerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *ContextPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if m.searchMode {
			return m.handleSearchMode(msg)
		}

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return ContextPickerDoneMsg{Cancelled: true}
			}

		case "enter":
			// Select and close
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				selected := m.filtered[m.cursor]
				return m, func() tea.Msg {
					return ContextPickerDoneMsg{
						SelectedContext: selected.Name,
						Cancelled:       false,
					}
				}
			}
			return m, nil

		case "j", "down":
			if len(m.filtered) > 0 {
				m.cursor++
				if m.cursor >= len(m.filtered) {
					m.cursor = 0
				}
			}
			return m, nil

		case "k", "up":
			if len(m.filtered) > 0 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.filtered) - 1
				}
			}
			return m, nil

		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink
		}
	}

	return m, nil
}

// handleSearchMode handles keyboard input while in search mode.
func (m *ContextPickerModel) handleSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchMode = false
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		m.filterContextList("")
		return m, nil

	case "enter":
		m.searchMode = false
		m.searchInput.Blur()
		return m, nil

	default:
		oldVal := m.searchInput.Value()
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		if oldVal != m.searchInput.Value() {
			m.filterContextList(m.searchInput.Value())
		}
		return m, cmd
	}
}

// View renders the context picker.
func (m *ContextPickerModel) View() string {
	var b strings.Builder

	// Calculate box width
	boxWidth := 50
	if m.width > 0 && m.width < boxWidth+10 {
		boxWidth = m.width - 10
	}

	// Build content
	var content strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary)
	content.WriteString(titleStyle.Render("Filter by Context"))
	content.WriteString("\n\n")

	if m.searchMode {
		content.WriteString("Filter: ")
		content.WriteString(m.searchInput.View())
		content.WriteString("\n\n")
	}

	// Current filter indicator
	if m.dashboard.filterContext != "" {
		content.WriteString(MutedStyle.Render(fmt.Sprintf("Current: %s", m.dashboard.filterContext)))
	} else {
		content.WriteString(MutedStyle.Render("Current: All Contexts"))
	}
	content.WriteString("\n\n")

	if len(m.filtered) == 0 {
		content.WriteString(MutedStyle.Render("No matching contexts"))
		content.WriteString("\n")
	} else {
		// Show list
		maxVisible := 15
		if m.height > 0 {
			maxVisible = m.height - 14
		}
		if maxVisible < 5 {
			maxVisible = 5
		}

		start := 0
		if m.cursor >= maxVisible {
			start = m.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := start; i < end; i++ {
			ctx := m.filtered[i]

			// Cursor indicator
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			// Display name
			displayName := ctx.Name
			if displayName == "" {
				displayName = "All Contexts"
			}

			// Check if this is the currently active filter
			isActive := ctx.Name == m.dashboard.filterContext

			// Build line
			line := fmt.Sprintf("%s%s ", cursor, displayName)
			countStr := fmt.Sprintf("(%d)", ctx.ProjectCount)

			if i == m.cursor {
				content.WriteString(TagManagerItemSelectedStyle.Render(line))
			} else if isActive {
				content.WriteString(lipgloss.NewStyle().Foreground(ColorPrimary).Render(line))
			} else {
				content.WriteString(TagManagerItemStyle.Render(line))
			}
			content.WriteString(TagManagerCountStyle.Render(countStr))
			content.WriteString("\n")
		}

		if len(m.filtered) > maxVisible {
			content.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", len(m.filtered)-maxVisible)))
			content.WriteString("\n")
		}
	}

	// Footer hint
	content.WriteString("\n")
	if m.searchMode {
		content.WriteString(MutedStyle.Render("enter done | esc clear"))
	} else if len(m.filtered) > 0 {
		content.WriteString(MutedStyle.Render("/ search | enter select | esc cancel"))
	} else {
		content.WriteString(MutedStyle.Render("esc cancel"))
	}

	// Apply box styling
	box := TagManagerBoxStyle.Width(boxWidth).Render(content.String())

	// Center the box
	boxHeight := lipgloss.Height(box)
	topPadding := (m.height - boxHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}
	leftPadding := (m.width - lipgloss.Width(box)) / 2
	if leftPadding < 0 {
		leftPadding = 0
	}

	// Build final output with padding
	for i := 0; i < topPadding; i++ {
		b.WriteString("\n")
	}

	lines := strings.Split(box, "\n")
	padding := strings.Repeat(" ", leftPadding)
	for _, line := range lines {
		b.WriteString(padding)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}
