package outliner

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/workspace"
)

// crossProjectsLoadedMsg carries the result of an async cross-project load.
type CrossProjectsLoadedMsg struct {
	Projects    []*models.Project
	FailedLoads []ProjectLoadFailure
	Generation  int

	// focusProjectID is set only when this load was started to enter a parked
	// project focus. Only that load pushes a scope, and only it cancels outright
	// when the project cannot be read.
	FocusProjectID string

	// retainedFocusProjectID names the already-adopted focus a steady-state
	// reload carried on top of the base workspace. It leaves the focus exactly
	// where it is and exists so the handler can re-decide whether the exception
	// that admits the project is still required.
	retainedFocusProjectID string

	// loadedProjectFilter is the fingerprint of the project filter this read was
	// actually issued under. It travels with the result because the live filter is
	// not always the one the population answers to: a lens applied under a project
	// scope writes its facets without loading, so a reload started before it lands
	// afterwards carrying the older population. The provenance has to be the
	// filter the read used, never the one the model happens to hold when it
	// arrives.
	LoadedProjectFilter string
}

// crossSpinnerDelay is how long an initial load runs before it animates a
// spinner, so a fast load never flashes one.
const crossSpinnerDelay = 200 * time.Millisecond

// crossSpinnerDelayMsg fires after crossSpinnerDelay to animate the initial-load
// spinner.
type CrossSpinnerDelayMsg struct{}

// crossSpinnerTickMsg wraps a spinner tick so it can be distinguished from
// the global save spinner's ticks.
type crossSpinnerTickMsg struct {
	inner tea.Msg
}

// crossSpinnerTickFwd wraps a spinner Cmd so its ticks are delivered as
// crossSpinnerTickMsg rather than bare spinner.TickMsg.
func crossSpinnerTickFwd(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		return crossSpinnerTickMsg{inner: msg}
	}
}

// startCrossProjectLoad sets loading state and returns a Cmd that begins async
// loading of all context-filtered projects plus a 200ms initial-spinner delay.
func (m *OutlinerModel) startCrossProjectLoad() tea.Cmd {
	if !m.hasCrossProjectSnapshot() {
		m.Population.resetSpinner()
	}
	// An ordinary reload reads the live project filter, which is not the candidate
	// filter a pending lens application is waiting on. That application can never
	// publish behind this load, so it is abandoned here rather than left to have
	// its result arrive against a model that has moved on.
	m.discardPendingLensApplication()
	m.Population.beginLoad()
	return tea.Batch(
		m.LoadCrossProjectsCmd(),
		status.TickCmd(crossSpinnerDelay, func(time.Time) tea.Msg {
			return CrossSpinnerDelayMsg{}
		}),
	)
}

// loadCrossProjectsCmd returns a Cmd that loads all active (context-filtered)
// projects from disk. Failed projects are skipped; their load errors are
// collected for a non-blocking status bar warning, matching the AllTasks
// continue-on-error pattern.
func (m *OutlinerModel) LoadCrossProjectsCmd() tea.Cmd {
	gen := m.Population.LoadGen
	runtime := m.Runtime
	svc := m.ProjectService
	adopting := m.Scope.PendingParkedID != ""
	focus := m.Scope.FocusRequest()
	projectFilter := m.ProjectFilter.Clone()
	return func() tea.Msg {
		_, loaded, failed := LoadCrossScopeTasks(runtime, svc, CrossScopeOpts{
			ActiveOnly:     true,
			FocusProjectID: focus,
			ProjectFilter:  projectFilter,
		})
		msg := CrossProjectsLoadedMsg{
			Projects:            loaded,
			FailedLoads:         failed,
			Generation:          gen,
			LoadedProjectFilter: query.ProjectFilterFingerprint(&projectFilter),
		}
		if adopting {
			msg.FocusProjectID = focus
		} else {
			msg.retainedFocusProjectID = focus
		}
		return msg
	}
}

// beginParkedProjectFocus starts the load that admits one parked project on top
// of the base workspace. The generation bump inside startCrossProjectLoad is
// what makes a repeated, changed, or abandoned request safe: only the newest
// request's result is ever accepted.
func (m *OutlinerModel) BeginParkedProjectFocus(projectID string) tea.Cmd {
	if m == nil || m.Project != nil || projectID == "" {
		return nil
	}
	m.Scope.requestParked(projectID)
	return m.ReloadCrossProjects()
}

// cancelParkedProjectFocus abandons an in-flight focus request. The base
// snapshot is untouched, so there is nothing to restore.
func (m *OutlinerModel) cancelParkedProjectFocus() {
	if m == nil || m.Scope.PendingParkedID == "" {
		return
	}
	m.Scope.clearPendingParked()
	m.CancelCrossProjectLoad()
}

// takeParkedFocusReload claims the reload owed after a parked focus was dropped
// synchronously. The Esc cascade drops the project from the live snapshot so no
// empty frame is rendered; this reload then re-reads the base workspace alone.
func (m *OutlinerModel) TakeParkedFocusReload() tea.Cmd {
	if m == nil || !m.Scope.ParkedReloadPending {
		return nil
	}
	return m.ReloadCrossProjects()
}

// handleViewDidAppear is called when the cross-project outliner becomes the
// top view again (e.g., after returning from a drill-down). It triggers an
// eager reload from disk while preserving the current cursor position; the
// cursor is clamped to the new row count once the load completes.
func (m *OutlinerModel) handleViewDidAppear() tea.Cmd {
	if m.Project != nil {
		// Only applies in cross-project mode.
		return nil
	}
	// Becoming visible again means the surface that asked for a parked focus was
	// dismissed before its load landed. Abandon it and reload the base workspace.
	m.cancelParkedProjectFocus()
	if m.Population.Loading {
		return nil
	}
	savedCursor := m.Cursor
	cmd := m.ReloadCrossProjects()
	m.Cursor = savedCursor
	return cmd
}

// handleCrossProjectsLoaded processes a completed load. Stale results
// (generation mismatch) are discarded. Successful results populate
// the population and rebuild rows. A status bar warning is returned for any
// failed projects.
func (m *OutlinerModel) HandleCrossProjectsLoaded(msg CrossProjectsLoadedMsg) tea.Cmd {
	if msg.Generation != m.Population.LoadGen {
		return nil // Stale result from a superseded load; discard.
	}
	if msg.FocusProjectID != "" {
		if msg.FocusProjectID != m.Scope.PendingParkedID {
			// The request was abandoned while its read ran — a scope push, a lens, or
			// a relationship hop replaced the projection it was for. This population
			// was read with the focused project appended as an exception the live view
			// no longer holds, so publishing it would open a parked project's tasks in
			// a view nobody asked for and push a scope over wherever the user now is.
			// Nothing is published; the read is put down, and the capture it was
			// carrying goes with it, because a read that replaced no rows must not move
			// a cursor later.
			m.Population.settleLoad()
			m.Population.clearReloadCursor()
			return nil
		}
		m.Scope.clearPendingParked()
		if detail, failed := parkedFocusFailureDetail(msg, msg.FocusProjectID); failed {
			// The base snapshot stays exactly as it was: a focus that could not be
			// read must not disturb the workspace the user is still looking at.
			m.Population.settleLoad()
			m.Population.clearReloadCursor()
			return func() tea.Msg { return status.ErrorMsg{Err: errors.New(detail)} }
		}
	}
	fallbackCursor := m.Cursor
	fallbackScrollOffset := m.ViewScrollOffset
	m.DiscardDisplacedRelocationCapture()
	m.Population.settleLoad()
	m.setLoadedProjects(msg.Projects)
	// Both halves of this read's answer are recorded together: the projects it
	// opened and the projects it could not. The generation check above is what
	// makes them honest — a superseded read publishes neither half, so a failure it
	// reported can never withdraw a project an accepted read opened — and the
	// pairing is what keeps the record per project, since a read that asked for
	// neither half of a project says nothing about it.
	m.Population.recordLoadFailures(msg.Projects, msg.FailedLoads)
	m.Population.setSnapshotReady(true)
	m.recordLoadedPopulation(msg.LoadedProjectFilter)
	m.pruneSelectionsOutsideLoadedProjects()
	m.Population.Invalidate()
	m.CrossProjectRebuildRows()
	m.RestoreCrossOutlinerCursorAfterReload(fallbackCursor, fallbackScrollOffset)
	switch {
	case msg.FocusProjectID != "":
		// Push after the cursor is restored so the captured base frame holds the
		// position the user actually left, not a transient reload artifact.
		m.settleParkedFocusException(msg.FocusProjectID)
		m.PushProjectScope(msg.FocusProjectID)
	case msg.retainedFocusProjectID != "" && msg.retainedFocusProjectID == m.Scope.ParkedProjectID:
		// A reload that merely carried the adopted focus leaves it in place and
		// only re-decides the exception.
		m.settleParkedFocusException(msg.retainedFocusProjectID)
	}

	var cmds []tea.Cmd
	if jumpCmd := m.FinishPendingOutlinerRelocation(); jumpCmd != nil {
		cmds = append(cmds, jumpCmd)
	}
	// A scoped project may have been archived or removed externally
	// while the reload was in flight; unwind to the nearest valid scope.
	if scopeMsg, repaired := m.reconcileProjectScope(); repaired {
		cmds = append(cmds, func() tea.Msg { return status.InfoStatusMsg(scopeMsg) })
	}

	if advisory := m.LoadFailureAdvisory(msg.FailedLoads); advisory != nil {
		cmds = append(cmds, advisory)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// parkedFocusFailureDetail reports why an explicit focus request cannot be
// honored. Ordinary cross-project loads treat a per-project failure as a
// non-fatal diagnostic, but the focused project is the whole point of this
// load: if it did not parse, or was archived or removed before the load ran,
// the request is canceled rather than silently opening an empty scope.
func parkedFocusFailureDetail(msg CrossProjectsLoadedMsg, projectID string) (string, bool) {
	for _, failure := range msg.FailedLoads {
		if failure.ProjectID == projectID {
			return failure.Detail, true
		}
	}
	for _, project := range msg.Projects {
		if project != nil && project.ID == projectID {
			return "", false
		}
	}
	return "Could not open that project — it is no longer available", true
}

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

	// Update guard state and consume the force flag.
	m.Population.record(currentFingerprint, currentFocusDepth, taskCount)
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
	commonParts := query.CommonFilterParts(&m.Filter.CommonFilterState)

	var b strings.Builder
	fmt.Fprintf(&b, "search=%q|", m.SearchQuery())
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

func (m *OutlinerModel) ReloadCrossProjects() tea.Cmd {
	m.Scope.clearParkedReload()
	if !m.Population.consumeReloadFromTop() {
		m.preserveCrossOutlinerCursorForReload()
	}
	return m.startCrossProjectLoad()
}

// preserveCrossOutlinerCursorForReload captures the selected task by project and
// task ID before reloadCrossProjects starts replacing the snapshot. If a
// superseding reload starts after the task has already disappeared, keep the
// existing target for the newest accepted load instead of losing the user's
// selection.
func (m *OutlinerModel) CancelCrossProjectLoad() {
	if m == nil || m.Project != nil {
		return
	}
	// An abandoned refresh takes any pending lens application with it: both are
	// waiting on the same in-flight read.
	m.discardPendingLensApplication()
	if m.Population.Loading {
		m.Population.supersede()
	}
	m.Population.settleLoad()
	m.Population.clearReloadTarget()
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
	return fmt.Sprintf("%s · %s · %d tasks", label, scopeName, taskCount)
}

// CrossScopeOpts controls which projects are selected and loaded by
// LoadCrossScopeTasks for cross-project surfaces such as the Outliner and
// ordered planning views.
type CrossScopeOpts struct {
	// ActiveOnly restricts selection to RuntimeState.TaskWorkspaceProjects,
	// which admits exactly the non-archived projects that are not parked — the
	// same rule a context scope resolves by, so a scoped Outliner load can
	// never be asked for a project this selection drops. When false, the
	// selection falls back to AvailableProjects (archived excluded only).
	ActiveOnly bool

	// FocusProjectID names one explicitly focused project to admit on top of the
	// selection above. It is the sole way a parked project enters a task
	// projection: the user asked for it by name, so it joins the base population
	// as an exception rather than widening the rule. Ignored when empty, already
	// selected, or archived.
	FocusProjectID string

	// ProjectFilter is the user-chosen population override. While any facet is
	// active it replaces the ActiveOnly base rather than narrowing it, because
	// the facets both widen (parked, archived) and restrict (tags, untagged);
	// composing them with a fixed base could never widen.
	ProjectFilter query.ProjectFilterState
}

type ProjectLoadFailure struct {
	ProjectID string
	Detail    string
}

// diagnosticToLoadFailure adapts a workspace snapshot diagnostic into the
// cross-scope failure shape the TUI already surfaces, preserving the historical
// "failed to load project X: <err>" detail format.
func diagnosticToLoadFailure(diag workspace.Diagnostic) ProjectLoadFailure {
	name := diag.ProjectName
	if name == "" {
		name = "<unknown>"
	}
	detail := diag.Detail
	if detail == "" {
		detail = "unknown error"
	}
	return ProjectLoadFailure{
		ProjectID: diag.ProjectID,
		Detail:    fmt.Sprintf("failed to load project %s: %s", name, detail),
	}
}

// LoadCrossScopeTasks selects projects per opts, loads them once into an
// immutable workspace snapshot, and returns the flat task union, the
// successfully loaded projects, and the details of any projects that failed to
// load. Individual load failures are non-fatal: the offending project is
// skipped and surfaced as a diagnostic for the caller to log or display.
//
// The view-owned ProjectService retains the snapshot loader across rebuilds.
// Changed projects are parsed once into the new snapshot while unchanged
// projects reuse their indexed parse.
func LoadCrossScopeTasks(runtime *config.RuntimeState, service *coordinator.ProjectService, opts CrossScopeOpts) (tasks []*models.Task, projects []*models.Project, failed []ProjectLoadFailure) {
	if service == nil {
		service = coordinator.NewProjectService()
	}
	return LoadCrossScopeSelection(SelectCrossScopeProjects(runtime, opts), service.WorkspaceLoader())
}

// loadCrossScopeSelection loads an already-selected project set into an immutable
// workspace snapshot and returns the flat task union, the successfully loaded
// projects, and any load-failure details. It is the source-agnostic core shared
// by the config-facade LoadCrossScopeTasks and the domain-snapshot workspace
// pulse, so both select projects from their own domain source and load them
// identically.
func LoadCrossScopeSelection(selected []*models.Project, loader *workspace.Loader) (tasks []*models.Task, projects []*models.Project, failed []ProjectLoadFailure) {
	if loader == nil {
		loader = workspace.NewLoader()
	}
	snap := loader.Build(selected)
	for _, diag := range snap.Diagnostics() {
		failed = append(failed, diagnosticToLoadFailure(diag))
	}
	projects = snap.Projects()
	catalogByID := make(map[string]*models.Project, len(selected))
	for _, project := range selected {
		if project != nil {
			catalogByID[project.ID] = project
		}
	}
	for _, project := range projects {
		coordinator.CopyCatalogMetadata(project, catalogByID[project.ID])
	}
	return snap.Tasks(), projects, failed
}

// computeWorkspacePulse selects and loads the base task workspace, then hands
// the loaded projects to the counting half in internal/tui/pulse.
//
// The split is what keeps that package at L0: the snapshot loader below is
// shell-and-outliner infrastructure, so pulse counts what it is given rather
// than reaching up for it. Partial failure behavior is unchanged — a project
// that fails to load is warned about here and is simply absent from the slice
// the counter walks.
func ComputeWorkspacePulse(rs *config.RuntimeState, todayIDs map[taskref.TaskIdentity]bool, now time.Time) pulse.WorkspacePulse {
	if rs == nil {
		return pulse.WorkspacePulse{}
	}
	_, projects, failed := LoadCrossScopeSelection(rs.TaskWorkspaceProjects(), nil)
	for _, failure := range failed {
		logging.Warn("workspace pulse: %s", failure.Detail)
	}
	return pulse.ComputeWorkspacePulse(projects, todayIDs, now)
}

// selectCrossScopeProjects is the one place task projections choose a project
// population. The active-only selection is the base task workspace: a parked
// project is absent from the loaded snapshot, so no mode, filter, pinned
// section, or planning projection downstream can reach its tasks.
func SelectCrossScopeProjects(runtime *config.RuntimeState, opts CrossScopeOpts) []*models.Project {
	var selected []*models.Project
	switch {
	case query.HasActiveProjectFilters(&opts.ProjectFilter):
		selected = filteredProjectPopulation(runtime, opts.ProjectFilter)
	case opts.ActiveOnly:
		selected = runtime.TaskWorkspaceProjects()
	default:
		selected = runtime.AvailableProjects()
	}
	return appendFocusedProject(runtime, selected, opts.FocusProjectID)
}

// filteredProjectPopulation applies the F facets to the candidate populations.
// The archived candidates are appended as their own population rather than
// filtered out of a combined list, so archival stays a population choice and the
// active projects keep their pinned-first presentation order ahead of them.
func filteredProjectPopulation(runtime *config.RuntimeState, filter query.ProjectFilterState) []*models.Project {
	candidates := runtime.AvailableProjects()
	switch filter.ArchivedMode {
	case query.ProjectPopulationOnly:
		candidates = runtime.ArchivedProjects()
	case query.ProjectPopulationInclude:
		candidates = append(candidates, runtime.ArchivedProjects()...)
	default:
	}
	selected := make([]*models.Project, 0, len(candidates))
	for _, project := range candidates {
		if query.ProjectFilterAdmits(project, filter) {
			selected = append(selected, project)
		}
	}
	return selected
}

// appendFocusedProject admits one explicitly focused project the base selection
// dropped. Resolving through AvailableProject keeps archival authoritative: a
// project archived between the focus request and the load simply never joins,
// and the caller sees it missing from the snapshot.
func appendFocusedProject(runtime *config.RuntimeState, selected []*models.Project, focusProjectID string) []*models.Project {
	if focusProjectID == "" {
		return selected
	}
	for _, project := range selected {
		if project != nil && project.ID == focusProjectID {
			return selected
		}
	}
	focused := runtime.AvailableProject(focusProjectID)
	if focused == nil {
		return selected
	}
	return append(selected, focused)
}

func (m *OutlinerModel) preserveCrossOutlinerCursorForReload() {
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil || row.ProjectID == "" || row.Task.ID == "" {
		return
	}
	m.Population.captureReloadCursor(row.ProjectID, row.Task.ID)
}

// rebaseReloadTargetAfterRelocation re-aims a read already in flight at the row
// the view is standing on now. It belongs to the acts that replace the whole
// projection rather than move within it — a relationship hop, its return, and
// the reset — because the capture a running read is carrying describes the view
// those acts have just left, and honoring it when the result lands would drag
// the cursor back out of the destination the user asked for.
//
// The read itself is left alone. Its projects are the newest reading there is
// and are still wanted; only where to stand in them has changed. With the cursor
// on no task at all the capture is dropped instead, which lets the read fall
// back to the numeric position the relocation established.
func (m *OutlinerModel) rebaseReloadTargetAfterRelocation() {
	if m == nil || m.Project != nil || !m.Population.Loading {
		return
	}
	identity := m.CurrentTaskIdentity()
	m.Population.rebaseReloadCursor(identity.ProjectID, identity.TaskID)
}

// restoreCrossOutlinerCursorAfterReload consumes the captured cursor identity
// after the accepted load rebuilds rows. If the task disappeared or is hidden,
// focus falls back to the nearest task around the old numeric row, preferring a
// following task at equal distance. When no tasks remain, the ordinary navigable
// row clamp still permits an empty section or project header to receive focus.
func (m *OutlinerModel) RestoreCrossOutlinerCursorAfterReload(fallbackCursor, fallbackScrollOffset int) {
	projectID, taskID, headerProjectID := m.Population.takeReloadTarget()
	m.ViewScrollOffset = fallbackScrollOffset
	if headerProjectID != "" {
		if !m.moveCursorToProjectHeader(headerProjectID) {
			// The successor went away too, so the top of the projection — a section
			// header — is the only landing spot left that still means something.
			m.Cursor = 0
			m.ClampCursor()
		}
		return
	}
	if projectID == "" || taskID == "" {
		m.Cursor = fallbackCursor
		m.ClampCursor()
		return
	}
	if m.MoveCursorToProjectTask(taskID, projectID) {
		return
	}
	m.moveCursorToNearestTask(fallbackCursor)
}

// projectHeaderAnchorAfterRemoval names the project group the cursor should hold
// once the named projects' rows leave: the first surviving group below the
// topmost departing one, else the closest surviving group above it. With no
// survivor it returns a departing project's own ID, which stops resolving after
// the reload and therefore lands the cursor on the owning section header rather
// than an arbitrary row. The second result is false when none of the named
// projects has a group on screen at all.
//
// It walks group identities rather than header rows, so the anchor is the same
// whether the group leads with a header or with its first task.
func (m *OutlinerModel) ProjectHeaderAnchorAfterRemoval(projectIDs ...string) (string, bool) {
	leaving := make(map[string]bool, len(projectIDs))
	for _, projectID := range projectIDs {
		if projectID != "" {
			leaving[projectID] = true
		}
	}
	index := -1
	for candidate, row := range m.Rows {
		if row.Group.ProjectGrouped() && leaving[row.Group.ProjectID] {
			index = candidate
			break
		}
	}
	if index < 0 {
		return "", false
	}
	survivor := func(row OutlinerRow) bool {
		return row.Group.ProjectGrouped() && row.Group.ProjectID != "" && !leaving[row.Group.ProjectID]
	}
	for _, row := range m.Rows[index+1:] {
		if survivor(row) {
			return row.Group.ProjectID, true
		}
	}
	for candidate := index - 1; candidate >= 0; candidate-- {
		if row := m.Rows[candidate]; survivor(row) {
			return row.Group.ProjectID, true
		}
	}
	return m.Rows[index].Group.ProjectID, true
}

// moveCursorToProjectHeader parks the cursor on the named project's group. The
// group's header is emitted above its members and carries the same identity, so
// this lands on that header wherever one is drawn.
func (m *OutlinerModel) moveCursorToProjectHeader(projectID string) bool {
	if projectID == "" {
		return false
	}
	for index, row := range m.Rows {
		if row.Group.ProjectGrouped() && row.Group.ProjectID == projectID {
			m.Cursor = index
			m.ClampCursor()
			return true
		}
	}
	return false
}

func (m *OutlinerModel) moveCursorToNearestTask(origin int) {
	if len(m.Rows) == 0 {
		m.Cursor = 0
		return
	}
	if origin < 0 {
		origin = 0
	}
	if origin >= len(m.Rows) {
		origin = len(m.Rows) - 1
	}
	for distance := 0; distance < len(m.Rows); distance++ {
		next := origin + distance
		if next < len(m.Rows) && m.Rows[next].IsTaskRow() && m.Rows[next].Task != nil {
			m.Cursor = next
			return
		}
		previous := origin - distance
		if distance > 0 && previous >= 0 && m.Rows[previous].IsTaskRow() && m.Rows[previous].Task != nil {
			m.Cursor = previous
			return
		}
	}
	m.Cursor = origin
	m.ClampCursor()
}

// cancelCrossProjectLoad settles an abandoned refresh and invalidates any
// result already in flight. The last accepted snapshot remains renderable.

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

// crossResolveProject returns the loaded project matching the given ID by
// scanning m.population.projects. Returns nil when not found.
func (m *OutlinerModel) crossResolveProject(projectID string) *models.Project {
	for _, p := range m.Population.Projects {
		if p.ID == projectID {
			return p
		}
	}
	return nil
}

// handleCrossProjectReparent applies a ParentSelectedMsg in cross-project mode
// through the single-task coordinator (app Reparent). The application layer owns
// the parent-existence, type-compatibility, and cycle guards and aborts an
// invalid reparent before any commit, so no undo snapshot is recorded for a
// rejected change. A nil ParentID promotes the task to a root.
func (m *OutlinerModel) handleCrossProjectReparent(msg listsurface.ParentSelectedMsg) (tea.Model, tea.Cmd) {
	return m, m.taskItem.Reparent(msg.ReparentTask, msg.ParentID)
}
