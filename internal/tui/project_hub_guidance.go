package tui

import (
	"sort"
	"strings"

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
	actions := m.projectHubGuidanceActions(state)
	spec, ok := listSurfaceGuidanceSpec(state, km, actions)
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

func (m *ProjectHubModel) projectHubGuidanceKeyMap(state ListGuidanceState) *keymap.KeyMap {
	profile := state.Profile
	hasRows := m.surface != nil && len(m.surface.Matches()) > 0
	browseList := func() bool {
		return hasRows && profile != ListGuidanceSearchEditing && profile != ListGuidanceEmpty
	}
	searchEditing := func() bool { return profile == ListGuidanceSearchEditing }
	selection := func() bool { return profile == ListGuidanceSelection }
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
		keymap.Bind("/", "Search projects", "Filters",
			keymap.WithGate(func() bool {
				return !searchEditing() &&
					(profile != ListGuidanceEmpty || state.Empty.Kind == ListEmptyNoMatches)
			}),
			keymap.WithFooter(12, "search", false)),
		// Deliberately ungated: the filter can empty the list itself, and this is
		// then the only way back, so it must survive ListGuidanceEmpty where the
		// '/' hint above does not. The label names the result, not the state.
		keymap.Bind("ctrl+t", "Cycle every project, only those holding tasks, only those holding none", "Filters",
			keymap.WithFooter(13, filterFooterLabel, false),
			keymap.WithFooterKey("Ctrl+T")),
		keymap.Bind("esc", "Browse the current matches", "Navigation",
			keymap.WithGate(searchEditing),
			keymap.WithFooter(1, "browse matches", true), keymap.WithFooterKey("Esc")),
		keymap.Bind("v", "Toggle project selection", "Selection",
			keymap.WithGate(browseList), keymap.WithFooter(8, "toggle", false)),
		keymap.Bind("a", "Select every match", "Selection",
			keymap.WithGate(selection), keymap.WithFooter(9, "all", false),
			keymap.WithFooterKey("a/A")),
	}}
}

func (m *ProjectHubModel) projectHubGuidanceActions(state ListGuidanceState) []GuidanceHint {
	if state.Profile == ListGuidanceEmpty {
		if state.Empty.Kind == ListEmptySource {
			if hint, ok := m.projectHubOperationGuidance(operationScopeCreateProject, "n", 1, true); ok {
				return []GuidanceHint{hint}
			}
		}
		return nil
	}
	if state.Profile == ListGuidanceSelection {
		if len(projectHubBulkOperationIDs(m.surface.ActionState().OperationIDs)) > 0 {
			return []GuidanceHint{newGuidanceHint("Space bulk actions", 1, true)}
		}
		return nil
	}

	var hints []GuidanceHint
	if entry, ok := m.selectedProjectHubEntry(); ok && entry.Kind == projectHubEntryActive && entry.Project != nil {
		if hint, live := m.projectHubOperationGuidance(operationProjectOpen, "Enter", 1, state.Profile == ListGuidanceSearchEditing); live {
			hints = append(hints, hint)
		}
	}
	if state.Profile != ListGuidanceSearchEditing {
		if m.surface.ActionState().Enabled() && m.registry != nil {
			hints = append(hints, newGuidanceHint("Space actions", 2, false))
		}
		if hint, ok := m.projectHubOperationGuidance(operationScopeCreateProject, "n", 3, false); ok {
			hints = append(hints, hint)
		}
	}
	return hints
}

func (m *ProjectHubModel) projectHubOperationGuidance(
	id OperationID,
	key string,
	priority int,
	required bool,
) (GuidanceHint, bool) {
	if m.registry == nil {
		return GuidanceHint{}, false
	}
	descriptor, ok := m.registry.Descriptor(id)
	if !ok {
		return GuidanceHint{}, false
	}
	projectID := ""
	if entry, selected := m.selectedProjectHubEntry(); selected && entry.Project != nil {
		projectID = entry.Project.ID
	}
	if !descriptor.Applicable(m.projectHubCommandContext(projectID)) {
		return GuidanceHint{}, false
	}
	return GuidanceHint{
		Key:      key,
		Label:    strings.ToLower(descriptor.Label),
		Priority: priority,
		Required: required,
	}, true
}
