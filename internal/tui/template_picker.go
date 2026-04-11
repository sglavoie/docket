package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

const minWidthForSplitView = 100 // Below this, use single-pane + TAB toggle

// TemplatePickerModel handles template selection during task creation.
type TemplatePickerModel struct {
	BaseSearchableView
	InteractiveViewState
	config  *config.Config
	project *models.Project

	templates         []template.TaskTemplate
	filteredTemplates []template.TaskTemplate

	// Group templates by category for display
	categoryOrder []string // Order of categories to display

	// Split-view state
	previewScroll  int  // Scroll offset for preview content
	showingPreview bool // For narrow mode: true = showing preview, false = showing list
}

// NewTemplatePickerModel creates a new template picker model.
func NewTemplatePickerModel(cfg *config.Config, project *models.Project) *TemplatePickerModel {
	templates := cfg.GetEnabledTaskTemplates()

	// Build category order (bug, feature, process, other)
	categoryOrder := []string{}
	categorySet := make(map[string]bool)
	for _, t := range templates {
		cat := t.Category
		if cat == "" {
			cat = "other"
		}
		if !categorySet[cat] {
			categorySet[cat] = true
			categoryOrder = append(categoryOrder, cat)
		}
	}

	// Sort category order: bug, feature, process, then others alphabetically
	sortedOrder := []string{}
	preferredOrder := []string{"bug", "feature", "process"}
	for _, cat := range preferredOrder {
		if categorySet[cat] {
			sortedOrder = append(sortedOrder, cat)
		}
	}
	for _, cat := range categoryOrder {
		if cat != "bug" && cat != "feature" && cat != "process" {
			sortedOrder = append(sortedOrder, cat)
		}
	}

	m := &TemplatePickerModel{
		config:            cfg,
		project:           project,
		templates:         templates,
		filteredTemplates: templates,
		categoryOrder:     sortedOrder,
	}
	m.SetSize(60, 20)
	m.InitSearchInput("Search templates...", 40, 25)
	return m
}

// SetSize sets the view dimensions.
func (m *TemplatePickerModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
}

func (m *TemplatePickerModel) Init() tea.Cmd {
	return m.ActivateSearch()
}

func (m *TemplatePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		// Navigation keys that work regardless of search mode
		switch msg.String() {
		case "up", "ctrl+k", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				m.previewScroll = 0
			}
			return m, nil
		case "down", "ctrl+j", "ctrl+n":
			if m.cursor < len(m.filteredTemplates) {
				m.cursor++
				m.previewScroll = 0
			}
			return m, nil
		case "enter":
			return m, m.selectTemplate()
		case "esc":
			return m, m.skipTemplate()
		case "tab":
			// Toggle preview in narrow mode
			if m.Width()-4 < minWidthForSplitView {
				m.showingPreview = !m.showingPreview
				return m, nil
			}
		case "ctrl+d", "ctrl+f":
			m.previewScroll += 5
			return m, nil
		case "ctrl+u", "ctrl+b":
			m.previewScroll = max(0, m.previewScroll-5)
			return m, nil
		}

		// j/k only navigate when NOT in search mode (allow typing j/k in search)
		if !m.searchActive {
			if m.SkipModifierInPrefix(msg) {
				return m, nil
			}
			if m.pendingPrefix == "g" {
				m.pendingPrefix = ""
				_, cmd := handleGotoTopPrefix(msg.String(), &m.cursor, &m.cursorFlash, func() { m.previewScroll = 0 })
				return m, cmd
			}

			switch msg.String() {
			case "j":
				if m.cursor < len(m.filteredTemplates) {
					m.cursor++
					m.previewScroll = 0
				}
				return m, nil
			case "k":
				if m.cursor > 0 {
					m.cursor--
					m.previewScroll = 0
				}
				return m, nil
			case "g":
				m.pendingPrefix = "g"
				return m, nil
			case "G":
				m.cursor = len(m.filteredTemplates)
				m.previewScroll = 0
				return m, nil
			case "/":
				return m, m.ActivateSearch()
			}
		}

		// Let search input handle remaining keys
		if m.searchActive {
			_, cmd, valueChanged := m.HandleSearchInput(msg)
			if valueChanged {
				m.filterTemplates()
				m.previewScroll = 0 // Reset scroll when filter changes
			}
			return m, cmd
		}
	}

	return m, nil
}

// filterTemplates filters templates based on the current search query.
func (m *TemplatePickerModel) filterTemplates() {
	query := strings.ToLower(m.searchQuery)
	if query == "" {
		m.filteredTemplates = m.templates
		if m.cursor > len(m.filteredTemplates) {
			m.cursor = len(m.filteredTemplates)
		}
		return
	}

	m.filteredTemplates = nil
	for _, t := range m.templates {
		nameLower := strings.ToLower(t.Name)
		categoryLower := strings.ToLower(t.Category)
		if strings.Contains(nameLower, query) || strings.Contains(categoryLower, query) {
			m.filteredTemplates = append(m.filteredTemplates, t)
		}
	}

	// Clamp cursor
	if m.cursor > len(m.filteredTemplates) {
		m.cursor = len(m.filteredTemplates)
	}
}

func (m *TemplatePickerModel) selectTemplate() tea.Cmd {
	return func() tea.Msg {
		// cursor == len(filteredTemplates) means "Skip" is selected
		if m.cursor >= len(m.filteredTemplates) {
			return TemplatePickerDoneMsg{Canceled: false, Template: nil}
		}
		t := m.filteredTemplates[m.cursor]
		return TemplatePickerDoneMsg{Canceled: false, Template: &t}
	}
}

func (m *TemplatePickerModel) skipTemplate() tea.Cmd {
	return func() tea.Msg {
		return TemplatePickerDoneMsg{Canceled: true, Template: nil}
	}
}

// renderListPanel renders the template list for the left pane.
func (m *TemplatePickerModel) renderListPanel(width int) string {
	var b strings.Builder

	// Templates list (grouped by category)
	if len(m.filteredTemplates) == 0 && len(m.templates) == 0 {
		b.WriteString(MutedStyle.Render("No templates available."))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("Use 'tt template init'"))
	} else if len(m.filteredTemplates) == 0 {
		b.WriteString(MutedStyle.Render("No templates match"))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("your search."))
	} else {
		// Group templates by category
		byCategory := make(map[string][]template.TaskTemplate)
		for _, t := range m.filteredTemplates {
			cat := t.Category
			if cat == "" {
				cat = "other"
			}
			byCategory[cat] = append(byCategory[cat], t)
		}

		// Calculate list index for cursor
		listIdx := 0
		for _, cat := range m.categoryOrder {
			templates, ok := byCategory[cat]
			if !ok || len(templates) == 0 {
				continue
			}

			// Category header
			b.WriteString(MutedStyle.Render(strings.ToUpper(cat)))
			b.WriteString("\n")

			for _, t := range templates {
				// Truncate name if too long for width (account for prefix "▸ " = 2 chars)
				maxNameLen := max(width-3, 1)
				name := truncateToVisualWidth(t.Name, maxNameLen)

				if listIdx == m.cursor {
					b.WriteString(SelectedItemTextStyle.Render(fmt.Sprintf("▸ %s", name)))
				} else {
					fmt.Fprintf(&b, "  %s", name)
				}
				b.WriteString("\n")
				listIdx++
			}
			b.WriteString("\n")
		}
	}

	// Skip option (always visible)
	b.WriteString(MutedStyle.Render("───────────"))
	b.WriteString("\n")
	if m.cursor == len(m.filteredTemplates) {
		b.WriteString(SelectedItemTextStyle.Render("▸ Skip"))
	} else {
		b.WriteString(MutedStyle.Render("  Skip"))
	}

	return b.String()
}

// renderPreviewPanel renders the template preview for the right pane.
func (m *TemplatePickerModel) renderPreviewPanel(t *template.TaskTemplate, width, height int) string {
	if t == nil {
		// No template selected (Skip is highlighted)
		msg := "No template selected"
		return MutedStyle.Render(msg)
	}

	var lines []string

	// Template name as header
	lines = append(lines, BoldStyle.Render(t.Name))
	lines = append(lines, "")

	// Type badge
	typeBadge := fmt.Sprintf("[%s]", t.Type)
	lines = append(lines, typeBadge)

	// Priority
	if t.Priority > 0 {
		lines = append(lines, fmt.Sprintf("Priority: P%d", t.Priority))
	}

	// Tags
	if len(t.Tags) > 0 {
		tagStr := strings.Join(t.Tags, ", ")
		tagStr = truncateToVisualWidth(tagStr, max(width-8, 1))
		lines = append(lines, fmt.Sprintf("Tags: %s", tagStr))
	}

	// Section
	if t.Section != "" {
		lines = append(lines, fmt.Sprintf("Section: %s", t.Section))
	}

	// Description
	if t.Description != "" {
		lines = append(lines, "")
		lines = append(lines, MutedStyle.Render("Description:"))
		// Word-wrap description (width-4 accounts for indent and margins)
		wrapWidth := max(width-4, 20)
		wrapped := wrapTemplateText(t.Description, wrapWidth)
		for _, line := range wrapped {
			lines = append(lines, "  "+line)
		}
	}

	// Checklist items
	if len(t.Checklist) > 0 {
		lines = append(lines, "")
		lines = append(lines, MutedStyle.Render("Checklist:"))
		for i, item := range t.Checklist {
			itemText := item
			maxLen := width - 6
			if maxLen > 0 && len(itemText) > maxLen {
				itemText = itemText[:maxLen-1] + "…"
			}
			lines = append(lines, fmt.Sprintf("  %d. %s", i+1, itemText))
		}
	}

	// Subtasks (Children)
	if len(t.Children) > 0 {
		lines = append(lines, "")
		lines = append(lines, MutedStyle.Render("Subtasks:"))
		for i, child := range t.Children {
			itemText := child.Title
			maxLen := width - 14
			if maxLen > 0 && len(itemText) > maxLen {
				itemText = itemText[:maxLen-1] + "…"
			}
			childType := child.GetChildType()
			optional := ""
			if child.Optional {
				optional = " (optional)"
			}
			lines = append(lines, fmt.Sprintf("  %d. %s [%s]%s", i+1, itemText, childType, optional))
		}
	}

	// Apply scroll offset
	if m.previewScroll > 0 && m.previewScroll < len(lines) {
		lines = lines[m.previewScroll:]
	} else if m.previewScroll >= len(lines) {
		// Clamp scroll to valid range
		m.previewScroll = max(0, len(lines)-1)
		if m.previewScroll > 0 {
			lines = lines[m.previewScroll:]
		}
	}

	// Limit to height
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// wrapTemplateText wraps text to the specified width for template preview.
func wrapTemplateText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return lines
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

func (m *TemplatePickerModel) View() tea.View {
	totalWidth := m.Width() - 4

	// Get selected template for preview
	var selectedTemplate *template.TaskTemplate
	if m.cursor < len(m.filteredTemplates) {
		t := m.filteredTemplates[m.cursor]
		selectedTemplate = &t
	}

	var content string
	var helpText string

	// Calculate content height (total height minus header, search, help footer)
	// Header: 2 lines, Search: 2 lines, Help: 2 lines = 6 lines overhead
	contentHeight := max(m.Height()-10, 5)

	if totalWidth >= minWidthForSplitView {
		// Wide mode: split view
		listWidth := (totalWidth * 40) / 100
		previewWidth := totalWidth - listWidth - 1 // -1 for separator

		leftPanel := m.renderListPanel(listWidth)
		rightPanel := m.renderPreviewPanel(selectedTemplate, previewWidth, contentHeight)
		content = renderOverlaySplitPane(leftPanel, rightPanel, listWidth, previewWidth, contentHeight)
		helpText = "↑/↓: select · Enter: use · Esc: close · Ctrl+U/D: scroll"
	} else {
		// Narrow mode: single pane with TAB toggle
		if m.showingPreview {
			content = m.renderPreviewPanel(selectedTemplate, totalWidth, contentHeight)
			helpText = "Tab: back to list · Ctrl+U/D: scroll · Esc: close"
		} else {
			content = m.renderListPanel(totalWidth)
			helpText = "↑/↓: select · Enter: use · Tab: preview · Esc: close"
		}
	}

	// Build final view
	var b strings.Builder

	b.WriteString(BoldStyle.Render("Select a Template"))
	b.WriteString("\n\n")

	// Search input
	b.WriteString("/")
	b.WriteString(m.SearchInputView())
	b.WriteString("\n\n")

	// Main content
	b.WriteString(content)
	b.WriteString("\n\n")

	// Help footer
	b.WriteString(HelpStyle.Render(helpText))

	// Wrap in a box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSubtleBorder).
		Padding(1, 2).
		Width(m.Width() - 4)

	return tea.NewView(boxStyle.Render(b.String()))
}

// HasTextInput returns true when search is active.
func (m *TemplatePickerModel) HasTextInput() bool {
	return m.DefaultHasTextInput()
}

// ShouldCaptureEsc returns true to handle escape internally.
func (m *TemplatePickerModel) ShouldCaptureEsc() bool {
	return true
}

// TemplatePickerDoneMsg is sent when a template is selected or skipped.
type TemplatePickerDoneMsg struct {
	Canceled bool
	Template *template.TaskTemplate
}

// ShowTemplatePickerMsg triggers showing the template picker.
type ShowTemplatePickerMsg struct {
	Project *models.Project
}

// ShowTaskCreatorWithTemplateMsg triggers showing the task creator with a template.
type ShowTaskCreatorWithTemplateMsg struct {
	Project  *models.Project
	Template *template.TaskTemplate
}

func (m *TemplatePickerModel) StatusBarLabel() string { return "Choose Template" }
