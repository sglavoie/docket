package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// CrossScopeOpts controls which projects are selected and loaded by
// LoadCrossScopeTasks for the cross-project surfaces (Cross-Outliner,
// Cross-Kanban, Focus Board).
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

func newProjectLoadFailure(project *models.Project, err error) projectLoadFailure {
	name := "<unknown>"
	if project != nil && project.Name != "" {
		name = project.Name
	}
	detail := "unknown error"
	if err != nil {
		detail = err.Error()
	}
	return projectLoadFailure{
		Project: name,
		Detail:  fmt.Sprintf("failed to load project %s: %s", name, detail),
	}
}

// LoadCrossScopeTasks selects projects per opts, loads each via svc, and
// returns the flat task union, the successfully loaded projects, and the
// details of any projects that failed to load. Individual load failures are
// non-fatal: the offending project is skipped and its load error is returned
// for the caller to surface in logs or the UI.
func LoadCrossScopeTasks(cfg *config.Config, svc *ProjectService, opts CrossScopeOpts) (tasks []*models.Task, projects []*models.Project, failed []projectLoadFailure) {
	for _, p := range selectCrossScopeProjects(cfg, opts) {
		if !opts.IncludeSandbox && p.Sandbox {
			continue
		}
		if err := svc.Load(p); err != nil {
			failed = append(failed, newProjectLoadFailure(p, err))
			continue
		}
		projects = append(projects, p)
		tasks = append(tasks, p.Tasks...)
	}
	return tasks, projects, failed
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
