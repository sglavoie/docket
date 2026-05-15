package tui

import (
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

// LoadCrossScopeTasks selects projects per opts, loads each via svc, and
// returns the flat task union, the successfully loaded projects, and the
// names of any projects that failed to load. Individual load failures are
// non-fatal: the offending project is skipped and its name is appended to
// failed for the caller to surface as a status warning.
func LoadCrossScopeTasks(cfg *config.Config, svc *ProjectService, opts CrossScopeOpts) (tasks []*models.Task, projects []*models.Project, failed []string) {
	for _, p := range selectCrossScopeProjects(cfg, opts) {
		if !opts.IncludeSandbox && p.Sandbox {
			continue
		}
		if err := svc.Load(p); err != nil {
			failed = append(failed, p.Name)
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
