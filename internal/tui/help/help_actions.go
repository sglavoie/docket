package help

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/operation"
)

// EnableActions freezes the opening context. Search gets the complete registry
// projection; the unfiltered, grouped reference keeps its original contents.
func (m *HelpViewModel) EnableActions(ctx operation.CommandContext, items []operation.HelpItem) {
	context := operation.CloneCommandContext(ctx)
	m.actionContext = &context
	m.actionItems = items
	m.searchInput.Placeholder = "Search actions or shortcuts..."
	m.filterItems()
	for i, item := range m.filteredItems {
		if helpItemKey(item.HelpItem) == m.restoreActionKey {
			m.actionCursor = i
			break
		}
	}
	m.restoreActionKey = ""
	if m.ready {
		m.SetSize(m.width, m.height)
	}
}

func helpItemKey(item operation.HelpItem) string {
	return item.Category + "\x00" + item.Key + "\x00" + item.Description
}

func (m *HelpViewModel) actionCategories(items []operation.HelpItem) {
	category := m.restoreCategory
	m.restoreCategory = ""
	if category == "" && m.activeTab > 0 && m.activeTab <= len(m.categories) {
		category = m.categories[m.activeTab-1]
	}
	categories := make([]string, 0, len(items))
	for _, item := range items {
		categories = append(categories, helpSectionForItem(item))
	}
	m.categories = helpSectionsForCategories(categories)
	m.activeTab = 0
	for i, candidate := range m.categories {
		if candidate == category {
			m.activeTab = i + 1
			break
		}
	}
}

// RefuseAction keeps help open when the source changed after discovery. Both
// the current results and later searches retain the explanation until reopening.
func (m *HelpViewModel) RefuseAction(id operation.OperationID, reason string) {
	for i, item := range m.actionItems {
		if item.Action != nil && item.Action.ID == id {
			m.actionItems[i].DisabledReason = reason
		}
	}
	for i, item := range m.filteredItems {
		if item.Action != nil && item.Action.ID == id {
			m.filteredItems[i].DisabledReason = reason
		}
	}
	m.SetSize(m.width, m.height)
}

func (m *HelpViewModel) actionSearch() bool {
	return m.actionContext != nil && strings.TrimSpace(m.searchQuery) != ""
}

func (m *HelpViewModel) prepareActionResults() {
	m.actionCursor = 0
	if !m.actionSearch() {
		return
	}
	categoryOrder := make(map[string]int, len(m.categories))
	for i, category := range m.categories {
		categoryOrder[category] = i
	}
	sort.SliceStable(m.filteredItems, func(i, j int) bool {
		a, b := m.filteredItems[i], m.filteredItems[j]
		if a.exactMatch != b.exactMatch {
			return a.exactMatch
		}
		ar, br := actionMatchRank(a, m.searchQuery), actionMatchRank(b, m.searchQuery)
		if ar != br {
			return ar < br
		}
		ac, bc := helpSectionForItem(a.HelpItem), helpSectionForItem(b.HelpItem)
		if ac != bc {
			return categoryOrder[ac] < categoryOrder[bc]
		}
		return ac == "Tasks" && taskHelpOrder(a) < taskHelpOrder(b)
	})
}

// Literal words outrank fuzzy-only hits when Enter can run a result: "lens"
// should lead with Named Lenses before a loose match in "description".
func actionMatchRank(item filteredHelpItem, query string) int {
	containsTerms := func(text string) bool {
		text = strings.ToLower(text)
		for _, term := range strings.Fields(strings.ToLower(query)) {
			if !strings.Contains(text, term) {
				return false
			}
		}
		return true
	}
	label := item.Key + " " + item.Description
	if containsTerms(label) {
		return 0
	}
	if containsTerms(label + " " + item.Category + " " + item.Group + " " + strings.Join(item.Aliases, " ")) {
		return 1
	}
	return 2
}

func (m *HelpViewModel) selectedActionItem() (filteredHelpItem, bool) {
	if !m.actionSearch() || len(m.filteredItems) == 0 {
		return filteredHelpItem{}, false
	}
	return m.filteredItems[min(max(m.actionCursor, 0), len(m.filteredItems)-1)], true
}

func (m *HelpViewModel) updateActionSearch(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if !m.actionSearch() || m.tabSearchMode {
		return false, nil
	}
	key := msg.String()
	// Letters and caret movement still belong to the search input. Arrows can
	// choose a result without first leaving that input.
	if m.searchMode && key != "up" && key != "down" && key != "enter" {
		return false, nil
	}
	if key != "g" {
		m.pendingG = false
	}
	switch key {
	case "enter":
		item, ok := m.selectedActionItem()
		if !ok || item.Action == nil || item.DisabledReason != "" {
			return true, nil
		}
		action := *item.Action
		request := operation.OperationExecutionRequest{
			ID: action.ID, Value: action.Value, HasValue: action.HasValue,
			Context: operation.CloneCommandContext(*m.actionContext),
		}
		return true, func() tea.Msg { return RunActionMsg{Source: m, Request: request} }
	case "up", "k":
		m.actionCursor--
	case "down", "j":
		m.actionCursor++
	case "pgup", "ctrl+u":
		m.moveActionPage(-1)
	case "pgdown", "ctrl+d":
		m.moveActionPage(1)
	case "home":
		m.actionCursor = 0
	case "end", "G":
		m.actionCursor = len(m.filteredItems) - 1
	case "g":
		if m.pendingG {
			m.actionCursor = 0
			m.pendingG = false
		} else {
			m.pendingG = true
		}
	default:
		return false, nil
	}
	m.actionCursor = max(0, min(m.actionCursor, len(m.filteredItems)-1))
	return true, nil
}

func (m *HelpViewModel) moveActionPage(direction int) {
	_, starts := m.actionSearchDocument()
	if len(starts) == 0 {
		return
	}
	row := starts[m.actionCursor] + direction*max(m.viewport.Height()/2, 1)
	for {
		next := m.actionCursor + direction
		if next < 0 || next >= len(starts) {
			break
		}
		m.actionCursor = next
		if (direction > 0 && starts[next] >= row) || (direction < 0 && starts[next] <= row) {
			break
		}
	}
}

func (m *HelpViewModel) keepActionVisible() {
	if _, ok := m.selectedActionItem(); !ok {
		return
	}
	content, starts := m.actionSearchDocument()
	start := starts[m.actionCursor]
	end := len(strings.Split(strings.TrimSuffix(content, "\n"), "\n"))
	if m.actionCursor+1 < len(starts) {
		end = starts[m.actionCursor+1]
	}
	if start < m.viewport.YOffset() || end-start > m.viewport.Height() {
		m.viewport.SetYOffset(start)
	} else if end > m.viewport.YOffset()+m.viewport.Height() {
		m.viewport.SetYOffset(end - m.viewport.Height())
	}
}
