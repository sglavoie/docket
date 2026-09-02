package config

import "fmt"

// This file owns the per-domain entry points: the catalog, planning, lens, and
// machine loads and saves the narrow store interfaces are satisfied by. Each is
// a thin policy layer over the shared load/saveObserved pair in
// domain_file_store_io.go, and the lens read precedence every machine reader
// must share lives here beside them.

// LoadCatalog implements WorkspaceCatalogStore.
func (s *DomainFileStore) LoadCatalog() (*WorkspaceCatalog, error) {
	c, _, err := s.LoadCatalogChecked()
	return c, err
}

// LoadCatalogChecked loads the catalog and returns the revision to pass back to
// SaveCatalogChecked for optimistic concurrency.
func (s *DomainFileStore) LoadCatalogChecked() (*WorkspaceCatalog, string, error) {
	out := NewWorkspaceCatalog()
	rev, err := s.load(domainCatalog, out)
	if err != nil {
		return nil, "", err
	}
	// Color slots are cosmetic and explicitly self-healing. Validate every
	// structural field while allowing EnsureProjectColorSlots at startup or the
	// next catalog mutation to repair and persist an out-of-range slot.
	if err := out.validateForLoad(); err != nil {
		return nil, rev, fmt.Errorf("invalid workspace catalog: %w", err)
	}
	return out, rev, nil
}

// bindWorkspaceIDFromCatalog resolves the synced catalog before any machine
// path is requested. A machine-only artifact cannot establish workspace
// identity because doing so would let a stale or foreign local file select its
// own namespace.
func (s *DomainFileStore) bindWorkspaceIDFromCatalog() error {
	if s.workspaceID != "" {
		if err := validateWorkspaceID(s.workspaceID); err != nil {
			return err
		}
	}
	_, rev, err := s.LoadCatalogChecked()
	if err != nil {
		return err
	}
	if rev == "" {
		return fmt.Errorf("%w: workspace catalog must exist before machine-state access", ErrWorkspaceIDRequired)
	}
	_, err = s.validatedWorkspaceID()
	return err
}

// SaveCatalog implements WorkspaceCatalogStore (last-writer-wins).
func (s *DomainFileStore) SaveCatalog(catalog *WorkspaceCatalog) error {
	return s.SaveCatalogChecked(catalog, "")
}

// SaveCatalogChecked saves the catalog, failing with ErrStaleObject when the
// on-disk revision no longer matches rev.
func (s *DomainFileStore) SaveCatalogChecked(catalog *WorkspaceCatalog, rev string) error {
	_, err := s.saveCatalogObserved(catalog, rev, false)
	return err
}

// saveCatalogObserved is SaveCatalogChecked against a baseline the caller
// observed, returning the revision it wrote.
func (s *DomainFileStore) saveCatalogObserved(catalog *WorkspaceCatalog, rev string, baselineObserved bool) (string, error) {
	if catalog == nil {
		return "", fmt.Errorf("config: nil workspace catalog")
	}
	if err := catalog.Validate(); err != nil {
		return "", fmt.Errorf("invalid workspace catalog: %w", err)
	}
	return s.saveObserved(domainCatalog, catalog, rev, baselineObserved)
}

// LoadPlanning implements PlanningStore.
func (s *DomainFileStore) LoadPlanning() (*PlanningState, error) {
	p, _, err := s.LoadPlanningChecked()
	return p, err
}

// LoadPlanningChecked loads planning state and its revision.
func (s *DomainFileStore) LoadPlanningChecked() (*PlanningState, string, error) {
	out := NewPlanningState()
	rev, err := s.load(domainPlanning, out)
	if err != nil {
		return nil, "", err
	}
	return out, rev, nil
}

// SavePlanning implements PlanningStore (last-writer-wins).
func (s *DomainFileStore) SavePlanning(planning *PlanningState) error {
	return s.SavePlanningChecked(planning, "")
}

// SavePlanningChecked saves planning state with an optimistic concurrency check.
func (s *DomainFileStore) SavePlanningChecked(planning *PlanningState, rev string) error {
	_, err := s.savePlanningObserved(planning, rev, false)
	return err
}

// savePlanningObserved is SavePlanningChecked against a baseline the caller
// observed, returning the revision it wrote.
func (s *DomainFileStore) savePlanningObserved(planning *PlanningState, rev string, baselineObserved bool) (string, error) {
	if planning == nil {
		return "", fmt.Errorf("config: nil planning state")
	}
	if err := s.bindWorkspaceIDFromCatalog(); err != nil {
		return "", err
	}
	return s.saveObserved(domainPlanning, planning, rev, baselineObserved)
}

// LoadLensDefinitionsChecked loads the portable lens definitions and their
// revision. A workspace with no lenses.json yields the undefined collection and
// an empty revision, which is what makes the machine-local set the fallback
// rather than an error.
func (s *DomainFileStore) LoadLensDefinitionsChecked() (*WorkspaceLensDefinitions, string, error) {
	out := NewWorkspaceLensDefinitions()
	rev, err := s.load(domainLenses, out)
	if err != nil {
		return nil, "", err
	}
	if err := out.Validate(); err != nil {
		return nil, rev, fmt.Errorf("invalid workspace lens definitions: %w", err)
	}
	return out, rev, nil
}

// SaveLensDefinitionsChecked saves the portable lens definitions with an
// optimistic concurrency check. An undefined collection is refused rather than
// written: the file's whole meaning is "this workspace owns the definitions", so
// persisting a nil list would publish "every lens was deleted" by accident.
func (s *DomainFileStore) SaveLensDefinitionsChecked(lenses *WorkspaceLensDefinitions, rev string) error {
	_, err := s.saveLensDefinitionsObserved(lenses, rev, false)
	return err
}

// saveLensDefinitionsObserved is SaveLensDefinitionsChecked against a baseline
// the caller observed, returning the revision it wrote.
func (s *DomainFileStore) saveLensDefinitionsObserved(lenses *WorkspaceLensDefinitions, rev string, baselineObserved bool) (string, error) {
	if lenses == nil {
		return "", fmt.Errorf("config: nil workspace lens definitions")
	}
	if !lenses.Defined() {
		return "", fmt.Errorf("config: workspace lens definitions are undefined; nothing to publish")
	}
	if err := s.bindWorkspaceIDFromCatalog(); err != nil {
		return "", err
	}
	return s.saveObserved(domainLenses, lenses, rev, baselineObserved)
}

// LoadMachine implements MachineStore.
func (s *DomainFileStore) LoadMachine() (*MachineState, error) {
	m, _, err := s.LoadMachineChecked()
	return m, err
}

// LoadMachineChecked loads machine state and its revision. Lens definitions are
// dual-sourced here, at the one seam every reader already passes through: a
// workspace that defines its own set wins, and one that does not leaves the
// machine-local set exactly as it was loaded. The revision returned is
// machine.json's alone, because that is the file a checked machine save writes.
func (s *DomainFileStore) LoadMachineChecked() (*MachineState, string, error) {
	if err := s.bindWorkspaceIDFromCatalog(); err != nil {
		return nil, "", err
	}
	out := NewMachineState()
	rev, err := s.load(domainMachine, out)
	if err != nil {
		return nil, "", err
	}
	if _, err := s.applyPortableLensDefinitions(out); err != nil {
		return nil, "", err
	}
	return out, rev, nil
}

// applyPortableLensDefinitions is the lens read precedence, factored out because
// machine state has two readers: this store's checked load and the bundle load
// in loadDomainBundleWithRevisions, which decodes machine.json directly. Both
// must apply the same rule or the definitions would be portable on one path and
// machine-local on the other.
//
// A workspace that defines its own collection wins; one that does not leaves the
// machine-local set exactly as it was decoded. Theme and ActiveLensID are never
// touched: they are the two halves that stay machine-local.
//
// The lens revision observed is returned even when the workspace defines no
// collection, because "absent when I looked" is itself the baseline a later
// checked lens save must be validated against: it is what turns a create into a
// require-absent contract rather than a blind overwrite.
func (s *DomainFileStore) applyPortableLensDefinitions(machine *MachineState) (string, error) {
	lenses, rev, err := s.LoadLensDefinitionsChecked()
	if err != nil {
		return "", err
	}
	if lenses.Defined() {
		machine.NamedLenses = lenses.Lenses.Clone()
	}
	return rev, nil
}

// LoadPersistedMachineLenses returns the lens list machine.json actually holds,
// distinguishing an omitted named_lenses key (nil) from an explicitly persisted
// one (non-nil, possibly empty). It decodes into a zero MachineState rather than
// NewMachineState's seeded value, which is the only way to tell "this machine
// never recorded a lens set" from "this machine recorded exactly the starters".
// Adoption needs that split: it is what keeps a machine joining a workspace
// whose owner deleted every lens from reseeding the starters into it.
func (s *DomainFileStore) LoadPersistedMachineLenses() (NamedLensList, error) {
	if err := s.bindWorkspaceIDFromCatalog(); err != nil {
		return nil, err
	}
	var persisted MachineState
	if _, err := s.load(domainMachine, &persisted); err != nil {
		return nil, err
	}
	return persisted.NamedLenses, nil
}

// SaveMachine implements MachineStore (last-writer-wins).
func (s *DomainFileStore) SaveMachine(machine *MachineState) error {
	return s.SaveMachineChecked(machine, "")
}

// SaveMachineChecked saves machine state with an optimistic concurrency check.
func (s *DomainFileStore) SaveMachineChecked(machine *MachineState, rev string) error {
	return s.saveMachineObserved(machine, rev, false)
}

// saveMachineObserved is SaveMachineChecked against a baseline the caller
// observed. Machine-local state has no remote writer, so unlike the synced
// domains no caller retains the revision it wrote.
func (s *DomainFileStore) saveMachineObserved(machine *MachineState, rev string, baselineObserved bool) error {
	if machine == nil {
		return fmt.Errorf("config: nil machine state")
	}
	if err := s.bindWorkspaceIDFromCatalog(); err != nil {
		return err
	}
	_, err := s.saveObserved(domainMachine, machine, rev, baselineObserved)
	return err
}
