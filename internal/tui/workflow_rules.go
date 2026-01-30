package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/workflow"
)

// WorkflowRulesModel handles the workflow rules management view.
type WorkflowRulesModel struct {
	BaseSearchableView
	config        *config.Config
	configManager *config.Manager

	allRules      []workflow.WorkflowRule
	filteredRules []workflow.WorkflowRule
	defaultCount  int // Number of default rules (at start of allRules slice)
	cursor        int

	confirm      ConfirmDialog
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
		cursor:        0,
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
		// Handle expanded view mode first
		if m.expandedView {
			switch msg.String() {
			case "esc", "enter", "q":
				m.expandedView = false
				return m, nil
			}
			return m, nil
		}

		if confirmed, handled := m.confirm.HandleKey(msg.String()); handled {
			if confirmed {
				return m, m.deleteCurrentRule()
			}
			return m, nil
		}

		// Handle search mode
		if m.searchActive {
			handled, cmd, valueChanged := m.HandleSearchInput(msg)
			if valueChanged {
				m.filterRules()
			}
			if handled {
				return m, cmd
			}
			return m, cmd
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
			m.cursor = 0
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
			// Show expanded view with full rule details
			if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
				m.expandedView = true
			}
			return m, nil

		case "d", "D":
			// Delete rule (with confirmation)
			if len(m.filteredRules) > 0 {
				m.confirm.Show("delete", fmt.Sprintf(" Delete rule '%s'? (y/n) ", m.filteredRules[m.cursor].Name))
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

func (m *WorkflowRulesModel) View() string {
	var b strings.Builder

	// Expanded view mode - show full rule details
	if m.expandedView && len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
		rule := m.filteredRules[m.cursor]
		isDefault := m.isDefaultRule(rule.ID)

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

		// Full summary split into triggers and actions
		b.WriteString(FieldLabelFocusedStyle.Render("Rule:"))
		b.WriteString("\n")
		summary := rule.Summary()
		maxLen := m.Width() - 12
		if maxLen < 40 {
			maxLen = 40
		}

		// Split triggers and actions at ": "
		if colonIdx := strings.Index(summary, ": "); colonIdx != -1 {
			triggers := summary[:colonIdx+1] // Include the colon
			actions := summary[colonIdx+2:]  // Skip ": "

			// Write triggers line with word wrap
			for len(triggers) > maxLen {
				breakAt := maxLen
				for i := maxLen; i > 0; i-- {
					if triggers[i] == ' ' {
						breakAt = i
						break
					}
				}
				b.WriteString(triggers[:breakAt])
				b.WriteString("\n")
				triggers = triggers[breakAt:]
				if len(triggers) > 0 && triggers[0] == ' ' {
					triggers = triggers[1:]
				}
			}
			b.WriteString(triggers)
			b.WriteString("\n")

			// Write actions on new line with indent and word wrap
			indent := "    "
			actionMaxLen := maxLen - len(indent)
			if actionMaxLen < 20 {
				actionMaxLen = 20
			}
			for len(actions) > actionMaxLen {
				breakAt := actionMaxLen
				for i := actionMaxLen; i > 0; i-- {
					if actions[i] == ' ' {
						breakAt = i
						break
					}
				}
				b.WriteString(indent)
				b.WriteString(actions[:breakAt])
				b.WriteString("\n")
				actions = actions[breakAt:]
				if len(actions) > 0 && actions[0] == ' ' {
					actions = actions[1:]
				}
			}
			b.WriteString(indent)
			b.WriteString(actions)
		} else {
			// No colon found, just write as-is with word wrap
			for len(summary) > maxLen {
				breakAt := maxLen
				for i := maxLen; i > 0; i-- {
					if summary[i] == ' ' {
						breakAt = i
						break
					}
				}
				b.WriteString(summary[:breakAt])
				b.WriteString("\n")
				summary = summary[breakAt:]
				if len(summary) > 0 && summary[0] == ' ' {
					summary = summary[1:]
				}
			}
			b.WriteString(summary)
		}
		b.WriteString("\n\n")

		b.WriteString(MutedStyle.Render("Enter/Esc: close"))

		cardWidth := 70
		if m.Width()-4 < cardWidth {
			cardWidth = m.Width() - 4
		}
		config := OverlayCardConfig{
			Title:        "Rule Details",
			Width:        cardWidth,
			Centered:     true,
			NoTruncation: true,
		}
		return renderOverlayCard(b.String(), m.Width(), m.Height(), config)
	}

	// Show search input when active
	if m.searchActive || m.searchQuery != "" {
		b.WriteString("/")
		b.WriteString(m.SearchInputView())
		if m.searchQuery != "" && !m.searchActive {
			b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d matches)", len(m.filteredRules))))
		}
		b.WriteString("\n\n")
	}

	if len(m.filteredRules) == 0 {
		if m.searchQuery != "" {
			b.WriteString(MutedStyle.Render("No rules match your search.\n\n"))
			b.WriteString(MutedStyle.Render("Press Esc to clear search."))
		} else {
			b.WriteString(MutedStyle.Render("No workflow rules configured.\n\n"))
			b.WriteString(MutedStyle.Render("Use 'tt workflow add' to create rules."))
		}
	} else {
		// Calculate visible area
		// Reserve: borders (2), section headers (3), selected summary (1),
		// scroll indicators (3), help footer (3), card padding (2), search (2) = 16 lines
		maxVisible := m.Height() - 16
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
			b.WriteString(MutedStyle.Render(fmt.Sprintf("↑ %d more above", startIdx)))
			b.WriteString("\n")
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
			if isDefault {
				defaultMarker = MutedStyle.Render("[D]") + " "
			}

			// Rule name and summary
			name := rule.Name
			maxNameLen := 30
			if isDefault {
				maxNameLen = 26 // Account for [D] marker
			}
			if len(name) > maxNameLen {
				name = name[:maxNameLen-3] + "..."
			}

			summary := rule.Summary()
			if len(summary) > 50 {
				summary = summary[:47] + "..."
			}

			if i == m.cursor {
				line := BoldStyle.Foreground(ColorPrimary).Render(fmt.Sprintf("▸ %s %s%-*s", status, defaultMarker, maxNameLen, name))
				b.WriteString(line)
				b.WriteString("\n")
				// Show summary for selected rule
				b.WriteString(MutedStyle.Render(fmt.Sprintf("    %s", summary)))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s%-*s", status, defaultMarker, maxNameLen, name))
			}
			b.WriteString("\n")
		}

		// Show scroll indicator if needed
		if len(m.filteredRules) > maxVisible {
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  %d of %d rules", m.cursor+1, len(m.filteredRules))))
		}
	}

	// Confirmation dialog
	if m.confirm.IsActive() {
		b.WriteString("\n\n")
		b.WriteString(m.confirm.Render())
	}

	// Help footer
	b.WriteString("\n\n")
	if len(m.filteredRules) > 0 {
		b.WriteString(MutedStyle.Render("/: search · j/k: navigate · Enter: details · Space: toggle · n: new · e: edit · d: delete · q: close"))
	} else if m.searchQuery == "" {
		b.WriteString(MutedStyle.Render("/: search · n: new rule · q: close"))
	} else {
		b.WriteString(MutedStyle.Render("Esc: clear search · n: new rule · q: close"))
	}

	cardWidth := 70
	if m.Width()-4 < cardWidth {
		cardWidth = m.Width() - 4
	}
	config := OverlayCardConfig{
		Title:        "Workflow Rules",
		Width:        cardWidth,
		CenterH:      true,
		NoTruncation: true,
	}
	return renderOverlayCard(b.String(), m.Width(), m.Height(), config)
}

// HasTextInput returns true when search is active.
func (m *WorkflowRulesModel) HasTextInput() bool {
	return m.searchActive
}

// ShouldCaptureEsc returns true when expanded view, delete confirmation, or search is active.
func (m *WorkflowRulesModel) ShouldCaptureEsc() bool {
	return m.expandedView || m.confirm.IsActive() || m.searchActive || m.searchQuery != ""
}

// WorkflowRulesUpdatedMsg is sent when rules are modified.
type WorkflowRulesUpdatedMsg struct{}

// ShowWorkflowRulesMsg triggers showing the workflow rules view.
type ShowWorkflowRulesMsg struct{}
