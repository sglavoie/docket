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

// SettingsStore loads and saves portable settings.
type SettingsStore interface {
	LoadSettings() (*Settings, error)
	SaveSettings(*Settings) error
}

// AutomationStore loads and saves automation configuration.
type AutomationStore interface {
	LoadAutomation() (*AutomationConfig, error)
	SaveAutomation(*AutomationConfig) error
}

// MachineStore loads and saves machine-local state.
type MachineStore interface {
	LoadMachine() (*MachineState, error)
	SaveMachine(*MachineState) error
}

// Domains bundles the five explicit configuration aggregates used by runtime
// composition and transactions. It is not a persistence facade: callers read
// and commit the owning domain directly.
type Domains struct {
	Catalog    *WorkspaceCatalog
	Planning   *PlanningState
	Settings   *Settings
	Automation *AutomationConfig
	Machine    *MachineState
}

// NewDomains returns a fully defaulted explicit domain bundle.
func NewDomains() *Domains {
	return &Domains{
		Catalog:    NewWorkspaceCatalog(),
		Planning:   NewPlanningState(),
		Settings:   NewSettings(),
		Automation: NewAutomationConfig(),
		Machine:    NewMachineState(),
	}
}

// RemoveProjectReferences fans a project deletion out to every domain, so each
// aggregate strips only the references it owns. It returns true when any domain
// changed.
func (d *Domains) RemoveProjectReferences(id, slug string) bool {
	if d == nil {
		return false
	}
	changed := false
	if d.Catalog != nil {
		changed = d.Catalog.RemoveProjectReferences(id, slug) || changed
	}
	if d.Planning != nil {
		changed = d.Planning.RemoveProjectReferences(id, slug) || changed
	}
	if d.Automation != nil {
		changed = d.Automation.RemoveProjectReferences(id, slug) || changed
	}
	if d.Machine != nil {
		changed = d.Machine.RemoveProjectReferences(id, slug) || changed
	}
	return changed
}
