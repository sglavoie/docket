package config

import (
	"fmt"
)

// DomainCoordinator loads the four synced configuration domains while retaining
// the raw revision observed for each, so a later save can be revision-checked
// against a concurrent writer instead of blindly overwriting it. It is the
// production seam through which domain mutations acquire optimistic concurrency:
// CheckedArtifacts emits DomainArtifact values stamped with the load-time
// ExpectedRevision, the cmd/tui bridge folds them into storage.ArtifactChange
// entries, and the transactional commit service validates each revision under its
// artifact locks before staging.
//
// A coordinator is single-workspace and not safe for concurrent use by multiple
// goroutines; each mutating flow holds its own. After a successful commit the
// caller calls AdoptArtifacts so the coordinator advances its baseline to the
// just-written revisions and the next mutation is checked against them rather than
// the stale original load.
type DomainCoordinator struct {
	store *DomainFileStore
	// revs holds the load-time raw revision of each synced domain. An empty string
	// records "absent at load", which CheckedArtifacts turns into a require-absent
	// contract so a create cannot silently clobber a file another writer produced
	// in the meantime.
	revs map[domainKind]string
}

// NewDomainCoordinator roots a coordinator at a workspace data directory. Its
// baseline revisions are empty until Load populates them.
func NewDomainCoordinator(dataDir string) *DomainCoordinator {
	return &DomainCoordinator{
		store: NewDomainFileStore(dataDir),
		revs:  make(map[domainKind]string, len(syncedDomainKinds)),
	}
}

// DomainCoordinator returns a coordinator rooted at this manager's workspace data
// directory, sharing the same synced-domain layout Load and Save use.
func (m *Manager) DomainCoordinator() *DomainCoordinator {
	return NewDomainCoordinator(m.workspaceDataDir())
}

// LoadDomains reads every explicit domain and records each synced domain's raw
// revision as the optimistic-concurrency baseline for later checked artifacts.
func (c *DomainCoordinator) LoadDomains() (*Domains, error) {
	catalog, catRev, err := c.store.LoadCatalogChecked()
	if err != nil {
		return nil, err
	}
	if catRev == "" {
		// A fresh coordinator needs one stable identity before it can render
		// checked catalog artifacts. There is no machine path to inspect until
		// that catalog is persisted, so machine state remains at its defaults.
		if _, bindErr := c.store.bindCatalogPayload(catalog); bindErr != nil {
			return nil, bindErr
		}
	}
	planning, planRev, err := c.store.LoadPlanningChecked()
	if err != nil {
		return nil, err
	}
	settings, setRev, err := c.store.LoadSettingsChecked()
	if err != nil {
		return nil, err
	}
	automation, autoRev, err := c.store.LoadAutomationChecked()
	if err != nil {
		return nil, err
	}
	machine := NewMachineState()
	if catRev != "" {
		machine, err = c.store.LoadMachine()
		if err != nil {
			return nil, err
		}
	}

	c.revs[domainCatalog] = catRev
	c.revs[domainPlanning] = planRev
	c.revs[domainSettings] = setRev
	c.revs[domainAutomation] = autoRev

	return &Domains{
		Catalog:    catalog,
		Planning:   planning,
		Settings:   settings,
		Automation: automation,
		Machine:    machine,
	}, nil
}

// CheckedDomainArtifacts renders the four explicit synced domains without
// rebuilding the legacy Config aggregate. It is the composition-root seam used
// by domain-native adapters when one transaction intentionally spans several
// configuration domains.
func (c *DomainCoordinator) CheckedDomainArtifacts(
	catalog *WorkspaceCatalog,
	planning *PlanningState,
	settings *Settings,
	automation *AutomationConfig,
) ([]DomainArtifact, error) {
	if catalog == nil || planning == nil || settings == nil || automation == nil {
		return nil, fmt.Errorf("all synced configuration domains are required")
	}
	// Bind the on-disk catalog first when it exists. A fresh multi-domain artifact
	// set may mint exactly one ID because it includes workspace.json itself.
	diskCatalog, diskRev, err := c.store.LoadCatalogChecked()
	if err != nil {
		return nil, err
	}
	if diskRev != "" && catalog.WorkspaceID == "" {
		catalog.WorkspaceID = diskCatalog.WorkspaceID
	}
	workspaceID, err := c.store.bindCatalogPayload(catalog)
	if err != nil {
		return nil, err
	}

	dataDir := c.store.DataDir()
	catalog.WorkspaceID = workspaceID
	specs := []struct {
		kind    domainKind
		path    string
		payload any
	}{
		{domainCatalog, WorkspaceCatalogPath(dataDir), catalog},
		{domainPlanning, PlanningStatePath(dataDir), planning},
		{domainSettings, SettingsPath(dataDir), settings},
		{domainAutomation, AutomationPath(dataDir), automation},
	}
	arts := make([]DomainArtifact, 0, len(specs))
	for _, s := range specs {
		data, err := encodeDomainEnvelope(s.kind, workspaceID, s.payload)
		if err != nil {
			return nil, err
		}
		rev := c.revs[s.kind]
		arts = append(arts, DomainArtifact{Path: s.path, Data: data, ExpectedRevision: &rev})
	}
	return arts, nil
}

// LoadPlanning reads only the planning domain, recording its load-time raw
// revision as the optimistic-concurrency baseline for a subsequent
// CheckedPlanningArtifact. It is the planning-scoped counterpart to Load for
// mutations that touch no other domain (e.g. folding a durable planning-ref
// cleanup into a task transaction).
func (c *DomainCoordinator) LoadPlanning() (*PlanningState, error) {
	planning, rev, err := c.store.LoadPlanningChecked()
	if err != nil {
		return nil, err
	}
	c.revs[domainPlanning] = rev
	return planning, nil
}

// LoadAutomation reads only the automation domain and records its raw revision
// for a subsequent CheckedAutomationArtifact call. It is the scoped load seam
// for template-use counts, workflow counters, and other automation-only
// mutations that must join a task transaction without rewriting planning.
func (c *DomainCoordinator) LoadAutomation() (*AutomationConfig, error) {
	automation, rev, err := c.store.LoadAutomationChecked()
	if err != nil {
		return nil, err
	}
	c.revs[domainAutomation] = rev
	return automation, nil
}

// LoadCatalog reads only the workspace catalog and records its raw revision for
// a subsequent CheckedCatalogArtifact call. It is the catalog-scoped counterpart
// to LoadPlanning for project metadata mutations folded into a larger workspace
// transaction.
func (c *DomainCoordinator) LoadCatalog() (*WorkspaceCatalog, error) {
	catalog, rev, err := c.store.LoadCatalogChecked()
	if err != nil {
		return nil, err
	}
	if rev == "" {
		if _, err := c.store.bindCatalogPayload(catalog); err != nil {
			return nil, err
		}
	}
	c.revs[domainCatalog] = rev
	return catalog, nil
}

// CheckedCatalogArtifact renders only workspace.json under the revision observed
// by LoadCatalog (or Load), leaving planning, settings, and automation untouched.
func (c *DomainCoordinator) CheckedCatalogArtifact(catalog *WorkspaceCatalog) (DomainArtifact, error) {
	if catalog == nil {
		return DomainArtifact{}, fmt.Errorf("workspace catalog is required")
	}
	diskCatalog, diskRev, err := c.store.LoadCatalogChecked()
	if err != nil {
		return DomainArtifact{}, err
	}
	if diskRev != "" && catalog.WorkspaceID == "" {
		catalog.WorkspaceID = diskCatalog.WorkspaceID
	}
	workspaceID, err := c.store.bindCatalogPayload(catalog)
	if err != nil {
		return DomainArtifact{}, err
	}
	data, err := encodeDomainEnvelope(domainCatalog, workspaceID, catalog)
	if err != nil {
		return DomainArtifact{}, err
	}
	rev := c.revs[domainCatalog]
	return DomainArtifact{Path: WorkspaceCatalogPath(c.store.DataDir()), Data: data, ExpectedRevision: &rev}, nil
}

// CheckedPlanningArtifact renders only the planning domain into a
// revision-checked DomainArtifact, stamped against the baseline LoadPlanning
// (or Load) recorded. Unlike CheckedArtifacts it leaves the catalog, settings,
// and automation domains untouched, so a planning-only mutation cannot clobber
// a concurrent write to another domain.
func (c *DomainCoordinator) CheckedPlanningArtifact(planning *PlanningState) (DomainArtifact, error) {
	if planning == nil {
		return DomainArtifact{}, fmt.Errorf("planning state is required")
	}
	_, catalogRev, err := c.store.LoadCatalogChecked()
	if err != nil {
		return DomainArtifact{}, err
	}
	if catalogRev == "" {
		return DomainArtifact{}, fmt.Errorf("%w: checked planning artifacts require an existing workspace catalog", ErrWorkspaceIDRequired)
	}
	workspaceID, err := c.store.validatedWorkspaceID()
	if err != nil {
		return DomainArtifact{}, err
	}
	data, err := encodeDomainEnvelope(domainPlanning, workspaceID, planning)
	if err != nil {
		return DomainArtifact{}, err
	}
	rev := c.revs[domainPlanning]
	return DomainArtifact{Path: PlanningStatePath(c.store.DataDir()), Data: data, ExpectedRevision: &rev}, nil
}

// CheckedAutomationArtifact renders only automation.json against the revision
// observed by LoadAutomation (or LoadDomains), leaving catalog, planning, and
// settings untouched. Template use-count updates use this seam so a workflow-
// generated planning artifact can coexist in the same transaction without a
// duplicate target or an unrelated stale baseline.
func (c *DomainCoordinator) CheckedAutomationArtifact(automation *AutomationConfig) (DomainArtifact, error) {
	if automation == nil {
		return DomainArtifact{}, fmt.Errorf("automation config is required")
	}
	_, catalogRev, err := c.store.LoadCatalogChecked()
	if err != nil {
		return DomainArtifact{}, err
	}
	if catalogRev == "" {
		return DomainArtifact{}, fmt.Errorf("%w: checked automation artifacts require an existing workspace catalog", ErrWorkspaceIDRequired)
	}
	workspaceID, err := c.store.validatedWorkspaceID()
	if err != nil {
		return DomainArtifact{}, err
	}
	data, err := encodeDomainEnvelope(domainAutomation, workspaceID, automation)
	if err != nil {
		return DomainArtifact{}, err
	}
	rev := c.revs[domainAutomation]
	return DomainArtifact{Path: AutomationPath(c.store.DataDir()), Data: data, ExpectedRevision: &rev}, nil
}

// AdoptArtifacts advances the coordinator's baseline to the revisions of a set of
// artifacts it just committed, so a subsequent CheckedArtifacts is validated
// against the freshly written state rather than the stale original load. Each
// artifact's new baseline revision is the raw hash of the bytes that were
// committed, matching what a reload would observe. Only recognized synced domain
// paths are adopted; other paths are ignored.
func (c *DomainCoordinator) AdoptArtifacts(arts []DomainArtifact) {
	byPath := map[string]domainKind{
		WorkspaceCatalogPath(c.store.DataDir()): domainCatalog,
		PlanningStatePath(c.store.DataDir()):    domainPlanning,
		SettingsPath(c.store.DataDir()):         domainSettings,
		AutomationPath(c.store.DataDir()):       domainAutomation,
	}
	for _, art := range arts {
		if kind, ok := byPath[art.Path]; ok {
			c.revs[kind] = revisionOf(art.Data)
		}
	}
}
