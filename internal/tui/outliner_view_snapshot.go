package tui

import (
	"maps"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
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
		expandedNodes:      copyBoolMap(m.expandedNodes),
		expandedChecklists: copyBoolMap(m.expandedChecklists),
		collapsedSections:  copyBoolMap(m.collapsedSections),
		focusStack:         slices.Clone(m.focusStack),
		crossFocusStack:    slices.Clone(m.crossFocusStack),
		zoom:               m.zoom,
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
	m.expandedNodes = copyBoolMap(snapshot.expandedNodes)
	m.expandedChecklists = copyBoolMap(snapshot.expandedChecklists)
	m.collapsedSections = copyBoolMap(snapshot.collapsedSections)
	m.focusStack = slices.Clone(snapshot.focusStack)
	m.crossFocusStack = slices.Clone(snapshot.crossFocusStack)
	m.zoom = snapshot.zoom
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
// The loaded projects are retained by value so restoration is a pure in-memory
// step: returning from a lens that widened the population must not depend on a
// second disk read that could fail, be slow, or land after the user moved on.
type outlinerViewSnapshot struct {
	presentation outlinerSearchSnapshot

	// Population: the facets that chose it and the snapshot they chose.
	projectFilter        query.ProjectFilterState
	loadedProjects       []*models.Project
	crossSnapshotReady   bool
	parkedFocusProjectID string

	// Scope: the single project focus and the one base frame behind it.
	scopedProjectID string
	scopeStack      []outlinerScopeFrame

	// Ordinary search. A lens replaces the projection outright, so the frame
	// carries the snapshot search itself was holding along with its query and
	// editing state; restoring hands search back its own restoration point.
	searchProjection  *outlinerSearchSnapshot
	rankedList        bool
	searchQuery       string
	searchInputValue  string
	searchInputActive bool

	// lensQuery is captured for symmetry: a baseline taken outside a lens holds
	// the empty query, and restoring it is what guarantees no narrowing survives.
	lensQuery string

	// Subordinate tools.
	peekMode        bool
	multiSelectMode bool
	selected        map[taskref.TaskIdentity]bool

	// Recent's frozen ordering keys, so returning to a Recent baseline reads the
	// list the user was triaging rather than a reshuffled one.
	recencyKeys       map[taskref.TaskIdentity]time.Time
	recencyProjection string
}

// captureOutlinerViewSnapshot takes the full restoration point.
func (m *OutlinerModel) captureOutlinerViewSnapshot() outlinerViewSnapshot {
	snapshot := outlinerViewSnapshot{
		presentation:         m.capturePresentationSnapshot(),
		projectFilter:        m.projectFilter.Clone(),
		loadedProjects:       slices.Clone(m.loadedProjects),
		crossSnapshotReady:   m.crossSnapshotReady,
		parkedFocusProjectID: m.parkedFocusProjectID,
		scopedProjectID:      m.scopedProjectID,
		scopeStack:           slices.Clone(m.scopeStack),
		searchProjection:     m.searchProjection,
		rankedList:           m.rankedList,
		searchQuery:          m.SearchQuery(),
		searchInputValue:     m.SearchInputValue(),
		searchInputActive:    m.IsSearchMode(),
		lensQuery:            m.lensQuery,
		peekMode:             m.peekMode,
		multiSelectMode:      m.MultiSelectMode,
		selected:             copyBoolMap(m.Selected),
		recencyProjection:    m.recencyProjection,
	}
	if m.recencyKeys != nil {
		snapshot.recencyKeys = maps.Clone(m.recencyKeys)
	}
	return snapshot
}

// restoreOutlinerViewSnapshot puts the captured view back and rebuilds once. The
// cursor lands through restoreSearchCursor, which seats the exact surviving
// identity and otherwise falls back to the nearest surviving task row — the same
// fallback ordinary search already uses when data moved underneath it.
func (m *OutlinerModel) restoreOutlinerViewSnapshot(snapshot outlinerViewSnapshot) {
	m.restorePresentationSnapshot(snapshot.presentation)
	m.projectFilter = snapshot.projectFilter.Clone()
	m.adoptFilterCarriers()
	m.setLoadedProjects(slices.Clone(snapshot.loadedProjects))
	m.crossSnapshotReady = snapshot.crossSnapshotReady
	m.parkedFocusProjectID = snapshot.parkedFocusProjectID
	m.scopedProjectID = snapshot.scopedProjectID
	m.scopeStack = slices.Clone(snapshot.scopeStack)

	m.setLensQuery(snapshot.lensQuery)
	m.searchProjection = snapshot.searchProjection
	m.rankedList = snapshot.rankedList
	m.resetSearchCompilation()
	m.BaseSearchableView.SetSearchQuery(snapshot.searchQuery)
	m.SetSearchInputValue(snapshot.searchInputValue)
	m.SetSearchActive(snapshot.searchInputActive)

	m.peekMode = snapshot.peekMode
	m.MultiSelectMode = snapshot.multiSelectMode
	m.Selected = copyBoolMap(snapshot.selected)
	if m.Selected == nil {
		m.Selected = make(map[taskref.TaskIdentity]bool)
	}
	m.FreezeRangeSelection()

	m.recencyKeys = nil
	if snapshot.recencyKeys != nil {
		m.recencyKeys = maps.Clone(snapshot.recencyKeys)
	}
	m.recencyProjection = snapshot.recencyProjection

	m.crossForceRebuild = true
	m.rebuildRows()
	m.restoreSearchCursor(snapshot.presentation.cursor)
}
