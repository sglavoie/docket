package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/template"
)

// TaskTemplatesModel handles the task templates management view.
type TaskTemplatesModel struct {
	BaseSearchableView
	config        *config.Config
	configManager *config.Manager

	InteractiveViewState
	allTemplates      []template.TaskTemplate
	filteredTemplates []template.TaskTemplate
	defaultCount      int // Number of default templates (at start of allTemplates slice)

	expandedView  bool
	previewScroll int // Scroll offset for preview pane content
}

// NewTaskTemplatesModel creates a new task templates model.
func NewTaskTemplatesModel(cfg *config.Config, manager *config.Manager) *TaskTemplatesModel {
	defaultTemplates, customTemplates := cfg.GetTaskTemplatesSorted()
	allTemplates := append(defaultTemplates, customTemplates...)
	m := &TaskTemplatesModel{
		config:            cfg,
		configManager:     manager,
		allTemplates:      allTemplates,
		filteredTemplates: allTemplates,
		defaultCount:      len(defaultTemplates),
	}
	m.SetSize(80, 24)
	m.InitSearchInput("Search templates...", 50, 30)
	return m
}

// SetSize sets the view dimensions.
func (m *TaskTemplatesModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
}

func (m *TaskTemplatesModel) Init() tea.Cmd {
	return nil
}

func (m *TaskTemplatesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TemplateDeletedMsg:
		if msg.Error != nil {
			return m, func() tea.Msg { return ErrorMsg{Err: msg.Error} }
		}
		// Rebuild arrays from config (state mutation happens here, not in tea.Cmd)
		defaultTemplates, customTemplates := m.config.GetTaskTemplatesSorted()
		m.allTemplates = append(defaultTemplates, customTemplates...)
		m.defaultCount = len(defaultTemplates)
		m.filterTemplates()
		// Clamp cursor after deletion
		if m.cursor >= len(m.filteredTemplates) && len(m.filteredTemplates) > 0 {
			m.cursor = len(m.filteredTemplates) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, func() tea.Msg { return TaskTemplatesUpdatedMsg{} }

	case tea.KeyMsg:
		// Handle expanded view mode first (narrow screen only)
		if m.expandedView && !m.isWideScreen() {
			switch msg.String() {
			case "esc", "enter", "q":
				m.expandedView = false
				m.previewScroll = 0
				return m, nil
			case " ":
				// Toggle enabled/disabled from preview
				if len(m.filteredTemplates) > 0 {
					return m, m.toggleCurrentTemplate()
				}
				return m, nil
			case "e":
				// Edit from preview
				if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
					m.expandedView = false
					m.previewScroll = 0
					t := m.filteredTemplates[m.cursor]
					return m, func() tea.Msg {
						return ShowTaskTemplateEditorMsg{Template: t}
					}
				}
				return m, nil
			case "d", "D":
				// Delete from preview
				if len(m.filteredTemplates) > 0 {
					m.confirm.Show("delete", fmt.Sprintf("Delete template '%s'? (y/n)", m.filteredTemplates[m.cursor].Name))
				}
				return m, nil
			case "ctrl+d":
				m.previewScroll += 5
				return m, nil
			case "ctrl+u":
				m.previewScroll = max(0, m.previewScroll-5)
				return m, nil
			case "j", "down":
				m.previewScroll++
				return m, nil
			case "k", "up":
				m.previewScroll = max(0, m.previewScroll-1)
				return m, nil
			}
			return m, nil
		}

		if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
			if confirmed {
				cmd := m.deleteCurrentTemplate()
				m.expandedView = false // Close preview after delete
				return m, cmd
			}
			return m, nil
		}

		// Handle search mode
		if model, cmd, handled := m.HandleSearchMode(msg, m, m.filterTemplates); handled {
			return model, cmd
		}

		// Preview pane scroll (wide screen mode)
		if m.isWideScreen() {
			switch msg.String() {
			case "ctrl+d":
				m.previewScroll += 5
				return m, nil
			case "ctrl+u":
				m.previewScroll = max(0, m.previewScroll-5)
				return m, nil
			}
		}

		if m.pendingPrefix == "g" {
			m.pendingPrefix = ""
			handleGotoTopPrefix(msg.String(), &m.cursor, func() { m.previewScroll = 0 })
			return m, nil
		}

		switch msg.String() {
		case "esc":
			// If there's an active search filter, clear it instead of closing
			if m.searchQuery != "" {
				m.ClearSearch()
				m.filterTemplates()
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "q":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "/":
			return m, m.ActivateSearch()

		case "j", "down":
			if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates)-1 {
				m.cursor++
				m.previewScroll = 0 // Reset scroll when changing selection
			}
			return m, nil

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.previewScroll = 0 // Reset scroll when changing selection
			}
			return m, nil

		case "g":
			m.pendingPrefix = "g"
			return m, nil

		case "G":
			if len(m.filteredTemplates) > 0 {
				m.cursor = len(m.filteredTemplates) - 1
				m.previewScroll = 0
			}
			return m, nil

		case " ":
			// Toggle enabled/disabled
			if len(m.filteredTemplates) > 0 {
				return m, m.toggleCurrentTemplate()
			}
			return m, nil

		case "enter":
			// Show expanded view with full template details (narrow screens only)
			if !m.isWideScreen() && len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
				m.expandedView = true
			}
			return m, nil

		case "d", "D":
			// Delete template (with confirmation)
			if len(m.filteredTemplates) > 0 {
				m.confirm.Show("delete", fmt.Sprintf("Delete template '%s'? (y/n)", m.filteredTemplates[m.cursor].Name))
			}
			return m, nil

		case "n":
			// Create new template
			return m, func() tea.Msg {
				return ShowTaskTemplateCreatorMsg{}
			}

		case "e":
			// Edit selected template
			if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
				t := m.filteredTemplates[m.cursor]
				return m, func() tea.Msg {
					return ShowTaskTemplateEditorMsg{Template: t}
				}
			}
			return m, nil
		}
	}

	return m, nil
}

// isDefaultTemplate returns true if the template with the given ID is a default template.
func (m *TaskTemplatesModel) isDefaultTemplate(id string) bool {
	for i := 0; i < m.defaultCount && i < len(m.allTemplates); i++ {
		if m.allTemplates[i].ID == id {
			return true
		}
	}
	return false
}

// filterTemplates filters allTemplates based on the current search query.
func (m *TaskTemplatesModel) filterTemplates() {
	query := strings.ToLower(m.searchQuery)
	if query == "" {
		m.filteredTemplates = m.allTemplates
		if m.cursor >= len(m.filteredTemplates) && len(m.filteredTemplates) > 0 {
			m.cursor = len(m.filteredTemplates) - 1
		}
		return
	}

	m.filteredTemplates = nil
	for _, t := range m.allTemplates {
		nameLower := strings.ToLower(t.Name)
		summaryLower := strings.ToLower(t.Summary())
		categoryLower := strings.ToLower(t.Category)
		if strings.Contains(nameLower, query) || strings.Contains(summaryLower, query) || strings.Contains(categoryLower, query) {
			m.filteredTemplates = append(m.filteredTemplates, t)
		}
	}

	// Clamp cursor to filtered results
	if m.cursor >= len(m.filteredTemplates) {
		if len(m.filteredTemplates) > 0 {
			m.cursor = len(m.filteredTemplates) - 1
		} else {
			m.cursor = 0
		}
	}
}

func (m *TaskTemplatesModel) toggleCurrentTemplate() tea.Cmd {
	return func() tea.Msg {
		if m.cursor >= len(m.filteredTemplates) {
			return nil
		}

		t := &m.filteredTemplates[m.cursor]
		enabled, found := m.config.ToggleTaskTemplate(t.ID)
		if !found {
			return ErrorMsg{Err: fmt.Errorf("template not found")}
		}

		// Update local copy
		t.Enabled = enabled

		// Also update in allTemplates
		for i := range m.allTemplates {
			if m.allTemplates[i].ID == t.ID {
				m.allTemplates[i].Enabled = enabled
				break
			}
		}

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return TaskTemplatesUpdatedMsg{}
	}
}

func (m *TaskTemplatesModel) deleteCurrentTemplate() tea.Cmd {
	// Capture the ID synchronously before returning the command
	if m.cursor >= len(m.filteredTemplates) {
		return nil
	}
	templateID := m.filteredTemplates[m.cursor].ID

	return func() tea.Msg {
		if !m.config.DeleteTaskTemplate(templateID) {
			return TemplateDeletedMsg{DeletedID: templateID, Error: fmt.Errorf("template not found")}
		}

		if err := m.configManager.Save(m.config); err != nil {
			return TemplateDeletedMsg{DeletedID: templateID, Error: fmt.Errorf("failed to save: %w", err)}
		}

		return TemplateDeletedMsg{DeletedID: templateID, Error: nil}
	}
}

// isWideScreen returns true if the screen is wide enough for split layout (≥81 chars)
func (m *TaskTemplatesModel) isWideScreen() bool {
	return m.Width() >= 81
}

func (m *TaskTemplatesModel) View() string {
	var b strings.Builder

	// Fullscreen preview mode on narrow screens
	if m.expandedView && !m.isWideScreen() && len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
		return m.renderFullscreenPreview()
	}

	b.WriteString(renderFullscreenTitle("Task Templates", fmt.Sprintf("%d templates", len(m.filteredTemplates))))
	b.WriteString("\n\n")

	// Show search input when active
	if m.searchActive || m.searchQuery != "" {
		b.WriteString("/")
		b.WriteString(m.SearchInputView())
		if m.searchQuery != "" && !m.searchActive {
			b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d matches)", len(m.filteredTemplates))))
		}
		b.WriteString("\n\n")
	}

	// Build list content
	listContent := m.renderTemplatesList()

	// Wide screen: split layout with preview
	if m.isWideScreen() {
		listWidth := m.Width() * 45 / 100
		previewWidth := m.Width() - listWidth - 3 // 3 for separator

		// Calculate available height for content
		// Title(1) + blank(1) + search(2 if active) + footer(2) = ~6 lines overhead
		availableHeight := m.Height() - 6
		if m.searchActive || m.searchQuery != "" {
			availableHeight -= 2
		}
		if availableHeight < 5 {
			availableHeight = 5
		}

		listPane := lipgloss.NewStyle().
			Width(listWidth).
			Height(availableHeight).
			Render(listContent)

		var previewPane string
		if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
			previewPane = m.renderPreviewPane(previewWidth, availableHeight)
		} else {
			previewPane = lipgloss.NewStyle().
				Width(previewWidth).
				Foreground(ColorMuted).
				Render("No template selected")
		}
		previewPane = lipgloss.NewStyle().
			Width(previewWidth).
			Height(availableHeight).
			Render(previewPane)

		// Full-height separator
		separator := SeparatorStyle.Render(strings.Repeat("│\n", availableHeight))

		mainContent := lipgloss.JoinHorizontal(lipgloss.Top, listPane, " ", separator, " ", previewPane)
		b.WriteString(mainContent)
	} else {
		// Narrow screen: list only
		b.WriteString(listContent)
	}

	// Footer area - confirmation dialog replaces help when active
	b.WriteString("\n")
	if m.confirm.IsActive() {
		b.WriteString(m.confirm.Render())
	} else {
		b.WriteString(renderStatusBarFooter(HelpStyle.Render(m.renderHelpText()), "", m.Width()))
	}

	return b.String()
}

// renderTemplatesList renders the list of templates.
func (m *TaskTemplatesModel) renderTemplatesList() string {
	var b strings.Builder

	if len(m.filteredTemplates) == 0 {
		if m.searchQuery != "" {
			b.WriteString(MutedStyle.Render("No templates match your search. Press Esc to clear search."))
		} else {
			b.WriteString(MutedStyle.Render("No task templates configured. Use 'tt template init' to add defaults."))
		}
		return b.String()
	}

	// Calculate visible area - adjust for wide screen (less height needed for summary)
	maxVisible := m.Height() - 12
	if m.isWideScreen() {
		maxVisible = m.Height() - 10 // More room since preview is on the side
	}
	if maxVisible < 3 {
		maxVisible = 3
	}

	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.filteredTemplates) {
		endIdx = len(m.filteredTemplates)
	}

	// Show scroll indicator for templates above
	if startIdx > 0 {
		b.WriteString(ScrollUpIndicator(startIdx))
		b.WriteString("\n")
	}

	// Calculate max name length based on available width
	var maxNameLen int
	if m.isWideScreen() {
		listWidth := m.Width() * 45 / 100
		maxNameLen = listWidth - 7
	} else {
		maxNameLen = m.Width() - 9
	}
	if maxNameLen < 15 {
		maxNameLen = 15
	}

	for i := startIdx; i < endIdx; i++ {
		t := m.filteredTemplates[i]
		isDefault := m.isDefaultTemplate(t.ID)

		// Status indicator
		var status string
		if t.Enabled {
			status = DoneStyle.Render("●")
		} else {
			status = MutedStyle.Render("○")
		}

		// Default marker
		defaultMarker := ""
		effectiveMaxLen := maxNameLen
		if isDefault {
			defaultMarker = MutedStyle.Render("[D]") + " "
			effectiveMaxLen -= 4 // Account for [D] marker
		}

		// Template name - use full available width
		name := t.Name
		if len(name) > effectiveMaxLen {
			name = name[:effectiveMaxLen-3] + "..."
		}

		if i == m.cursor {
			line := SelectedItemTextStyle.Render(fmt.Sprintf("▸ %s %s%s", status, defaultMarker, name))
			b.WriteString(line)
			b.WriteString("\n")
			// Show summary for selected template on narrow screens only
			if !m.isWideScreen() {
				summary := t.Summary()
				maxSummaryLen := m.Width() - 8
				if maxSummaryLen < 20 {
					maxSummaryLen = 20
				}
				if len(summary) > maxSummaryLen {
					summary = summary[:maxSummaryLen-3] + "..."
				}
				b.WriteString(MutedStyle.Render(fmt.Sprintf("    %s", summary)))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(fmt.Sprintf("  %s %s%s", status, defaultMarker, name))
			b.WriteString("\n")
		}
	}

	// Show scroll indicator if needed
	if len(m.filteredTemplates) > maxVisible {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  %d of %d templates", m.cursor+1, len(m.filteredTemplates))))
	}

	return b.String()
}

// renderPreviewPane renders the template preview with details.
func (m *TaskTemplatesModel) renderPreviewPane(width, height int) string {
	if m.cursor >= len(m.filteredTemplates) {
		return ""
	}

	t := m.filteredTemplates[m.cursor]
	isDefault := m.isDefaultTemplate(t.ID)

	var lines []string

	// Template name header
	header := BoldStyle.Render("Preview")
	if isDefault {
		header += " " + MutedStyle.Render("[Default]")
	}
	lines = append(lines, header, "")

	// Template name
	lines = append(lines, BoldStyle.Foreground(ColorPrimary).Render(t.Name))

	// Status
	if t.Enabled {
		lines = append(lines, DoneStyle.Render("● Enabled"))
	} else {
		lines = append(lines, MutedStyle.Render("○ Disabled"))
	}
	lines = append(lines, "")

	// Category
	if t.Category != "" {
		lines = append(lines, MutedStyle.Render("Category: ")+t.Category)
	}

	// Type
	lines = append(lines, MutedStyle.Render("Type: ")+string(t.Type))

	// Priority
	if t.Priority > 0 {
		lines = append(lines, MutedStyle.Render("Priority: ")+fmt.Sprintf("%d", t.Priority))
	}

	// Section
	if t.Section != "" {
		lines = append(lines, MutedStyle.Render("Section: ")+t.Section)
	}

	// Tags
	if len(t.Tags) > 0 {
		lines = append(lines, MutedStyle.Render("Tags: ")+strings.Join(t.Tags, ", "))
	}

	// Variables
	if varNames := template.VariableNames(&t); len(varNames) > 0 {
		lines = append(lines, MutedStyle.Render("Variables: ")+strings.Join(varNames, ", "))
	}

	// Description
	if t.Description != "" {
		lines = append(lines, "", MutedStyle.Render("Description:"))
		// Word-wrap description to fit pane width
		wrapped := wrapText(t.Description, width-2)
		for _, line := range wrapped {
			lines = append(lines, "  "+line)
		}
	}

	// Checklist
	if len(t.Checklist) > 0 {
		lines = append(lines, "", MutedStyle.Render("Checklist:"))
		for _, item := range t.Checklist {
			lines = append(lines, "  ☐ "+item)
		}
	}

	// Subtasks
	if len(t.Children) > 0 {
		lines = append(lines, "", MutedStyle.Render("Subtasks:"))
		for _, child := range t.Children {
			marker := "  ○ "
			if child.Optional {
				marker = "  ◌ "
			}
			// Show type badge
			childType := child.GetChildType()
			line := marker + MutedStyle.Render(fmt.Sprintf("[%s] ", childType)) + child.Title
			if child.Optional {
				line += MutedStyle.Render(" (optional)")
			}
			lines = append(lines, line)
		}
	}

	// Use count if any
	if t.UseCount > 0 {
		lines = append(lines, "", MutedStyle.Render(fmt.Sprintf("Used %d times", t.UseCount)))
	}

	// Apply scroll offset
	if m.previewScroll > 0 && m.previewScroll < len(lines) {
		lines = lines[m.previewScroll:]
	} else if m.previewScroll >= len(lines) && len(lines) > 0 {
		// Clamp scroll to valid range
		m.previewScroll = len(lines) - 1
		lines = lines[m.previewScroll:]
	}

	// Limit to available height
	if len(lines) > height {
		lines = lines[:height]
	}

	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

// renderFullscreenPreview renders the preview as a fullscreen view for narrow screens.
func (m *TaskTemplatesModel) renderFullscreenPreview() string {
	t := m.filteredTemplates[m.cursor]
	isDefault := m.isDefaultTemplate(t.ID)

	var b strings.Builder

	// Title (always shown, not scrolled)
	b.WriteString(renderFullscreenTitle("Template Preview", ""))
	b.WriteString("\n\n")

	// Build content lines for scrolling
	var lines []string

	// Template name with default indicator
	nameLine := BoldStyle.Foreground(ColorPrimary).Render(t.Name)
	if isDefault {
		nameLine += " " + MutedStyle.Render("[Default]")
	}
	lines = append(lines, nameLine, "")

	// Status
	if t.Enabled {
		lines = append(lines, DoneStyle.Render("● Enabled"))
	} else {
		lines = append(lines, MutedStyle.Render("○ Disabled"))
	}
	lines = append(lines, "")

	// Category
	if t.Category != "" {
		lines = append(lines, MutedStyle.Render("Category: ")+t.Category)
	}

	// Type
	lines = append(lines, MutedStyle.Render("Type: ")+string(t.Type))

	// Priority
	if t.Priority > 0 {
		lines = append(lines, MutedStyle.Render("Priority: ")+fmt.Sprintf("%d", t.Priority))
	}

	// Section
	if t.Section != "" {
		lines = append(lines, MutedStyle.Render("Section: ")+t.Section)
	}

	// Tags
	if len(t.Tags) > 0 {
		lines = append(lines, MutedStyle.Render("Tags: ")+strings.Join(t.Tags, ", "))
	}

	// Variables
	if varNames := template.VariableNames(&t); len(varNames) > 0 {
		lines = append(lines, MutedStyle.Render("Variables: ")+strings.Join(varNames, ", "))
	}

	// Description
	if t.Description != "" {
		lines = append(lines, "", MutedStyle.Render("Description:"))
		// Word-wrap description
		wrapped := wrapText(t.Description, m.Width()-4)
		for _, line := range wrapped {
			lines = append(lines, "  "+line)
		}
	}

	// Checklist
	if len(t.Checklist) > 0 {
		lines = append(lines, "", MutedStyle.Render("Checklist:"))
		for _, item := range t.Checklist {
			lines = append(lines, "  ☐ "+item)
		}
	}

	// Subtasks
	if len(t.Children) > 0 {
		lines = append(lines, "", MutedStyle.Render("Subtasks:"))
		for _, child := range t.Children {
			marker := "  ○ "
			if child.Optional {
				marker = "  ◌ "
			}
			// Show type badge
			childType := child.GetChildType()
			line := marker + MutedStyle.Render(fmt.Sprintf("[%s] ", childType)) + child.Title
			if child.Optional {
				line += MutedStyle.Render(" (optional)")
			}
			lines = append(lines, line)
		}
	}

	// Use count if any
	if t.UseCount > 0 {
		lines = append(lines, "", MutedStyle.Render(fmt.Sprintf("Used %d times", t.UseCount)))
	}

	// Apply scroll offset
	if m.previewScroll > 0 && m.previewScroll < len(lines) {
		lines = lines[m.previewScroll:]
	} else if m.previewScroll >= len(lines) && len(lines) > 0 {
		// Clamp scroll to valid range
		m.previewScroll = len(lines) - 1
		lines = lines[m.previewScroll:]
	}

	// Calculate available content height (minus header and footer)
	availableHeight := m.Height() - 6
	if availableHeight < 5 {
		availableHeight = 5
	}
	if len(lines) > availableHeight {
		lines = lines[:availableHeight]
	}

	b.WriteString(strings.Join(lines, "\n"))
	b.WriteString("\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("j/k: scroll · Ctrl+D/U: page · Enter/Esc: close · e: edit · Space: toggle · d: delete"), "", m.Width()))

	return b.String()
}

// renderHelpText renders the help text for the footer, wrapping if necessary.
func (m *TaskTemplatesModel) renderHelpText() string {
	var parts []string

	if len(m.filteredTemplates) == 0 {
		if m.searchQuery == "" {
			parts = []string{"/: search", "n: new template", "q: close"}
		} else {
			parts = []string{"Esc: clear search", "n: new template", "q: close"}
		}
	} else if m.isWideScreen() {
		parts = []string{"/: search", "j/k: nav", "Ctrl+D/U: scroll", "Space: toggle", "n: new", "e: edit", "d: delete", "q: close"}
	} else {
		parts = []string{"/: search", "j/k: nav", "Enter: preview", "Space: toggle", "n: new", "e: edit", "d: delete", "q: close"}
	}

	return renderWrappingHelpLine(strings.Join(parts, " · "), m.Width()-2)
}

// GetHelpItems returns help items for this view.
func (m *TaskTemplatesModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewTaskTemplates)
}

// HasTextInput returns true when search is active.
func (m *TaskTemplatesModel) HasTextInput() bool {
	return m.searchActive
}

// ShouldCaptureEsc returns true when expanded view, delete confirmation, or search is active.
func (m *TaskTemplatesModel) ShouldCaptureEsc() bool {
	return m.expandedView || m.DefaultShouldCaptureEsc() || m.SearchShouldCaptureEsc() || m.searchQuery != ""
}

// TaskTemplatesUpdatedMsg is sent when templates are modified.
type TaskTemplatesUpdatedMsg struct{}

// ShowTaskTemplatesMsg triggers showing the task templates view.
type ShowTaskTemplatesMsg struct{}

// ShowTaskTemplateCreatorMsg triggers showing the template creator.
type ShowTaskTemplateCreatorMsg struct{}

// ShowTaskTemplateEditorMsg triggers showing the template editor.
type ShowTaskTemplateEditorMsg struct {
	Template template.TaskTemplate
}

// TaskTemplateCreatedMsg signals a new template was created.
type TaskTemplateCreatedMsg struct {
	Template template.TaskTemplate
}

// TaskTemplateEditedMsg signals an existing template was edited.
type TaskTemplateEditedMsg struct {
	Template template.TaskTemplate
}

// TemplateDeletedMsg is sent when a template deletion completes.
type TemplateDeletedMsg struct {
	DeletedID string
	Error     error
}
