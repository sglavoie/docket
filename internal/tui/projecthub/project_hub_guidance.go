package projecthub

import (
	"sort"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// ContextualGuidance projects Project Hub list state through the shared list
// profiles. Navigation and unwind controls come from a gated keymap; direct
// project-operation labels and applicability come from the operation registry.
func (m *ProjectHubModel) ContextualGuidance() (chrome.GuidanceSpec, bool) {
	if m.actions != nil {
		return m.actions.GuidanceSpec()
	}
	if m.surface == nil {
		return chrome.GuidanceSpec{}, false
	}
	state := m.surface.GuidanceState(m.HasActiveSearch())
	km := m.projectHubGuidanceKeyMap(state)
	spec, ok := listsurface.ListSurfaceGuidanceSpec(state, km, m.projectHubActionHints(state))
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
// describe here: the cursor keys and both ends of the list, `?`, and the
// ledger's u and Ctrl+R, whose core descriptors answer to the Outliner, the Esc
// unwind, which is no operation at all, and Ctrl+T, whose hint names the next
// filter rather than the operation's fixed label.
func (m *ProjectHubModel) projectHubGuidanceKeyMap(state listsurface.ListGuidanceState) *keymap.KeyMap {
	profile := state.Profile
	hasRows := m.surface != nil && len(m.surface.Matches()) > 0
	browseList := func() bool {
		return hasRows && profile != listsurface.ListGuidanceSearchEditing && profile != listsurface.ListGuidanceEmpty
	}
	searchEditing := func() bool { return profile == listsurface.ListGuidanceSearchEditing }
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
		// Ranked just below Ctrl+D so the pane's own paging is the first navigation
		// hint the single footer row drops: the list cursor is what the surface is
		// for, and the preview follows it.
		keymap.Bind("ctrl+f", "Page project tasks", "Navigation",
			keymap.WithGate(m.previewTasksScrollable),
			keymap.WithFooter(31, "task page", false), keymap.WithFooterKey("Ctrl+F/B")),
		// Ranked below both paging hints: moving the divider is an occasional
		// adjustment, not something the pane is read for.
		// Gated off the query, where < and > are literal characters.
		keymap.Bind("<", "Move pane split", "Navigation",
			keymap.WithGate(func() bool { return !searchEditing() && m.previewPaneVisible() }),
			keymap.WithFooter(32, "split", false), keymap.WithFooterKey("</>"),
			keymap.WithHelpKey("</>")),
		// Unranked, like `?` and the ledger keys below: the Hub's ranked bindings
		// are the project acts it exists for. Each row is spelled as the whole path
		// the strategy binds — the gg chord and its Home alias, G and End — so the
		// one row names every press that reaches that end.
		keymap.Bind("home", "Jump to first project", "Navigation",
			keymap.WithGate(browseList), keymap.WithHelpKey("gg/Home")),
		keymap.Bind("G", "Jump to last project", "Navigation",
			keymap.WithGate(browseList), keymap.WithHelpKey("G/End")),
		// Deliberately ungated: the filter can empty the list itself and is then
		// the only way back. The label names the result, not the state.
		keymap.Bind("ctrl+t", "Cycle every project, only those holding open tasks, only those holding none open", "Filters",
			keymap.WithFooter(13, filterFooterLabel, false),
			keymap.WithFooterKey("Ctrl+T")),
		keymap.Bind("esc", "Browse the current matches", "Navigation",
			keymap.WithGate(searchEditing),
			keymap.WithFooter(1, "browse matches", true), keymap.WithFooterKey("Esc")),
		// Gated off the query because `?` is a literal character there, and
		// deliberately unranked: the shell strip already prints "? help" beside the
		// breadcrumb, so a footer hint here would say it a second time. "System" is
		// the core descriptor's own category, so the row lands in the same help
		// bucket as the Outliner's `?`.
		keymap.Bind("?", "Help", "System",
			keymap.WithGate(func() bool { return !searchEditing() })),
		// Gated off the query for the same reason as `?`, and unranked for a
		// different one: the strip already offers "u undo" while a toast is live,
		// and the Hub's seventeen ranked bindings are the project acts the surface
		// exists for. "System" is the core descriptors' own category, so the two
		// rows land in the same help bucket as the Outliner's undo.
		keymap.Bind("u", "Undo workspace change", "System",
			keymap.WithGate(func() bool { return !searchEditing() })),
		keymap.Bind("ctrl+r", "Redo workspace change", "System",
			keymap.WithGate(func() bool { return !searchEditing() })),
	}}
}

// projectHubActionHints projects the direct grammar's ranked bindings into
// footer hints. Two states narrow that to the single press that resolves them:
// while the query is being typed the browse keys are not live, and a list its
// own source emptied cannot be searched or browsed at all.
func (m *ProjectHubModel) projectHubActionHints(state listsurface.ListGuidanceState) []chrome.GuidanceHint {
	if m.registry == nil {
		return nil
	}
	hints := m.registry.GuidanceHintsForInputStrategy(
		operation.InputStrategyProjectHub,
		m.projectHubCommandContext(m.cursorProjectID()),
	)
	switch {
	case state.Profile == listsurface.ListGuidanceSearchEditing:
		return m.soleRequiredHint(hints, operation.OperationProjectOpen)
	case state.Profile == listsurface.ListGuidanceEmpty && state.Empty.Kind == listsurface.ListEmptySource:
		return m.soleRequiredHint(hints, operation.OperationScopeCreateProject)
	}
	return hints
}

// soleRequiredHint keeps one operation's hint and promotes it, so a narrowed
// state advertises the press that leaves it rather than the whole grammar.
func (m *ProjectHubModel) soleRequiredHint(hints []chrome.GuidanceHint, id operation.OperationID) []chrome.GuidanceHint {
	key := m.registry.InputShortcutForOperation(operation.InputStrategyProjectHub, id)
	for _, hint := range hints {
		if hint.Key != key {
			continue
		}
		hint.Required = true
		return []chrome.GuidanceHint{hint}
	}
	return nil
}
