package config

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
)

// DomainStore is the composite of the three narrow domain-store interfaces. It is
// the single backing capability a RuntimeDependencies is built from. The
// production implementation is *DomainFileStore (one file per domain);
// tests may supply a fake that satisfies the three interfaces.
type DomainStore interface {
	WorkspaceCatalogStore
	PlanningStore
	MachineStore
}

// RuntimeDependencies is the narrow, domain-backed composition seam constructed
// at startup. It lets callers obtain each configuration domain's capability
// (catalog, planning, machine) without loading the whole
// config.Config facade. Every field is one of the narrow domain-store
// interfaces, so a caller depends only on the domain it actually needs and tests
// can inject per-domain fakes.
//
// It is the seam the remaining Config-facade migration children route callers
// through: callers that need durable Today/Next ordering read it via the
// Planning dependency rather than treating an order entry as workflow state.
type RuntimeDependencies struct {
	Catalog  WorkspaceCatalogStore
	Planning PlanningStore
	Machine  MachineStore
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
		Catalog:  store,
		Planning: store,
		Machine:  store,
	}
}

// RuntimeDependencies constructs the domain-backed runtime composition for this
// manager's workspace, backed by the canonical per-domain-file store. It is the
// centralized startup construction point: callers obtain catalog, planning, and
// machine capabilities from the returned seam without loading config.Config.
func (m *Manager) RuntimeDependencies() *RuntimeDependencies {
	return NewRuntimeDependencies(m.domainStore())
}

// NextOrder returns the persisted workspace-wide order of StateNext tasks via
// the Planning dependency. Presence does not confer Next membership.
func (r *RuntimeDependencies) NextOrder() ([]ProjectTaskLocator, error) {
	if r == nil || r.Planning == nil {
		return nil, nil
	}
	planning, err := r.Planning.LoadPlanning()
	if err != nil {
		return nil, err
	}
	return planning.NextOrder, nil
}

// TaskInNextOrder reports whether the exact project-qualified task has a
// persisted order entry. It deliberately does not report workflow membership.
func (r *RuntimeDependencies) TaskInNextOrder(locator ProjectTaskLocator) (bool, error) {
	if !locator.Valid() {
		return false, nil
	}
	order, err := r.NextOrder()
	if err != nil {
		return false, err
	}
	for _, item := range order {
		if item == locator {
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

// ActiveProjects returns non-archived projects sorted with pinned first.
func (r *RuntimeDependencies) ActiveProjects(dataDir string) ([]*models.Project, error) {
	all, err := r.projectsFromCatalog(dataDir)
	if err != nil {
		return nil, err
	}
	var active []*models.Project
	for _, p := range all {
		if !p.Archived {
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

// AllProjects returns every cataloged project in catalog order.
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
