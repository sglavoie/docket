package outliner

import (
	"context"
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func (m *OutlinerModel) buildSearchProjectionRows() {
	if m.useBackgroundRows() {
		return
	}
	projects := m.ProjectsInCatalogOrder(m.WorkModeScopeProjects())
	candidates := m.SearchCandidateBoundary(projects)

	query, predicates := m.compileSearchQuery(m.SearchQuery(), projects)
	// The pass opens unrelaxed: the retry below is the only thing that can raise
	// the case reading, and it has not run yet.
	m.Search.setCompilation(query, predicates, false)

	// The one place the notes tier is admitted or refused. A sigil compiles to an
	// AND gate applied before scoring and a note satisfies no conjunct of it, so
	// the tier is absent whole rather than exempted per conjunct. Deciding once
	// here rather than inside the scorer is what makes the case-relaxed retry
	// below behave identically for free: both passes read the same decision.
	notesAdmitted := !predicates.Invalid() && !predicates.HasAnySigil()

	matcher := fuzzy.SmartCase(query)
	matches := searchProjectionMatchesWithinContext(m.searchContext(), projects, query, candidates, matcher, predicates, m.searchScoreCache())
	if m.searchContext().Err() != nil {
		return
	}
	notes := m.scoreNoteSearchTier(query, matcher, notesAdmitted)
	// One query, one case reading: relaxation is decided over the whole result
	// set, so a query that relaxes relaxes for tasks and notes together.
	if len(matches) == 0 && len(notes) == 0 && matcher.CaseSensitive() {
		relaxed := fuzzy.NewMatcher(false)
		matches = searchProjectionMatchesWithinContext(m.searchContext(), projects, query, candidates, relaxed, predicates, m.searchScoreCache())
		notes = m.scoreNoteSearchTier(query, relaxed, notesAdmitted)
		m.Search.setCaseRelaxed(len(matches) > 0 || len(notes) > 0)
		matcher = relaxed
	}
	if m.searchContext().Err() != nil {
		return
	}
	m.Search.withheld = searchWithheldMatches{}
	if !predicates.Invalid() {
		m.Search.withheld = countWithheldSearchMatches(m.searchContext(), projects, query, candidates, matcher, predicates, m.searchScoreCache())
	}
	m.Search.hiddenFilterHint = ""
	m.Search.eliminationHint = ""
	if len(matches) == 0 && len(notes) == 0 && !predicates.Invalid() {
		m.Search.hiddenFilterHint = m.removableFilterSearchHint(projects)
		m.Search.eliminationHint = m.searchEliminationHint(projects, candidates)
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

// countWithheldSearchMatches sorts the matches a widened gate found into the two
// populations the live gate kept out, by the rule AdmitsPopulation reads:
// terminal first, then a pending deferral.
func countWithheldSearchMatches(ctx context.Context, projects []*models.Project, text string, candidates map[string][]*models.Task, matcher fuzzy.Matcher, gate *query.SearchPredicates, cache *fuzzy.ScoreCache) searchWithheldMatches {
	// Visible tasks have already been scored. Widen only the population that
	// was withheld, keeping the same project, Focus and facet boundaries.
	hidden := make(map[string][]*models.Task, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		tasks := project.Tasks
		if bounded, ok := candidates[project.ID]; ok {
			tasks = bounded
		}
		hidden[project.ID] = nil
		for _, task := range tasks {
			if task != nil && !gate.AdmitsPopulation(task, project) {
				hidden[project.ID] = append(hidden[project.ID], task)
			}
		}
	}
	matches := searchProjectionMatchesWithinContext(ctx, projects, text, hidden, matcher, gate.WithEveryPopulation(), cache)
	var withheld searchWithheldMatches
	for _, match := range matches {
		if gate.AdmitsPopulation(match.task, match.project) {
			continue
		}
		if match.task.IsTerminalState() {
			// ?completed reads the completion instant, so a finished task
			// carrying none is not one that sigil would bring back.
			if match.task.CompletedAt != nil && !match.task.CompletedAt.IsZero() {
				withheld.completed++
			}
		} else {
			withheld.deferred++
		}
	}
	return withheld
}

func (m *OutlinerModel) searchScoreCache() *fuzzy.ScoreCache {
	if m.Search.scoreCache == nil {
		m.Search.scoreCache = fuzzy.NewScoreCache(128 << 20)
	}
	return m.Search.scoreCache
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
		m.Rows = append(m.Rows, row)
	}
}

// appendSearchNoteRows emits the notes tier and retains what rendering it needs.
// The row carries the note's workspace identity alone, exactly as the Notes
// manager keys its own match map, so the wide payload a render reads — the note,
// its owner label, and the title positions that ranked it — stays off a struct
// every projection copies per row. The map is replaced on every rebuild, so it
// can never describe a result set that is no longer on screen.
func (m *OutlinerModel) appendSearchNoteRows(matches []outlinerSearchNoteMatch) {
	m.Search.clearNoteMatches()
	if len(matches) == 0 {
		return
	}
	payloads := make(map[notes.NoteRowIdentity]outlinerSearchNoteMatch, len(matches))
	for _, note := range matches {
		identity := note.row.Identity()
		payloads[identity] = note
		m.Rows = append(m.Rows, OutlinerRow{
			IsNoteRow:   true,
			NoteRow:     identity,
			MatchSource: MatchSourceNote,
		})
	}
	m.Search.setNoteMatches(payloads)
}

// sortSearchMatchesWithinTier reorders a resolved result set by the active sort
// axis without ever crossing a relevance tier. In a search projection the tier
// is the group, and a sort reorders tasks within their group only, so no
// `· description` hit can float above a title hit. It runs here rather than
// inside searchProjectionMatchesWithin so the lens call site keeps the saved
// order the lens itself installs.
func (m *OutlinerModel) sortSearchMatchesWithinTier(matches []outlinerSearchMatch) {
	if m == nil || m.Sort.Field == query.SortByDefault || len(matches) < 2 {
		return
	}
	field, ascending := m.Sort.Field, m.Sort.Ascending
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
func (m *OutlinerModel) SearchCandidateBoundary(projects []*models.Project) map[string][]*models.Task {
	if m == nil {
		return nil
	}
	if !m.Focus.Active() && !query.HasActiveCommonFilters(&m.Filter.CommonFilterState) {
		return nil
	}
	return m.searchCandidateBoundary(projects, m.FacetPredicateAt(time.Now()))
}

func (m *OutlinerModel) searchCandidateBoundary(projects []*models.Project, facets func(*models.Task, *models.Project) bool) map[string][]*models.Task {
	focused := m.Focus.Active()
	if !focused && facets == nil {
		return nil
	}
	// buildCanonicalModeRows populates the open-child census above the search
	// branch, so the hierarchy half of the facet gate reads a live count here.
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
			if task != nil && (facets == nil || facets(task, project)) {
				admitted = append(admitted, task)
			}
		}
		candidates[project.ID] = admitted
	}
	return candidates
}

// compileSearchQuery hands the Outliner's own carriers to the shared gate. The
// gate reads two things this model owns and nothing else does: the durable plan
// that decides Today and Next membership, and the canonical filter query whose
// facets open the terminal and deferred populations beside the sigils.
//
// A nil model still compiles: the sigils answer for themselves, and a view with
// no plan and no facets neither names a planned task nor opens a population.
func (m *OutlinerModel) compileSearchQuery(text string, projects []*models.Project) (string, *query.SearchPredicates) {
	if m == nil {
		return query.CompileSearchQuery(text, projects, nil, models.Query{})
	}
	return query.CompileSearchQuery(text, projects, outlinerPlanningMembership{outliner: m}, m.FilterQuery)
}

// outlinerPlanningMembership is the Outliner's answer to the gate's planning
// question. It is a wrapper rather than a set of methods on the model itself so
// the gate's contract stays the two memberships it names, not whatever else the
// Outliner happens to expose.
type outlinerPlanningMembership struct {
	outliner *OutlinerModel
}

func (p outlinerPlanningMembership) TodayIdentities(projects []*models.Project) map[taskref.TaskIdentity]bool {
	return p.outliner.resolveTodayIDs(projects)
}

func (p outlinerPlanningMembership) NextIdentities(projects []*models.Project) map[taskref.TaskIdentity]bool {
	return resolveSearchNextIDs(p.outliner, projects)
}

// resolveSearchNextIDs reads Next membership from the durable plan, because Next
// and Today both store Backlog tasks and a state compare cannot tell them apart.
func resolveSearchNextIDs(m *OutlinerModel, projects []*models.Project) map[taskref.TaskIdentity]bool {
	ids := make(map[taskref.TaskIdentity]bool)
	if m == nil || m.Runtime == nil {
		return ids
	}
	for _, item := range planningNextOrder(m.Runtime) {
		for _, project := range projects {
			if project == nil || project.ID != item.ProjectID {
				continue
			}
			if task := project.GetTaskByID(item.TaskID); task != nil {
				ids[taskref.For(task, project)] = true
			}
			break
		}
	}
	return ids
}

// outlinerSearchNoteMatch is one scored note from the shell's index. It is the
// note counterpart of outlinerSearchMatch and carries the same two things a
// task match does: the subject and the evidence that ranked it.
type outlinerSearchNoteMatch struct {
	row   notes.LocatedNoteRow
	match notes.NoteSearchMatch
	// sourceIndex is the note's position in the in-scope index under the index's
	// own order — location, then filename. It is the tier's tiebreak exactly as
	// it is for tasks, and it is carried rather than recompared so that ordering
	// is stated once, in sortNoteSearchIndexRows.
	sourceIndex int
}

// setNoteSearchIndex injects the shell's note accessor. It is deliberately a
// setter rather than a constructor parameter: NewOutlinerModel builds the Hub
// preview painter too, and that surface must keep the nil accessor.
func (m *OutlinerModel) SetNoteSearchIndex(accessor func() []notes.LocatedNoteRow) {
	m.Search.cancelBackground()
	m.Search.snapshot = nil
	m.Search.setNoteIndex(accessor)
}

// noteSearchRowsInScope applies the Outliner's own project scope to the
// workspace-wide index. A single-project scope yields that project's notes
// alone — not Global — matching the way a scoped Outliner shows only that
// project's work; unscoped yields Global plus every project the index carries.
// Archived owners were already refused when the index was built, so there is
// no archive test here.
func (m *OutlinerModel) noteSearchRowsInScope() []notes.LocatedNoteRow {
	if m == nil || m.Search.noteIndex == nil {
		return nil
	}
	rows := m.Search.noteIndex()
	scopeID := m.Scope.ProjectID
	if m.Project != nil {
		scopeID = m.Project.ID
	}
	if scopeID == "" {
		return rows
	}
	scoped := make([]notes.LocatedNoteRow, 0, len(rows))
	for _, row := range rows {
		if row.Location.Scope == app.NoteScopeProject && row.Location.ProjectID == scopeID {
			scoped = append(scoped, row)
		}
	}
	return scoped
}

// scoreNoteSearchTier ranks the in-scope notes against the free-text remainder
// under the matcher the task pass is using, so one query never relaxes its case
// for tasks and not for notes. admitted is the caller's single decision about
// whether the tier exists for this query at all; the scorer does not re-derive
// it, which is what keeps the sigil rule stated in exactly one place.
//
// ScoreNoteSearch is reused as-is: the Notes manager and this tier must agree
// about what a note matches and how strongly, or the same query would rank the
// same note two ways on two surfaces.
func (m *OutlinerModel) scoreNoteSearchTier(
	query string,
	matcher fuzzy.Matcher,
	admitted bool,
) []outlinerSearchNoteMatch {
	if !admitted {
		return nil
	}
	rows := m.noteSearchRowsInScope()
	if len(rows) == 0 {
		return nil
	}
	// The shell's index arrives in this order already and scoping filters it
	// without reordering, so this is a no-op in production. It is applied here
	// anyway because sourceIndex is what the projection's rows are tiebroken and
	// seated by: a caller that hands the tier an unsorted snapshot must still get
	// the one order notes.SortLocatedNoteRows names, not its slice's.
	rows = slices.Clone(rows)
	notes.SortLocatedNoteRows(rows)
	matches := make([]outlinerSearchNoteMatch, 0, len(rows))
	for sourceIndex, row := range rows {
		if m.searchContext().Err() != nil {
			return nil
		}
		match, ok := notes.ScoreNoteSearch(row.Note, query, matcher, m.searchScoreCache())
		if !ok {
			continue
		}
		matches = append(matches, outlinerSearchNoteMatch{
			row: row, match: match, sourceIndex: sourceIndex,
		})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		left, right := matches[i], matches[j]
		if left.match.Score != right.match.Score {
			return left.match.Score > right.match.Score
		}
		return left.sourceIndex < right.sourceIndex
	})
	return matches
}
