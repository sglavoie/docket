package config

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

// DomainStore is the composite of the five narrow domain-store interfaces. It is
// the single backing capability a RuntimeDependencies is built from. The
// production implementation is *DomainFileStore (one synced file per domain);
// tests may supply a fake that satisfies the five interfaces.
type DomainStore interface {
	WorkspaceCatalogStore
	PlanningStore
	SettingsStore
	AutomationStore
	MachineStore
}

// RuntimeDependencies is the narrow, domain-backed composition seam constructed
// at startup. It lets callers obtain each configuration domain's capability
// (catalog, planning, settings, automation, machine) without loading the whole
// config.Config facade. Every field is one of the narrow domain-store
// interfaces, so a caller depends only on the domain it actually needs and tests
// can inject per-domain fakes.
//
// It is the seam the remaining Config-facade migration children route callers
// through: today a caller that needs the Focus Queue reads it via the Planning
// dependency (see FocusQueue) instead of dereferencing config.Config or the
// retired per-task Active flag.
type RuntimeDependencies struct {
	Catalog    WorkspaceCatalogStore
	Planning   PlanningStore
	Settings   SettingsStore
	Automation AutomationStore
	Machine    MachineStore
}

// NewRuntimeDependencies builds the runtime composition from a single backing
// domain store, wiring every domain to it. Passing a *DomainFileStore is the
// production path; passing a fake that satisfies DomainStore is the test seam. A
// nil store yields a nil result.
func NewRuntimeDependencies(store DomainStore) *RuntimeDependencies {
	if store == nil {
		return nil
	}
	return &RuntimeDependencies{
		Catalog:    store,
		Planning:   store,
		Settings:   store,
		Automation: store,
		Machine:    store,
	}
}

// RuntimeDependencies constructs the domain-backed runtime composition for this
// manager's workspace, backed by the canonical per-domain-file store. It is the
// centralized startup construction point: callers obtain catalog, planning,
// settings, automation, and machine capabilities from the returned seam without
// loading config.Config.
func (m *Manager) RuntimeDependencies() *RuntimeDependencies {
	return NewRuntimeDependencies(m.domainStore())
}

// FocusQueue returns the persisted Focus Queue via the Planning dependency. It
// is the domain-backed replacement for reading config.Config.FocusQueue or the
// retired per-task Active compatibility field: Focus Queue membership is loaded
// from the owning planning domain, not derived from task state.
func (r *RuntimeDependencies) FocusQueue() ([]FocusQueueItem, error) {
	if r == nil || r.Planning == nil {
		return nil, nil
	}
	planning, err := r.Planning.LoadPlanning()
	if err != nil {
		return nil, err
	}
	return planning.FocusQueue, nil
}

// TaskInFocusQueue reports whether the task is a member of the persisted Focus
// Queue, resolved through the Planning dependency. It mirrors the membership
// check callers previously performed against config.Config.FocusQueue.
func (r *RuntimeDependencies) TaskInFocusQueue(taskID string) (bool, error) {
	if taskID == "" {
		return false, nil
	}
	queue, err := r.FocusQueue()
	if err != nil {
		return false, err
	}
	for _, item := range queue {
		if item.TaskID == taskID {
			return true, nil
		}
	}
	return false, nil
}

// projectsFromCatalog loads the workspace catalog and rehydrates every cataloged
// project into a models.Project with its Path derived from dataDir. It mirrors
// the reconstitution config.Config performs on a fresh (command-line) load,
// where the in-memory project slice starts empty so every project is rebuilt via
// CatalogProject.ToModel — the result therefore matches cfg.Projects for the
// command-layer callers this seam is migrating.
func (r *RuntimeDependencies) projectsFromCatalog(dataDir string) ([]*models.Project, error) {
	if r == nil || r.Catalog == nil {
		return nil, nil
	}
	catalog, err := r.Catalog.LoadCatalog()
	if err != nil {
		return nil, err
	}
	projects := make([]*models.Project, 0, len(catalog.Projects))
	for _, cp := range catalog.Projects {
		projects = append(projects, cp.ToModel(dataDir))
	}
	return projects, nil
}

// ActiveProjects returns the non-archived, non-sandbox, non-internal projects
// sorted with pinned first then alphabetically, mirroring
// config.Config.GetActiveProjects but sourced from the catalog domain.
func (r *RuntimeDependencies) ActiveProjects(dataDir string) ([]*models.Project, error) {
	all, err := r.projectsFromCatalog(dataDir)
	if err != nil {
		return nil, err
	}
	var active []*models.Project
	for _, p := range all {
		if !p.Archived && !p.Sandbox && !p.Internal {
			active = append(active, p)
		}
	}
	sortProjectsWithPinnedFirst(active)
	return active, nil
}

// AllLoadableProjects returns all non-archived projects (including internal
// ones) sorted with pinned first, mirroring
// config.Config.GetAllLoadableProjects but sourced from the catalog domain.
func (r *RuntimeDependencies) AllLoadableProjects(dataDir string) ([]*models.Project, error) {
	all, err := r.projectsFromCatalog(dataDir)
	if err != nil {
		return nil, err
	}
	var projects []*models.Project
	for _, p := range all {
		if !p.Archived {
			projects = append(projects, p)
		}
	}
	sortProjectsWithPinnedFirst(projects)
	return projects, nil
}

// Project returns the cataloged project whose ID or slug matches idOrSlug, or
// nil when none matches (mirroring config.Config.GetProject's nil-not-found
// contract). An empty idOrSlug never matches.
func (r *RuntimeDependencies) Project(dataDir, idOrSlug string) (*models.Project, error) {
	if idOrSlug == "" {
		return nil, nil
	}
	all, err := r.projectsFromCatalog(dataDir)
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if p.ID == idOrSlug || p.Slug == idOrSlug {
			return p, nil
		}
	}
	return nil, nil
}

// SandboxProject returns the sandbox project, or nil when the workspace has none
// (mirroring config.Config.GetSandboxProject) sourced from the catalog domain.
func (r *RuntimeDependencies) SandboxProject(dataDir string) (*models.Project, error) {
	all, err := r.projectsFromCatalog(dataDir)
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if p.Sandbox {
			return p, nil
		}
	}
	return nil, nil
}

// AllProjects returns every cataloged project (archived, sandbox, and internal
// included) in catalog order, mirroring the raw config.Config.Projects slice for
// the command consumers that need the unfiltered set (e.g. doctor's cross-project
// reference audit). It is sourced from the catalog domain rather than the facade.
func (r *RuntimeDependencies) AllProjects(dataDir string) ([]*models.Project, error) {
	return r.projectsFromCatalog(dataDir)
}

// CustomContextNames returns the sorted custom-context names via the Machine
// dependency. It is the domain-backed replacement for
// config.Config.GetCustomContextNames: custom contexts are machine-local
// navigation state, so the names are read from the owning machine domain rather
// than the config.Config facade. A nil map yields an empty (non-nil) slice,
// matching the facade contract.
func (r *RuntimeDependencies) CustomContextNames() ([]string, error) {
	if r == nil || r.Machine == nil {
		return []string{}, nil
	}
	machine, err := r.Machine.LoadMachine()
	if err != nil {
		return nil, err
	}
	if machine.CustomContexts == nil {
		return []string{}, nil
	}
	names := make([]string, 0, len(machine.CustomContexts))
	for name := range machine.CustomContexts {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// TaskTemplates returns the persisted task templates via the Automation
// dependency, mirroring reads of config.Config.TaskTemplates. Templates are
// automation configuration, so they are loaded from the owning automation domain
// instead of the config.Config facade.
func (r *RuntimeDependencies) TaskTemplates() ([]template.TaskTemplate, error) {
	if r == nil || r.Automation == nil {
		return nil, nil
	}
	automation, err := r.Automation.LoadAutomation()
	if err != nil {
		return nil, err
	}
	return automation.TaskTemplates, nil
}
