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
// independently, while mode, facets, folds, ordering, and viewport are restored.
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
	cursor             modeCursorAnchor
}

type outlinerSearchMatch struct {
	task        *models.Task
	project     *models.Project
	fieldRank   int
	fuzzyScore  int
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
		cursor:             m.captureModeCursor(),
	}
	m.searchProjection = snapshot
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
		m.BaseSearchableView.ClearSearch()
		return
	}
	snapshot := m.searchProjection
	m.searchProjection = nil
	m.searchCaseRelaxed = false
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
	m.searchCaseRelaxed = false
	m.BaseSearchableView.ClearSearch()
}

// searchCaseSensitive reports the case mode the visible result set was actually
// produced under, so highlighting and predicates never disagree with the rows.
func (m *OutlinerModel) searchCaseSensitive() bool {
	if m == nil || m.searchCaseRelaxed {
		return false
	}
	return fuzzy.CaseSensitiveQuery(strings.TrimSpace(m.SearchQuery()))
}

func (m *OutlinerModel) buildSearchProjectionRows() {
	projects := m.projectsInCatalogOrder(m.workModeScopeProjects())
	query := strings.TrimSpace(m.SearchQuery())
	candidates := m.searchFocusCandidates(projects)

	m.searchCaseRelaxed = false
	matcher := fuzzy.SmartCase(query)
	matches := searchProjectionMatchesWithin(projects, query, candidates, matcher)
	if len(matches) == 0 && matcher.CaseSensitive() {
		matches = searchProjectionMatchesWithin(projects, query, candidates, fuzzy.NewMatcher(false))
		m.searchCaseRelaxed = len(matches) > 0
	}

	for _, match := range matches {
		m.rows = append(m.rows, m.outlinerRowFromModeRow(OutlinerModeRow{
			Task:       match.task,
			Project:    match.project,
			Breadcrumb: taskAncestorBreadcrumb(match.task),
		}))
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
) []outlinerSearchMatch {
	if projectRef, shortID, exact := parseProjectShortIDQuery(query); exact {
		return exactProjectShortIDMatchesWithin(projects, projectRef, shortID, candidates)
	}

	matches := make([]outlinerSearchMatch, 0)
	sourceIndex := 0
	for _, project := range projects {
		if project == nil {
			continue
		}
		tasks := project.Tasks
		if bounded, ok := candidates[project.ID]; ok {
			tasks = bounded
		}
		for _, task := range tasks {
			if task == nil || task.IsTerminalState() {
				sourceIndex++
				continue
			}
			fieldRank, score, ok := scoreSearchProjectionTask(task, project, query, matcher)
			if ok {
				matches = append(matches, outlinerSearchMatch{
					task: task, project: project, fieldRank: fieldRank,
					fuzzyScore: score, sourceIndex: sourceIndex,
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

func scoreSearchProjectionTask(
	task *models.Task,
	project *models.Project,
	query string,
	matcher fuzzy.Matcher,
) (int, int, bool) {
	if query == "" {
		return 0, 1, true
	}
	if score := matcher.ScoreTerms(task.Title, query); score > 0 {
		return 0, score, true
	}
	secondary := strings.Join([]string{
		task.Description,
		strings.Join(task.Tags, " "),
		project.Name,
		project.Slug,
		project.Description,
		fmt.Sprintf("%s#%d", project.Slug, task.ShortID),
	}, " ")
	if score := matcher.ScoreTerms(secondary, query); score > 0 {
		return 1, score, true
	}
	return 0, 0, false
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
) []outlinerSearchMatch {
	for _, project := range projects {
		if project == nil || !projectSearchRefMatches(project, projectRef) {
			continue
		}
		tasks := project.Tasks
		if bounded, ok := candidates[project.ID]; ok {
			tasks = bounded
		}
		for sourceIndex, task := range tasks {
			if task != nil && !task.IsTerminalState() && task.ShortID == shortID {
				return []outlinerSearchMatch{{
					task: task, project: project, fieldRank: 0,
					fuzzyScore: 1, sourceIndex: sourceIndex,
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
