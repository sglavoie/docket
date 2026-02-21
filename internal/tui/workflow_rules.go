package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/workflow"
)

// WorkflowRulesModel handles the workflow rules management view.
type WorkflowRulesModel struct {
	BaseSearchableView
	config        *config.Config
	configManager *config.Manager

	InteractiveViewState
	allRules      []workflow.WorkflowRule
	filteredRules []workflow.WorkflowRule
	defaultCount  int // Number of default rules (at start of allRules slice)

	expandedView bool
}

// NewWorkflowRulesModel creates a new workflow rules model.
func NewWorkflowRulesModel(cfg *config.Config, manager *config.Manager) *WorkflowRulesModel {
	defaultRules, customRules := cfg.GetWorkflowRulesSorted()
	allRules := append(defaultRules, customRules...)
	m := &WorkflowRulesModel{
		config:        cfg,
		configManager: manager,
		allRules:      allRules,
		filteredRules: allRules,
		defaultCount:  len(defaultRules),
	}
	m.SetSize(80, 24)
	m.InitSearchInput("Search rules...", 50, 30)
	return m
}

// SetSize sets the view dimensions.
func (m *WorkflowRulesModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
}

func (m *WorkflowRulesModel) Init() tea.Cmd {
	return nil
}

func (m *WorkflowRulesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle expanded view mode first (narrow screen only)
		if m.expandedView && !m.isWideScreen() {
			switch msg.String() {
			case "esc", "enter", "q":
				m.expandedView = false
				return m, nil
			case " ":
				// Toggle enabled/disabled from preview
				if len(m.filteredRules) > 0 {
					return m, m.toggleCurrentRule()
				}
				return m, nil
			case "e":
				// Edit from preview
				if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
					m.expandedView = false
					rule := m.filteredRules[m.cursor]
					return m, func() tea.Msg {
						return ShowWorkflowRuleEditorMsg{Rule: rule}
					}
				}
				return m, nil
			case "d", "D":
				// Delete from preview
				if len(m.filteredRules) > 0 {
					m.confirm.Show("delete", fmt.Sprintf("Delete rule '%s'? (y/n)", m.filteredRules[m.cursor].Name))
				}
				return m, nil
			}
			return m, nil
		}

		if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
			if confirmed {
				cmd := m.deleteCurrentRule()
				m.expandedView = false // Close preview after delete
				return m, cmd
			}
			return m, nil
		}

		// Handle search mode
		if model, cmd, handled := m.HandleSearchMode(msg, m, m.filterRules); handled {
			return model, cmd
		}

		if m.pendingPrefix == "g" {
			m.pendingPrefix = ""
			if msg.String() == "g" {
				m.cursor = 0
			}
			return m, nil
		}

		switch msg.String() {
		case "esc":
			// If there's an active search filter, clear it instead of closing
			if m.searchQuery != "" {
				m.ClearSearch()
				m.filterRules()
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
			if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules)-1 {
				m.cursor++
			}
			return m, nil

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "g":
			m.pendingPrefix = "g"
			return m, nil

		case "G":
			if len(m.filteredRules) > 0 {
				m.cursor = len(m.filteredRules) - 1
			}
			return m, nil

		case " ":
			// Toggle enabled/disabled
			if len(m.filteredRules) > 0 {
				return m, m.toggleCurrentRule()
			}
			return m, nil

		case "enter":
			// Show expanded view with full rule details (narrow screens only)
			if !m.isWideScreen() && len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
				m.expandedView = true
			}
			return m, nil

		case "d", "D":
			// Delete rule (with confirmation)
			if len(m.filteredRules) > 0 {
				m.confirm.Show("delete", fmt.Sprintf("Delete rule '%s'? (y/n)", m.filteredRules[m.cursor].Name))
			}
			return m, nil

		case "n":
			// Create new rule
			return m, func() tea.Msg {
				return ShowWorkflowRuleCreatorMsg{}
			}

		case "e":
			// Edit selected rule
			if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
				rule := m.filteredRules[m.cursor]
				return m, func() tea.Msg {
					return ShowWorkflowRuleEditorMsg{Rule: rule}
				}
			}
			return m, nil
		}
	}

	return m, nil
}

// isDefaultRule returns true if the rule with the given ID is a default rule.
func (m *WorkflowRulesModel) isDefaultRule(id string) bool {
	for i := 0; i < m.defaultCount && i < len(m.allRules); i++ {
		if m.allRules[i].ID == id {
			return true
		}
	}
	return false
}

// filterRules filters allRules based on the current search query.
func (m *WorkflowRulesModel) filterRules() {
	query := strings.ToLower(m.searchQuery)
	if query == "" {
		m.filteredRules = m.allRules
		if m.cursor >= len(m.filteredRules) && len(m.filteredRules) > 0 {
			m.cursor = len(m.filteredRules) - 1
		}
		return
	}

	m.filteredRules = nil
	for _, rule := range m.allRules {
		nameLower := strings.ToLower(rule.Name)
		summaryLower := strings.ToLower(rule.Summary())
		if strings.Contains(nameLower, query) || strings.Contains(summaryLower, query) {
			m.filteredRules = append(m.filteredRules, rule)
		}
	}

	// Clamp cursor to filtered results
	if m.cursor >= len(m.filteredRules) {
		if len(m.filteredRules) > 0 {
			m.cursor = len(m.filteredRules) - 1
		} else {
			m.cursor = 0
		}
	}
}

func (m *WorkflowRulesModel) toggleCurrentRule() tea.Cmd {
	return func() tea.Msg {
		if m.cursor >= len(m.filteredRules) {
			return nil
		}

		rule := &m.filteredRules[m.cursor]
		enabled, found := m.config.ToggleWorkflowRule(rule.ID)
		if !found {
			return ErrorMsg{Err: fmt.Errorf("rule not found")}
		}

		// Update local copy
		rule.Enabled = enabled

		// Also update in allRules
		for i := range m.allRules {
			if m.allRules[i].ID == rule.ID {
				m.allRules[i].Enabled = enabled
				break
			}
		}

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return WorkflowRulesUpdatedMsg{}
	}
}

func (m *WorkflowRulesModel) deleteCurrentRule() tea.Cmd {
	return func() tea.Msg {
		if m.cursor >= len(m.filteredRules) {
			return nil
		}

		rule := m.filteredRules[m.cursor]
		if !m.config.DeleteWorkflowRule(rule.ID) {
			return ErrorMsg{Err: fmt.Errorf("rule not found")}
		}

		// Update local copy with sorted rules
		defaultRules, customRules := m.config.GetWorkflowRulesSorted()
		m.allRules = append(defaultRules, customRules...)
		m.defaultCount = len(defaultRules)

		// Re-filter to update filteredRules
		m.filterRules()

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return WorkflowRulesUpdatedMsg{}
	}
}

// isWideScreen returns true if the screen is wide enough for split layout (≥81 chars)
func (m *WorkflowRulesModel) isWideScreen() bool {
	return m.Width() >= 81
}

func (m *WorkflowRulesModel) View() string {
	var b strings.Builder

	// Fullscreen preview mode on narrow screens
	if m.expandedView && !m.isWideScreen() && len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
		return m.renderFullscreenPreview()
	}

	b.WriteString(renderFullscreenTitle("Workflow Rules", fmt.Sprintf("%d rules", len(m.filteredRules))))
	b.WriteString("\n\n")

	// Show search input when active
	if m.searchActive || m.searchQuery != "" {
		b.WriteString("/")
		b.WriteString(m.SearchInputView())
		if m.searchQuery != "" && !m.searchActive {
			b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d matches)", len(m.filteredRules))))
		}
		b.WriteString("\n\n")
	}

	// Build list content
	listContent := m.renderRulesList()

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
		if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
			previewPane = m.renderPreviewPane(previewWidth)
		} else {
			previewPane = lipgloss.NewStyle().
				Width(previewWidth).
				Foreground(ColorMuted).
				Render("No rule selected")
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

	// Confirmation dialog
	if m.confirm.IsActive() {
		b.WriteString("\n\n")
		b.WriteString(m.confirm.Render())
	}

	// Help footer
	helpText := m.renderHelpText()
	b.WriteString("\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render(helpText), "", m.Width()))

	return b.String()
}

// renderRulesList renders the list of rules.
func (m *WorkflowRulesModel) renderRulesList() string {
	var b strings.Builder

	if len(m.filteredRules) == 0 {
		if m.searchQuery != "" {
			b.WriteString(MutedStyle.Render("No rules match your search.\n\n"))
			b.WriteString(MutedStyle.Render("Press Esc to clear search."))
		} else {
			b.WriteString(MutedStyle.Render("No workflow rules configured.\n\n"))
			b.WriteString(MutedStyle.Render("Use 'tt workflow add' to create rules."))
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
	if endIdx > len(m.filteredRules) {
		endIdx = len(m.filteredRules)
	}

	// Show scroll indicator for rules above
	if startIdx > 0 {
		b.WriteString(ScrollUpIndicator(startIdx))
		b.WriteString("\n")
	}

	// Calculate max name length based on available width
	// On wide screen: use list pane width (45% of total)
	// On narrow screen: use full width minus some padding
	var maxNameLen int
	if m.isWideScreen() {
		listWidth := m.Width() * 45 / 100
		// Account for: cursor(2) + status(2) + space(1) + padding(2) = 7
		maxNameLen = listWidth - 7
	} else {
		// Account for: cursor(2) + status(2) + space(1) + padding(4) = 9
		maxNameLen = m.Width() - 9
	}
	if maxNameLen < 15 {
		maxNameLen = 15
	}

	for i := startIdx; i < endIdx; i++ {
		rule := m.filteredRules[i]
		isDefault := m.isDefaultRule(rule.ID)

		// Status indicator
		var status string
		if rule.Enabled {
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

		// Rule name - use full available width
		name := rule.Name
		if len(name) > effectiveMaxLen {
			name = name[:effectiveMaxLen-3] + "..."
		}

		if i == m.cursor {
			line := SelectedItemTextStyle.Render(fmt.Sprintf("▸ %s %s%s", status, defaultMarker, name))
			b.WriteString(line)
			b.WriteString("\n")
			// Show summary for selected rule on narrow screens only
			if !m.isWideScreen() {
				summary := rule.Summary()
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
	if len(m.filteredRules) > maxVisible {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  %d of %d rules", m.cursor+1, len(m.filteredRules))))
	}

	return b.String()
}

// renderPreviewPane renders the rule preview with syntax highlighting.
func (m *WorkflowRulesModel) renderPreviewPane(width int) string {
	if m.cursor >= len(m.filteredRules) {
		return ""
	}

	rule := m.filteredRules[m.cursor]
	isDefault := m.isDefaultRule(rule.ID)

	var b strings.Builder

	// Styles for syntax highlighting
	keywordStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))  // Cyan for IF/AND/OR/THEN
	operatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("13")) // Magenta for =, !=, <, >, :

	// Rule name header
	b.WriteString(BoldStyle.Render("Preview"))
	if isDefault {
		b.WriteString(" ")
		b.WriteString(MutedStyle.Render("[Default]"))
	}
	b.WriteString("\n\n")

	// Rule name
	b.WriteString(BoldStyle.Foreground(ColorPrimary).Render(rule.Name))
	b.WriteString("\n")

	// Status
	if rule.Enabled {
		b.WriteString(DoneStyle.Render("● Enabled"))
	} else {
		b.WriteString(MutedStyle.Render("○ Disabled"))
	}
	b.WriteString("\n\n")

	// Render triggers with syntax highlighting
	triggers := rule.GetTriggers()
	for i, t := range triggers {
		var prefix string
		if i == 0 {
			prefix = keywordStyle.Render("IF") + " "
		} else {
			logic := rule.GetTriggerLogic()
			if logic == workflow.LogicAnd {
				prefix = keywordStyle.Render("AND") + " "
			} else {
				prefix = keywordStyle.Render("OR") + " "
			}
		}

		field := string(t.Field)
		op := m.getOpSymbol(t.Operation)
		value := t.Value
		if value == "" {
			value = "?"
		}

		b.WriteString(fmt.Sprintf("%s%s %s %s\n", prefix, field, operatorStyle.Render(op), value))
	}

	// Render actions with syntax highlighting
	for i, a := range rule.Actions {
		var prefix string
		if i == 0 {
			prefix = keywordStyle.Render("THEN") + " "
		} else {
			prefix = keywordStyle.Render("AND") + " "
		}

		actionType := string(a.ActionType)
		value := a.Value

		if value == "" {
			b.WriteString(fmt.Sprintf("%s%s\n", prefix, actionType))
		} else {
			b.WriteString(fmt.Sprintf("%s%s%s %s\n", prefix, actionType, operatorStyle.Render(":"), value))
		}
	}

	// Trigger count if any
	if rule.TriggerCount > 0 {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Triggered %d times", rule.TriggerCount)))
	}

	return lipgloss.NewStyle().Width(width).Render(b.String())
}

// renderFullscreenPreview renders the preview as a fullscreen view for narrow screens.
func (m *WorkflowRulesModel) renderFullscreenPreview() string {
	rule := m.filteredRules[m.cursor]
	isDefault := m.isDefaultRule(rule.ID)

	var b strings.Builder

	// Title
	b.WriteString(renderFullscreenTitle("Rule Preview", ""))
	b.WriteString("\n\n")

	// Styles for syntax highlighting
	keywordStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))  // Cyan for IF/AND/OR/THEN
	operatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("13")) // Magenta for =, !=, <, >, :

	// Rule name with default indicator
	b.WriteString(BoldStyle.Foreground(ColorPrimary).Render(rule.Name))
	if isDefault {
		b.WriteString(" ")
		b.WriteString(MutedStyle.Render("[Default]"))
	}
	b.WriteString("\n\n")

	// Status
	if rule.Enabled {
		b.WriteString(DoneStyle.Render("● Enabled"))
	} else {
		b.WriteString(MutedStyle.Render("○ Disabled"))
	}
	b.WriteString("\n\n")

	// Render triggers with syntax highlighting
	triggers := rule.GetTriggers()
	for i, t := range triggers {
		var prefix string
		if i == 0 {
			prefix = keywordStyle.Render("IF") + " "
		} else {
			logic := rule.GetTriggerLogic()
			if logic == workflow.LogicAnd {
				prefix = keywordStyle.Render("AND") + " "
			} else {
				prefix = keywordStyle.Render("OR") + " "
			}
		}

		field := string(t.Field)
		op := m.getOpSymbol(t.Operation)
		value := t.Value
		if value == "" {
			value = "?"
		}

		b.WriteString(fmt.Sprintf("%s%s %s %s\n", prefix, field, operatorStyle.Render(op), value))
	}

	// Render actions with syntax highlighting
	for i, a := range rule.Actions {
		var prefix string
		if i == 0 {
			prefix = keywordStyle.Render("THEN") + " "
		} else {
			prefix = keywordStyle.Render("AND") + " "
		}

		actionType := string(a.ActionType)
		value := a.Value

		if value == "" {
			b.WriteString(fmt.Sprintf("%s%s\n", prefix, actionType))
		} else {
			b.WriteString(fmt.Sprintf("%s%s%s %s\n", prefix, actionType, operatorStyle.Render(":"), value))
		}
	}

	// Trigger count if any
	if rule.TriggerCount > 0 {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Triggered %d times", rule.TriggerCount)))
	}

	b.WriteString("\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Enter/Esc: close · e: edit · Space: toggle · d: delete"), "", m.Width()))

	return b.String()
}

// getOpSymbol returns a symbol for the operation.
func (m *WorkflowRulesModel) getOpSymbol(op workflow.TriggerOperation) string {
	switch op {
	case workflow.OpEquals:
		return "="
	case workflow.OpNotEquals:
		return "!="
	case workflow.OpContains:
		return "has"
	case workflow.OpGreaterThan:
		return ">"
	case workflow.OpLessThan:
		return "<"
	default:
		return string(op)
	}
}

// renderHelpText renders the help text for the footer, wrapping if necessary.
func (m *WorkflowRulesModel) renderHelpText() string {
	var parts []string

	if len(m.filteredRules) == 0 {
		if m.searchQuery == "" {
			parts = []string{"/: search", "n: new rule", "q: close"}
		} else {
			parts = []string{"Esc: clear search", "n: new rule", "q: close"}
		}
	} else if m.isWideScreen() {
		parts = []string{"/: search", "j/k: nav", "Space: toggle", "n: new", "e: edit", "d: delete", "q: close"}
	} else {
		parts = []string{"/: search", "j/k: nav", "Enter: preview", "Space: toggle", "n: new", "e: edit", "d: delete", "q: close"}
	}

	return renderWrappingHelpLine(strings.Join(parts, " · "), m.Width()-2)
}

// GetHelpItems returns help items for this view.
func (m *WorkflowRulesModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewWorkflowRules)
}

// HasTextInput returns true when search is active.
func (m *WorkflowRulesModel) HasTextInput() bool {
	return m.searchActive
}

// ShouldCaptureEsc returns true when expanded view, delete confirmation, or search is active.
func (m *WorkflowRulesModel) ShouldCaptureEsc() bool {
	return m.expandedView || m.DefaultShouldCaptureEsc() || m.SearchShouldCaptureEsc() || m.searchQuery != ""
}

// WorkflowRulesUpdatedMsg is sent when rules are modified.
type WorkflowRulesUpdatedMsg struct{}

// ShowWorkflowRulesMsg triggers showing the workflow rules view.
type ShowWorkflowRulesMsg struct{}
