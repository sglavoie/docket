package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// ContextualGuidance projects Project Hub list state through the shared list
// profiles. Navigation and unwind controls come from a gated keymap; direct
// project-operation labels and applicability come from the operation registry.
func (m *ProjectHubModel) ContextualGuidance() (GuidanceSpec, bool) {
	if m.actions != nil {
		return m.actions.guidanceSpec()
	}
	if m.surface == nil {
		return GuidanceSpec{}, false
	}
	state := m.surface.GuidanceState(m.HasActiveSearch())
	km := m.projectHubGuidanceKeyMap(state)
	spec, ok := listSurfaceGuidanceSpec(state, km, m.projectHubActionHints(state))
	// Registry-derived operations are appended by the shared list adapter. Sort
	// the complete projection so required interaction controls precede optional
	// browse actions regardless of which source supplied them.
	sort.SliceStable(spec.Hints, func(i, j int) bool {
		if spec.Hints[i].Required != spec.Hints[j].Required {
			return spec.Hints[i].Required
		}
		return spec.Hints[i].Priority < spec.Hints[j].Priority
	})
	if m.HasActiveSearch() && len(m.surface.Matches()) == 0 {
		spec.Supplemental = "no matches"
	}
	return spec, ok
}

// projectHubGuidanceKeyMap carries only the keys the operation registry cannot
// describe here: the cursor keys, whose core descriptors answer to the Outliner,
// the Esc unwind, which is no operation at all, and Ctrl+T, whose hint names the
// next filter rather than the operation's fixed label.
func (m *ProjectHubModel) projectHubGuidanceKeyMap(state ListGuidanceState) *keymap.KeyMap {
	profile := state.Profile
	hasRows := m.surface != nil && len(m.surface.Matches()) > 0
	browseList := func() bool {
		return hasRows && profile != ListGuidanceSearchEditing && profile != ListGuidanceEmpty
	}
	searchEditing := func() bool { return profile == ListGuidanceSearchEditing }
	// The hint names what the next press shows, so a three-state cycle is legible
	// one press at a time.
	filterFooterLabel := m.taskFilter.next().shortLabel()

	return &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("j", "Move selection", "Navigation",
			keymap.WithGate(browseList), keymap.WithFooter(20, "navigate", false),
			keymap.WithFooterKey("j/k")),
		keymap.Bind("ctrl+j", "Move search selection", "Navigation",
			keymap.WithGate(searchEditing), keymap.WithFooter(20, "navigate", false),
			keymap.WithFooterKey("↑/↓ or Ctrl+J/K")),
		keymap.Bind("ctrl+d", "Move half page", "Navigation",
			keymap.WithGate(func() bool { return browseList() || searchEditing() }),
			keymap.WithFooter(30, "half page", false), keymap.WithFooterKey("Ctrl+D/U")),
		// Deliberately ungated: the filter can empty the list itself and is then
		// the only way back. The label names the result, not the state.
		keymap.Bind("ctrl+t", "Cycle every project, only those holding tasks, only those holding none", "Filters",
			keymap.WithFooter(13, filterFooterLabel, false),
			keymap.WithFooterKey("Ctrl+T")),
		keymap.Bind("esc", "Browse the current matches", "Navigation",
			keymap.WithGate(searchEditing),
			keymap.WithFooter(1, "browse matches", true), keymap.WithFooterKey("Esc")),
	}}
}

// projectHubActionHints projects the direct grammar's ranked bindings into
// footer hints. Two states narrow that to the single press that resolves them:
// while the query is being typed the browse keys are not live, and a list its
// own source emptied cannot be searched or browsed at all.
func (m *ProjectHubModel) projectHubActionHints(state ListGuidanceState) []GuidanceHint {
	if m.registry == nil {
		return nil
	}
	hints := m.registry.GuidanceHintsForInputStrategy(
		InputStrategyProjectHub,
		m.projectHubCommandContext(m.cursorProjectID()),
	)
	switch {
	case state.Profile == ListGuidanceSearchEditing:
		return m.soleRequiredHint(hints, operationProjectOpen)
	case state.Profile == ListGuidanceEmpty && state.Empty.Kind == ListEmptySource:
		return m.soleRequiredHint(hints, operationScopeCreateProject)
	}
	return hints
}

// soleRequiredHint keeps one operation's hint and promotes it, so a narrowed
// state advertises the press that leaves it rather than the whole grammar.
func (m *ProjectHubModel) soleRequiredHint(hints []GuidanceHint, id OperationID) []GuidanceHint {
	key := m.registry.InputShortcutForOperation(InputStrategyProjectHub, id)
	for _, hint := range hints {
		if hint.Key != key {
			continue
		}
		hint.Required = true
		return []GuidanceHint{hint}
	}
	return nil
}
