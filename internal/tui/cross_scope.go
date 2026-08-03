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
	// ActiveOnly restricts selection to RuntimeState.ActiveProjects, which
	// admits exactly config.IsActiveContextMember — the same rule a context
	// scope resolves by, so a scoped Outliner load can never be asked for a
	// project this selection drops. When false, the selection falls back to
	// AllLoadableProjects (archived excluded only).
	ActiveOnly bool
}

type projectLoadFailure struct {
	Detail string
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
		Detail: fmt.Sprintf("failed to load project %s: %s", name, detail),
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

func selectCrossScopeProjects(runtime *config.RuntimeState, opts CrossScopeOpts) []*models.Project {
	if opts.ActiveOnly {
		return runtime.ActiveProjects()
	}
	return runtime.AllLoadableProjects()
}
