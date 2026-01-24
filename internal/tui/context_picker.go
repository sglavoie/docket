package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ContextInfo holds a context name and its project count.
type ContextInfo struct {
	Name         string
	ProjectCount int
}

// ContextPickerModel allows selecting a single context to filter by.
type ContextPickerModel struct {
	*BasePicker[ContextInfo]
	currentFilter string // Current filter context for highlighting
	fromCycle     bool   // True if opened from Project Cycle Manager
}

// ShowContextPickerMsg is sent to open the context picker.
type ShowContextPickerMsg struct {
	Dashboard *DashboardModel
}

// ContextPickerDoneMsg is sent when the context picker is done.
type ContextPickerDoneMsg struct {
	SelectedContext string
	Canceled        bool
	FromCycle       bool // True if picker was opened from cycle manager
}

// NewContextPickerModel creates a new context picker from dashboard.
func NewContextPickerModel(dashboard *DashboardModel) *ContextPickerModel {
	contexts := buildContextListFromProjects(dashboard.projects)

	picker := NewBasePicker(contexts, PickerConfig{
		Title:          "Filter by Context",
		EmptyMessage:   "No contexts available",
		NoMatchMessage: "No matching contexts",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	// Position cursor on currently filtered context
	if dashboard.filterContext != "" {
		picker.SetCursorToID(dashboard.filterContext)
	}

	return &ContextPickerModel{
		BasePicker:    picker,
		currentFilter: dashboard.filterContext,
		fromCycle:     false,
	}
}

// NewContextPickerModelFromConfig creates a context picker using config.
func NewContextPickerModelFromConfig(cfg *config.Config) *ContextPickerModel {
	projects := cfg.GetActiveProjects()
	contexts := buildContextListFromProjects(projects)

	picker := NewBasePicker(contexts, PickerConfig{
		Title:          "Filter by Context",
		EmptyMessage:   "No contexts available",
		NoMatchMessage: "No matching contexts",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	// Position cursor on currently filtered context
	if cfg.FilterContext != "" {
		picker.SetCursorToID(cfg.FilterContext)
	}

	return &ContextPickerModel{
		BasePicker:    picker,
		currentFilter: cfg.FilterContext,
		fromCycle:     true,
	}
}

// buildContextListFromProjects gathers unique contexts from projects with counts.
func buildContextListFromProjects(projects []*models.Project) []ContextInfo {
	contextCounts := make(map[string]int)
	for _, p := range projects {
		if p.Context != "" {
			contextCounts[p.Context]++
		}
	}

	// Build sorted list
	contexts := make([]ContextInfo, 0, len(contextCounts)+1)

	// Add "All Contexts" option first (empty string value)
	contexts = append(contexts, ContextInfo{
		Name:         "",
		ProjectCount: len(projects),
	})

	// Add actual contexts sorted alphabetically
	names := make([]string, 0, len(contextCounts))
	for name := range contextCounts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		contexts = append(contexts, ContextInfo{
			Name:         name,
			ProjectCount: contextCounts[name],
		})
	}

	return contexts
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
		switch msg.String() {
		case "q", "esc":
			fromCycle := m.fromCycle
			return m, func() tea.Msg {
				return ContextPickerDoneMsg{Canceled: true, FromCycle: fromCycle}
			}

		case "enter":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				selected := m.Filtered[m.Cursor]
				fromCycle := m.fromCycle
				return m, func() tea.Msg {
					return ContextPickerDoneMsg{
						SelectedContext: selected.Name,
						Canceled:        false,
						FromCycle:       fromCycle,
					}
				}
			}
			return m, nil

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

// View renders the context picker.
func (m *ContextPickerModel) View() string {
	boxWidth := 50
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	// Current filter indicator
	var headerContent string
	if m.currentFilter != "" {
		headerContent = MutedStyle.Render(fmt.Sprintf("Current: %s", m.currentFilter))
	} else {
		headerContent = MutedStyle.Render("Current: All Contexts")
	}

	// Custom render since context picker has special highlighting for active filter
	return m.renderContextPicker(boxWidth, headerContent)
}

// renderContextPicker renders the context picker with custom active highlighting.
func (m *ContextPickerModel) renderContextPicker(boxWidth int, headerContent string) string {
	var content strings.Builder

	// Search bar
	content.WriteString(m.RenderSearchBar())

	// Header
	content.WriteString(headerContent)
	content.WriteString("\n\n")

	// List
	if len(m.Filtered) == 0 {
		content.WriteString(MutedStyle.Render(m.Config.NoMatchMessage))
		content.WriteString("\n")
	} else {
		maxVisible := 15
		if m.Height() > 0 {
			maxVisible = m.Height() - 14
		}
		if maxVisible < 5 {
			maxVisible = 5
		}

		start := 0
		if m.Cursor >= maxVisible {
			start = m.Cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.Filtered) {
			end = len(m.Filtered)
		}

		for i := start; i < end; i++ {
			ctx := m.Filtered[i]

			// Cursor indicator
			cursor := "  "
			if i == m.Cursor {
				cursor = "> "
			}

			// Display name
			displayName := ctx.DisplayText()

			// Check if this is the currently active filter
			isActive := ctx.Name == m.currentFilter

			// Build line
			line := fmt.Sprintf("%s%s ", cursor, displayName)
			countStr := fmt.Sprintf("(%d)", ctx.ProjectCount)

			if i == m.Cursor {
				content.WriteString(TagManagerItemSelectedStyle.Render(line))
			} else if isActive {
				content.WriteString(lipgloss.NewStyle().Foreground(ColorPrimary).Render(line))
			} else {
				content.WriteString(TagManagerItemStyle.Render(line))
			}
			content.WriteString(TagManagerCountStyle.Render(countStr))
			content.WriteString("\n")
		}

		if len(m.Filtered) > maxVisible {
			content.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", len(m.Filtered)-maxVisible)))
			content.WriteString("\n")
		}
	}

	// Footer
	content.WriteString("\n")
	content.WriteString(m.RenderFooter())

	// Build subtitle showing count
	subtitle := fmt.Sprintf("%d contexts", len(m.Items)-1) // -1 for "All Contexts" option
	if len(m.Items) == 1 {
		subtitle = "No contexts defined"
	}

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    "Filter by Context",
		Subtitle: subtitle,
		Width:    boxWidth,
		Centered: true,
	})
}
