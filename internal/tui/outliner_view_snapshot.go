package tui

import (
	"slices"

	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
)

// A temporary Outliner narrowing owes the user the exact view it replaced. Two
// of them exist — the slash-search projection and the active lens — and they
// differ only in how wide the replacement reaches: search leaves scope and the
// loaded population alone, while a lens may change both. This file owns the one
// capture/restore facility both read, so neither can grow its own private idea
// of what "the view I was in" means.

// capturePresentationSnapshot reads the projection axes every narrowing
// replaces: the mode, its facets and ordering, the folds, both focus stacks, the
// section zoom, and the cursor. Scope is deliberately absent — it names the
// candidate population rather than the projection over it, so the callers that
// own scope capture it beside this rather than inside it.
func (m *OutlinerModel) capturePresentationSnapshot() outlinerSearchSnapshot {
	return outlinerSearchSnapshot{
		mode:               m.primaryMode,
		filter:             copyFilterState(m.filter),
		sort:               m.sort,
		treeSort:           m.treeSort,
		expandedNodes:      copyBoolMap(m.folds.expandedNodes),
		expandedChecklists: copyBoolMap(m.folds.expandedChecklists),
		collapsedSections:  copyBoolMap(m.folds.collapsedSections),
		focusStack:         slices.Clone(m.focus.stack),
		crossFocusStack:    slices.Clone(m.focus.crossStack),
		zoom:               m.focus.zoom,
		cursor:             m.captureModeCursor(),
	}
}

// restorePresentationSnapshot writes those axes back, deep-copying every map and
// slice out so the live model never aliases a frame that may be restored again.
// It rebuilds nothing and moves no cursor: each caller owns a different rebuild
// and a different cursor fallback, and computeOutlinerViewport stays the sole
// window authority.
func (m *OutlinerModel) restorePresentationSnapshot(snapshot outlinerSearchSnapshot) {
	m.primaryMode = snapshot.mode
	m.filter = copyFilterState(snapshot.filter)
	m.sort = snapshot.sort
	m.treeSort = snapshot.treeSort
	m.folds.setNodes(copyBoolMap(snapshot.expandedNodes))
	m.folds.setChecklists(copyBoolMap(snapshot.expandedChecklists))
	m.folds.setSections(copyBoolMap(snapshot.collapsedSections))
	m.focus.restore(
		slices.Clone(snapshot.focusStack),
		slices.Clone(snapshot.crossFocusStack),
		snapshot.zoom,
	)
	// A captured frame is still carrier-shaped, so the restored facets enter
	// through the one inverse lift and the narrowing predicate is rebuilt from
	// them rather than left holding the frame that was replaced.
	m.adoptFilterCarriers()
}

// outlinerViewSnapshot is the complete restoration point one lens application
// captures. It is the presentation snapshot plus everything a lens may reach
// that ordinary search never does: the project population and the facets that
// selected it, the scope frame the user is standing in, the ordinary search
// projection a lens replaces outright, the subordinate tools (Peek, selection),
// and Recent's frozen ordering keys.
//
// It is one field per cluster, and each of those fields is the cluster's own
// answer to what a frame owes it — populationState, scopeState, searchState,
// MultiSelectState and recencyState each name their own snapshot type, and each
// decides for itself which of its members describes a view worth restoring and
// which describes work in flight that must not be restarted. That is what keeps
// this type a ledger rather than a second, competing model: nothing here knows
// the shape of any cluster it names.
//
// Two members are single values rather than groups because their clusters
// contribute one apiece. lensQuery is captured for symmetry — a baseline taken
// outside a lens holds the empty query, and restoring it is what guarantees no
// narrowing survives. peekMode is Peek entire, and it sits beside the selection
// it is mutually exclusive with rather than inside it, because Peek is the
// Outliner's and the selection is every list surface's.
type outlinerViewSnapshot struct {
	presentation outlinerSearchSnapshot

	// Population: the facets that chose it and the snapshot they chose.
	projectFilter query.ProjectFilterState
	population    populationSnapshot

	// Scope: the single project focus, the base frame behind it, and the parked
	// exception that may be why the focus exists.
	scope scopeSnapshot

	// Ordinary search. A lens replaces the projection outright, so the frame
	// carries the snapshot search itself was holding along with the input that
	// produced it; restoring hands search back its own restoration point.
	search      searchSnapshot
	searchInput listsurface.SearchInputSnapshot

	lensQuery string

	// Subordinate tools.
	peekMode  bool
	selection listsurface.SelectionSnapshot

	// Recent's frozen ordering, so returning to a Recent baseline reads the list
	// the user was triaging rather than a reshuffled one.
	recency recencySnapshot
}

// captureOutlinerViewSnapshot takes the full restoration point, one cluster at a
// time. Every line here is a whole cluster asked what it holds, which is the
// point: a member added to any of those sub-states is carried by the frame
// without this function learning about it, and a member that must not be carried
// is refused where it lives rather than by an omission here nobody would notice.
func (m *OutlinerModel) captureOutlinerViewSnapshot() outlinerViewSnapshot {
	return outlinerViewSnapshot{
		presentation:  m.capturePresentationSnapshot(),
		projectFilter: m.projectFilter.Clone(),
		population:    m.population.capture(),
		scope:         m.scope.capture(),
		search:        m.search.capture(),
		searchInput:   m.CaptureSearchInput(),
		lensQuery:     m.lens.query,
		peekMode:      m.peekMode,
		selection:     m.CaptureSelection(),
		recency:       m.recency.capture(),
	}
}

// restoreOutlinerViewSnapshot puts the captured view back and rebuilds once. It
// is capture read backwards, cluster by cluster, and it holds no deep copy, no
// nil repair and no paired write of its own: each of those is an invariant of the
// cluster it belongs to and is stated there. What is left here is the rebuild,
// which is the one thing no cluster can own — the population guard is
// invalidated, the rows are built once over every axis just written, and the
// cursor lands through restoreSearchCursor, which seats the exact surviving
// identity and otherwise falls back to the nearest surviving task row, the same
// fallback ordinary search already uses when data moved underneath it.
func (m *OutlinerModel) restoreOutlinerViewSnapshot(snapshot outlinerViewSnapshot) {
	m.restorePresentationSnapshot(snapshot.presentation)
	m.restoreFilterCarriers(snapshot.projectFilter)
	m.restorePopulation(snapshot.population)
	m.scope.restore(snapshot.scope)
	m.search.restore(snapshot.search)
	m.RestoreSearchInput(snapshot.searchInput)
	m.setLensQuery(snapshot.lensQuery)
	m.peekMode = snapshot.peekMode
	m.RestoreSelection(snapshot.selection)
	m.recency.restore(snapshot.recency)

	m.population.invalidate()
	m.rebuildRows()
	m.restoreSearchCursor(snapshot.presentation.cursor)
}
