package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

type outlinerEmptyCause uint8

const (
	outlinerEmptyNoTasks outlinerEmptyCause = iota
	outlinerEmptyNoProjects
	outlinerEmptyMode
	outlinerEmptyFiltered
	outlinerEmptyProjectFiltered
	outlinerEmptySearch
	outlinerEmptySearchFiltered
	outlinerEmptySearchInvalid
	outlinerEmptyLens
	outlinerEmptyFocus
	outlinerEmptySection
)

func (m *OutlinerModel) HandledKeys() []string {
	if m.operationRegistry == nil {
		return nil
	}
	return m.operationRegistry.HandledKeysForInputStrategy(InputStrategySimplified)
}

// The Outliner has no GetHelpItems of its own: `?` is a core input operation, so
// AppModel.coreInputHelpItems is the one projection that answers it. A second
// copy here would be unreachable and would drift.

// ContextualGuidance selects one Outliner-specific profile before falling back
// to the ranked operation projection for an ordinary task row. Interaction and
// causal recovery controls are required so narrow terminals retain the action
// that exits the current state.
func (m *OutlinerModel) ContextualGuidance() (GuidanceSpec, bool) {
	if m == nil {
		return GuidanceSpec{}, false
	}
	switch {
	case m.confirm.IsActive():
		return GuidanceSpec{Hints: requiredGuidanceHints("Y confirm", "N/Esc cancel"), Rows: 2}, true
	case m.quickEdit.IsActive():
		hints := requiredGuidanceHints("Enter save", "Esc cancel")
		if !m.quickEdit.IsRenameOnly() {
			hints = append(hints, optionalGuidanceHints(10, quickEditGrammarGuidance()...)...)
		}
		return GuidanceSpec{Hints: hints, Rows: 2}, true
	case m.editingLensQuery():
		// The shared input is holding the lens's narrowing query, not an ordinary
		// search: Enter keeps what was typed and Esc puts back the query the edit
		// opened on, neither of which is what the search wording promises.
		return GuidanceSpec{
			Hints: append(requiredGuidanceHints("Enter narrow lens", "Esc cancel edit"),
				optionalGuidanceHints(10, "lens query narrows this lens only")...),
			Rows: 2,
		}, true
	case m.IsSearchMode() && m.searchAutocomplete.active():
		// Only while the pane is open: an Esc here closes suggestions rather than
		// the search, and saying so is the whole point of the swap.
		return m.searchAutocompleteGuidance(), true
	case m.IsSearchMode():
		return GuidanceSpec{Hints: requiredGuidanceHints("Enter apply search", "Esc clear search"), Rows: 2}, true
	case len(m.rows) == 0:
		return m.emptyGuidance(), true
	case m.multiSelectMode:
		return m.multiSelectGuidance(), true
	case m.peekMode:
		peekKey := "Ctrl+Enter"
		if m.operationRegistry != nil {
			if key := m.operationRegistry.InputShortcutForOperation(InputStrategySimplified, operationCorePeek); key != "" {
				peekKey = key
			}
		}
		return GuidanceSpec{
			Hints: append(requiredGuidanceHints(peekKey+" close Peek", "Esc close Peek"),
				optionalGuidanceHints(10, "Space actions", "e edit", "? help")...),
			Rows: 2,
		}, true
	case m.hasSearchProjection():
		return GuidanceSpec{
			Hints: append(requiredGuidanceHints("Esc clear search"),
				optionalGuidanceHints(10, "/ edit search", "a capture", "P projects", "? help")...),
			Rows: 1,
		}, true
	case m.hasActiveLens():
		// One press leaves the whole saved projection, so it is the required hint
		// here exactly as clearing search is above. `/` is named beside it because
		// under a lens that key edits the lens's narrowing query rather than
		// opening a scope-wide search, which nothing else on screen says.
		return GuidanceSpec{
			Hints: append(requiredGuidanceHints("Esc restore lens"),
				optionalGuidanceHints(10, `/ lens query`, `\ lenses`, "a capture", "? help")...),
			Rows: 1,
		}, true
	case m.cursorNamesHeader():
		return m.headerGuidance(), true
	case m.operationRegistry == nil:
		return GuidanceSpec{}, false
	}

	hints := m.operationRegistry.GuidanceHintsForInputStrategy(
		InputStrategySimplified,
		m.operationCommandContext(),
	)
	if len(hints) == 0 {
		return GuidanceSpec{}, false
	}
	return GuidanceSpec{Hints: hints, Rows: 1}, true
}

func quickEditGrammarGuidance() []string {
	labels := map[textgrammar.Field]string{
		textgrammar.FieldPriority:    "priority",
		textgrammar.FieldTag:         "add tag",
		textgrammar.FieldRemoveTag:   "remove tag",
		textgrammar.FieldDue:         "due",
		textgrammar.FieldDefer:       "defer",
		textgrammar.FieldDestination: "destination",
		textgrammar.FieldPinned:      "pin",
		textgrammar.FieldProject:     "move",
		textgrammar.FieldParent:      "parent",
		textgrammar.FieldChild:       "adopt",
	}
	// The reference forms are named for the same reason the tag pair is: the
	// sigil is the only route to a stored set no key opens. They are appended
	// rather than labeled in place because parse order puts the tilde first,
	// and a two-row footer that led with the newest sigil would drop the
	// structural ones every surface above it already advertises.
	trailing := map[textgrammar.Field]string{
		textgrammar.FieldDependency:       "depends on",
		textgrammar.FieldRemoveDependency: "drop dependency",
	}
	entries := make([]string, 0, len(labels)+len(trailing))
	for _, source := range []map[textgrammar.Field]string{labels, trailing} {
		for _, definition := range textgrammar.Definitions(textgrammar.SurfaceEdit) {
			label, ok := source[definition.Field]
			if !ok || len(definition.Forms) == 0 {
				continue
			}
			entries = append(entries, strings.Join(definition.Forms, "/")+" "+label)
		}
	}
	return entries
}

func (m *OutlinerModel) multiSelectGuidance() GuidanceSpec {
	rangeAction := "V range"
	if m.IsRangeSelectionActive() {
		rangeAction = "V freeze range"
	}
	hints := []GuidanceHint{
		newGuidanceHint("Space actions", 1, m.SelectedCount() > 0),
		newGuidanceHint("v toggle", 2, false),
		newGuidanceHint(rangeAction, 3, false),
		newGuidanceHint("Ctrl+A select visible", 4, false),
		newGuidanceHint("Esc clear selection", 0, true),
		newGuidanceHint("? help", 10, false),
	}
	return GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) cursorNamesHeader() bool {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	return m.rows[m.cursor].isHeaderRow()
}

// headerGuidance names only the keys the header under the cursor can act on. On
// the zoomed section header three of them cannot: Enter re-enters the zoom it is
// already in, and the zoom overrides that section's fold, so h/l and s refuse
// rather than write one. Esc is the act that remains, so it takes the fold
// hint's rank and is required.
//
// The two projected hints are re-ranked to 1 rather than kept at the rank the
// strategy declares. That rank orders them among a task row's many keys, where
// they are minor; on a header, opening what the row names is the act the row
// exists for, so it must outlive the mode and search chrome when the terminal
// narrows.
func (m *OutlinerModel) headerGuidance() GuidanceSpec {
	row := m.rows[m.cursor]
	zoomed := m.isZoomedSectionRow(row)
	hints := make([]GuidanceHint, 0, 8)
	if m.operationRegistry != nil {
		for _, hint := range m.operationRegistry.GuidanceHintsForInputStrategy(
			InputStrategySimplified,
			m.operationCommandContext(),
		) {
			if (hint.Key == "Enter" && !zoomed) || hint.Key == "Ctrl+Enter" {
				hint.Priority = 1
				hints = append(hints, hint)
			}
		}
	}
	switch {
	case zoomed:
		hints = append(hints, newGuidanceHint("Esc unzoom", 1, true))
	case m.isSectionRowCollapsed(row):
		hints = append(hints, newGuidanceHint("l/Right expand", 1, false))
	default:
		hints = append(hints, newGuidanceHint("h/Left collapse", 1, false))
	}
	if row.IsProjectHeader {
		// All three sit at rank 2 and the stable sort keeps them in this order, so
		// the narrowest header footer that carries a project act carries the menu
		// first: it still reaches Edit, Archive and Delete, which no key beside it
		// does. The two keys that left the menu follow it, each labeled as the
		// strategy labels it.
		hints = append(hints,
			newGuidanceHint("Space actions", 2, false),
			newGuidanceHint("# project tags", 2, false),
			newGuidanceHint("* project pin", 2, false),
		)
	} else if !zoomed {
		hints = append(hints, newGuidanceHint("s sections", 2, false))
	}
	hints = append(hints, optionalGuidanceHints(3, "Tab next mode", "P projects", "/ search", "? help")...)
	return GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) emptyGuidance() GuidanceSpec {
	var required string
	cause := m.classifyEmptyCause()
	switch cause {
	case outlinerEmptyNoProjects:
		required = "P projects"
	case outlinerEmptyMode:
		if m.primaryMode == ModeInbox {
			required = "a capture"
		} else {
			required = "Tab next mode"
		}
	case outlinerEmptyFiltered:
		required = "fx clear filters"
	case outlinerEmptyProjectFiltered:
		required = "Fx clear project filters"
	case outlinerEmptySearch, outlinerEmptySearchInvalid:
		required = "Esc clear search"
	case outlinerEmptySearchFiltered:
		// The query is what the user is holding, so widening the facets is the act
		// that keeps it; leaving search is named beside it rather than in its place.
		required = "fx clear filters"
	case outlinerEmptyLens:
		required = "Esc restore lens"
	case outlinerEmptyFocus, outlinerEmptySection:
		required = "Esc return"
	default:
		required = "a capture"
	}
	optional := []string{"P projects", "? help"}
	if strings.HasPrefix(required, "P ") {
		optional = []string{"? help"}
	}
	if cause == outlinerEmptySearchFiltered {
		optional = append([]string{"Esc clear search"}, optional...)
	}
	hints := append(requiredGuidanceHints(required), optionalGuidanceHints(10, optional...)...)
	return GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) classifyEmptyCause() outlinerEmptyCause {
	if m.hasSearchProjection() && m.SearchQuery() != "" {
		if m.searchGate.invalid() {
			return outlinerEmptySearchInvalid
		}
		// Facets are an AND gate applied before scoring, so the plain search
		// message would claim the query matched nothing when a facet is what hid
		// the result. Naming both exits is the difference.
		if hasActiveCommonFilters(&m.filter.CommonFilterState) {
			return outlinerEmptySearchFiltered
		}
		return outlinerEmptySearch
	}
	if m.project == nil && len(m.loadedProjects) == 0 {
		if hasActiveProjectFilters(&m.projectFilter) {
			return outlinerEmptyProjectFiltered
		}
		return outlinerEmptyNoProjects
	}
	// A lens owns the mode, both filter families, the sort, and its narrowing
	// query all at once, so naming any one of them would send the user to a key
	// that cannot widen the list. It outranks zoom and focus for the same reason
	// the Esc cascade unwinds it first.
	if m.hasActiveLens() {
		return outlinerEmptyLens
	}
	// A zoom outranks the filter and focus causes below it: whatever else
	// narrowed the list, Esc is the press that widens it back, and naming a
	// filter would send the user to a key that leaves the section empty.
	if m.zoom.active() {
		return outlinerEmptySection
	}
	if hasActiveCommonFilters(&m.filter.CommonFilterState) {
		return outlinerEmptyFiltered
	}
	if len(m.focusStack) > 0 || len(m.crossFocusStack) > 0 {
		return outlinerEmptyFocus
	}
	if m.primaryMode != ModeAll {
		return outlinerEmptyMode
	}
	return outlinerEmptyNoTasks
}

// decoratePendingChordHint adds effective filter state to the existing live
// registry entry. It never creates or reorders a key, so which-key and dispatch
// continue sharing the same metadata and applicability checks.
func (m *OutlinerModel) decoratePendingChordHint(path []string, hint ChordHint) ChordHint {
	if m == nil || len(path) != 2 {
		return hint
	}
	switch path[0] {
	case "f":
		filter := &m.filter.CommonFilterState
		switch path[1] {
		case "!":
			if filter.Priority > 0 {
				hint.Label = "priority:" + strings.ToLower(models.PriorityName(filter.Priority)) + "+"
			}
		case "$":
			if filter.hasStateFilter() {
				hint.Label = fmt.Sprintf("states:%d", len(filter.States.Values()))
			}
		case "a":
			if filter.hasCompletionAgeFilter() {
				hint.Label = "completed:all"
				if days := filter.CompletionAge.days(); days > 0 {
					hint.Label = fmt.Sprintf("completed:%dd+", days)
				}
			}
		case "c":
			// The no-context row is one more admitted alternative, so it counts
			// alongside the names rather than as a facet of its own.
			count := len(filter.Contexts)
			if filter.NoContext {
				count++
			}
			if count > 0 {
				hint.Label = fmt.Sprintf("contexts:%d", count)
			}
		case "@":
			switch {
			case filter.Deferred:
				hint.Label = "deferred:on"
			case filter.DueDate != DueDateAll:
				hint.Label = "due:" + dueDateFilterLabel(filter.DueDate)
			}
		case "#":
			if len(filter.Tags) > 0 {
				hint.Label = fmt.Sprintf("tags:%d", len(filter.Tags))
			}
		case "r":
			if filter.Tags[reviewTaskTag] {
				hint.Label = "review:on"
			}
		case "h":
			if filter.Hierarchy != HierarchyAll {
				hint.Label = "hierarchy:" + hierarchyFilterLabel(filter.Hierarchy)
			}
		case "s":
			if filter.Stale {
				hint.Label = "stale:on"
			}
		case "x":
			if count := commonFilterFacetCount(filter); count > 0 {
				hint.Label = fmt.Sprintf("clear:%d", count)
			}
		}
	case "F":
		return m.decorateProjectFilterChordHint(path[1], hint)
	}
	return hint
}

// decorateProjectFilterChordHint decorates one F continuation. Each population
// key only reports the mode it owns, so the other key of the same axis keeps
// its static affordance instead of both rows naming one state.
func (m *OutlinerModel) decorateProjectFilterChordHint(key string, hint ChordHint) ChordHint {
	filter := &m.projectFilter
	switch key {
	case "#":
		if selected := projectFilterSelection(filter); len(selected) > 0 {
			hint.Label = fmt.Sprintf("project tags:%d", len(selected))
		}
	case "p":
		if filter.ParkedMode == projectPopulationInclude {
			hint.Label = "parked:on"
		}
	case "P":
		if filter.ParkedMode == projectPopulationOnly {
			hint.Label = "parked:only"
		}
	case "a":
		if filter.ArchivedMode == projectPopulationInclude {
			hint.Label = "archived:on"
		}
	case "A":
		if filter.ArchivedMode == projectPopulationOnly {
			hint.Label = "archived:only"
		}
	case "x":
		if count := projectFilterFacetCount(filter); count > 0 {
			hint.Label = fmt.Sprintf("clear:%d", count)
		}
	}
	return hint
}
