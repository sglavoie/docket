package config

import (
	"fmt"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

const previousConfigVersion = 3

// loadDomainBundleWithRevisions loads a complete split-domain bundle without
// allowing per-domain compatibility hooks to write independently. It applies
// schema and payload cleanup to the fresh candidate, validates the complete
// result, and only then commits every affected synced payload in one checked
// artifact transaction.
func loadDomainBundleWithRevisions(store *DomainFileStore) (*Domains, map[domainKind]string, error) {
	if store == nil {
		return nil, nil, fmt.Errorf("configuration domain store is required")
	}

	domains, revisions, machineRevision, err := loadRawDomainBundle(store)
	if err != nil {
		return nil, nil, err
	}

	sourceVersion := domains.Catalog.SchemaVersion
	if sourceVersion == CurrentConfigVersion {
		if validationErr := validateCanonicalCurrentInputs(domains); validationErr != nil {
			return nil, nil, validationErr
		}
	}
	changedKinds, err := migrateLoadedDomainBundle(domains)
	if err != nil {
		return nil, nil, err
	}
	if validationErr := validateDomainBundleForLoad(domains); validationErr != nil {
		return nil, nil, validationErr
	}
	if sourceVersion == previousConfigVersion {
		if validationErr := validateCanonicalAutomation(domains.Automation); validationErr != nil {
			return nil, nil, validationErr
		}
	}

	if len(changedKinds) > 0 {
		encoded, encodeErr := encodeMigratedDomains(store, domains, revisions, changedKinds)
		if encodeErr != nil {
			return nil, nil, encodeErr
		}
		commit := storage.NewCommitService(TransactionsDir(store.DataDir()), []string{store.DataDir()})
		changes := make([]storage.ArtifactChange, 0, len(encoded))
		for _, artifact := range encoded {
			changes = append(changes, artifact.StorageChange())
		}
		if _, commitErr := commit.CommitArtifacts(changes); commitErr != nil {
			return nil, nil, fmt.Errorf("failed to commit split-domain schema migration: %w", commitErr)
		}
		for kind, artifact := range encoded {
			revisions[kind] = revisionOf(artifact.Data)
		}
	}

	// Machine TUI cleanup is not part of the shared payload schema and lives
	// outside the synced workspace transaction. Its candidate was nevertheless
	// loaded and validated before the synced commit.
	machineChanged, err := domains.Machine.MigrateTUIState()
	if err != nil {
		return nil, nil, err
	}
	if machineChanged {
		if _, err := store.persistLoadedMigration(domainMachine, domains.Machine, machineRevision); err != nil {
			return nil, nil, err
		}
	}

	return domains, revisions, nil
}

func validateCanonicalCurrentInputs(domains *Domains) error {
	for i := range domains.Automation.TaskTemplates {
		if err := domains.Automation.TaskTemplates[i].ValidateForAuthoring(); err != nil {
			return fmt.Errorf("invalid automation domain: task template %d: %w", i, err)
		}
	}
	for i := range domains.Automation.WorkflowRules {
		if err := domains.Automation.WorkflowRules[i].ValidateForAuthoring(); err != nil {
			return fmt.Errorf("invalid automation domain: workflow rule %d: %w", i, err)
		}
	}
	return nil
}

func loadRawDomainBundle(store *DomainFileStore) (*Domains, map[domainKind]string, string, error) {
	revisions := make(map[domainKind]string, len(syncedDomainKinds))

	catalog := NewWorkspaceCatalog()
	catalogRevision, err := store.load(domainCatalog, catalog)
	if err != nil {
		return nil, nil, "", err
	}
	revisions[domainCatalog] = catalogRevision

	planning := NewPlanningState()
	planningRevision, err := store.load(domainPlanning, planning)
	if err != nil {
		return nil, nil, "", err
	}
	revisions[domainPlanning] = planningRevision

	settings := NewSettings()
	settingsRevision, err := store.load(domainSettings, settings)
	if err != nil {
		return nil, nil, "", err
	}
	revisions[domainSettings] = settingsRevision

	automation := NewAutomationConfig()
	automationRevision, err := store.load(domainAutomation, automation)
	if err != nil {
		return nil, nil, "", err
	}
	revisions[domainAutomation] = automationRevision

	machine := NewMachineState()
	if catalogRevision == "" {
		if _, bindErr := store.bindCatalogPayload(catalog); bindErr != nil {
			return nil, nil, "", bindErr
		}
		return &Domains{
			Catalog:    catalog,
			Planning:   planning,
			Settings:   settings,
			Automation: automation,
			Machine:    machine,
		}, revisions, "", nil
	}
	if migrationErr := store.migrateLegacyMachineState(); migrationErr != nil {
		return nil, nil, "", migrationErr
	}
	machineRevision, err := store.load(domainMachine, machine)
	if err != nil {
		return nil, nil, "", err
	}

	return &Domains{
		Catalog:    catalog,
		Planning:   planning,
		Settings:   settings,
		Automation: automation,
		Machine:    machine,
	}, revisions, machineRevision, nil
}

func migrateLoadedDomainBundle(domains *Domains) ([]domainKind, error) {
	if domains == nil || domains.Catalog == nil || domains.Planning == nil ||
		domains.Settings == nil || domains.Automation == nil || domains.Machine == nil {
		return nil, fmt.Errorf("all configuration domains are required")
	}
	if domains.Catalog.SchemaVersion > CurrentConfigVersion {
		return nil, fmt.Errorf(
			"workspace payload schema version %d not supported, please upgrade tt",
			domains.Catalog.SchemaVersion,
		)
	}

	switch domains.Catalog.SchemaVersion {
	case previousConfigVersion:
		if _, err := domains.Planning.MigrateTUIState(); err != nil {
			return nil, err
		}
		if _, err := domains.Settings.MigrateTUIState(); err != nil {
			return nil, err
		}
		domains.Automation.MigrateTaskTemplateStates()
		domains.Automation.SanitizeRetiredWorkflowActions()

		rules, _, err := workflow.MigrateLegacyLifecycleRules(domains.Automation.WorkflowRules)
		if err != nil {
			return nil, fmt.Errorf("automation workflow migration: %w", err)
		}
		domains.Automation.WorkflowRules = rules
		domains.Catalog.SchemaVersion = CurrentConfigVersion
		return append([]domainKind(nil), syncedDomainKinds...), nil

	case CurrentConfigVersion:
		var changed []domainKind
		if planningChanged, err := domains.Planning.MigrateTUIState(); err != nil {
			return nil, err
		} else if planningChanged {
			changed = append(changed, domainPlanning)
		}
		if settingsChanged, err := domains.Settings.MigrateTUIState(); err != nil {
			return nil, err
		} else if settingsChanged {
			changed = append(changed, domainSettings)
		}
		if domains.Automation.MigrateTaskTemplateStates() {
			changed = append(changed, domainAutomation)
		}
		return changed, nil

	default:
		return nil, fmt.Errorf(
			"workspace payload schema version %d has no migration path to version %d",
			domains.Catalog.SchemaVersion,
			CurrentConfigVersion,
		)
	}
}

func validateCanonicalAutomation(automation *AutomationConfig) error {
	if automation == nil {
		return fmt.Errorf("invalid automation domain: automation config is required")
	}
	for i := range automation.WorkflowRules {
		if err := automation.WorkflowRules[i].Validate(); err != nil {
			return fmt.Errorf("invalid automation domain: workflow rule %d: %w", i, err)
		}
	}
	for i := range automation.TaskTemplates {
		if err := automation.TaskTemplates[i].Validate(); err != nil {
			return fmt.Errorf("invalid automation domain: task template %d: %w", i, err)
		}
	}
	return nil
}

func encodeMigratedDomains(
	store *DomainFileStore,
	domains *Domains,
	revisions map[domainKind]string,
	kinds []domainKind,
) (map[domainKind]DomainArtifact, error) {
	workspaceID := domains.Catalog.WorkspaceID
	if err := store.BindWorkspaceID(workspaceID); err != nil {
		return nil, err
	}
	encoded := make(map[domainKind]DomainArtifact, len(kinds))
	for _, kind := range kinds {
		payload := payloadForMigratedDomain(domains, kind)
		if payload == nil {
			return nil, fmt.Errorf("unsupported migrated domain %q", kind.name())
		}
		path, err := store.pathFor(kind)
		if err != nil {
			return nil, err
		}
		data, err := encodeDomainEnvelope(kind, workspaceID, payload)
		if err != nil {
			return nil, err
		}
		revision := revisions[kind]
		encoded[kind] = DomainArtifact{
			Path:             path,
			Data:             data,
			ExpectedRevision: &revision,
		}
	}
	return encoded, nil
}

func payloadForMigratedDomain(domains *Domains, kind domainKind) any {
	switch kind {
	case domainCatalog:
		return domains.Catalog
	case domainPlanning:
		return domains.Planning
	case domainSettings:
		return domains.Settings
	case domainAutomation:
		return domains.Automation
	default:
		return nil
	}
}
