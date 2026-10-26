package outliner

import (
	"fmt"
	"reflect"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/util"
)

// crossProjectRebuildRows aggregates tasks from the loaded population through
// the canonical mode pipeline. Called by rebuildRows() when m.project == nil.
//
// A memoization guard short-circuits rebuilding when mode, search/filter state,
// focus stack depth, and total task count are all unchanged from
// the previous call. The guard is bypassed when m.population.invalidate() has
// been called (after mutations) or when a fresh load completes.
func (m *OutlinerModel) CrossProjectRebuildRows() {
	m.SyncJumpProjection()
	// Compute current guard inputs.
	taskCount := 0
	for _, proj := range m.Population.Projects {
		taskCount += len(proj.Tasks)
	}
	currentFingerprint := m.crossProjectRowsFingerprint()
	currentFocusDepth := len(m.Focus.CrossStack)

	// Short-circuit when all guard inputs match the previous call and no force
	// rebuild has been requested. This prevents redundant rebuilds on no-op
	// events like window resize or view reactivation.
	if m.Population.memoized(currentFingerprint, currentFocusDepth, taskCount) {
		return
	}

	m.feedbackBudget = feedbackBudgetAnchor{}

	// Update guard state and consume the force flag.
	m.Population.record(currentFingerprint, currentFocusDepth, taskCount)
	m.Population.recordRebuiltFrom(m.Population.Projects)
	// A day-boundary or defer-wake invalidation captures the cursor before the
	// projection can change lanes. Every real rebuild exit below must consume it;
	// a memoized return above deliberately leaves it standing with the force flag.
	defer m.restoreTimeDrivenCursor()
	// The reference index is a reading of the population this pass is about to
	// walk, so it is dropped here and nowhere else on this path: every force
	// flag and every fresh load reaches this line, and a pass the guard
	// short-circuited changed nothing the index could have gone stale against.
	m.invalidateDependencyIndex()

	m.Rows = m.Rows[:0]
	m.hiddenLayout.reset()
	m.validateCrossFocusStack()
	// Validation may have shortened the stack after the guard captured its old
	// depth. Record the repaired depth so the next render remains a no-op.
	m.Population.recordFocusDepth(len(m.Focus.CrossStack))
	if len(m.Population.Projects) == 0 {
		m.pruneInvisibleSelections()
		m.ClampCursor()
		m.reseatNoteCursorAfterRebuild()
		return
	}

	m.buildCanonicalModeRows()
	m.reseatNoteCursorAfterRebuild()
}

// reseatNoteCursorAfterRebuild consumes the note identity an accepted
// note-index refresh captured and puts the cursor back on that note in the
// rebuilt rows. It runs after the row pipeline has already clamped, so a note
// that left the refreshed projection keeps that deterministic numeric landing
// rather than being moved again. The viewport is untouched either way:
// computeOutlinerViewport is the single window authority and follows the cursor
// on the next render.
//
// A memoized no-op rebuild returns before this point, which is what keeps the
// capture alive until the rebuild the force flag guarantees actually runs.
func (m *OutlinerModel) reseatNoteCursorAfterRebuild() {
	identity := m.Search.takeNoteCursor()
	if identity.Filename == "" {
		return
	}
	for index, row := range m.Rows {
		if row.IsNoteRow && row.NoteRow == identity {
			m.Cursor = index
			return
		}
	}
}

func (m *OutlinerModel) crossProjectRowsFingerprint() string {
	return m.crossProjectRowsFingerprintForSearch(m.SearchQuery())
}

func (m *OutlinerModel) crossProjectRowsFingerprintForSearch(search string) string {
	commonParts := query.CommonFilterParts(&m.Filter.CommonFilterState)

	var b strings.Builder
	fmt.Fprintf(&b, "search=%q|", search)
	fmt.Fprintf(&b, "search-projection=%t|", m.HasSearchProjection())
	// The same query yields lanes or one flat list, so the presentation toggle is
	// a membership input here exactly as the query itself is.
	fmt.Fprintf(&b, "ranked-list=%t|", m.ShowsRankedList())
	// The lens query decides membership without changing any other guard input,
	// so retyping it must be able to invalidate the memoized rows on its own.
	fmt.Fprintf(&b, "lens=%q|", m.Lens.Query)
	fmt.Fprintf(&b, "sort=%d:%t|", m.Sort.Field, m.Sort.Ascending)
	fmt.Fprintf(&b, "common=%q|", strings.Join(commonParts, "\x1f"))
	fmt.Fprintf(&b, "project-filter=%q|", query.ProjectFilterFingerprint(&m.ProjectFilter))
	fmt.Fprintf(&b, "loaded=%s|", loadedProjectIDsFingerprint(m.Population.Projects))
	fmt.Fprintf(&b, "scope=%q|", m.Scope.ProjectID)
	fmt.Fprintf(&b, "mode=%s|", m.PrimaryMode)
	fmt.Fprintf(&b, "zoom=%q|", m.Focus.Zoom.key)
	fmt.Fprintf(&b, "today-fold=%t|next-fold=%t|", m.Folds.CollapsedSections[todaySectionCollapseKey], m.Folds.CollapsedSections[nextSectionCollapseKey])
	b.WriteString("today-plan=")
	if m.Runtime != nil {
		for _, item := range m.Runtime.Planning().GetToday() {
			fmt.Fprintf(&b, "%s:%s,", item.ProjectID, item.TaskID)
		}
		b.WriteString("|next-plan=")
		for _, item := range m.Runtime.Planning().GetNext() {
			fmt.Fprintf(&b, "%s:%s,", item.ProjectID, item.TaskID)
		}
	}
	return b.String()
}

func loadedProjectIDsFingerprint(projects []*models.Project) string {
	if len(projects) == 0 {
		return ""
	}
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			ids = append(ids, "<nil>")
			continue
		}
		ids = append(ids, project.ID)
	}
	return strings.Join(ids, "\x1f")
}

// parkedProjectCount counts the non-archived projects the parked tag keeps out
// of the task workspace.
func (m *OutlinerModel) parkedProjectCount() int {
	if m.Runtime == nil {
		return 0
	}
	parked := 0
	for _, project := range m.Runtime.AvailableProjects() {
		if project != nil && models.ProjectTagsParked(project.Tags) {
			parked++
		}
	}
	return parked
}

// baseScopeChromeLabel names the unscoped population the Outliner is actually
// reading: the base label, the active F facets, and the count of parked projects
// still hidden. The count is dropped once the facets admit parked projects,
// because nothing is being hidden to report.
func (m *OutlinerModel) baseScopeChromeLabel() string {
	parts := query.ProjectFilterParts(&m.ProjectFilter)
	if !m.ProjectFilter.ShowsParked() {
		if parked := m.parkedProjectCount(); parked > 0 {
			parts = append(parts, fmt.Sprintf("%d parked", parked))
		}
	}
	if len(parts) == 0 {
		return activeProjectsScopeLabel
	}
	return activeProjectsScopeLabel + ScopeChromeSeparator + strings.Join(parts, ScopeChromeSeparator)
}

// crossStatusBarLabel returns the status bar label for the global Outliner,
// formatted as 'Outliner · [scope-name] · N tasks'.
// N counts only task rows, excluding section headers, project sub-headers,
// and separators.
func (m *OutlinerModel) crossStatusBarLabel() string {
	scopeName := m.baseScopeChromeLabel()
	if m.IsProjectScoped() {
		// Under a project scope the label names the scoped project chain.
		scopeName = strings.Join(m.scopeChainNames(), scopeChainSeparator)
	}
	taskCount := 0
	for _, row := range m.Rows {
		if row.IsTaskRow() {
			taskCount++
		}
	}
	label := "Outliner"
	if m.PrimaryMode != chrome.ModeAll {
		label += ": " + m.ActiveOutlinerModeEntry().Label
	}
	return fmt.Sprintf("%s · %s · %s", label, scopeName, util.Count(taskCount, "task"))
}

// crossProjectView renders the cross-project outliner. A cold load shows a
// centered loading state. Later reloads retain the last accepted snapshot until
// its replacement is ready, so rendering never passes through an empty frame.
func (m *OutlinerModel) crossProjectView() string {
	if m.Population.Loading && !m.hasCrossProjectSnapshot() {
		return m.renderCrossLoadingState()
	}
	return m.renderCrossProjectContent()
}

// hasCrossProjectSnapshot reports whether there is stable content to render
// while a refresh is in flight. The loaded-projects fallback keeps older tests
// and callers that install an in-memory snapshot directly compatible.
func (m *OutlinerModel) hasCrossProjectSnapshot() bool {
	return m.Population.SnapshotReady || len(m.Population.Projects) > 0
}

// LaunchFocusSettled reports whether the population the launch rule must read is
// the accepted one. Both halves are needed and neither implies the other:
// SnapshotReady is what separates a genuinely empty workspace from a read that
// has not landed, and Loading is what a saved lens still reading its own
// projects leaves set, so a launch lens's population is waited for rather than
// pre-empted. A superseded read publishes neither, so it never reaches here.
func (m *OutlinerModel) LaunchFocusSettled() bool {
	return m != nil && m.Project == nil && m.Population.SnapshotReady && !m.Population.Loading
}

// FocusFirstLaunchRow seats the cursor where a fresh launch begins: the first
// task row the settled projection draws, else the row the ordinary clamp leaves
// — the first heading — when it draws no task at all. It reports whether it
// found a task.
//
// It writes m.Cursor alone. ComputeOutlinerViewport stays the one window
// authority and follows the cursor on the next render, so nothing here opens a
// fold, changes an axis, or moves the window itself.
func (m *OutlinerModel) FocusFirstLaunchRow() bool {
	if m == nil || len(m.Rows) == 0 {
		return false
	}
	for index, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil {
			m.Cursor = index
			return true
		}
	}
	m.Cursor = 0
	m.ClampCursor()
	return false
}

// renderCrossLoadingState renders a non-empty cold-load frame immediately,
// adding the animated Dot spinner only after its short anti-flicker delay.
func (m *OutlinerModel) renderCrossLoadingState() string {
	label := "Loading projects…"
	if m.Population.SpinnerVisible {
		label = m.Population.spinner.View() + " " + label
	}
	centered := lipgloss.NewStyle().
		Width(m.Width()).
		Height(m.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Render(label)
	return centered
}

// renderCrossProjectContent renders the aggregated cross-project outliner tree.
// It delegates to the unified renderOutlinerTreeContent after rebuilding rows.
func (m *OutlinerModel) renderCrossProjectContent() string {
	m.CrossProjectRebuildRows()
	return m.renderFullWidth()
}

// reloadCrossProjects triggers a new load, incrementing the generation counter
// so any in-flight stale result is discarded on arrival.

// scopeChainSeparator joins nested project-scope names inside the breadcrumb
// anchor (e.g. "[Alpha › Beta]").
const scopeChainSeparator = " › "

// scopeChromeSeparator joins the qualifiers appended to a scope or project name
// (e.g. "[Client Migration · 12 tasks]").
const ScopeChromeSeparator = " · "

// activeProjectsScopeLabel names the unscoped global Outliner population in
// every piece of scope chrome.
const activeProjectsScopeLabel = "Active Projects"

// crossProjectFocusContext returns the full project-qualified Focus path.
// Width fitting belongs to the shared title renderer.
func (m *OutlinerModel) CrossProjectFocusContext() string {
	if len(m.Focus.CrossStack) == 0 {
		return ""
	}
	top := m.Focus.CrossStack[len(m.Focus.CrossStack)-1]
	project := m.crossResolveProject(top.ProjectID)
	if project == nil {
		return ""
	}
	return taskFocusContext(project, project.GetTaskByID(top.TaskID))
}

// populationSource is what one loaded project gave a rebuild: the object, its
// own fields, and the identity of the collections it held. Every load parses a
// new task slice, so an unchanged slice is the same tasks the rows were built
// from, and a rename, a retag, or any other catalog change shows in the fields.
// A difference it reports that the rows would not show costs a rebuild; it
// never reports the same for a population the rows were not built from.
type populationSource struct {
	project  *models.Project
	fields   models.Project
	tags     string
	tasks    *models.Task
	taskLen  int
	comments *models.ProjectComment
}

// same compares the identities directly and the fields by value, so a field
// added to models.Project later is compared without being named here.
func (s populationSource) same(other populationSource) bool {
	return s.project == other.project && s.tasks == other.tasks && s.taskLen == other.taskLen &&
		s.comments == other.comments && s.tags == other.tags && reflect.DeepEqual(s.fields, other.fields)
}

func populationSourceOf(project *models.Project) populationSource {
	source := populationSource{project: project}
	if project == nil {
		return source
	}
	source.fields = *project
	source.fields.Tags, source.fields.Tasks, source.fields.Comments = nil, nil, nil
	source.tags = strings.Join(project.Tags, "\x1f")
	source.taskLen = len(project.Tasks)
	if len(project.Tasks) > 0 {
		source.tasks = project.Tasks[0]
	}
	if len(project.Comments) > 0 {
		source.comments = project.Comments[0]
	}
	return source
}

// recordRebuiltFrom notes the population a real rebuild just read.
func (p *populationState) recordRebuiltFrom(projects []*models.Project) {
	p.rebuiltFrom = p.rebuiltFrom[:0]
	for _, project := range projects {
		p.rebuiltFrom = append(p.rebuiltFrom, populationSourceOf(project))
	}
}

// rebuiltFromSame reports whether projects is exactly the population the last
// real rebuild read: the same objects, holding the same task slices and the same
// catalog fields. A publish answering yes changes nothing the rows were built
// from, so it need not force a rebuild; the guard still compares the view.
func (p *populationState) rebuiltFromSame(projects []*models.Project) bool {
	if len(projects) != len(p.rebuiltFrom) {
		return false
	}
	for i, project := range projects {
		if !populationSourceOf(project).same(p.rebuiltFrom[i]) {
			return false
		}
	}
	return true
}
