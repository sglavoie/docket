package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *WorkflowRulesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		if m.expandedView && !m.isWideScreen() {
			return m, m.handleExpandedViewKey(msg.String())
		}

		if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
			if confirmed {
				cmd := m.deleteCurrentRule()
				m.expandedView = false
				return m, cmd
			}
			return m, nil
		}

		if model, cmd, handled := m.HandleSearchMode(msg, m, m.filterRules); handled {
			return model, cmd
		}

		if m.pendingPrefix == "g" {
			m.pendingPrefix = ""
			_, cmd := handleGotoTopPrefix(msg.String(), &m.cursor, &m.cursorFlash)
			return m, cmd
		}

		return m.handleDefaultKey(msg.String())
	}

	return m, nil
}

func (m *WorkflowRulesModel) handleExpandedViewKey(key string) tea.Cmd {
	switch key {
	case "esc", "enter":
		m.expandedView = false
		return nil
	case "space":
		if len(m.filteredRules) > 0 {
			return m.toggleCurrentRule()
		}
		return nil
	case "e":
		if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
			m.expandedView = false
			rule := m.filteredRules[m.cursor]
			return func() tea.Msg {
				return ShowWorkflowRuleEditorMsg{Rule: rule}
			}
		}
		return nil
	case "d", "D":
		if len(m.filteredRules) > 0 {
			m.confirm.Show("delete", fmt.Sprintf("Delete rule '%s'?", m.filteredRules[m.cursor].Name))
		}
		return nil
	default:
		return nil
	}
}

func (m *WorkflowRulesModel) handleDefaultKey(key string) (tea.Model, tea.Cmd) {
	if handled, model, cmd := m.handleNavigationKey(key); handled {
		return model, cmd
	}
	if handled, model, cmd := m.handleRuleActionKey(key); handled {
		return model, cmd
	}
	return m, nil
}

func (m *WorkflowRulesModel) handleNavigationKey(key string) (bool, tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		if m.searchQuery != "" {
			m.ClearSearch()
			m.filterRules()
			return true, m, nil
		}
		return true, m, func() tea.Msg { return BackMsg{} }
	case "/":
		return true, m, m.ActivateSearch()
	case "j", "down":
		if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules)-1 {
			m.cursor++
		}
		return true, m, nil
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return true, m, nil
	case "g":
		m.pendingPrefix = "g"
		return true, m, nil
	case "G":
		if len(m.filteredRules) > 0 {
			m.cursor = len(m.filteredRules) - 1
		}
		return true, m, nil
	default:
		return false, m, nil
	}
}

func (m *WorkflowRulesModel) handleRuleActionKey(key string) (bool, tea.Model, tea.Cmd) {
	switch key {
	case "space":
		if len(m.filteredRules) > 0 {
			return true, m, m.toggleCurrentRule()
		}
		return true, m, nil
	case "enter":
		if !m.isWideScreen() && len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
			m.expandedView = true
		}
		return true, m, nil
	case "d", "D":
		if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
			m.confirm.Show("delete", fmt.Sprintf("Delete rule '%s'?", m.filteredRules[m.cursor].Name))
		}
		return true, m, nil
	case "n":
		return true, m, func() tea.Msg { return ShowWorkflowRuleCreatorMsg{} }
	case "e":
		if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
			rule := m.filteredRules[m.cursor]
			return true, m, func() tea.Msg { return ShowWorkflowRuleEditorMsg{Rule: rule} }
		}
		return true, m, nil
	case "J":
		if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
			return true, m, m.adjustCurrentRulePriority(1)
		}
		return true, m, nil
	case "K":
		if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
			return true, m, m.adjustCurrentRulePriority(-1)
		}
		return true, m, nil
	case "t":
		return true, m, func() tea.Msg { return ShowWorkflowDryRunMsg{} }
	case "T":
		if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
			rule := m.filteredRules[m.cursor]
			return true, m, func() tea.Msg { return ShowWorkflowDryRunMsg{SingleRule: &rule} }
		}
		return true, m, nil
	default:
		return false, m, nil
	}
}

func (m *WorkflowRulesModel) HandledKeys() []string {
	return []string{
		// Navigation
		"j", "down", "k", "up", "g", "G", "/",
		// Actions
		"space", "enter", "d", "D", "n", "e", "J", "K", "t", "T",
	}
}

// filterRules filters allRules based on the current search query.
func (m *WorkflowRulesModel) filterRules() {
	query := strings.ToLower(m.searchQuery)
	if query == "" {
		m.filteredRules = m.allRules
		m.cursor = ClampCursor(m.cursor, len(m.filteredRules))
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

	m.cursor = ClampCursor(m.cursor, len(m.filteredRules))
}

func (m *WorkflowRulesModel) toggleCurrentRule() tea.Cmd {
	if m.cursor >= len(m.filteredRules) {
		return nil
	}

	rule := &m.filteredRules[m.cursor]
	enabled, found := m.config.ToggleWorkflowRule(rule.ID)
	if !found {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("rule not found")} }
	}

	rule.Enabled = enabled
	for i := range m.allRules {
		if m.allRules[i].ID == rule.ID {
			m.allRules[i].Enabled = enabled
			break
		}
	}

	mgr := m.configManager
	cfg := m.config
	return func() tea.Msg {
		if err := mgr.Save(cfg); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return WorkflowRulesUpdatedMsg{}
	}
}

func (m *WorkflowRulesModel) adjustCurrentRulePriority(delta int) tea.Cmd {
	if m.cursor >= len(m.filteredRules) {
		return nil
	}

	rule := &m.filteredRules[m.cursor]
	newPriority := min(max(rule.Priority+delta, 0), 99)
	rule.Priority = newPriority

	configRule := m.config.GetWorkflowRule(rule.ID)
	if configRule == nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("rule not found")} }
	}
	configRule.Priority = newPriority

	for i := range m.allRules {
		if m.allRules[i].ID == rule.ID {
			m.allRules[i].Priority = newPriority
			break
		}
	}

	mgr := m.configManager
	cfg := m.config
	return func() tea.Msg {
		if err := mgr.Save(cfg); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return WorkflowRulesUpdatedMsg{}
	}
}

func (m *WorkflowRulesModel) deleteCurrentRule() tea.Cmd {
	if m.cursor >= len(m.filteredRules) {
		return nil
	}

	rule := m.filteredRules[m.cursor]
	if !m.config.DeleteWorkflowRule(rule.ID) {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("rule not found")} }
	}

	defaultRules, customRules := m.config.GetWorkflowRulesSorted()
	m.allRules = append(defaultRules, customRules...)
	m.defaultCount = len(defaultRules)
	m.filterRules()

	mgr := m.configManager
	cfg := m.config
	return func() tea.Msg {
		if err := mgr.Save(cfg); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}
		return WorkflowRulesUpdatedMsg{}
	}
}
