package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workspace"
)

// CrossScopeOpts controls which projects are selected and loaded by
// LoadCrossScopeTasks for the cross-project surfaces (Cross-Outliner,
// Cross-Kanban, Today).
type CrossScopeOpts struct {
	// IncludeSandbox includes Sandbox projects in the result when true.
	// Defaults to false; matches existing per-caller behavior.
	IncludeSandbox bool

	// ActiveOnly restricts selection to cfg.GetActiveProjects() (which
	// already excludes archived/sandbox/internal). When false, the
	// selection falls back to cfg.GetAllLoadableProjects() (archived
	// excluded only). Ignored when CycleProjectsOnly is true.
	ActiveOnly bool

	// CycleProjectsOnly restricts selection to the configured cycle
	// projects for ContextKey. Takes precedence over ActiveOnly.
	CycleProjectsOnly bool

	// ContextKey is the filter-context key passed to
	// cfg.GetCycleProjectsForContext when CycleProjectsOnly is true.
	ContextKey string
}

type projectLoadFailure struct {
	Project string
	Detail  string
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
		Project: name,
		Detail:  fmt.Sprintf("failed to load project %s: %s", name, detail),
	}
}

// LoadCrossScopeTasks selects projects per opts, loads them once into an
// immutable workspace snapshot, and returns the flat task union, the
// successfully loaded projects, and the details of any projects that failed to
// load. Individual load failures are non-fatal: the offending project is
// skipped and surfaced as a diagnostic for the caller to log or display.
//
// A fresh snapshot loader is used per call so the returned projects are the
// caller's own freshly loaded pointers, keeping mutation semantics intact for
// callers that go on to edit and save a project.
func LoadCrossScopeTasks(cfg *config.Config, _ *ProjectService, opts CrossScopeOpts) (tasks []*models.Task, projects []*models.Project, failed []projectLoadFailure) {
	var selected []*models.Project
	for _, p := range selectCrossScopeProjects(cfg, opts) {
		if !opts.IncludeSandbox && p.Sandbox {
			continue
		}
		selected = append(selected, p)
	}

	snap := workspace.NewLoader().Build(selected)
	for _, diag := range snap.Diagnostics() {
		failed = append(failed, diagnosticToLoadFailure(diag))
	}
	return snap.Tasks(), snap.Projects(), failed
}

func selectCrossScopeProjects(cfg *config.Config, opts CrossScopeOpts) []*models.Project {
	if opts.CycleProjectsOnly {
		return cfg.GetCycleProjectsForContext(opts.ContextKey)
	}
	if opts.ActiveOnly {
		return cfg.GetActiveProjects()
	}
	return cfg.GetAllLoadableProjects()
}
