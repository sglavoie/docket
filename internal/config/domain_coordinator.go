package config

import (
	"fmt"
)

// DomainCoordinator loads the two synced configuration domains while retaining
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
	// observed distinguishes "loaded, and the artifact was absent" from "never
	// loaded", which revs alone cannot express because both read as the empty
	// string. Only an observed baseline may be used as a commit's expected
	// revision; an unobserved domain has to read the current revision first, or a
	// coordinator that never loaded would emit a require-absent contract against
	// a file that has existed all along.
	observed map[domainKind]bool
}

// recordBaseline sets the optimistic-concurrency baseline for one domain and
// marks it observed. Every writer of revs goes through here so the two maps
// cannot drift apart.
func (c *DomainCoordinator) recordBaseline(kind domainKind, rev string) {
	c.revs[kind] = rev
	c.observed[kind] = true
}

// baselineFor returns the revision a commit of this domain must be checked
// against. A session that already observed the domain is checked against what
// it observed, which is what refuses a write that landed after this session's
// load; a coordinator that never loaded observes the current revision now and
// is checked against that, which is same-call CAS and still refuses a writer
// racing this one.
func (c *DomainCoordinator) baselineFor(kind domainKind, loadRev func() (string, error)) (string, error) {
	if c.observed[kind] {
		return c.revs[kind], nil
	}
	rev, err := loadRev()
	if err != nil {
		return "", err
	}
	c.recordBaseline(kind, rev)
	return rev, nil
}

// NewDomainCoordinator roots a coordinator at a workspace data directory. Its
// baseline revisions are empty until Load populates them. Machine-local state
// resolves beneath this process's home.
func NewDomainCoordinator(dataDir string) *DomainCoordinator {
	return newDomainCoordinatorOnMachine(dataDir, "")
}

// newDomainCoordinatorOnMachine roots a coordinator at a workspace data
// directory whose machine-local artifact resolves beneath a named machine home.
func newDomainCoordinatorOnMachine(dataDir, machineHome string) *DomainCoordinator {
	return &DomainCoordinator{
		store:    NewDomainFileStoreOnMachine(dataDir, machineHome),
		revs:     make(map[domainKind]string, len(allDomainKinds)),
		observed: make(map[domainKind]bool, len(allDomainKinds)),
	}
}

// DomainCoordinator returns a coordinator rooted at this manager's workspace data
// directory, sharing the same synced-domain layout Load and Save use, and
// resolving machine-local state beneath this manager's machine home.
func (m *Manager) DomainCoordinator() *DomainCoordinator {
	return newDomainCoordinatorOnMachine(m.workspaceDataDir(), m.machineHome)
}

// LoadDomains reads every explicit domain and records each loaded domain's raw
// revision as the optimistic-concurrency baseline for later checked artifacts.
// Every kind the bundle reported is recorded, not just the synced pair, because
// the lens definitions are loaded on this path too and their baseline is what
// keeps a later lens commit from overwriting another machine's write.
func (c *DomainCoordinator) LoadDomains() (*Domains, error) {
	domains, revisions, err := loadDomainBundleWithRevisions(c.store)
	if err != nil {
		return nil, err
	}
	for kind, rev := range revisions {
		c.recordBaseline(kind, rev)
	}

	return domains, nil
}

// SyncRevisionBaseline records the current on-disk revision of each synced
// domain as the optimistic-concurrency baseline while leaving the caller's own
// in-memory snapshot in place. A caller that assembled its domains elsewhere
// uses it once, when that snapshot is established, so a later checked artifact
// is stamped with the revision the snapshot actually describes. An unreadable
// domain leaves its baseline empty, which fails a later commit closed rather
// than adopting a revision nothing observed.
func (c *DomainCoordinator) SyncRevisionBaseline() error {
	_, catalogRev, err := c.store.LoadCatalogChecked()
	if err != nil {
		return err
	}
	_, planningRev, err := c.store.LoadPlanningChecked()
	if err != nil {
		return err
	}
	c.recordBaseline(domainCatalog, catalogRev)
	c.recordBaseline(domainPlanning, planningRev)
	return nil
}

// CheckedDomainArtifacts renders the two explicit synced domains without
// rebuilding the legacy Config aggregate. It is the composition-root seam used
// by domain-native adapters when one transaction intentionally spans several
// configuration domains.
func (c *DomainCoordinator) CheckedDomainArtifacts(
	catalog *WorkspaceCatalog,
	planning *PlanningState,
) ([]DomainArtifact, error) {
	if catalog == nil || planning == nil {
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
	c.recordBaseline(domainPlanning, rev)
	return planning, nil
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
	c.recordBaseline(domainCatalog, rev)
	return catalog, nil
}

// CheckedCatalogArtifact renders only workspace.json under the revision observed
// by LoadCatalog (or Load), leaving every other domain untouched.
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
// (or Load) recorded. Unlike CheckedArtifacts it leaves every other domain
// untouched, so a planning-only mutation cannot clobber a concurrent write.
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
	}
	for _, art := range arts {
		if kind, ok := byPath[art.Path]; ok {
			c.recordBaseline(kind, revisionOf(art.Data))
		}
	}
}
