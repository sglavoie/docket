package outliner

import (
	"maps"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// OutlinerToggleState captures section/fold state so it can survive outliner
// recreation within the same TUI session.
type OutlinerToggleState struct {
	CollapsedSections map[string]bool               // Section collapse state for All mode
	ExpandedNodes     map[taskref.TaskIdentity]bool // Project-qualified node fold state (empty = all expanded)
	CursorTaskID      string                        // Task ID at cursor position (session-only)
	FocusStack        []FocusEntry                  // Focus stack (session-only)
}

// SaveToggleState snapshots the current section/fold state.
func (m *OutlinerModel) SaveToggleState() *OutlinerToggleState {
	var cursorTaskID string
	if task := m.getCursorTask(); task != nil {
		cursorTaskID = task.ID
	}

	var focusCopy []FocusEntry
	if len(m.Focus.Stack) > 0 {
		focusCopy = make([]FocusEntry, len(m.Focus.Stack))
		copy(focusCopy, m.Focus.Stack)
	}

	return &OutlinerToggleState{
		CollapsedSections: copyBoolMap(m.Folds.CollapsedSections),
		ExpandedNodes:     copyBoolMap(m.Folds.ExpandedNodes),
		CursorTaskID:      cursorTaskID,
		FocusStack:        focusCopy,
	}
}

// RestoreToggleState applies a previously saved section/fold state.
func (m *OutlinerModel) RestoreToggleState(state *OutlinerToggleState) {
	if state == nil {
		return
	}

	m.Folds.setSections(copyBoolMap(state.CollapsedSections))
	m.Folds.setNodes(copyBoolMap(state.ExpandedNodes))
	if len(state.FocusStack) > 0 {
		m.Focus.setStack(slices.Clone(state.FocusStack))
	}
	m.RebuildRows()
	if state.CursorTaskID != "" {
		m.MoveCursorToTask(state.CursorTaskID)
	}
}

// copyBoolMap returns a shallow copy of a bool map.
func copyBoolMap[K comparable](src map[K]bool) map[K]bool {
	if src == nil {
		return nil
	}
	dst := make(map[K]bool, len(src))
	maps.Copy(dst, src)
	return dst
}

// persistActiveRouteState is intentionally run-local in the lean machine model.
func (m *OutlinerModel) PersistActiveRouteState() error {
	return nil
}

// captureOutlinerRouteState persists only the active mode. Filter facets are
// run-local view refinements and must never survive relaunch through legacy
// machine-route storage.
// applyOutlinerRouteState restores only the canonical mode through the
// mode selection seam. Legacy route filter fields remain migration input,
// but live filter facets belong to the current run and are deliberately ignored.
func (m *OutlinerModel) ApplyOutlinerRouteState(route config.OutlinerRouteState) {
	if m == nil {
		return
	}
	route = config.NormalizeOutlinerRouteState(route)

	m.SelectOutlinerMode(chrome.OutlinerModeID(route.Mode))
	m.Population.Invalidate()
	m.RebuildRows()
}

// OutlinerOpenOptions applies canonical mode, sort, or persisted route state
// without changing the typed TaskWorkspace route that owns scope.
type OutlinerOpenOptions struct {
	Sort    query.SortField
	SetSort bool
	Mode    chrome.OutlinerModeID
	SetMode bool

	RouteState    config.OutlinerRouteState
	SetRouteState bool
}

func (m *OutlinerModel) ApplyOpenOptions(opts OutlinerOpenOptions) {
	if opts.SetRouteState {
		m.ApplyOutlinerRouteState(opts.RouteState)
		return
	}

	mode := m.ActiveOutlinerModeEntry()
	if opts.SetMode {
		if selected, ok := chrome.OutlinerModeByID(opts.Mode); ok {
			mode = selected
			m.SelectOutlinerMode(selected.ID)
		}
	}
	if opts.SetSort && mode.SupportsManualSort() {
		sortState := query.SortState{Field: opts.Sort, Ascending: true}
		m.Sort = sortState
		m.TreeSort = sortState
		m.RebuildRows()
	}
}

// simplifiedInlineControlOwnsInput is the Outliner's half of the root grammar's
// dispatch guard in app_simplified_input.go: the shell asks the mounted surface
// whether an inline control already owns the keystroke before resolving it
// against the OperationRegistry.
func (m *OutlinerModel) SimplifiedInlineControlOwnsInput() bool {
	if m == nil {
		return false
	}
	if m.IsSearchMode() ||
		m.QuickEdit.IsActive() ||
		m.Confirm.IsActive() ||
		m.QueryReadout.Expanded ||
		// An open menu claims the key first even though most keys end up running
		// their root operation anyway: it has to close before the accelerator
		// fires, or the act would run under a dropdown still covering the rows it
		// changed. handleMenuBarKey closes and re-emits, so the grammar sees the
		// same key one update later with nothing owning input.
		m.MenuBar.Open {
		return true
	}
	return false
}

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
		Mode:               m.PrimaryMode,
		filter:             CopyFilterState(m.Filter),
		Sort:               m.Sort,
		treeSort:           m.TreeSort,
		ExpandedNodes:      copyBoolMap(m.Folds.ExpandedNodes),
		ExpandedChecklists: copyBoolMap(m.Folds.ExpandedChecklists),
		CollapsedSections:  copyBoolMap(m.Folds.CollapsedSections),
		focusStack:         slices.Clone(m.Focus.Stack),
		crossFocusStack:    slices.Clone(m.Focus.CrossStack),
		zoom:               m.Focus.Zoom,
		cursor:             m.captureModeCursor(),
	}
}

// restorePresentationSnapshot writes those axes back, deep-copying every map and
// slice out so the live model never aliases a frame that may be restored again.
// It rebuilds nothing and moves no cursor: each caller owns a different rebuild
// and a different cursor fallback, and computeOutlinerViewport stays the sole
// window authority.
func (m *OutlinerModel) restorePresentationSnapshot(snapshot outlinerSearchSnapshot) {
	m.PrimaryMode = snapshot.Mode
	m.Filter = CopyFilterState(snapshot.filter)
	m.Sort = snapshot.Sort
	m.TreeSort = snapshot.treeSort
	m.Folds.setNodes(copyBoolMap(snapshot.ExpandedNodes))
	m.Folds.setChecklists(copyBoolMap(snapshot.ExpandedChecklists))
	m.Folds.setSections(copyBoolMap(snapshot.CollapsedSections))
	m.Focus.Restore(
		slices.Clone(snapshot.focusStack),
		slices.Clone(snapshot.crossFocusStack),
		snapshot.zoom,
	)
	// A captured frame is still carrier-shaped, so the restored facets enter
	// through the one inverse lift and the narrowing predicate is rebuilt from
	// them rather than left holding the frame that was replaced.
	m.AdoptFilterCarriers()
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
type OutlinerViewSnapshot struct {
	Presentation outlinerSearchSnapshot

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
func (m *OutlinerModel) CaptureOutlinerViewSnapshot() OutlinerViewSnapshot {
	return OutlinerViewSnapshot{
		Presentation:  m.capturePresentationSnapshot(),
		projectFilter: m.ProjectFilter.Clone(),
		population:    m.Population.capture(),
		scope:         m.Scope.capture(),
		search:        m.Search.capture(),
		searchInput:   m.CaptureSearchInput(),
		lensQuery:     m.Lens.Query,
		peekMode:      m.PeekMode,
		selection:     m.CaptureSelection(),
		recency:       m.Recency.capture(),
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
func (m *OutlinerModel) restoreOutlinerViewSnapshot(snapshot OutlinerViewSnapshot) {
	m.restorePresentationSnapshot(snapshot.Presentation)
	m.restoreFilterCarriers(snapshot.projectFilter)
	m.restorePopulation(snapshot.population)
	m.Scope.restore(snapshot.scope)
	m.Search.restore(snapshot.search)
	m.RestoreSearchInput(snapshot.searchInput)
	m.SetLensQuery(snapshot.lensQuery)
	m.PeekMode = snapshot.peekMode
	m.RestoreSelection(snapshot.selection)
	m.Recency.restore(snapshot.recency)

	m.Population.Invalidate()
	m.RebuildRows()
	m.restoreSearchCursor(snapshot.Presentation.cursor)
}

// recencyState is Recent mode's frozen ordering, held against the projection it
// was taken in. Two members, one owner, because neither means anything without
// the other: the timestamps are only trustworthy as the answer some particular
// projection gave, and the projection fingerprint is only worth storing because
// timestamps are hanging off it. Writing one without the other is the bug this
// type exists to make unspellable — a stale fingerprint would serve a frozen
// order for a view that no longer exists, and a stale map would reorder the list
// the user is triaging out from under them.
//
// The freeze is what makes Recent usable: a mutation performed from a Recent row
// updates that task's UpdatedAt, and reading it live would move the row the user
// just acted on. It is deliberately narrower than a cache, since it is rebased
// rather than invalidated: a scope, filter, search, sort, or fold change gives a
// new fingerprint and re-reads the world, while an ordinary mutation leaves the
// list in place.
//
// The zero value is thawed, which is what makes the first pass in any projection
// freeze rather than serve nothing.
type recencyState struct {
	keys       map[taskref.TaskIdentity]time.Time
	Projection string
}

// rebase opens a fresh freeze whenever the projection on screen is no longer the
// one the keys were taken in, and leaves an existing freeze alone otherwise. A
// nil map counts as a different projection so a thawed state re-freezes without
// the caller having to notice it was thawed.
func (r *recencyState) rebase(projection string) {
	if r == nil {
		return
	}
	if r.Projection == projection && r.keys != nil {
		return
	}
	r.keys = make(map[taskref.TaskIdentity]time.Time)
	r.Projection = projection
}

// freeze records a task's activity the first time this projection sees it. First
// write wins, which is the whole of the guarantee: a later pass over the same
// projection cannot move a row by re-reading a timestamp the mutation just moved.
func (r *recencyState) freeze(identity taskref.TaskIdentity, at time.Time) {
	if r == nil || r.keys == nil {
		return
	}
	if _, frozen := r.keys[identity]; frozen {
		return
	}
	r.keys[identity] = at
}

// frozenKeys hands the whole freeze to the ordering pass that sorts by it.
func (r *recencyState) frozenKeys() map[taskref.TaskIdentity]time.Time {
	if r == nil {
		return nil
	}
	return r.keys
}

// frozenAt answers for one row, so the chip a render draws agrees with the
// bucket the ordering pass put that row in. A task the freeze does not name is
// the caller's to fall back on.
func (r *recencyState) frozenAt(identity taskref.TaskIdentity) (time.Time, bool) {
	if r == nil {
		return time.Time{}, false
	}
	frozen, ok := r.keys[identity]
	return frozen, ok
}

// thaw drops the freeze and the projection together, so the next pass re-reads
// activity from the world. Entering Recent over an already-open Recent is the
// manual refresh, and this is what makes it one.
func (r *recencyState) thaw() {
	if r == nil {
		return
	}
	r.keys = nil
	r.Projection = ""
}

// recencySnapshot is the freeze as a restoration point. It carries both members
// for the same reason the state holds both: a frame that restored the timestamps
// without the fingerprint they were taken under would serve them to the first
// projection that asked, whatever it was.
type recencySnapshot struct {
	keys       map[taskref.TaskIdentity]time.Time
	Projection string
}

// capture copies the freeze out, so a retained frame can never alias the map a
// later pass goes on to extend.
func (r *recencyState) capture() recencySnapshot {
	if r == nil {
		return recencySnapshot{}
	}
	return recencySnapshot{keys: maps.Clone(r.keys), Projection: r.Projection}
}

// restore writes a captured freeze back. A frame taken outside Recent carries no
// keys, and restoring that is a thaw: the view it returns to had none either.
func (r *recencyState) restore(snapshot recencySnapshot) {
	if r == nil {
		return
	}
	r.keys = maps.Clone(snapshot.keys)
	r.Projection = snapshot.Projection
}
