package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// outlinerSearchSnapshot is the complete session-only presentation state that
// slash search temporarily replaces. Scope is deliberately not captured: the
// active project/context scope owns the search candidate set and may be changed
// independently, while mode, facets, folds, zoom, ordering, and viewport are
// restored.
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
)

type outlinerSearchMatch struct {
	task        *models.Task
	project     *models.Project
	fieldRank   int
	fuzzyScore  int
	source      outlinerMatchSource
	sourceIndex int
}

func (m *OutlinerModel) hasSearchProjection() bool {
	return m != nil && m.searchProjection != nil
}

func copyFilterState(src FilterState) FilterState {
	return FilterState{CommonFilterState: *copyCommonFilterState(&src.CommonFilterState)}
}

func (m *OutlinerModel) beginSearchProjection() {
	if m == nil || m.searchProjection != nil {
		return
	}
	snapshot := &outlinerSearchSnapshot{
		mode:               m.primaryMode,
		filter:             copyFilterState(m.filter),
		sort:               m.sort,
		treeSort:           m.treeSort,
		expandedNodes:      copyBoolMap(m.expandedNodes),
		expandedChecklists: copyBoolMap(m.expandedChecklists),
		collapsedSections:  copyBoolMap(m.collapsedSections),
		focusStack:         append([]FocusEntry(nil), m.focusStack...),
		crossFocusStack:    append([]CrossProjectFocusEntry(nil), m.crossFocusStack...),
		zoom:               m.zoom,
		cursor:             m.captureModeCursor(),
	}
	m.searchProjection = snapshot
	// Search results bypass the section emitters the zoom gates, so a zoom left
	// live here would be state no projection reads. Suspending it keeps the model
	// honest for the Esc cascade and the fingerprints.
	m.clearSectionZoom()
	m.crossForceRebuild = true
	m.rebuildRows()
}

// ActivateSearch starts or resumes the one temporary search projection. A
// second slash edits the existing query without replacing its restoration
// snapshot.
func (m *OutlinerModel) ActivateSearch() tea.Cmd {
	m.beginSearchProjection()
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
	snapshot := m.searchProjection
	m.searchProjection = nil
	m.resetSearchCompilation()
	m.BaseSearchableView.ClearSearch()
	m.primaryMode = snapshot.mode
	m.filter = copyFilterState(snapshot.filter)
	m.sort = snapshot.sort
	m.treeSort = snapshot.treeSort
	m.expandedNodes = copyBoolMap(snapshot.expandedNodes)
	m.expandedChecklists = copyBoolMap(snapshot.expandedChecklists)
	m.collapsedSections = copyBoolMap(snapshot.collapsedSections)
	m.focusStack = append([]FocusEntry(nil), snapshot.focusStack...)
	m.crossFocusStack = append([]CrossProjectFocusEntry(nil), snapshot.crossFocusStack...)
	m.zoom = snapshot.zoom
	m.crossForceRebuild = true
	m.rebuildRows()
	m.restoreSearchCursor(snapshot.cursor)
}

// exitSearchOntoCursorTask dismisses the temporary projection and lands the
// ordinary one on the result the cursor named, so a jump reached through search
// is not a dead end. The restored projection can hide the result behind the mode
// it was searched out of, so a hidden result forces All once before it reports
// that a facet still excludes it.
func (m *OutlinerModel) exitSearchOntoCursorTask() (string, bool) {
	if !m.hasSearchProjection() {
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
	candidates := m.searchFocusCandidates(projects)

	query, predicates := m.compileSearchQuery(projects)
	m.searchRemainder = query
	m.searchGate = predicates

	m.searchCaseRelaxed = false
	matcher := fuzzy.SmartCase(query)
	matches := searchProjectionMatchesWithin(projects, query, candidates, matcher, predicates)
	if len(matches) == 0 && matcher.CaseSensitive() {
		matches = searchProjectionMatchesWithin(projects, query, candidates, fuzzy.NewMatcher(false), predicates)
		m.searchCaseRelaxed = len(matches) > 0
	}

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

// searchFocusCandidates returns an authoritative per-project candidate set only
// while task Focus is active. Search intentionally ignores the current mode,
// facets, sort, and folds, but it must never escape the descendants-only branch
// the user entered.
func (m *OutlinerModel) searchFocusCandidates(projects []*models.Project) map[string][]*models.Task {
	if m == nil || (len(m.focusStack) == 0 && len(m.crossFocusStack) == 0) {
		return nil
	}
	candidates := make(map[string][]*models.Task, len(projects))
	for _, project := range projects {
		if project != nil {
			candidates[project.ID] = flattenTaskPointers(m.primaryModeRoots(project))
		}
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
			if task == nil || (!predicates.allowsTerminal() && task.IsTerminalState()) {
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
			if !predicates.allowsTerminal() && task.IsTerminalState() {
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
