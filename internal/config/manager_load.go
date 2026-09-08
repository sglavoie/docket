package config

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// LoadDomains loads the five explicit domains. An uninitialized workspace
// yields defaults; a partial synced set is rejected.
func (m *Manager) LoadDomains() (*Domains, error) {
	store := m.domainStore()
	switch state, partialErr := m.classifyDomainState(store); state {
	case domainStatePartial:
		return nil, partialErr
	case domainStateAbsent:
		if err := m.rejectLegacyOnlyState(store); err != nil {
			return nil, err
		}
		return NewDomains(), nil
	case domainStateComplete:
		// Load the complete split-domain set below.
	}
	return loadDomainBundle(store)
}

func loadDomainBundle(store *DomainFileStore) (*Domains, error) {
	domains, _, err := loadDomainBundleWithRevisions(store)
	return domains, err
}

// LoadOrCreateDomains loads explicit domains and persists their defaults when
// the workspace has not yet been initialized.
func (m *Manager) LoadOrCreateDomains() (*Domains, error) {
	dataDir := m.workspaceDataDir()
	if err := m.RecoverWorkspace(); err != nil {
		return nil, err
	}
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir}).OnMachine(m.machineID())

	store := m.domainStore()
	state, partialErr := m.classifyDomainState(store)
	if state == domainStatePartial {
		return nil, partialErr
	}
	if state == domainStateComplete {
		return loadDomainBundle(store)
	}

	domains := NewDomains()
	// A workspace minted at the current schema is born adopted: it never passes
	// through RecoverWorkspace's migration block, and stamping the marker here
	// rather than adopting afterwards keeps the bootstrap to a single
	// machine.json write, so a fresh workspace leaves no .bak sidecar behind.
	domains.Machine.LensHomeAdopted = true
	unfiledArtifact, err := prepareFreshSystemUnfiled(dataDir, domains.Catalog)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare protected system Unfiled: %w", err)
	}
	if err := createDomainBundle(commit, store, domains, unfiledArtifact); err != nil {
		if errors.Is(err, storage.ErrStaleArtifact) {
			return m.loadAfterBootstrapRace(err)
		}
		return nil, fmt.Errorf("failed to create split configuration domains: %w", err)
	}
	// The portable lens home is published beside the machine file the bundle
	// just wrote, so a workspace has one from its first launch instead of
	// acquiring it on the next. It stays outside the synced bootstrap
	// transaction for the same reason a lens mutation stays outside every
	// workspace transaction.
	if err := m.DomainCoordinator().CommitLensDefinitions(&WorkspaceLensDefinitions{
		Lenses: DefinedLensList(domains.Machine.NamedLenses),
	}); err != nil {
		return nil, fmt.Errorf("failed to publish portable lens definitions after synced bootstrap: %w", err)
	}
	return domains, nil
}

// createDomainBundle validates and encodes the complete default bundle before
// the synced transaction creates anything. Machine state lives outside the
// workspace root and is intentionally saved only after all four synced domains
// have committed successfully.
func createDomainBundle(commit *storage.CommitService, store *DomainFileStore, domains *Domains, additional ...storage.ArtifactChange) error {
	if commit == nil {
		return fmt.Errorf("configuration commit service is required")
	}
	if domains == nil || domains.Catalog == nil || domains.Planning == nil || domains.Machine == nil {
		return fmt.Errorf("all configuration domains are required")
	}
	workspaceID, err := store.bindCatalogPayload(domains.Catalog)
	if err != nil {
		return err
	}
	if err := validateDomainBundle(domains); err != nil {
		return err
	}

	type syncedDomain struct {
		kind    domainKind
		payload any
	}
	synced := []syncedDomain{
		{kind: domainCatalog, payload: domains.Catalog},
		{kind: domainPlanning, payload: domains.Planning},
	}
	absentRevision := ""
	changes := make([]storage.ArtifactChange, 0, len(synced)+len(additional))
	for _, domain := range synced {
		path, pathErr := store.pathFor(domain.kind)
		if pathErr != nil {
			return pathErr
		}
		data, encodeErr := encodeDomainEnvelope(domain.kind, workspaceID, domain.payload)
		if encodeErr != nil {
			return encodeErr
		}
		changes = append(changes, DomainArtifact{
			Path:             path,
			Data:             data,
			ExpectedRevision: &absentRevision,
		}.StorageChange())
	}
	changes = append(changes, additional...)
	if _, err := commit.CommitArtifacts(changes); err != nil {
		return err
	}
	if err := store.SaveMachine(domains.Machine); err != nil {
		return fmt.Errorf("failed to save machine-local configuration after synced bootstrap: %w", err)
	}
	return nil
}

// prepareFreshSystemUnfiled installs the protected Unfiled into a brand-new
// catalog and renders its empty task document. The caller commits this artifact
// together with every synced domain, so a clean workspace can never expose a
// catalog entry without its project file (or vice versa).
//
// This helper is intentionally limited to the absent-workspace bootstrap path.
// Established workspaces are loaded strictly and must surface a missing or
// inconsistent Unfiled to diagnostics instead of silently repairing it.
func prepareFreshSystemUnfiled(dataDir string, catalog *WorkspaceCatalog) (storage.ArtifactChange, error) {
	if catalog == nil {
		return storage.ArtifactChange{}, fmt.Errorf("workspace catalog is required")
	}
	project, err := models.NewProject(
		"system-unfiled-"+uuid.NewString(),
		SystemUnfiledName,
		filepath.Join(dataDir, "projects", SystemUnfiledSlug),
	)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	if repairErr := catalog.RepairSystemUnfiledProject(project); repairErr != nil {
		return storage.ArtifactChange{}, repairErr
	}
	// The live writer, not a named schema version: a fresh Unfiled must be born
	// at whatever version the workspace is current at, and this bootstrap is not
	// a migration that gets to pin one.
	document, err := storage.WriteTasksMarkdown(project.Name, nil)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	absentRevision := ""
	return storage.ArtifactChange{
		Path:             storage.TasksFilePath(project.Path),
		Data:             []byte(document),
		Mode:             storage.FilePermissions,
		ExpectedRevision: &absentRevision,
	}, nil
}

func validateDomainBundle(domains *Domains) error {
	return validateDomainBundleWithCatalogPolicy(domains, (*WorkspaceCatalog).Validate)
}

func validateDomainBundleForLoad(domains *Domains) error {
	return validateDomainBundleWithCatalogPolicy(domains, (*WorkspaceCatalog).validateForLoad)
}

func validateDomainBundleWithCatalogPolicy(domains *Domains, validateCatalog func(*WorkspaceCatalog) error) error {
	if domains == nil || domains.Catalog == nil || domains.Planning == nil || domains.Machine == nil {
		return fmt.Errorf("all configuration domains are required")
	}
	checks := []struct {
		name     string
		validate func() error
	}{
		{name: "catalog", validate: func() error { return validateCatalog(domains.Catalog) }},
		{name: "planning", validate: domains.Planning.Validate},
		{name: "machine", validate: domains.Machine.Validate},
	}
	for _, check := range checks {
		if err := check.validate(); err != nil {
			return fmt.Errorf("invalid %s domain: %w", check.name, err)
		}
	}
	return nil
}

// loadAfterBootstrapRace resolves the only expected create conflict: another
// initializer committed first. A fresh store must see either the winner's whole
// bundle or an explicit partial state; an absent result preserves the original
// stale-artifact error rather than silently retrying with a new identity.
func (m *Manager) loadAfterBootstrapRace(staleErr error) (*Domains, error) {
	store := m.domainStore()
	switch state, partialErr := m.classifyDomainState(store); state {
	case domainStateComplete:
		return loadDomainBundle(store)
	case domainStatePartial:
		return nil, partialErr
	default:
		return nil, staleErr
	}
}
