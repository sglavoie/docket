package outliner

import (
	"fmt"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/workspace"
)

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

// ComputeWorkspacePulse selects and loads the base task workspace, then hands
// the loaded projects to the counting half in internal/tui/pulse.
//
// The split is what keeps that package at L0: the snapshot loader below is
// shell-and-outliner infrastructure, so pulse counts what it is given rather
// than reaching up for it. Partial failure behavior is unchanged — a project
// that fails to load is warned about here and is simply absent from the slice
// the counter walks.
//
// The loader is the caller's to retain. The pulse is recounted after every
// commit, so a loader built per call re-read and re-parsed every project each
// time; a retained one re-reads only the files that changed. It is not safe for
// concurrent use, so it must be owned by the one goroutine that recounts the
// pulse. A nil loader reads everything fresh.
func ComputeWorkspacePulse(rs *config.RuntimeState, todayIDs map[taskref.TaskIdentity]bool, now time.Time, loader *workspace.Loader) pulse.WorkspacePulse {
	if rs == nil {
		return pulse.WorkspacePulse{}
	}
	_, projects, failed := LoadCrossScopeSelection(rs.TaskWorkspaceProjects(), loader)
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
