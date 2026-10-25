package outliner

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/fuzzy"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/util"
)

// hasSearchProjection reports that a slash-search frame is open, which is the
// same thing as saying the view holds a query. It says nothing about how that
// query is presented: by default the query narrows the mode's own lanes, and
// only showsRankedList reports the flat presentation.
func (m *OutlinerModel) HasSearchProjection() bool {
	return m != nil && m.Search.Active()
}

// showsRankedList reports that the held query is being presented as one flat
// relevance-ordered list. It is the single test every consumer that cannot
// survive a bypassed mode must ask: the ranked list has no lanes, no grouping,
// no manual order, and no section the zoom or the folds gate, so the operations
// that reshape those axes refuse here and nowhere else. A query alone never
// freezes an axis.
func (m *OutlinerModel) ShowsRankedList() bool {
	return m != nil && m.Search.showsRanked()
}

// toggleRankedList switches the held query between narrowing the mode and the
// flat ranked list. Entering suspends the section zoom, which gates emitters the
// flat list does not run; leaving restores nothing else, because every other
// axis stayed live under both presentations.
func (m *OutlinerModel) ToggleRankedList() bool {
	if !m.HasSearchProjection() {
		return false
	}
	if m.Search.toggleRanked() {
		m.clearSectionZoom()
	}
	m.Population.Invalidate()
	RebuildOutlinerPreservingCursor(m)
	return true
}

// BeginSearchProjection opens the frame with the query narrowing the mode's own
// lanes. It is the programmatic seam — SetSearchQuery and the navigation and
// tests that reach through it — and it keeps that presentation deliberately: a
// caller installing a query without a keystroke is asking the mode it left
// standing to answer it, not asking for the flat list instead of it.
func (m *OutlinerModel) BeginSearchProjection() {
	m.beginSearchProjection(false)
}

// beginRankedSearchProjection is the slash key's own open. A typed query is a
// question about the workspace rather than about the lane the cursor happened to
// be standing in, so the user's entry path starts the flat relevance-ordered
// list across modes and the ranked toggle is what narrows it back. Entering
// ranked suspends the section zoom for the one reason ToggleRankedList does —
// the flat list runs none of the emitters that zoom gates — and the frame
// captured in the same act is what hands the zoom back when the query is
// dropped.
func (m *OutlinerModel) beginRankedSearchProjection() {
	m.beginSearchProjection(true)
}

func (m *OutlinerModel) beginSearchProjection(ranked bool) {
	if m == nil || m.Search.Active() {
		return
	}
	m.closeMenuBar()
	m.Search.begin(m.capturePresentationSnapshot(), ranked)
	if ranked {
		m.clearSectionZoom()
	}
	m.Population.Invalidate()
	m.RebuildRows()
}

// ActivateSearch starts or resumes the one temporary search projection, ranked.
// A second slash edits the existing query without replacing its restoration
// snapshot and without restating the presentation: the toggle is a deliberate
// act, so reopening the input over a query already narrowed back into its mode
// must not silently undo it. Under an active lens the same key suspends that
// lens rather than refusing or editing it: search asks one question of the
// workspace, the answer is worth the same inside a lens as outside it, and the
// suspension is what makes the visit temporary.
func (m *OutlinerModel) ActivateSearch() tea.Cmd {
	// Taken before the open, because the checkpoint is the view the search is
	// about to replace. A second slash finds no lens to take and so records
	// nothing, leaving the first slash's suspension exactly as it was.
	suspended := m.suspendActiveLensForSearch()
	m.beginRankedSearchProjection()
	m.Search.suspend(suspended)
	// Every open starts a fresh walk: the ring survives the input closing, the
	// position in it does not, so the first Up always names the newest query.
	m.Search.history.resetRecall()
	return m.BaseSearchableView.ActivateSearch()
}

// SetSearchQuery retains the programmatic seam used by tests and navigation. It
// opens the same one temporary projection slash does and answers the same
// compiled query; what it does not do is choose the flat list, because only a
// keystroke asks for that. Called into an open frame it restates nothing, so
// typing into the input slash opened keeps the presentation slash declared.
func (m *OutlinerModel) SetSearchQuery(query string) {
	if strings.TrimSpace(query) != "" {
		m.BeginSearchProjection()
	}
	m.BaseSearchableView.SetSearchQuery(query)
	if m.HasSearchProjection() {
		m.Population.Invalidate()
		RebuildOutlinerPreservingCursor(m)
	}
}

// suspendActiveLensForSearch puts the live lens down for the duration of one
// temporary search and hands back the frame that reinstates it. The checkpoint
// is the whole return: mode, facets, sort, folds, focus, zoom, cursor,
// selection, Peek, the frozen Recent ordering, and the lens's own narrowing
// query. Scope is deliberately not disturbed — search never leaves the scope it
// was pressed in, so the lens gets back the one it owned by never losing it.
//
// forgetActiveLensFrame is the seam for the reason the relationship hop gives:
// it drops the frame, the application still reading a population for it, and the
// narrowing it installed, while leaving the machine-local resume pointer exactly
// as it was. A temporary search is not a lens being put down, so the next launch
// still opens the lens whatever the search did.
func (m *OutlinerModel) suspendActiveLensForSearch() *lensSuspension {
	if m == nil || !m.HasActiveLens() || m.HasSearchProjection() {
		return nil
	}
	checkpoint := m.CaptureOutlinerPresentationCheckpoint()
	// Read while the lens still owns the view: the marker is a comparison the
	// suspended projection cannot make for itself.
	modified := m.LensModified()
	frame := m.Lens.takeActive()
	m.forgetActiveLensFrame()
	return &lensSuspension{presentation: checkpoint, lens: frame, modified: modified}
}

// HasSuspendedLens reports that a temporary search is standing in front of a
// lens one Esc puts back. It is not HasActiveLens: no lens owns this projection,
// which is why the mode, the query, and every operation reading them answer for
// the search instead.
func (m *OutlinerModel) HasSuspendedLens() bool {
	return m != nil && m.Search.suspendedLens != nil
}

// SearchExitAction names what Esc does to the held query, in the imperative the
// Esc cascade carries and the footer spells after the key. Over a suspended lens
// the key clears nothing: it puts a projection back, and a footer promising
// "clear search" would send the user looking for the lens it silently restored.
// It is the search step's own Action, so the cascade and every hint below read
// one answer.
func (m *OutlinerModel) SearchExitAction() string {
	if m.HasSuspendedLens() {
		return "restore lens"
	}
	return "clear search"
}

// SearchExitHint is that action as a complete footer entry, and
// searchExitClause says the same thing inside a sentence. The live-input hint,
// the empty-state guidance, and the empty-state message all read these, so one
// key cannot be advertised three ways.
func (m *OutlinerModel) SearchExitHint() string {
	return "Esc " + m.SearchExitAction()
}

func (m *OutlinerModel) searchExitClause() string {
	if m.HasSuspendedLens() {
		return "Esc to restore the lens"
	}
	return "Esc to clear"
}

// ClearSearch exits the temporary projection and restores the exact ordinary
// Outliner state. If the original task vanished while search operations were
// mutating data, focus falls back to the nearest surviving task row.
func (m *OutlinerModel) ClearSearch() {
	if m == nil || !m.Search.Active() {
		m.Search.Autocomplete.clear()
		m.BaseSearchableView.ClearSearch()
		return
	}
	m.closeMenuBar()
	// A suspended lens is restored instead of the bare presentation snapshot, and
	// it is the same restore the relationship trail performs: the lens frame
	// first, because the checkpoint carries the narrowing query and installing it
	// second would write an empty one over the lens just put back. The checkpoint
	// subsumes the snapshot begin took — it holds that snapshot — so this path
	// owes neither a second restore nor a second cursor seat.
	if suspended := m.Search.takeSuspendedLens(); suspended != nil {
		m.BaseSearchableView.ClearSearch()
		m.Lens.setActive(suspended.lens)
		m.RestoreOutlinerPresentationCheckpoint(suspended.presentation)
		return
	}
	snapshot := m.Search.take()
	m.resetSearchCompilation()
	m.BaseSearchableView.ClearSearch()
	m.restorePresentationSnapshot(*snapshot)
	m.Population.Invalidate()
	m.RebuildRows()
	m.restoreSearchCursor(snapshot.cursor)
}

// exitSearchOntoCursorTask dismisses the flat ranked list and lands the ordinary
// projection on the result the cursor named, so a jump reached through it is not
// a dead end. The restored projection can hide the result behind the mode it was
// searched out of, so a hidden result forces All once before it reports that a
// facet still excludes it. A query that is merely narrowing has nothing to exit:
// its rows are already the mode's own, so the caller keeps the query.
func (m *OutlinerModel) ExitSearchOntoCursorTask() (string, bool) {
	if !m.ShowsRankedList() {
		return "", true
	}
	identity := m.CurrentTaskIdentity()
	m.ClearSearch()
	if !identity.Valid() || m.landOnSearchResult(identity) {
		return "", true
	}
	m.SelectOutlinerMode(chrome.ModeAll)
	if m.landOnSearchResult(identity) {
		return "", true
	}
	return "Task is hidden by the active facets", false
}

func (m *OutlinerModel) landOnSearchResult(identity taskref.TaskIdentity) bool {
	task, project := m.TaskAndProjectForIdentity(identity)
	if task == nil {
		return false
	}
	if m.Project != nil {
		m.NavigateToTask(task)
	} else {
		m.NavigateToTaskInProject(task, project)
	}
	if !m.MoveCursorToIdentity(identity) {
		return false
	}
	// The navigation kernels report a hidden target themselves, and that sentence
	// would outlive a landing the forced mode went on to make.
	m.ClearStatus()
	return true
}

func (m *OutlinerModel) restoreSearchCursor(anchor modeCursorAnchor) {
	m.ViewScrollOffset = anchor.scrollOffset
	if anchor.hasTask {
		for index, row := range m.Rows {
			if !row.IsTaskRow() || row.Task == nil || row.Task.ID != anchor.identity.TaskID {
				continue
			}
			if m.Project == nil && row.ProjectID != anchor.identity.ProjectID {
				continue
			}
			m.Cursor = index
			return
		}
		m.moveCursorToNearestTask(anchor.prevCursor)
		return
	}
	m.Cursor = anchor.prevCursor
	m.ClampCursor()
}

// clearSearchProjectionWithoutRestore is reserved for intentional navigation
// resets that replace the current projection rather than dismissing search.
func (m *OutlinerModel) clearSearchProjectionWithoutRestore() {
	if m == nil {
		return
	}
	m.Search.discard()
	m.resetSearchCompilation()
	m.BaseSearchableView.ClearSearch()
}

// resetSearchCompilation is the model's name for dropping everything derived
// from the held query. Both clear paths route here, so neither can leave a pane
// open over no search; the restoration path reaches the same act through
// searchState.restore, which owes it for the query it is replacing.
func (m *OutlinerModel) resetSearchCompilation() {
	m.Search.resetCompilation()
}

// searchTextQuery is the free-text remainder every consumer of the query must
// read: scoring, smart case, highlighting, and the work-mode predicate. Reading
// the raw query instead would score and highlight the sigils as prose.
func (m *OutlinerModel) searchTextQuery() string {
	if m == nil {
		return ""
	}
	if m.HasSearchProjection() {
		return m.Search.remainder
	}
	return strings.TrimSpace(m.SearchQuery())
}

// searchCaseSensitive reports the case mode the visible result set was actually
// produced under, so highlighting and predicates never disagree with the rows.
func (m *OutlinerModel) searchCaseSensitive() bool {
	if m == nil || m.Search.caseRelaxed {
		return false
	}
	return fuzzy.CaseSensitiveQuery(m.searchTextQuery())
}

// Explain only removals proven to reveal tasks. Each pass removes one ordinary
// facet and retains every other facet, scope, focus, and typed query clause.
func (m *OutlinerModel) removableFilterSearchHint(projects []*models.Project) string {
	rows := m.rowPredicateQuery()
	names := map[models.QueryField]string{
		models.QueryFieldPriority: "priority", models.QueryFieldTag: "tag",
		models.QueryFieldContext: "context", models.QueryFieldProject: "project",
		models.QueryFieldDue: "due date", models.QueryFieldState: "state",
		models.QueryFieldPinned: "pin", models.QueryFieldReview: "review",
		models.QueryFieldStale: "staleness", models.QueryFieldDeferred: "deferral",
	}
	var hints []string
	seen := make(map[models.QueryField]bool)
	now := time.Now()
	for _, clause := range rows.Clauses {
		field := clause.Field
		name := names[field]
		if name == "" || seen[field] || (field == models.QueryFieldState && query.CompletionAgeOwnsTerminalStates(&m.Filter.CommonFilterState)) {
			continue
		}
		seen[field] = true
		cleared := query.WithoutFields(m.FilterQuery, field)
		text, gate := query.CompileSearchQuery(m.SearchQuery(), projects, outlinerPlanningMembership{outliner: m}, cleared)
		if gate.Invalid() {
			continue
		}
		predicate := m.facetPredicateForRowsAt(query.WithoutFields(rows, field), now)
		candidates := m.searchCandidateBoundary(projects, predicate)
		matcher := fuzzy.SmartCase(text)
		matches := searchProjectionMatchesWithinContext(m.searchContext(), projects, text, candidates, matcher, gate, m.searchScoreCache())
		if len(matches) == 0 && matcher.CaseSensitive() {
			matches = searchProjectionMatchesWithinContext(m.searchContext(), projects, text, candidates, fuzzy.NewMatcher(false), gate, m.searchScoreCache())
		}
		if len(matches) == 0 {
			continue
		}
		count := util.CountPlural(len(matches), "match", "matches")
		if query.CommonFilterFacetCount(&m.Filter.CommonFilterState) == 1 {
			return fmt.Sprintf("%s hidden by your %s filter.", count, name)
		}
		hints = append(hints, fmt.Sprintf("Clearing your %s filter reveals %s.", name, count))
	}
	return strings.Join(hints, " ")
}

// maxEliminationClauses bounds the questions an empty result asks. Each clause
// costs at most two passes of the existing gate over the loaded population —
// once alone when it names a stored value, once left out — so a long query
// cannot turn one keystroke into an unbounded walk. A query past the bound
// keeps the plain sentence.
const maxEliminationClauses = 8

// searchEliminationHint explains a ranked list that found nothing by naming the
// sigil clause responsible. A positive tag, context, or project clause that no
// task in the searched population carries at all is named first, since that is
// a value to correct rather than a combination to loosen. Otherwise it reruns
// the gate once per clause with that clause left out and names the one whose
// removal restores the most matches, the first typed winning a tie. It answers
// nothing when no single clause is responsible, when removing the clause would
// leave no query at all, or when active filters own the empty state, since that
// sentence already names its own exits.
func (m *OutlinerModel) searchEliminationHint(projects []*models.Project, candidates map[string][]*models.Task) string {
	if m == nil || !m.ShowsRankedList() || query.HasActiveCommonFilters(&m.Filter.CommonFilterState) {
		return ""
	}
	text := m.SearchQuery()
	remainder, clauses := query.SearchClauses(text)
	if len(clauses) == 0 || len(clauses) > maxEliminationClauses {
		return ""
	}
	plan := outlinerPlanningMembership{outliner: m}

	for _, clause := range clauses {
		if !clause.Vocabulary() {
			continue
		}
		gate := query.CompileSearchClauseAlone(text, projects, plan, clause)
		if gate.Invalid() || len(searchProjectionMatchesWithinContext(m.searchContext(), projects, "", candidates, fuzzy.NewMatcher(false), gate, m.searchScoreCache())) > 0 {
			continue
		}
		if clause.NamesProject() {
			return fmt.Sprintf("No project matches %s.", clause.Spelling)
		}
		return fmt.Sprintf("No task carries %s.", clause.Spelling)
	}

	// Removing the only clause of a sigil-only query leaves an empty query, which
	// matches everything and explains nothing.
	if remainder == "" && len(clauses) == 1 {
		return ""
	}
	best, bestCount := query.SearchClause{}, 0
	for _, clause := range clauses {
		rest, gate := query.CompileSearchQueryWithout(text, projects, plan, m.FilterQuery, clause)
		if gate.Invalid() {
			continue
		}
		matcher := fuzzy.SmartCase(rest)
		matches := searchProjectionMatchesWithinContext(m.searchContext(), projects, rest, candidates, matcher, gate, m.searchScoreCache())
		if len(matches) == 0 && matcher.CaseSensitive() {
			matches = searchProjectionMatchesWithinContext(m.searchContext(), projects, rest, candidates, fuzzy.NewMatcher(false), gate, m.searchScoreCache())
		}
		if len(matches) > bestCount {
			best, bestCount = clause, len(matches)
		}
	}
	if bestCount == 0 {
		return ""
	}
	count := util.Count(bestCount, "task")
	verb, object := "matches", "it"
	if bestCount != 1 {
		verb, object = "match", "them"
	}
	if len(clauses) == 1 {
		return fmt.Sprintf("%s %s %q; %s removes %s.", count, verb, remainder, best.Spelling, object)
	}
	return fmt.Sprintf("%s %s without %s.", count, verb, best.Spelling)
}
