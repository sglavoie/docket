package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildWeeklyReviewKeyMap constructs the declarative KeyMap for WeeklyReviewModel.
func buildWeeklyReviewKeyMap(m *WeeklyReviewModel) *keymap.KeyMap {
	var entries []keymap.Entry
	entries = append(entries, weeklyReviewNavigationEntries(m)...)
	entries = append(entries, weeklyReviewPreviewEntries(m)...)
	entries = append(entries, weeklyReviewViewSpecificEntries(m)...)

	return &keymap.KeyMap{
		ViewName: "Weekly Review",
		Entries:  entries,
	}
}

func weeklyReviewNavigationEntries(m *WeeklyReviewModel) []keymap.Entry {
	return keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown:   m.weeklyReviewCursorDownAction,
		CursorUp:     m.weeklyReviewCursorUpAction,
		JumpToBottom: m.weeklyReviewJumpToBottomAction,
		PageUp:       m.weeklyReviewPageUpAction,
		PageDown:     m.weeklyReviewPageDownAction,
	})
}

func (m *WeeklyReviewModel) weeklyReviewCursorDownAction() (tea.Model, tea.Cmd) {
	if m.cursor < m.totalItems-1 {
		m.cursor++
	}
	return m, nil
}

func (m *WeeklyReviewModel) weeklyReviewCursorUpAction() (tea.Model, tea.Cmd) {
	if m.cursor > 0 {
		m.cursor--
	}
	return m, nil
}

func (m *WeeklyReviewModel) weeklyReviewJumpToBottomAction() (tea.Model, tea.Cmd) {
	old := m.cursor
	if m.totalItems > 0 {
		m.cursor = m.totalItems - 1
	}
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *WeeklyReviewModel) weeklyReviewPageUpAction() (tea.Model, tea.Cmd) {
	old := m.cursor
	jumpCursor(&m.cursor, m.totalItems, -HalfPage(m.Height()))
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *WeeklyReviewModel) weeklyReviewPageDownAction() (tea.Model, tea.Cmd) {
	old := m.cursor
	jumpCursor(&m.cursor, m.totalItems, HalfPage(m.Height()))
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func weeklyReviewPreviewEntries(m *WeeklyReviewModel) []keymap.Entry {
	return []keymap.Entry{
		keymap.Bind("home", "Jump to top", "Navigation", func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() }),
		keymap.Bind("f", "Toggle section fold", "View", m.weeklyReviewToggleSectionAction),
		keymap.Bind("v", "Toggle inline preview", "View", m.weeklyReviewTogglePreviewAction),
	}
}

func (m *WeeklyReviewModel) weeklyReviewToggleSectionAction() (tea.Model, tea.Cmd) {
	return m.toggleCurrentSection()
}

func (m *WeeklyReviewModel) weeklyReviewTogglePreviewAction() (tea.Model, tea.Cmd) {
	m.previewMode = !m.previewMode
	return m, nil
}
