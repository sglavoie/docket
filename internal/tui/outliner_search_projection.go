package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// outlinerSearchSnapshot is the navigation frame a slash search was typed in.
// Scope is deliberately not captured, because scope is a navigation frame rather
// than a clause of the query: a search runs inside the frame it was typed in, a
// lens moves the frame, and the frames stack. Changing scope under a held query
// therefore re-answers that query in the new frame instead of editing it, while
// mode, facets, folds, zoom, ordering, and viewport are the frame's own axes and
// are restored when the query is dropped.
type outlinerSearchSnapshot struct {
	mode               OutlinerModeID
	filter             FilterState
	sort               SortState
	treeSort           SortState
	expandedNodes      map[TaskIdentity]bool
	expandedChecklists map[TaskIdentity]bool
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
	return m != nil && m.searchProjection != nil
}

// showsRankedList reports that the held query is being presented as one flat
// relevance-ordered list. It is the single test every consumer that cannot
// survive a bypassed mode must ask: the ranked list has no lanes, no grouping,
// no manual order, and no section the zoom or the folds gate, so the operations
// that reshape those axes refuse here and nowhere else. A query alone never
// freezes an axis.
func (m *OutlinerModel) showsRankedList() bool {
	return m != nil && m.rankedList && m.searchProjection != nil
}

// toggleRankedList switches the held query between narrowing the mode and the
// flat ranked list. Entering suspends the section zoom, which gates emitters the
// flat list does not run; leaving restores nothing else, because every other
// axis stayed live under both presentations.
func (m *OutlinerModel) toggleRankedList() bool {
	if !m.hasSearchProjection() {
		return false
	}
	m.rankedList = !m.rankedList
	if m.rankedList {
		m.clearSectionZoom()
	}
	m.crossForceRebuild = true
	rebuildOutlinerPreservingCursor(m)
	return true
}

func copyFilterState(src FilterState) FilterState {
	return FilterState{CommonFilterState: *copyCommonFilterState(&src.CommonFilterState)}
}

func (m *OutlinerModel) beginSearchProjection() {
	if m == nil || m.searchProjection != nil {
		return
	}
	m.closeMenuBar()
	snapshot := m.capturePresentationSnapshot()
	m.searchProjection = &snapshot
	m.crossForceRebuild = true
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
	m.searchHistory.resetRecall()
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
		m.crossForceRebuild = true
		rebuildOutlinerPreservingCursor(m)
	}
}

// ClearSearch exits the temporary projection and restores the exact ordinary
// Outliner state. If the original task vanished while search operations were
// mutating data, focus falls back to the nearest surviving task row.
func (m *OutlinerModel) ClearSearch() {
	if m == nil || m.searchProjection == nil {
		m.searchAutocomplete.clear()
		m.BaseSearchableView.ClearSearch()
		return
	}
	m.closeMenuBar()
	snapshot := m.searchProjection
	m.searchProjection = nil
	m.rankedList = false
	m.resetSearchCompilation()
	m.BaseSearchableView.ClearSearch()
	m.restorePresentationSnapshot(*snapshot)
	m.crossForceRebuild = true
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
	identity := m.currentTaskIdentity()
	m.ClearSearch()
	if !identity.Valid() || m.landOnSearchResult(identity) {
		return "", true
	}
	m.selectOutlinerMode(ModeAll)
	if m.landOnSearchResult(identity) {
		return "", true
	}
	return "Task is hidden by the active facets", false
}

func (m *OutlinerModel) landOnSearchResult(identity TaskIdentity) bool {
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
	m.clearStatus()
	return true
}

func (m *OutlinerModel) restoreSearchCursor(anchor modeCursorAnchor) {
	m.viewScrollOffset = anchor.scrollOffset
	if anchor.hasTask {
		for index, row := range m.rows {
			if !row.isTaskRow() || row.task == nil || row.task.ID != anchor.identity.TaskID {
				continue
			}
			if m.project == nil && row.ProjectID != anchor.identity.ProjectID {
				continue
			}
			m.cursor = index
			return
		}
		m.moveCursorToNearestTask(anchor.prevCursor)
		return
	}
	m.cursor = anchor.prevCursor
	m.clampCursor()
}

// clearSearchProjectionWithoutRestore is reserved for intentional navigation
// resets that replace the current projection rather than dismissing search.
func (m *OutlinerModel) clearSearchProjectionWithoutRestore() {
	if m == nil {
		return
	}
	m.searchProjection = nil
	m.rankedList = false
	m.resetSearchCompilation()
	m.BaseSearchableView.ClearSearch()
}

func (m *OutlinerModel) resetSearchCompilation() {
	// Suggestions describe the query being compiled, so they die with it: both
	// clear paths route here, and neither can leave a pane open over no search.
	m.searchAutocomplete.clear()
	m.searchCaseRelaxed = false
	m.searchRemainder = ""
	m.searchGate = nil
	// The retained note payloads outlive the rows that named them otherwise, and
	// they hold every matched note's body.
	m.searchNoteMatches = nil
}

// searchTextQuery is the free-text remainder every consumer of the query must
// read: scoring, smart case, highlighting, and the work-mode predicate. Reading
// the raw query instead would score and highlight the sigils as prose.
func (m *OutlinerModel) searchTextQuery() string {
	if m == nil {
		return ""
	}
	if m.hasSearchProjection() {
		return m.searchRemainder
	}
	return strings.TrimSpace(m.SearchQuery())
}

// searchCaseSensitive reports the case mode the visible result set was actually
// produced under, so highlighting and predicates never disagree with the rows.
func (m *OutlinerModel) searchCaseSensitive() bool {
	if m == nil || m.searchCaseRelaxed {
		return false
	}
	return fuzzy.CaseSensitiveQuery(m.searchTextQuery())
}

func (m *OutlinerModel) buildSearchProjectionRows() {
	projects := m.projectsInCatalogOrder(m.workModeScopeProjects())
	candidates := m.searchCandidateBoundary(projects)

	query, predicates := m.compileSearchQuery(m.SearchQuery(), projects)
	m.searchRemainder = query
	m.searchGate = predicates

	// The one place the notes tier is admitted or refused. A sigil compiles to an
	// AND gate applied before scoring and a note satisfies no conjunct of it, so
	// the tier is absent whole rather than exempted per conjunct. Deciding once
	// here rather than inside the scorer is what makes the case-relaxed retry
	// below behave identically for free: both passes read the same decision.
	notesAdmitted := !predicates.invalid() && !predicates.hasAnySigil()

	m.searchCaseRelaxed = false
	matcher := fuzzy.SmartCase(query)
	matches := searchProjectionMatchesWithin(projects, query, candidates, matcher, predicates)
	notes := m.scoreNoteSearchTier(query, matcher, notesAdmitted)
	// One query, one case reading: relaxation is decided over the whole result
	// set, so a query that relaxes relaxes for tasks and notes together.
	if len(matches) == 0 && len(notes) == 0 && matcher.CaseSensitive() {
		relaxed := fuzzy.NewMatcher(false)
		matches = searchProjectionMatchesWithin(projects, query, candidates, relaxed, predicates)
		notes = m.scoreNoteSearchTier(query, relaxed, notesAdmitted)
		m.searchCaseRelaxed = len(matches) > 0 || len(notes) > 0
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
func (m *OutlinerModel) appendSearchNoteRows(notes []outlinerSearchNoteMatch) {
	m.searchNoteMatches = nil
	if len(notes) == 0 {
		return
	}
	m.searchNoteMatches = make(map[NoteRowIdentity]outlinerSearchNoteMatch, len(notes))
	for _, note := range notes {
		identity := note.row.Identity()
		m.searchNoteMatches[identity] = note
		m.rows = append(m.rows, outlinerRow{
			IsNoteRow:   true,
			NoteRow:     identity,
			MatchSource: matchSourceNote,
		})
	}
}

// sortSearchMatchesWithinTier reorders a resolved result set by the active sort
// axis without ever crossing a relevance tier. In a search projection the tier
// is the group, and a sort reorders tasks within their group only, so no
// `· description` hit can float above a title hit. It runs here rather than
// inside searchProjectionMatchesWithin so the lens call site keeps the saved
// order the lens itself installs.
func (m *OutlinerModel) sortSearchMatchesWithinTier(matches []outlinerSearchMatch) {
	if m == nil || m.sort.Field == SortByDefault || len(matches) < 2 {
		return
	}
	field, ascending := m.sort.Field, m.sort.Ascending
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].fieldRank != matches[j].fieldRank {
			return matches[i].fieldRank < matches[j].fieldRank
		}
		return ApplySortDirection(CompareTasks(matches[i].task, matches[j].task, field), ascending)
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
	focused := len(m.focusStack) > 0 || len(m.crossFocusStack) > 0
	faceted := hasActiveCommonFilters(&m.filter.CommonFilterState)
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

// searchProjectionMatchesWithin applies an optional authoritative candidate
// boundary. A present empty entry means that project contributes no matches;
// a missing entry retains the scope-wide behavior used outside Focus.
func searchProjectionMatchesWithin(
	projects []*models.Project,
	query string,
	candidates map[string][]*models.Task,
	matcher fuzzy.Matcher,
	predicates *searchPredicates,
) []outlinerSearchMatch {
	if predicates.invalid() {
		return nil
	}
	if projectRef, shortID, exact := parseProjectShortIDQuery(query); exact {
		return exactProjectShortIDMatchesWithin(projects, projectRef, shortID, candidates, predicates)
	}

	matches := make([]outlinerSearchMatch, 0)
	sourceIndex := 0
	for _, project := range projects {
		if project == nil || !predicates.admitsProject(project) {
			continue
		}
		tasks := project.Tasks
		if bounded, ok := candidates[project.ID]; ok {
			tasks = bounded
		}
		for _, task := range tasks {
			if task == nil || !predicates.admitsPopulation(task) {
				sourceIndex++
				continue
			}
			if !predicates.admits(task, project) {
				sourceIndex++
				continue
			}
			fieldRank, score, source, ok := scoreSearchProjectionTask(task, project, query, matcher)
			if ok {
				matches = append(matches, outlinerSearchMatch{
					task: task, project: project, fieldRank: fieldRank,
					fuzzyScore: score, source: source, sourceIndex: sourceIndex,
				})
			}
			sourceIndex++
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].fieldRank != matches[j].fieldRank {
			return matches[i].fieldRank < matches[j].fieldRank
		}
		if matches[i].fuzzyScore != matches[j].fuzzyScore {
			return matches[i].fuzzyScore > matches[j].fuzzyScore
		}
		return matches[i].sourceIndex < matches[j].sourceIndex
	})
	return matches
}

// scoreSearchProjectionTask ranks one task against the free-text remainder in
// the declared field tiers. Each tier is scored on its own text, so a hit is
// attributable to exactly one source; only the last tier, details, scores the
// concatenation, and it exists solely for multi-term queries whose terms are
// satisfied across fields. Because rank sorts before score, a details hit can
// never outrank a single-field one.
func scoreSearchProjectionTask(
	task *models.Task,
	project *models.Project,
	query string,
	matcher fuzzy.Matcher,
) (int, int, outlinerMatchSource, bool) {
	if query == "" {
		return searchFieldRankTitle, 1, matchSourceNone, true
	}

	checklistText := searchChecklistText(task)
	commentText := searchCommentText(task)
	tagText := strings.Join(task.Tags, " ")
	attachmentText := searchAttachmentText(task)
	projectText := searchProjectText(task, project)

	if score := matcher.ScoreTerms(task.Title, query); score > 0 {
		return searchFieldRankTitle, score, matchSourceTitle, true
	}
	if score := matcher.ScoreTerms(task.Description, query); score > 0 {
		return searchFieldRankDescription, score, matchSourceDescription, true
	}
	if score := matcher.ScoreTerms(checklistText, query); score > 0 {
		return searchFieldRankChecklist, score, matchSourceChecklist, true
	}
	if score := matcher.ScoreTerms(commentText, query); score > 0 {
		return searchFieldRankComment, score, matchSourceComment, true
	}
	// Tags and context share a rank, so the better score wins and an exact tie
	// falls to the declared field order: tags precede context.
	tagScore := matcher.ScoreTerms(tagText, query)
	contextScore := matcher.ScoreTerms(task.Context, query)
	if tagScore > 0 || contextScore > 0 {
		if tagScore >= contextScore {
			return searchFieldRankTagContext, tagScore, matchSourceTag, true
		}
		return searchFieldRankTagContext, contextScore, matchSourceContext, true
	}
	if score := matcher.ScoreTerms(attachmentText, query); score > 0 {
		return searchFieldRankAttachment, score, matchSourceAttachment, true
	}
	if score := matcher.ScoreTerms(projectText, query); score > 0 {
		return searchFieldRankProject, score, matchSourceProject, true
	}

	details := strings.Join([]string{
		task.Title,
		task.Description,
		checklistText,
		commentText,
		tagText,
		task.Context,
		attachmentText,
		projectText,
	}, " ")
	if score := matcher.ScoreTerms(details, query); score > 0 {
		return searchFieldRankDetails, score, matchSourceDetails, true
	}
	return searchFieldRankTitle, 0, matchSourceNone, false
}

// searchChecklistText is the item text alone: completion state is presentation,
// never a term a text query can match or miss on.
func searchChecklistText(task *models.Task) string {
	if len(task.Checklist) == 0 {
		return ""
	}
	parts := make([]string, 0, len(task.Checklist))
	for _, item := range task.Checklist {
		parts = append(parts, item.Text)
	}
	return strings.Join(parts, " ")
}

func searchCommentText(task *models.Task) string {
	if len(task.Comments) == 0 {
		return ""
	}
	parts := make([]string, 0, len(task.Comments))
	for _, comment := range task.Comments {
		if comment != nil {
			parts = append(parts, comment.Text)
		}
	}
	return strings.Join(parts, " ")
}

// searchAttachmentText covers both the displayed name (filename for files, the
// full URL for links) and the original path a code reference or local file was
// captured from.
func searchAttachmentText(task *models.Task) string {
	if len(task.Attachments) == 0 {
		return ""
	}
	parts := make([]string, 0, 2*len(task.Attachments))
	for _, attachment := range task.Attachments {
		parts = append(parts, attachment.Filename, attachment.OriginalPath)
	}
	return strings.Join(parts, " ")
}

// searchProjectText carries the owning project's metadata plus the qualified
// task reference, which is the fuzzy counterpart of the exact project#ID path.
func searchProjectText(task *models.Task, project *models.Project) string {
	return strings.Join([]string{
		project.Name,
		project.Slug,
		project.Description,
		fmt.Sprintf("%s#%d", project.Slug, task.ShortID),
	}, " ")
}

func parseProjectShortIDQuery(query string) (string, int, bool) {
	query = strings.TrimSpace(query)
	separator := strings.LastIndex(query, "#")
	if separator <= 0 || separator == len(query)-1 {
		return "", 0, false
	}
	shortID, err := strconv.Atoi(query[separator+1:])
	if err != nil || shortID <= 0 {
		return "", 0, false
	}
	projectRef := strings.TrimSpace(query[:separator])
	return projectRef, shortID, projectRef != ""
}

func exactProjectShortIDMatchesWithin(
	projects []*models.Project,
	projectRef string,
	shortID int,
	candidates map[string][]*models.Task,
	predicates *searchPredicates,
) []outlinerSearchMatch {
	for _, project := range projects {
		if project == nil || !projectSearchRefMatches(project, projectRef) {
			continue
		}
		if !predicates.admitsProject(project) {
			return nil
		}
		tasks := project.Tasks
		if bounded, ok := candidates[project.ID]; ok {
			tasks = bounded
		}
		for sourceIndex, task := range tasks {
			if task == nil || task.ShortID != shortID {
				continue
			}
			if !predicates.admitsPopulation(task) {
				continue
			}
			if predicates.admits(task, project) {
				// The qualified reference lives in the project tier, so an
				// exact hit is attributed there while keeping the exact
				// path's top rank.
				return []outlinerSearchMatch{{
					task: task, project: project, fieldRank: searchFieldRankTitle,
					fuzzyScore: 1, source: matchSourceProject, sourceIndex: sourceIndex,
				}}
			}
		}
		return nil
	}
	return nil
}

func projectSearchRefMatches(project *models.Project, ref string) bool {
	return strings.EqualFold(ref, project.Slug) ||
		strings.EqualFold(ref, project.Name) ||
		strings.EqualFold(ref, project.ID)
}
