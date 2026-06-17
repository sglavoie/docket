package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

func weeklyReviewViewSpecificEntries(m *WeeklyReviewModel) []keymap.Entry {
	entries := []keymap.Entry{
		keymap.Bind("enter", "Navigate to project/task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.handleEnter()
		}),
		keymap.Bind("tab", "Collapse/expand section", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.toggleCurrentSection()
		}),
		keymap.Bind("n", "Next section (show all)", "Navigation", func() (tea.Model, tea.Cmd) {
			weeklyReviewShowAllSections(m)
			return m.goToNextSection()
		}),
		keymap.Bind("N", "Previous section (show all)", "Navigation", func() (tea.Model, tea.Cmd) {
			weeklyReviewShowAllSections(m)
			return m.goToPrevSection()
		}),
		keymap.Bind("ctrl+n", "Next unreviewed section", "Review", func() (tea.Model, tea.Cmd) {
			return m.goToNextUnreviewed()
		}, keymap.AsHidden()),
		weeklyReviewTabCycleEntry(m, "shift+right", "Next tab"),
		weeklyReviewTabCycleEntry(m, "shift+left", "Previous tab"),
		keymap.Bind("R", "Mark section reviewed / reset", "Review", func() (tea.Model, tea.Cmd) {
			if m.reviewCompleted {
				weeklyReviewResetProgress(m)
				return m, func() tea.Msg { return ResetWeeklyReviewMsg{} }
			}
			return m.markCurrentSectionReviewed()
		}),
	}

	entries = append(entries, weeklyReviewPriorityEntries(m)...)
	entries = append(entries, weeklyReviewMultiSelectEntries(m)...)
	entries = append(entries,
		keymap.Bind("ctrl+f", "Switch to Daily Review", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowDailyReviewMsg{} }
		}),
	)

	return entries
}

func weeklyReviewShowAllSections(m *WeeklyReviewModel) {
	m.focusedSection = nil
	m.calculateNavigation()
}

func weeklyReviewTabCycleEntry(m *WeeklyReviewModel, key, description string) keymap.Entry {
	return keymap.Bind(key, description, "Navigation", func() (tea.Model, tea.Cmd) {
		handleTabCycleKey(key, &m.tabs, func() {
			m.focusedSection = nil
			m.scrollOffset = 0
			m.calculateNavigation()
			m.clampCursor()
		})
		return m, nil
	})
}

func weeklyReviewResetProgress(m *WeeklyReviewModel) {
	m.reviewCompleted = false
	m.showingSummary = false
	for section := range m.reviewedSections {
		delete(m.reviewedSections, section)
	}
	m.loadData()
}

func weeklyReviewPriorityEntries(m *WeeklyReviewModel) []keymap.Entry {
	keys := []string{"1", "2", "3", "4", "5"}
	entries := make([]keymap.Entry, 0, len(keys))
	for _, k := range keys {
		key := k
		entries = append(entries, keymap.Bind(key, "Set priority "+key, "Actions", func() (tea.Model, tea.Cmd) {
			model, cmd := taskOpsChangePriority(m, key)
			if m.err == nil {
				m.trackMutation("priority")
			}
			return model, cmd
		}, keymap.AsHidden()))
	}
	return entries
}

func weeklyReviewMultiSelectEntries(m *WeeklyReviewModel) []keymap.Entry {
	return []keymap.Entry{
		keymap.Bind("v", "Toggle multi-select", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.toggleMultiSelectMode()
			return m, nil
		}),
		keymap.Bind("space", "Toggle selection (multi-select) / toggle peek", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.toggleTaskSelection()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("a", "Select all visible", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.selectAllVisible()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("A", "Deselect all", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.deselectAll()
			}
			return m, nil
		}, keymap.AsHidden()),
	}
}
