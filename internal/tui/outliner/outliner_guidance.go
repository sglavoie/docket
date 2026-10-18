package outliner

import (
	"strings"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
)

type outlinerEmptyCause uint8

const (
	outlinerEmptyNoTasks outlinerEmptyCause = iota
	outlinerEmptyNoProjects
	outlinerEmptyMode
	outlinerEmptyFiltered
	outlinerEmptyProjectFiltered
	outlinerEmptySearch
	OutlinerEmptySearchFiltered
	outlinerEmptySearchInvalid
	outlinerEmptyLens
	outlinerEmptyFocus
	OutlinerEmptySection
)

// The Outliner has no GetHelpItems of its own: `?` is a core input operation, so
// AppModel.coreInputHelpItems is the one projection that answers it. A second
// copy here would be unreachable and would drift.

// ContextualGuidance selects one Outliner-specific profile before falling back
// to the ranked operation projection for an ordinary task row. Interaction and
// causal recovery controls are required so narrow terminals retain the action
// that exits the current state.
func (m *OutlinerModel) ContextualGuidance() (chrome.GuidanceSpec, bool) {
	if m == nil {
		return chrome.GuidanceSpec{}, false
	}
	switch {
	case m.RelationPickerOpen():
		// First because the picker is drawn over the whole frame: every profile below
		// describes keys the rows it hid would have answered.
		return m.relationPickerGuidance(), true
	case m.MenuBar.Open:
		// First for the same reason the readout is first below it: while a menu is
		// open its own keymap is what the four navigation keys mean, and every
		// profile after this one would name acts they are not doing right now.
		return m.menuBarGuidance(), true
	case m.QueryReadout.Expanded:
		// First because the readout owns input outright: every other profile below
		// describes keys the stack is currently refusing.
		return m.queryReadoutGuidance(), true
	case m.Confirm.IsActive():
		return chrome.GuidanceSpec{Hints: chrome.RequiredGuidanceHints("y confirm", "n/Esc cancel"), Rows: 2}, true
	case m.QuickEdit.IsActive():
		hints := chrome.RequiredGuidanceHints("Enter save", "Esc cancel")
		if !m.QuickEdit.IsRenameOnly() {
			hints = append(hints, chrome.OptionalGuidanceHints(10, quickEditGrammarGuidance()...)...)
		}
		return chrome.GuidanceSpec{Hints: hints, Rows: 2}, true
	case m.IsSearchMode() && m.Search.historyPicker != nil:
		return chrome.RequiredExitGuidanceSpec("Enter use query", "Esc return to search", "↑/↓ select", "Ctrl+X forget query", "Type to filter history"), true
	case m.IsSearchMode() && m.Search.Autocomplete.Active():
		// Only while the pane is open: an Esc here closes suggestions rather than
		// the search, and saying so is the whole point of the swap.
		return m.searchAutocompleteGuidance(), true
	case m.IsSearchMode():
		hints := chrome.RequiredGuidanceHints("Enter apply search", m.SearchExitHint())
		if m.hasSearchHistory() {
			// Only once something has been submitted: an empty ring would advertise
			// a walk that does nothing on its first press.
			hints = append(hints, chrome.OptionalGuidanceHints(10, "↑/↓ recall query", "Ctrl+R search history")...)
		}
		return chrome.GuidanceSpec{Hints: hints, Rows: 2}, true
	// A focused projection with nothing in it still carries its project header,
	// so emptiness is asked of the content rather than of the row count. The
	// explanation and the way out of it are what the user needs there; the
	// header's own actions stay in `?` help and its action sheet.
	//
	// An open dock is the exception: it is the innermost thing on screen and the
	// one Esc puts down first, so naming the empty state's recovery key here would
	// promise an act the first press is not doing. Closing the dock restores this
	// branch and its cause unchanged.
	case !m.HasContentRows() && !m.PeekDockVisible():
		return m.emptyGuidance(), true
	case m.MultiSelectMode:
		return m.multiSelectGuidance(), true
	case m.PeekDockVisible():
		peekKey := m.peekOperationKey(operation.OperationCorePeek, peekDisplayKey)
		// One entry for two keys that do one thing. Peek is the cascade's first
		// step, so the act is read off the cascade rather than spelled here, and
		// the columns a second copy of the same sentence used to spend go to the
		// scrolling offers below it.
		hints := chrome.RequiredGuidanceHints(peekKey + "/" + m.EscHint())
		sizeAction := "expand Peek"
		if m.PeekExpanded {
			sizeAction = "compact Peek"
		}
		hints = append(hints, chrome.NewGuidanceHint(m.peekOperationKey(operation.OperationCorePeekSize, "B")+" "+sizeAction, 1, true))
		// The four scrolling keys are offered only in the direction the document
		// can actually move: a detail that fits its budget advertises no movement
		// at all. Their descriptors are applicable on a wider fact — a visible
		// Peek — because consuming a key at the end of a document is what keeps the
		// press off the list cursor underneath, and an inert offer is not the same
		// thing as an inert key.
		if scrolling := m.peekScrollHints(); len(scrolling) > 0 {
			hints = append(hints, scrolling...)
		}
		return chrome.GuidanceSpec{
			Hints: append(hints,
				chrome.OptionalGuidanceHints(10, "Space actions", "e edit", "? help")...),
			Rows: 2,
			// The paging hints are what wrap this footer, and the entry they push
			// onto a second row alone is not worth the row.
			DropOrphan: true,
		}, true
	case m.HasSearchProjection():
		return chrome.GuidanceSpec{
			Hints: append(chrome.RequiredGuidanceHints(m.EscHint()),
				chrome.OptionalGuidanceHints(10, "/ edit search", "a capture", "P projects", "? help")...),
			Rows: 1,
		}, true
	case m.HasActiveLens():
		// One press leaves the whole saved projection, so it is the required hint
		// here exactly as clearing search is above. `/` is named beside it because
		// under a lens that key suspends the lens for one scope-wide search rather
		// than narrowing it further, which nothing else on screen says.
		return chrome.GuidanceSpec{
			Hints: append(chrome.RequiredGuidanceHints(m.EscHint()),
				chrome.OptionalGuidanceHints(10, `/ search scope`, `\ lenses`, "a capture", "? help")...),
			Rows: 1,
		}, true
	case m.cursorNamesHeader():
		return m.headerGuidance(), true
	case m.OperationRegistry == nil:
		return chrome.GuidanceSpec{}, false
	}

	hints := m.OperationRegistry.GuidanceHintsForInputStrategy(
		operation.InputStrategySimplified,
		m.OperationCommandContext(),
	)
	// A Focus, a zoom, a scope, and a relationship hop all leave the ordinary row
	// footer standing, and none of them is a key the cursor names: the strategy's
	// ranks cannot answer what Esc does, because the answer is what is standing
	// rather than what is under the cursor. The cascade is asked instead, and the
	// entry is required so a narrow terminal keeps the way out of the state it is
	// in. An empty cascade adds nothing, so the browse footer stays as quiet as it
	// was.
	if esc := m.EscHint(); esc != "" {
		hints = append(hints, chrome.NewGuidanceHint(esc, 0, true))
	}
	if len(hints) == 0 {
		return chrome.GuidanceSpec{}, false
	}
	return chrome.GuidanceSpec{Hints: hints, Rows: 1}, true
}

func (m *OutlinerModel) multiSelectGuidance() chrome.GuidanceSpec {
	rangeAction := "V range"
	if m.IsRangeSelectionActive() {
		rangeAction = "V freeze range"
	}
	hints := []chrome.GuidanceHint{
		chrome.NewGuidanceHint("Space actions", 1, m.SelectedCount() > 0),
		chrome.NewGuidanceHint("v toggle", 2, false),
		chrome.NewGuidanceHint(rangeAction, 3, false),
		chrome.NewGuidanceHint("Ctrl+A select visible", 4, false),
		// A live range is its own cascade step above the selection, so the key
		// freezes the range here and clears the selection only once it is frozen.
		// The hint is read off the cascade for exactly that reason: spelling
		// "clear selection" at both moments would name an act one of them refuses.
		chrome.NewGuidanceHint(m.EscHint(), 0, true),
		chrome.NewGuidanceHint("? help", 10, false),
	}
	return chrome.GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) cursorNamesHeader() bool {
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	return m.Rows[m.Cursor].IsHeaderRow()
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
func (m *OutlinerModel) headerGuidance() chrome.GuidanceSpec {
	row := m.Rows[m.Cursor]
	zoomed := m.IsZoomedSectionRow(row)
	hints := make([]chrome.GuidanceHint, 0, 8)
	if m.OperationRegistry != nil {
		for _, hint := range m.OperationRegistry.GuidanceHintsForInputStrategy(
			operation.InputStrategySimplified,
			m.OperationCommandContext(),
		) {
			if (hint.Key == "Enter" && !zoomed) || hint.Key == peekDisplayKey {
				hint.Priority = 1
				hints = append(hints, hint)
			}
		}
	}
	escHint := m.EscHint()
	switch {
	case row.IsFocusedProjectHeader:
		// The one header with no fold advertises no fold key. Everything below it
		// is still foldable on its own row, so the outline loses nothing by this
		// row staying silent about h and l.
	case zoomed:
		// The way out takes the fold hint's place rather than the tail every other
		// header gives it, and names the cascade's own act: a zoom reached from
		// inside a Focus or a lens is left by those first, and "unzoom" would have
		// promised a press that does something else.
		hints = append(hints, chrome.NewGuidanceHint(escHint, 1, true))
		escHint = ""
	case m.IsSectionRowCollapsed(row):
		hints = append(hints, chrome.NewGuidanceHint("l/Right expand", 1, false))
	default:
		hints = append(hints, chrome.NewGuidanceHint("h/Left collapse", 1, false))
	}
	if row.IsProjectHeader {
		// All four sit at rank 2 and the stable sort keeps them in this order, so
		// the narrowest header footer that carries a project act carries the menu
		// first: it still reaches Archive and Delete, which no key beside it does.
		// The three keys that left the menu follow it, each labeled as the strategy
		// labels it.
		hints = append(hints,
			chrome.NewGuidanceHint("Space actions", 2, false),
			chrome.NewGuidanceHint("e project", 2, false),
			chrome.NewGuidanceHint("# project tags", 2, false),
			chrome.NewGuidanceHint("* project pin", 2, false),
		)
	} else if !zoomed {
		hints = append(hints, chrome.NewGuidanceHint("s sections", 2, false))
	}
	// A header row under a Focus, a scope, or a relationship hop owes the way out
	// as much as a task row does, and the zoomed header has already spent it above.
	if escHint != "" {
		hints = append(hints, chrome.NewGuidanceHint(escHint, 0, true))
	}
	hints = append(hints, chrome.OptionalGuidanceHints(3, "Tab next mode", "P projects", "/ search", "? help")...)
	return chrome.GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) emptyGuidance() chrome.GuidanceSpec {
	var required string
	cause := m.ClassifyEmptyCause()
	switch cause {
	case outlinerEmptyNoProjects:
		required = "P projects"
	case outlinerEmptyMode:
		if _, hint := m.emptyModeNextStep(m.PrimaryMode); hint != "" {
			required = hint
		} else {
			required = "Tab next mode"
		}
	case outlinerEmptyFiltered:
		required = "fx clear filters"
	case outlinerEmptyProjectFiltered:
		required = "Fx clear project filters"
	case outlinerEmptySearch, outlinerEmptySearchInvalid:
		required = m.EscHint()
	case OutlinerEmptySearchFiltered:
		// The query is what the user is holding, so widening the facets is the act
		// that keeps it; leaving search is named beside it rather than in its place.
		required = "fx clear filters"
	case outlinerEmptyLens, outlinerEmptyFocus, OutlinerEmptySection:
		// All three are cascade steps, so all three name the press the cascade
		// would actually run: a lens standing over a Focus is left first, and a
		// profile spelling its own recovery here would say otherwise.
		required = m.EscHint()
	default:
		required = "a capture"
	}
	optional := []string{"P projects", "? help"}
	if strings.HasPrefix(required, "P ") {
		optional = []string{"? help"}
	}
	if cause == OutlinerEmptySearchFiltered {
		optional = append([]string{m.EscHint()}, optional...)
	}
	hints := append(chrome.RequiredGuidanceHints(required), chrome.OptionalGuidanceHints(10, optional...)...)
	return chrome.GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) ClassifyEmptyCause() outlinerEmptyCause {
	if m.HasSearchProjection() && m.SearchQuery() != "" {
		if m.Search.Gate.Invalid() {
			return outlinerEmptySearchInvalid
		}
		// Facets are an AND gate applied before scoring, so the plain search
		// message would claim the query matched nothing when a facet is what hid
		// the result. Naming both exits is the difference.
		if query.HasActiveCommonFilters(&m.Filter.CommonFilterState) {
			return OutlinerEmptySearchFiltered
		}
		return outlinerEmptySearch
	}
	if m.Project == nil && len(m.Population.Projects) == 0 {
		if query.HasActiveProjectFilters(&m.ProjectFilter) {
			return outlinerEmptyProjectFiltered
		}
		return outlinerEmptyNoProjects
	}
	// A lens owns the mode, the whole narrowing, the sort, and the scope all at
	// once, so naming any one of them would send the user to a key that cannot
	// widen the list. It outranks zoom and focus for the same reason
	// the Esc cascade unwinds it first.
	if m.HasActiveLens() {
		return outlinerEmptyLens
	}
	// A zoom outranks the filter and focus causes below it: whatever else
	// narrowed the list, Esc is the press that widens it back, and naming a
	// filter would send the user to a key that leaves the section empty.
	if m.Focus.Zoom.Active() {
		return OutlinerEmptySection
	}
	if query.HasActiveCommonFilters(&m.Filter.CommonFilterState) {
		return outlinerEmptyFiltered
	}
	if m.Focus.Active() {
		return outlinerEmptyFocus
	}
	if m.PrimaryMode != chrome.ModeAll {
		return outlinerEmptyMode
	}
	return outlinerEmptyNoTasks
}
