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
	config        *config.Config
	configManager *config.Manager

	rules  []workflow.WorkflowRule
	cursor int

	width  int
	height int

	confirmDelete bool
	expandedView  bool
}

// NewWorkflowRulesModel creates a new workflow rules model.
func NewWorkflowRulesModel(cfg *config.Config, manager *config.Manager) *WorkflowRulesModel {
	return &WorkflowRulesModel{
		config:        cfg,
		configManager: manager,
		rules:         cfg.GetWorkflowRules(),
		cursor:        0,
		width:         80,
		height:        24,
	}
}

// SetSize sets the view dimensions.
func (m *WorkflowRulesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
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

		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y":
				m.confirmDelete = false
				return m, m.deleteCurrentRule()
			case "n", "N", "esc":
				m.confirmDelete = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "j", "down":
			if len(m.rules) > 0 && m.cursor < len(m.rules)-1 {
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
			if len(m.rules) > 0 {
				m.cursor = len(m.rules) - 1
			}
			return m, nil

		case " ":
			// Toggle enabled/disabled
			if len(m.rules) > 0 {
				return m, m.toggleCurrentRule()
			}
			return m, nil

		case "enter":
			// Show expanded view with full rule details
			if len(m.rules) > 0 && m.cursor < len(m.rules) {
				m.expandedView = true
			}
			return m, nil

		case "d", "D":
			// Delete rule (with confirmation)
			if len(m.rules) > 0 {
				m.confirmDelete = true
			}
			return m, nil

		case "n":
			// Create new rule
			return m, func() tea.Msg {
				return ShowWorkflowRuleCreatorMsg{}
			}

		case "e":
			// Edit selected rule
			if len(m.rules) > 0 && m.cursor < len(m.rules) {
				rule := m.rules[m.cursor]
				return m, func() tea.Msg {
					return ShowWorkflowRuleEditorMsg{Rule: rule}
				}
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *WorkflowRulesModel) toggleCurrentRule() tea.Cmd {
	return func() tea.Msg {
		if m.cursor >= len(m.rules) {
			return nil
		}

		rule := &m.rules[m.cursor]
		enabled, found := m.config.ToggleWorkflowRule(rule.ID)
		if !found {
			return ErrorMsg{Err: fmt.Errorf("rule not found")}
		}

		// Update local copy
		rule.Enabled = enabled

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return WorkflowRulesUpdatedMsg{}
	}
}

func (m *WorkflowRulesModel) deleteCurrentRule() tea.Cmd {
	return func() tea.Msg {
		if m.cursor >= len(m.rules) {
			return nil
		}

		rule := m.rules[m.cursor]
		if !m.config.DeleteWorkflowRule(rule.ID) {
			return ErrorMsg{Err: fmt.Errorf("rule not found")}
		}

		// Update local copy
		m.rules = m.config.GetWorkflowRules()
		if m.cursor >= len(m.rules) && m.cursor > 0 {
			m.cursor--
		}

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return WorkflowRulesUpdatedMsg{}
	}
}

func (m *WorkflowRulesModel) View() string {
	var b strings.Builder

	// Expanded view mode - show full rule details
	if m.expandedView && len(m.rules) > 0 && m.cursor < len(m.rules) {
		rule := m.rules[m.cursor]

		// Rule name
		b.WriteString(BoldStyle.Foreground(ColorPrimary).Render(rule.Name))
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
		maxLen := m.width - 12
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
		if m.width-4 < cardWidth {
			cardWidth = m.width - 4
		}
		config := OverlayCardConfig{
			Title:        "Rule Details",
			Width:        cardWidth,
			Centered:     true,
			NoTruncation: true,
		}
		return renderOverlayCard(b.String(), m.width, m.height, config)
	}

	if len(m.rules) == 0 {
		b.WriteString(MutedStyle.Render("No workflow rules configured.\n\n"))
		b.WriteString(MutedStyle.Render("Use 'tt workflow add' to create rules."))
	} else {
		// Calculate visible area
		maxVisible := m.height - 10 // Leave room for header, footer, card padding
		if maxVisible < 3 {
			maxVisible = 3
		}

		startIdx := 0
		if m.cursor >= maxVisible {
			startIdx = m.cursor - maxVisible + 1
		}
		endIdx := startIdx + maxVisible
		if endIdx > len(m.rules) {
			endIdx = len(m.rules)
		}

		for i := startIdx; i < endIdx; i++ {
			rule := m.rules[i]

			// Status indicator
			var status string
			if rule.Enabled {
				status = DoneStyle.Render("●")
			} else {
				status = MutedStyle.Render("○")
			}

			// Rule name and summary
			name := rule.Name
			if len(name) > 30 {
				name = name[:27] + "..."
			}

			summary := rule.Summary()
			if len(summary) > 50 {
				summary = summary[:47] + "..."
			}

			if i == m.cursor {
				line := BoldStyle.Foreground(ColorPrimary).Render(fmt.Sprintf("▸ %s %-30s", status, name))
				b.WriteString(line)
				b.WriteString("\n")
				// Show summary for selected rule
				b.WriteString(MutedStyle.Render(fmt.Sprintf("    %s", summary)))
			} else {
				b.WriteString(fmt.Sprintf("  %s %-30s", status, name))
			}
			b.WriteString("\n")
		}

		// Show scroll indicator if needed
		if len(m.rules) > maxVisible {
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  %d of %d rules", m.cursor+1, len(m.rules))))
		}
	}

	// Confirmation dialog
	if m.confirmDelete && len(m.rules) > 0 {
		b.WriteString("\n\n")
		b.WriteString(ConfirmStyle.Render(fmt.Sprintf(" Delete rule '%s'? (y/n) ", m.rules[m.cursor].Name)))
	}

	// Help footer
	b.WriteString("\n\n")
	if len(m.rules) > 0 {
		b.WriteString(MutedStyle.Render("j/k: navigate · Enter: details · Space: toggle · n: new · e: edit · d: delete · q: close"))
	} else {
		b.WriteString(MutedStyle.Render("n: new rule · q: close"))
	}

	cardWidth := 70
	if m.width-4 < cardWidth {
		cardWidth = m.width - 4
	}
	config := OverlayCardConfig{
		Title:        "Workflow Rules",
		Width:        cardWidth,
		Centered:     true,
		NoTruncation: true,
	}
	return renderOverlayCard(b.String(), m.width, m.height, config)
}

// WorkflowRulesUpdatedMsg is sent when rules are modified.
type WorkflowRulesUpdatedMsg struct{}

// ShowWorkflowRulesMsg triggers showing the workflow rules view.
type ShowWorkflowRulesMsg struct{}
