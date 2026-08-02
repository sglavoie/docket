package config

// Narrow store interfaces for each configuration domain. New code depends on
// only the domain it needs instead of the whole configuration surface.
// DomainFileStore (domain_file_store.go) is the canonical implementation that
// persists each domain to its own synced file.

// WorkspaceCatalogStore loads and saves the workspace catalog.
type WorkspaceCatalogStore interface {
	LoadCatalog() (*WorkspaceCatalog, error)
	SaveCatalog(*WorkspaceCatalog) error
}

// PlanningStore loads and saves planning collections.
type PlanningStore interface {
	LoadPlanning() (*PlanningState, error)
	SavePlanning(*PlanningState) error
}

// MachineStore loads and saves machine-local state.
type MachineStore interface {
	LoadMachine() (*MachineState, error)
	SaveMachine(*MachineState) error
}

// Domains bundles the three explicit configuration aggregates used by runtime
// composition and transactions. It is not a persistence facade: callers read
// and commit the owning domain directly.
type Domains struct {
	Catalog  *WorkspaceCatalog
	Planning *PlanningState
	Machine  *MachineState
}

// NewDomains returns a fully defaulted explicit domain bundle.
func NewDomains() *Domains {
	return &Domains{
		Catalog:  NewWorkspaceCatalog(),
		Planning: NewPlanningState(),
		Machine:  NewMachineState(),
	}
}

// RemoveProject fans a project deletion out to every domain after the catalog
// accepts it. The catalog check happens first so a protected system Inbox
// rejection cannot partially strip planning references.
func (d *Domains) RemoveProject(id, slug string) (bool, error) {
	if d == nil {
		return false, nil
	}
	changed := false
	if d.Catalog != nil {
		catalogChanged, err := d.Catalog.RemoveProject(id, slug)
		if err != nil {
			return false, err
		}
		changed = catalogChanged || changed
	}
	if d.Planning != nil {
		changed = d.Planning.RemoveProjectReferences(id, slug) || changed
	}
	return changed, nil
}

// RemoveProjectReferences is the compatibility form for callers that only
// need best-effort cleanup. It fails closed when RemoveProject rejects the
// protected system Inbox. Lifecycle adapters should call RemoveProject.
func (d *Domains) RemoveProjectReferences(id, slug string) bool {
	changed, _ := d.RemoveProject(id, slug)
	return changed
}
