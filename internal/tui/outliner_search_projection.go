package tui

import (
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// outlinerSearchSnapshot is the navigation frame a slash search was typed in.
// Scope is deliberately not captured, because scope is a navigation frame rather
// than a clause of the query: a search runs inside the frame it was typed in, a
// lens moves the frame, and the frames stack. Changing scope under a held query
// therefore re-answers that query in the new frame instead of editing it, while
// mode, facets, folds, zoom, ordering, and viewport are the frame's own axes and
// are restored when the query is dropped.
type outlinerSearchSnapshot struct {
	mode               chrome.OutlinerModeID
	filter             query.FilterState
	sort               query.SortState
	treeSort           query.SortState
	expandedNodes      map[taskref.TaskIdentity]bool
	expandedChecklists map[taskref.TaskIdentity]bool
	collapsedSections  map[string]bool
	focusStack         []FocusEntry
	crossFocusStack    []CrossProjectFocusEntry
	zoom               sectionZoom
	cursor             modeCursorAnchor
}

// outlinerMatchSource names the field that produced a search hit. It is
// transient presentation attribution carried on the projected row only: no
// persisted task or project field records it, and an ordinary (non-search) row
// keeps the zero value, which means "no cue".
type outlinerMatchSource int

const (
	matchSourceNone outlinerMatchSource = iota
	matchSourceTitle
	matchSourceDescription
	matchSourceChecklist
	matchSourceComment
	matchSourceTag
	matchSourceContext
	matchSourceAttachment
	matchSourceProject
	matchSourceDetails
	// matchSourceNote attributes a row to the notes tier rather than to a field
	// of a task. It is the one source whose row carries no task at all.
	matchSourceNote
)

// The deterministic free-text tiers. Rank is the primary sort key, so a lower
// tier can never be outranked by a higher one regardless of fuzzy score, and
// searchFieldRankDetails is evaluated only after every single-field tier fails.
const (
	searchFieldRankTitle = iota
	searchFieldRankDescription
	searchFieldRankChecklist
	searchFieldRankComment
	// Tags and context share one rank; the source retains which axis won.
	searchFieldRankTagContext
	searchFieldRankAttachment
	searchFieldRankProject
	searchFieldRankDetails
	// Notes are the last tier: a task the query names is what the Outliner is
	// for, so no note may outrank one however strongly it scored. It is also the
	// only tier whose rows carry no task.
	searchFieldRankNote
)

type outlinerSearchMatch struct {
	task        *models.Task
	project     *models.Project
	fieldRank   int
	fuzzyScore  int
	source      outlinerMatchSource
	sourceIndex int
}

// hasSearchProjection reports that a slash-search frame is open, which is the
// same thing as saying the view holds a query. It says nothing about how that
// query is presented: by default the query narrows the mode's own lanes, and
// only showsRankedList reports the flat presentation.
func (m *OutlinerModel) hasSearchProjection() bool {
	return m != nil && m.search.active()
}

// showsRankedList reports that the held query is being presented as one flat
// relevance-ordered list. It is the single test every consumer that cannot
// survive a bypassed mode must ask: the ranked list has no lanes, no grouping,
// no manual order, and no section the zoom or the folds gate, so the operations
// that reshape those axes refuse here and nowhere else. A query alone never
// freezes an axis.
func (m *OutlinerModel) showsRankedList() bool {
	return m != nil && m.search.showsRanked()
}

// toggleRankedList switches the held query between narrowing the mode and the
// flat ranked list. Entering suspends the section zoom, which gates emitters the
// flat list does not run; leaving restores nothing else, because every other
// axis stayed live under both presentations.
func (m *OutlinerModel) toggleRankedList() bool {
	if !m.hasSearchProjection() {
		return false
	}
	if m.search.toggleRanked() {
		m.clearSectionZoom()
	}
	m.population.invalidate()
	rebuildOutlinerPreservingCursor(m)
	return true
}

func copyFilterState(src query.FilterState) query.FilterState {
	return query.FilterState{CommonFilterState: *query.CopyCommonFilterState(&src.CommonFilterState)}
}

func (m *OutlinerModel) beginSearchProjection() {
	if m == nil || m.search.active() {
		return
	}
	m.closeMenuBar()
	m.search.begin(m.capturePresentationSnapshot())
	m.population.invalidate()
	m.rebuildRows()
}

// ActivateSearch starts or resumes the one temporary search projection. A
// second slash edits the existing query without replacing its restoration
// snapshot. Under an active lens the same key edits that lens's narrowing query
// instead: the lens already owns the projection, and replacing it with a
// scope-wide result set is the one thing the key must not do there.
func (m *OutlinerModel) ActivateSearch() tea.Cmd {
	if m.hasActiveLens() && !m.hasSearchProjection() {
		return m.beginLensQueryEdit()
	}
	m.beginSearchProjection()
	// Every open starts a fresh walk: the ring survives the input closing, the
	// position in it does not, so the first Up always names the newest query.
	m.search.history.resetRecall()
	return m.BaseSearchableView.ActivateSearch()
}

// SetSearchQuery retains the programmatic seam used by tests and navigation,
// but gives it the same scope-wide projection semantics as slash search.
func (m *OutlinerModel) SetSearchQuery(query string) {
	if strings.TrimSpace(query) != "" {
		m.beginSearchProjection()
	}
	m.BaseSearchableView.SetSearchQuery(query)
	if m.hasSearchProjection() {
		m.population.invalidate()
		rebuildOutlinerPreservingCursor(m)
	}
}

// ClearSearch exits the temporary projection and restores the exact ordinary
// Outliner state. If the original task vanished while search operations were
// mutating data, focus falls back to the nearest surviving task row.
func (m *OutlinerModel) ClearSearch() {
	if m == nil || !m.search.active() {
		m.search.autocomplete.clear()
		m.BaseSearchableView.ClearSearch()
		return
	}
	m.closeMenuBar()
	snapshot := m.search.take()
	m.resetSearchCompilation()
	m.BaseSearchableView.ClearSearch()
	m.restorePresentationSnapshot(*snapshot)
	m.population.invalidate()
	m.rebuildRows()
	m.restoreSearchCursor(snapshot.cursor)
}

// exitSearchOntoCursorTask dismisses the flat ranked list and lands the ordinary
// projection on the result the cursor named, so a jump reached through it is not
// a dead end. The restored projection can hide the result behind the mode it was
// searched out of, so a hidden result forces All once before it reports that a
// facet still excludes it. A query that is merely narrowing has nothing to exit:
// its rows are already the mode's own, so the caller keeps the query.
func (m *OutlinerModel) exitSearchOntoCursorTask() (string, bool) {
	if !m.showsRankedList() {
		return "", true
	}
	identity := m.CurrentTaskIdentity()
	m.ClearSearch()
	if !identity.Valid() || m.landOnSearchResult(identity) {
		return "", true
	}
	m.selectOutlinerMode(chrome.ModeAll)
	if m.landOnSearchResult(identity) {
		return "", true
	}
	return "Task is hidden by the active facets", false
}

func (m *OutlinerModel) landOnSearchResult(identity taskref.TaskIdentity) bool {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil {
		return false
	}
	if m.project != nil {
		m.NavigateToTask(task)
	} else {
		m.NavigateToTaskInProject(task, project)
	}
	if !m.moveCursorToIdentity(identity) {
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
		for index, row := range m.rows {
			if !row.isTaskRow() || row.task == nil || row.task.ID != anchor.identity.TaskID {
				continue
			}
			if m.project == nil && row.ProjectID != anchor.identity.ProjectID {
				continue
			}
			m.Cursor = index
			return
		}
		m.moveCursorToNearestTask(anchor.prevCursor)
		return
	}
	m.Cursor = anchor.prevCursor
	m.clampCursor()
}

// clearSearchProjectionWithoutRestore is reserved for intentional navigation
// resets that replace the current projection rather than dismissing search.
func (m *OutlinerModel) clearSearchProjectionWithoutRestore() {
	if m == nil {
		return
	}
	m.search.discard()
	m.resetSearchCompilation()
	m.BaseSearchableView.ClearSearch()
}

func (m *OutlinerModel) resetSearchCompilation() {
	// Suggestions describe the query being compiled, so they die with it: both
	// clear paths route here, and neither can leave a pane open over no search.
	m.search.autocomplete.clear()
	m.search.setCompilation("", nil, false)
	// The retained note payloads outlive the rows that named them otherwise, and
	// they hold every matched note's body.
	m.search.clearNoteMatches()
}

// searchTextQuery is the free-text remainder every consumer of the query must
// read: scoring, smart case, highlighting, and the work-mode predicate. Reading
// the raw query instead would score and highlight the sigils as prose.
func (m *OutlinerModel) searchTextQuery() string {
	if m == nil {
		return ""
	}
	if m.hasSearchProjection() {
		return m.search.remainder
	}
	return strings.TrimSpace(m.SearchQuery())
}

// searchCaseSensitive reports the case mode the visible result set was actually
// produced under, so highlighting and predicates never disagree with the rows.
func (m *OutlinerModel) searchCaseSensitive() bool {
	if m == nil || m.search.caseRelaxed {
		return false
	}
	return fuzzy.CaseSensitiveQuery(m.searchTextQuery())
}

func (m *OutlinerModel) buildSearchProjectionRows() {
	projects := m.projectsInCatalogOrder(m.workModeScopeProjects())
	candidates := m.searchCandidateBoundary(projects)

	query, predicates := m.compileSearchQuery(m.SearchQuery(), projects)
	// The pass opens unrelaxed: the retry below is the only thing that can raise
	// the case reading, and it has not run yet.
	m.search.setCompilation(query, predicates, false)

	// The one place the notes tier is admitted or refused. A sigil compiles to an
	// AND gate applied before scoring and a note satisfies no conjunct of it, so
	// the tier is absent whole rather than exempted per conjunct. Deciding once
	// here rather than inside the scorer is what makes the case-relaxed retry
	// below behave identically for free: both passes read the same decision.
	notesAdmitted := !predicates.Invalid() && !predicates.HasAnySigil()

	matcher := fuzzy.SmartCase(query)
	matches := searchProjectionMatchesWithin(projects, query, candidates, matcher, predicates)
	notes := m.scoreNoteSearchTier(query, matcher, notesAdmitted)
	// One query, one case reading: relaxation is decided over the whole result
	// set, so a query that relaxes relaxes for tasks and notes together.
	if len(matches) == 0 && len(notes) == 0 && matcher.CaseSensitive() {
		relaxed := fuzzy.NewMatcher(false)
		matches = searchProjectionMatchesWithin(projects, query, candidates, relaxed, predicates)
		notes = m.scoreNoteSearchTier(query, relaxed, notesAdmitted)
		m.search.setCaseRelaxed(len(matches) > 0 || len(notes) > 0)
	}
	m.sortSearchMatchesWithinTier(matches)

	// The task matches arrive ordered by rank, so the notes tier is seated at the
	// first task match its own rank no longer outranks. Reading
	// searchFieldRankNote here rather than appending blind keeps that constant the
	// one statement of where the tier sits, so moving it up the ladder later moves
	// the rows with it.
	boundary := len(matches)
	for index, match := range matches {
		if match.fieldRank >= searchFieldRankNote {
			boundary = index
			break
		}
	}
	m.appendSearchTaskRows(matches[:boundary])
	m.appendSearchNoteRows(notes)
	m.appendSearchTaskRows(matches[boundary:])
}

func (m *OutlinerModel) appendSearchTaskRows(matches []outlinerSearchMatch) {
	for _, match := range matches {
		row := m.outlinerRowFromModeRow(OutlinerModeRow{
			Task:       match.task,
			Project:    match.project,
			Breadcrumb: taskAncestorBreadcrumb(match.task),
		})
		// Attribution is recomputed by this rebuild alongside membership, so a
		// mutation that moves a hit from one field to another moves its cue too.
		row.MatchSource = match.source
		m.rows = append(m.rows, row)
	}
}

// appendSearchNoteRows emits the notes tier and retains what rendering it needs.
// The row carries the note's workspace identity alone, exactly as the Notes
// manager keys its own match map, so the wide payload a render reads — the note,
// its owner label, and the title positions that ranked it — stays off a struct
// every projection copies per row. The map is replaced on every rebuild, so it
// can never describe a result set that is no longer on screen.
func (m *OutlinerModel) appendSearchNoteRows(matches []outlinerSearchNoteMatch) {
	m.search.clearNoteMatches()
	if len(matches) == 0 {
		return
	}
	payloads := make(map[notes.NoteRowIdentity]outlinerSearchNoteMatch, len(matches))
	for _, note := range matches {
		identity := note.row.Identity()
		payloads[identity] = note
		m.rows = append(m.rows, outlinerRow{
			IsNoteRow:   true,
			NoteRow:     identity,
			MatchSource: matchSourceNote,
		})
	}
	m.search.setNoteMatches(payloads)
}

// sortSearchMatchesWithinTier reorders a resolved result set by the active sort
// axis without ever crossing a relevance tier. In a search projection the tier
// is the group, and a sort reorders tasks within their group only, so no
// `· description` hit can float above a title hit. It runs here rather than
// inside searchProjectionMatchesWithin so the lens call site keeps the saved
// order the lens itself installs.
func (m *OutlinerModel) sortSearchMatchesWithinTier(matches []outlinerSearchMatch) {
	if m == nil || m.sort.Field == query.SortByDefault || len(matches) < 2 {
		return
	}
	field, ascending := m.sort.Field, m.sort.Ascending
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].fieldRank != matches[j].fieldRank {
			return matches[i].fieldRank < matches[j].fieldRank
		}
		return query.ApplySortDirection(query.CompareTasks(matches[i].task, matches[j].task, field), ascending)
	})
}

// searchCandidateBoundary returns an authoritative per-project candidate set
// whenever something other than the query itself bounds the result. Two things
// do: task Focus, whose descendants-only branch search must never escape, and
// the active common facets, which are an AND gate applied before scoring
// exactly like the sigils. Search still ignores the current mode, sort, and
// folds. With neither in force the nil return keeps the scope-wide behavior.
//
// The facets compose through this channel rather than inside
// searchProjectionMatchesWithin because the lens path calls that function with
// its own facets already applied, and checking them there would apply them twice.
func (m *OutlinerModel) searchCandidateBoundary(projects []*models.Project) map[string][]*models.Task {
	if m == nil {
		return nil
	}
	focused := m.focus.active()
	faceted := query.HasActiveCommonFilters(&m.filter.CommonFilterState)
	if !focused && !faceted {
		return nil
	}
	// buildCanonicalModeRows populates the open-child census above the search
	// branch, so the hierarchy half of the facet gate reads a live count here.
	facets := m.facetPredicateAt(time.Now())
	candidates := make(map[string][]*models.Task, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		tasks := project.Tasks
		if focused {
			tasks = flattenTaskPointers(m.primaryModeRoots(project))
		}
		admitted := make([]*models.Task, 0, len(tasks))
		for _, task := range tasks {
			if task != nil && facets(task, project) {
				admitted = append(admitted, task)
			}
		}
		candidates[project.ID] = admitted
	}
	return candidates
}
