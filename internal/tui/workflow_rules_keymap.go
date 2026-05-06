package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for WorkflowRulesModel.
func (m *WorkflowRulesModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// Navigation
	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules)-1 {
				m.cursor++
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules)-1 {
				m.cursor++
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("G", "Jump to bottom", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.filteredRules) > 0 {
				m.cursor = len(m.filteredRules) - 1
			}
			return m, nil
		}),
		keymap.Bind("/", "Search rules", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.ActivateSearch()
		}),
	)

	// Goto group: g prefix (gg = top)
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) {
			m.cursor = 0
			return m, nil
		},
		GoToProject: nil,
	})...)

	// Actions
	entries = append(entries,
		keymap.Bind("space", "Toggle enabled/disabled", "Actions", func() (tea.Model, tea.Cmd) {
			if len(m.filteredRules) > 0 {
				return m, m.toggleCurrentRule()
			}
			return m, nil
		}),
		keymap.Bind("enter", "Preview (narrow screen)", "Actions", func() (tea.Model, tea.Cmd) {
			if !m.isWideScreen() && len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
				m.expandedView = true
			}
			return m, nil
		}),
		keymap.Bind("d", "Delete rule", "Actions", func() (tea.Model, tea.Cmd) {
			if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
				m.confirm.Show("delete", fmt.Sprintf("Delete rule '%s'?", m.filteredRules[m.cursor].Name))
			}
			return m, nil
		}),
		keymap.Bind("D", "Delete rule", "Actions", func() (tea.Model, tea.Cmd) {
			if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
				m.confirm.Show("delete", fmt.Sprintf("Delete rule '%s'?", m.filteredRules[m.cursor].Name))
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("n", "New rule (form)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowWorkflowRuleCreatorMsg{} }
		}),
		keymap.Bind("w", "New rule (wizard)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowWorkflowWizardMsg{} }
		}),
		keymap.Bind("e", "Edit rule", "Actions", func() (tea.Model, tea.Cmd) {
			if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
				rule := m.filteredRules[m.cursor]
				return m, func() tea.Msg { return ShowWorkflowRuleEditorMsg{Rule: rule} }
			}
			return m, nil
		}),
		keymap.Bind("J", "Adjust rule priority down", "Actions", func() (tea.Model, tea.Cmd) {
			if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
				return m, m.adjustCurrentRulePriority(1)
			}
			return m, nil
		}),
		keymap.Bind("K", "Adjust rule priority up", "Actions", func() (tea.Model, tea.Cmd) {
			if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
				return m, m.adjustCurrentRulePriority(-1)
			}
			return m, nil
		}),
		keymap.Bind("t", "Dry run all rules", "Actions", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowWorkflowDryRunMsg{} }
		}),
		keymap.Bind("T", "Test selected rule", "Actions", func() (tea.Model, tea.Cmd) {
			if len(m.filteredRules) > 0 && m.cursor < len(m.filteredRules) {
				rule := m.filteredRules[m.cursor]
				return m, func() tea.Msg { return ShowWorkflowDryRunMsg{SingleRule: &rule} }
			}
			return m, nil
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Workflow Rules",
		Entries:  entries,
	}
}
