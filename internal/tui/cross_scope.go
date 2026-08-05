package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workspace"
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
	ProjectFilter ProjectFilterState
}

type projectLoadFailure struct {
	ProjectID string
	Detail    string
}

// diagnosticToLoadFailure adapts a workspace snapshot diagnostic into the
// cross-scope failure shape the TUI already surfaces, preserving the historical
// "failed to load project X: <err>" detail format.
func diagnosticToLoadFailure(diag workspace.Diagnostic) projectLoadFailure {
	name := diag.ProjectName
	if name == "" {
		name = "<unknown>"
	}
	detail := diag.Detail
	if detail == "" {
		detail = "unknown error"
	}
	return projectLoadFailure{
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
func LoadCrossScopeTasks(runtime *config.RuntimeState, service *ProjectService, opts CrossScopeOpts) (tasks []*models.Task, projects []*models.Project, failed []projectLoadFailure) {
	if service == nil {
		service = NewProjectService()
	}
	return loadCrossScopeSelection(selectCrossScopeProjects(runtime, opts), service.workspaceLoader)
}

// loadCrossScopeSelection loads an already-selected project set into an immutable
// workspace snapshot and returns the flat task union, the successfully loaded
// projects, and any load-failure details. It is the source-agnostic core shared
// by the config-facade LoadCrossScopeTasks and the domain-snapshot open-loops
// counter, so both select projects from their own domain source and load them
// identically.
func loadCrossScopeSelection(selected []*models.Project, loader *workspace.Loader) (tasks []*models.Task, projects []*models.Project, failed []projectLoadFailure) {
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
		copyCatalogMetadata(project, catalogByID[project.ID])
	}
	return snap.Tasks(), projects, failed
}

// selectCrossScopeProjects is the one place task projections choose a project
// population. The active-only selection is the base task workspace: a parked
// project is absent from the loaded snapshot, so no mode, filter, pinned
// section, or planning projection downstream can reach its tasks.
func selectCrossScopeProjects(runtime *config.RuntimeState, opts CrossScopeOpts) []*models.Project {
	var selected []*models.Project
	switch {
	case hasActiveProjectFilters(&opts.ProjectFilter):
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
func filteredProjectPopulation(runtime *config.RuntimeState, filter ProjectFilterState) []*models.Project {
	candidates := runtime.AvailableProjects()
	if filter.IncludeArchived {
		candidates = append(candidates, runtime.ArchivedProjects()...)
	}
	selected := make([]*models.Project, 0, len(candidates))
	for _, project := range candidates {
		if projectFilterAdmits(project, filter) {
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
