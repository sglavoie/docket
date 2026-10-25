package config

import "github.com/sglavoie/docket/internal/models"

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
// (catalog and planning) without loading the whole
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
	}
}

// RuntimeDependencies constructs the domain-backed runtime composition for this
// manager's workspace, backed by the canonical per-domain-file store. It is the
// centralized startup construction point: callers obtain catalog and planning
// capabilities from the returned seam without loading config.Config.
func (m *Manager) RuntimeDependencies() *RuntimeDependencies {
	return NewRuntimeDependencies(m.domainStore())
}

// Next returns the persisted workspace-wide Next membership/order via
// the Planning dependency. Presence does not confer Next membership. Unlike
// RuntimeState.NextOrder it does not hide parked owners: this facade reads
// planning.json alone and has no catalog to test tags against, so it reports the
// persisted set whole. Projection consumers must read RuntimeState.
func (r *RuntimeDependencies) Next() ([]ProjectTaskLocator, error) {
	if r == nil || r.Planning == nil {
		return nil, nil
	}
	planning, err := r.Planning.LoadPlanning()
	if err != nil {
		return nil, err
	}
	return planning.GetNext(), nil
}

// TaskInNext reports whether the exact project-qualified task has a
// persisted order entry. It deliberately does not report workflow membership.
func (r *RuntimeDependencies) TaskInNext(locator ProjectTaskLocator) (bool, error) {
	if !locator.Valid() {
		return false, nil
	}
	order, err := r.Next()
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

// AvailableProjects returns every non-archived project regardless of its tags,
// sorted with pinned first. It is the store-backed counterpart of
// RuntimeState.AvailableProjects. The narrower TaskWorkspaceProjects population
// is deliberately not mirrored here: this facade has no task-projection
// consumer that would read it.
func (r *RuntimeDependencies) AvailableProjects(dataDir string) ([]*models.Project, error) {
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
