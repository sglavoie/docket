package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ContextInfo holds a context name and its project count.
type ContextInfo struct {
	Name         string
	ProjectCount int
	IsCustom     bool // True if this is a custom context (project grouping)
}

// ContextPickerModel allows selecting a single context to filter by.
type ContextPickerModel struct {
	*BasePicker[ContextInfo]
	config              *config.Config // Config reference for custom context operations
	currentFilter       string         // Current filter context for highlighting
	currentCustomFilter string         // Current custom context filter for highlighting
	fromCycle           bool           // True if opened from Project Cycle Manager
	confirm             ConfirmDialog  // confirmation dialog
	statusMessage       string         // Status message to display
	renameMode          bool           // True when renaming a context
	renameInput         textinput.Model
	renameOldName       string // The context name being renamed
	confirmDiscard      bool
	confirmOverlay      ConfirmDialog
}

// ShowContextPickerMsg is sent to open the context picker.
type ShowContextPickerMsg struct {
	Dashboard *DashboardModel
}

// ContextPickerDoneMsg is sent when the context picker is done.
type ContextPickerDoneMsg struct {
	SelectedContext string
	IsCustomContext bool // True if a custom context was selected
	Canceled        bool
	FromCycle       bool // True if picker was opened from cycle manager
}

// ShowCustomContextEditorMsg is sent to open the custom context editor.
type ShowCustomContextEditorMsg struct {
	EditName string // Empty for new, non-empty for edit
}

// CustomContextDeletedMsg is sent when a custom context is deleted.
type CustomContextDeletedMsg struct {
	Name      string
	WasActive bool // True if the deleted context was the active filter
}

// ContextRenamedMsg is sent when a project context is renamed.
type ContextRenamedMsg struct {
	OldName      string
	NewName      string
	ProjectCount int
}

// NewContextPickerModel creates a new context picker from dashboard.
func NewContextPickerModel(dashboard *DashboardModel, cfg *config.Config) *ContextPickerModel {
	contexts := buildContextListWithCustom(dashboard.projects, cfg)

	picker := NewBasePicker(contexts, PickerConfig{
		Title:          "Filter by Context",
		EmptyMessage:   "No contexts available.",
		NoMatchMessage: "No matching contexts.",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	// Position cursor on currently filtered context (custom or regular)
	if cfg.FilterCustomContext != "" {
		picker.SetCursorToID(customContextID(cfg.FilterCustomContext))
	} else if dashboard.filterContext != "" {
		picker.SetCursorToID(dashboard.filterContext)
	}

	return &ContextPickerModel{
		BasePicker:          picker,
		config:              cfg,
		currentFilter:       dashboard.filterContext,
		currentCustomFilter: cfg.FilterCustomContext,
		fromCycle:           false,
	}
}

// NewContextPickerModelFromConfig creates a context picker using config.
func NewContextPickerModelFromConfig(cfg *config.Config) *ContextPickerModel {
	projects := cfg.GetActiveProjects()
	contexts := buildContextListWithCustom(projects, cfg)

	picker := NewBasePicker(contexts, PickerConfig{
		Title:          "Filter by Context",
		EmptyMessage:   "No contexts available.",
		NoMatchMessage: "No matching contexts.",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	// Position cursor on currently filtered context (custom or regular)
	if cfg.FilterCustomContext != "" {
		picker.SetCursorToID(customContextID(cfg.FilterCustomContext))
	} else if cfg.FilterContext != "" {
		picker.SetCursorToID(cfg.FilterContext)
	}

	return &ContextPickerModel{
		BasePicker:          picker,
		config:              cfg,
		currentFilter:       cfg.FilterContext,
		currentCustomFilter: cfg.FilterCustomContext,
		fromCycle:           true,
	}
}

// customContextID returns the ID used to identify custom contexts (prefixed with "+").
func customContextID(name string) string {
	return "+" + name
}

// buildContextListWithCustom gathers contexts including custom contexts from config.
func buildContextListWithCustom(projects []*models.Project, cfg *config.Config) []ContextInfo {
	// Count projects per context
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
		IsCustom:     false,
	})

	// Add custom contexts with "+" prefix, sorted alphabetically
	customNames := cfg.GetCustomContextNames()
	for _, name := range customNames {
		contexts = append(contexts, ContextInfo{
			Name:         name,
			ProjectCount: cfg.GetCustomContextProjectCount(name),
			IsCustom:     true,
		})
	}

	// Add actual project contexts sorted alphabetically
	names := make([]string, 0, len(contextCounts))
	for name := range contextCounts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		contexts = append(contexts, ContextInfo{
			Name:         name,
			ProjectCount: contextCounts[name],
			IsCustom:     false,
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
		// Handle discard confirmation for rename mode
		if m.confirmDiscard {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				m.renameMode = false
				m.renameOldName = ""
				return m, nil
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}

		// Handle rename mode input
		if m.renameMode {
			switch msg.String() {
			case "esc":
				if strings.TrimSpace(m.renameInput.Value()) != m.renameOldName {
					m.confirmDiscard = true
					return m, nil
				}
				m.renameMode = false
				m.renameOldName = ""
				return m, nil
			case "enter":
				newName := strings.TrimSpace(m.renameInput.Value())
				if newName == "" || newName == m.renameOldName {
					m.renameMode = false
					m.renameOldName = ""
					return m, nil
				}
				// Rename context in all projects
				oldName := m.renameOldName
				count := 0
				for _, p := range m.config.Projects {
					if p.Context == oldName {
						p.Context = newName
						count++
					}
				}
				m.renameMode = false
				m.renameOldName = ""
				// Rebuild the list
				projects := m.config.GetActiveProjects()
				newContexts := buildContextListWithCustom(projects, m.config)
				m.Items = newContexts
				m.Filter(m.SearchQuery())
				return m, func() tea.Msg {
					return ContextRenamedMsg{OldName: oldName, NewName: newName, ProjectCount: count}
				}
			default:
				var cmd tea.Cmd
				m.renameInput, cmd = m.renameInput.Update(msg)
				return m, cmd
			}
		}

		// Handle confirmation
		if confirmed, handled := m.confirm.HandleKey(msg.String()); handled {
			if confirmed {
				if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
					selected := m.Filtered[m.Cursor]
					if selected.IsCustom && m.config != nil {
						name := selected.Name
						// If deleting the currently active custom context, clear the filter
						wasActive := m.currentCustomFilter == name
						if wasActive {
							m.config.FilterCustomContext = ""
							m.currentCustomFilter = ""
						}
						m.config.DeleteCustomContext(name)
						// Also clear any cycle projects for this custom context
						m.config.SetCycleProjectsForContext("+"+name, nil)
						// Rebuild the list
						projects := m.config.GetActiveProjects()
						newContexts := buildContextListWithCustom(projects, m.config)
						m.Items = newContexts
						m.Filter(m.SearchQuery())
						if m.Cursor >= len(m.Filtered) {
							m.Cursor = len(m.Filtered) - 1
						}
						if m.Cursor < 0 {
							m.Cursor = 0
						}
						return m, func() tea.Msg {
							return CustomContextDeletedMsg{Name: name, WasActive: wasActive}
						}
					}
				}
			}
			return m, nil
		}

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
				isCustom := selected.IsCustom
				return m, func() tea.Msg {
					return ContextPickerDoneMsg{
						SelectedContext: selected.Name,
						IsCustomContext: isCustom,
						Canceled:        false,
						FromCycle:       fromCycle,
					}
				}
			}
			return m, nil

		case "n":
			// Create new custom context
			return m, func() tea.Msg {
				return ShowCustomContextEditorMsg{EditName: ""}
			}

		case "e":
			// Edit selected custom context (only if it's a custom context)
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				selected := m.Filtered[m.Cursor]
				if selected.IsCustom {
					name := selected.Name
					return m, func() tea.Msg {
						return ShowCustomContextEditorMsg{EditName: name}
					}
				}
			}
			return m, nil

		case "d":
			// Delete selected custom context (only if it's a custom context)
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				selected := m.Filtered[m.Cursor]
				if selected.IsCustom && m.config != nil {
					m.confirm.Show("delete", "Delete this custom context?")
					m.statusMessage = ""
				} else if selected.Name != "" {
					// Can't delete regular contexts
					m.statusMessage = "Cannot delete regular contexts.\nRename them with 'r' instead."
				}
			}
			return m, nil

		case "r":
			// Rename a regular project context (not custom, not "All Contexts")
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				selected := m.Filtered[m.Cursor]
				if !selected.IsCustom && selected.Name != "" {
					m.renameMode = true
					m.renameOldName = selected.Name
					m.renameInput = textinput.New()
					m.renameInput.Placeholder = "New context name"
					m.renameInput.SetValue(selected.Name)
					m.renameInput.Focus()
					m.renameInput.CharLimit = 50
					m.statusMessage = ""
					return m, textinput.Blink
				} else if selected.IsCustom {
					m.statusMessage = "Use 'e' to edit custom contexts"
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
	// Current filter indicator
	var headerContent string
	if m.currentCustomFilter != "" {
		headerContent = MutedStyle.Render(fmt.Sprintf("Current: + %s", m.currentCustomFilter))
	} else if m.currentFilter != "" {
		headerContent = MutedStyle.Render(fmt.Sprintf("Current: %s", m.currentFilter))
	} else {
		headerContent = MutedStyle.Render("Current: All Contexts")
	}

	// Custom render since context picker has special highlighting for active filter
	return m.renderContextPicker(headerContent)
}

// renderContextPicker renders the context picker with custom active highlighting.
func (m *ContextPickerModel) renderContextPicker(headerContent string) string {
	var b strings.Builder

	// Build subtitle showing count
	customCount := 0
	projectContextCount := 0
	for _, ctx := range m.Items {
		if ctx.Name == "" {
			continue // Skip "All Contexts"
		}
		if ctx.IsCustom {
			customCount++
		} else {
			projectContextCount++
		}
	}
	var subtitle string
	if customCount > 0 && projectContextCount > 0 {
		subtitle = fmt.Sprintf("%d custom, %d project contexts", customCount, projectContextCount)
	} else if customCount > 0 {
		subtitle = fmt.Sprintf("%d custom contexts", customCount)
	} else if projectContextCount > 0 {
		subtitle = fmt.Sprintf("%d contexts", projectContextCount)
	} else {
		subtitle = "No contexts defined"
	}

	// Title
	b.WriteString(renderFullscreenTitle("Filter by Context", subtitle))
	b.WriteString("\n\n")

	// Search bar
	b.WriteString(m.RenderSearchBar())

	// Header
	b.WriteString(headerContent)
	b.WriteString("\n\n")

	// List
	if len(m.Filtered) == 0 {
		b.WriteString(MutedStyle.Render(m.Config.NoMatchMessage))
		b.WriteString("\n")
	} else {
		maxVisible := 15
		if m.Height() > 0 {
			maxVisible = m.Height() - 10
		}
		if maxVisible < 5 {
			maxVisible = 5
		}

		start := 0
		if m.Cursor >= maxVisible {
			start = m.Cursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(m.Filtered))

		for i := start; i < end; i++ {
			ctx := m.Filtered[i]

			// Cursor indicator
			cursor := "  "
			if i == m.Cursor {
				cursor = "> "
			}

			// Display name with "+" prefix for custom contexts
			displayName := ctx.DisplayText()
			if ctx.IsCustom {
				displayName = "+ " + displayName
			}

			// Check if this is the currently active filter
			isActive := false
			if ctx.IsCustom && ctx.Name == m.currentCustomFilter {
				isActive = true
			} else if !ctx.IsCustom && ctx.Name == m.currentFilter && m.currentCustomFilter == "" {
				isActive = true
			}

			// Build line
			line := fmt.Sprintf("%s%s ", cursor, displayName)
			countStr := fmt.Sprintf("(%d)", ctx.ProjectCount)

			if i == m.Cursor {
				b.WriteString(TagManagerItemSelectedStyle.Render(line))
			} else if isActive {
				b.WriteString(lipgloss.NewStyle().Foreground(ColorPrimary).Render(line))
			} else {
				b.WriteString(TagManagerItemStyle.Render(line))
			}
			b.WriteString(TagManagerCountStyle.Render(countStr))
			b.WriteString("\n")
		}

		if len(m.Filtered) > maxVisible {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", len(m.Filtered)-maxVisible)))
			b.WriteString("\n")
		}
	}

	// Status message or delete confirmation or rename input
	b.WriteString("\n")
	if m.renameMode {
		b.WriteString("Rename to: ")
		b.WriteString(m.renameInput.View())
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("enter save · esc cancel"))
	} else if m.statusMessage != "" {
		b.WriteString(MutedStyle.Render(m.statusMessage))
	}

	// Footer
	var footerLeft string
	if m.renameMode {
		footerLeft = ""
	} else if m.confirm.IsActive() {
		footerLeft = ""
	} else {
		footerLeft = HelpStyle.Render("n:new · e:edit · d:del · r:rename · enter · esc")
	}
	right := MutedStyle.Render(fmt.Sprintf("%d items", len(m.Filtered)))
	b.WriteString(renderStatusBarFooter(footerLeft, right, m.Width()))

	return b.String()
}

// ShouldCaptureEsc returns true when in rename mode or delete confirmation.
func (m *ContextPickerModel) ShouldCaptureEsc() bool {
	return m.renameMode || m.confirm.IsActive() || m.confirmDiscard
}

// HasTextInput returns true when in rename mode.
func (m *ContextPickerModel) HasTextInput() bool {
	return m.renameMode
}

func (m *ContextPickerModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
		return &m.confirmOverlay
	}
	m.confirmOverlay.Cancel()
	return &m.confirm
}

func (m *ContextPickerModel) StatusBarLabel() string { return "Filter by Context" }
