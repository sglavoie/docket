package config

import "fmt"

// loadDomainBundleWithRevisions loads only the three live runtime domains.
// Retired v4 settings and automation artifacts are deliberately ignored here;
// migration and doctor code read those files through their legacy DTOs.
func loadDomainBundleWithRevisions(store *DomainFileStore) (*Domains, map[domainKind]string, error) {
	if store == nil {
		return nil, nil, fmt.Errorf("configuration domain store is required")
	}

	revisions := make(map[domainKind]string, len(syncedDomainKinds)+1)
	catalog := NewWorkspaceCatalog()
	catalogRevision, err := store.load(domainCatalog, catalog)
	if err != nil {
		return nil, nil, err
	}
	revisions[domainCatalog] = catalogRevision

	planning := NewPlanningState()
	planningRevision, err := store.load(domainPlanning, planning)
	if err != nil {
		return nil, nil, err
	}
	revisions[domainPlanning] = planningRevision

	machine := NewMachineState()
	if catalogRevision == "" {
		if _, err := store.bindCatalogPayload(catalog); err != nil {
			return nil, nil, err
		}
	} else {
		if _, err := store.load(domainMachine, machine); err != nil {
			return nil, nil, err
		}
		// The bundle decodes machine.json directly rather than through
		// LoadMachineChecked, so it has to apply the portable lens precedence
		// itself. This is the load every frontend actually reads, which is why
		// the lens revision it observed is carried out as a baseline: a later
		// lens commit is checked against what this load saw, not against
		// whatever landed in between.
		lensRevision, err := store.applyPortableLensDefinitions(machine)
		if err != nil {
			return nil, nil, err
		}
		revisions[domainLenses] = lensRevision
	}

	domains := &Domains{Catalog: catalog, Planning: planning, Machine: machine}
	if err := validateDomainBundleForLoad(domains); err != nil {
		return nil, nil, err
	}
	return domains, revisions, nil
}
