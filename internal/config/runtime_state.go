package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sglavoie/docket/internal/models"
)

// RuntimeState is the startup-loaded, in-memory snapshot of the three owned
// configuration domains, the raw revision observed for each at load time, and
// the workspace data directory. It is the composition-root state a long-lived
// runtime (the TUI shell) holds instead of the config.Config facade: views read
// each domain's data through the narrow accessors here (no render-time disk
// loads), and a mutation refreshes the snapshot only through the injected
// DomainCoordinator after a checked commit.
//
// It wraps a DomainCoordinator so the same optimistic-concurrency machinery the
// CLI uses is available to the TUI runtime: the CommitCatalog/CommitPlanning/
// CommitMachine seams persist only the changed
// domain(s) under a revision check (no config.Config rebuilt or saved), and each
// refreshes the in-memory snapshot so the views observe the committed state
// without a render-time disk load.
//
// A RuntimeState is single-workspace and not safe for concurrent use by multiple
// goroutines; the shell holds one and drives it from its update loop.
type RuntimeState struct {
	coord   *DomainCoordinator
	domains *Domains
	revs    map[domainKind]string
	dataDir string
}

// LoadRuntimeState builds the runtime snapshot for a workspace data directory,
// loading every domain once and recording each synced domain's raw revision as
// the optimistic-concurrency baseline. A load failure returns the error; callers
// that must always obtain a usable (possibly empty) state can fall back to
// EmptyRuntimeState.
func LoadRuntimeState(dataDir string) (*RuntimeState, error) {
	return LoadRuntimeStateOnMachine(dataDir, "")
}

// LoadRuntimeStateOnMachine is LoadRuntimeState with machine-local state
// resolved beneath a named machine home rather than $HOME.
func LoadRuntimeStateOnMachine(dataDir, machineHome string) (*RuntimeState, error) {
	rs := &RuntimeState{
		coord:   newDomainCoordinatorOnMachine(dataDir, machineHome),
		revs:    make(map[domainKind]string, len(syncedDomainKinds)),
		dataDir: dataDir,
	}
	if err := rs.Reload(); err != nil {
		return nil, err
	}
	return rs, nil
}

// EmptyRuntimeState returns a fully defaulted, non-nil runtime snapshot rooted at
// a data directory. It is the safe fallback for callers (and tests) that cannot
// load real domain files but must still hold a usable state whose accessors never
// return nil.
func EmptyRuntimeState(dataDir string) *RuntimeState {
	return NewRuntimeState(dataDir, nil)
}

// NewRuntimeState builds an in-memory runtime snapshot from explicit domains
// the caller already owns. It is primarily useful for tests and composition
// paths that assembled a domain bundle elsewhere.
//
// The caller asserts that its bundle describes the workspace as it stands, so a
// rooted snapshot adopts the revisions on disk right now as its
// optimistic-concurrency baseline. Without that, every checked artifact this
// snapshot renders would carry a require-absent contract and a later commit
// against an existing file could only fail.
func NewRuntimeState(dataDir string, domains *Domains) *RuntimeState {
	if domains == nil {
		domains = NewDomains()
	}
	rs := &RuntimeState{
		coord:   NewDomainCoordinator(dataDir),
		domains: domains,
		revs:    make(map[domainKind]string, len(syncedDomainKinds)),
		dataDir: dataDir,
	}
	if dataDir != "" {
		_ = rs.coord.SyncRevisionBaseline()
	}
	return rs
}

// LoadRuntimeState builds the runtime snapshot for this manager's workspace
// from the canonical split-domain layout, reading machine-local state beneath
// this manager's machine home.
func (m *Manager) LoadRuntimeState() (*RuntimeState, error) {
	if _, err := m.LoadOrCreateDomains(); err != nil {
		return nil, err
	}
	return LoadRuntimeStateOnMachine(m.workspaceDataDir(), m.machineHome)
}

// Reload re-reads every domain from disk through the coordinator, refreshing both
// the in-memory domain snapshot the accessors return and the raw-revision
// baseline CheckedArtifacts stamps. It is the only place the snapshot touches
// disk, so render/update helpers stay disk-free by reading the cached accessors.
func (rs *RuntimeState) Reload() error {
	domains, err := rs.coord.LoadDomains()
	if err != nil {
		return err
	}
	rs.domains = domains
	rs.revs = make(map[domainKind]string, len(rs.coord.revs))
	for k, v := range rs.coord.revs {
		rs.revs[k] = v
	}
	return nil
}

// RefreshAfterCommit advances the snapshot to the state a checked commit just
// wrote. It is the sole runtime-state refresh path: a mutation commits its
// config artifacts through CheckedArtifacts under the transaction locks, and on
// success calls this so the views observe the committed domains without any
// render-time disk load of their own.
func (rs *RuntimeState) RefreshAfterCommit() error {
	return rs.Reload()
}

// DataDir returns the workspace data directory the snapshot is rooted at.
func (rs *RuntimeState) DataDir() string {
	if rs == nil {
		return ""
	}
	return rs.dataDir
}

// domainsOrDefault returns the cached domains, defaulting a never-loaded snapshot
// so accessors never dereference nil.
func (rs *RuntimeState) domainsOrDefault() *Domains {
	if rs.domains == nil {
		rs.domains = NewDomains()
	}
	return rs.domains
}

// Catalog returns the loaded workspace catalog domain.
func (rs *RuntimeState) Catalog() *WorkspaceCatalog {
	if rs == nil {
		return NewWorkspaceCatalog()
	}
	return rs.domainsOrDefault().Catalog
}

// Planning returns the loaded planning domain.
func (rs *RuntimeState) Planning() *PlanningState {
	if rs == nil {
		return NewPlanningState()
	}
	return rs.domainsOrDefault().Planning
}

// GetNext lets domain-native consumers read durable Next ordering without
// accepting a Config facade. It is an alias for NextOrder so no consumer can
// reach an unfiltered read.
func (rs *RuntimeState) GetNext() []ProjectTaskLocator { return rs.NextOrder() }

// Machine returns the loaded machine-local domain.
func (rs *RuntimeState) Machine() *MachineState {
	if rs == nil {
		return NewMachineState()
	}
	return rs.domainsOrDefault().Machine
}

// CloneDomains returns a deep, independently mutable copy of the runtime's three
// cached domains. CLI lifecycle commands use the copy to assemble a checked
// multi-domain transaction without mutating the loaded snapshot before the
// transaction succeeds. The domains are their own JSON persistence aggregates,
// so a JSON round trip is also the canonical deep-copy boundary for nested
// slices, maps, and pointer fields.
func (rs *RuntimeState) CloneDomains() (*Domains, error) {
	if rs == nil {
		return nil, fmt.Errorf("runtime state is unavailable")
	}
	source := &Domains{
		Catalog:  rs.Catalog(),
		Planning: rs.Planning(),
		Machine:  rs.Machine(),
	}
	data, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("clone runtime domains: %w", err)
	}
	var cloned Domains
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("clone runtime domains: %w", err)
	}
	return &cloned, nil
}

// CheckedSyncedArtifacts renders a mutated domain snapshot against the revisions
// this runtime observed at command startup. Keeping the original baseline is
// essential: reloading immediately before encoding would stamp stale in-memory
// intent with a concurrent writer's newer revision and permit a lost update.
func (rs *RuntimeState) CheckedSyncedArtifacts(domains *Domains) ([]DomainArtifact, error) {
	if rs == nil || rs.coord == nil {
		return nil, fmt.Errorf("runtime state is unavailable")
	}
	if domains == nil {
		return nil, fmt.Errorf("configuration domains are required")
	}
	return rs.coord.CheckedDomainArtifacts(
		domains.Catalog,
		domains.Planning,
	)
}

// CheckedCatalogArtifact renders a catalog mutation against the revision this
// runtime observed at command startup, leaving every other domain untouched.
func (rs *RuntimeState) CheckedCatalogArtifact(catalog *WorkspaceCatalog) (DomainArtifact, error) {
	if rs == nil || rs.coord == nil {
		return DomainArtifact{}, fmt.Errorf("runtime state is unavailable")
	}
	return rs.coord.CheckedCatalogArtifact(catalog)
}

// CheckedPlanningArtifact renders a planning-only mutation against the
// revision this runtime observed at startup, leaving every other domain
// untouched. Task transactions use it to make planning-reference cleanup
// atomic without widening the config write set.
func (rs *RuntimeState) CheckedPlanningArtifact(planning *PlanningState) (DomainArtifact, error) {
	if rs == nil || rs.coord == nil {
		return DomainArtifact{}, fmt.Errorf("runtime state is unavailable")
	}
	return rs.coord.CheckedPlanningArtifact(planning)
}

// CommitCatalog persists a mutated workspace-catalog domain under an optimistic
// concurrency check and refreshes the snapshot to the committed state, so the
// views observe the change without a render-time disk load. Only the catalog
// domain is written; the other synced domains are left untouched. It is one of
// the domain-native commit seams that replace the former Config-round-tripping
// checked-artifact path.
func (rs *RuntimeState) CommitCatalog(catalog *WorkspaceCatalog) error {
	return rs.commitThenRefresh(func() error { return rs.coord.CommitCatalog(catalog) })
}

// CommitPlanning persists mutated Today/Next ordering under an optimistic
// concurrency check and refreshes the snapshot.
func (rs *RuntimeState) CommitPlanning(planning *PlanningState) error {
	return rs.commitThenRefresh(func() error { return rs.coord.CommitPlanning(planning) })
}

// CommitMachine persists a mutated machine-local domain (navigation, layout,
// histories) under an optimistic concurrency check and refreshes the snapshot.
// Only the machine-local file is written; it stays outside the synced set.
func (rs *RuntimeState) CommitMachine(machine *MachineState) error {
	return rs.commitThenRefresh(func() error { return rs.coord.CommitMachine(machine) })
}

// SetMachineInMemory replaces the cached machine-local domain without persisting
// to disk. It is the no-disk apply path a caller uses when the runtime has no
// writable data directory (isolated tests): the in-memory accessors observe the
// mutation so view-state round-trips behave as they did against the config
// facade, while no stray machine.json is written to the process working
// directory or the real home runtime root.
func (rs *RuntimeState) SetMachineInMemory(machine *MachineState) {
	if rs == nil {
		return
	}
	rs.domainsOrDefault().Machine = machine
}

// SetCatalogInMemory replaces the cached workspace-catalog domain without
// persisting to disk. It is the no-disk apply path a caller uses when the
// runtime has no writable data directory (isolated tests): the in-memory
// accessors observe the catalog mutation (create/archive/pin/rename/delete)
// so lifecycle round-trips behave as they did against the config facade, while
// no stray workspace.json is written.
func (rs *RuntimeState) SetCatalogInMemory(catalog *WorkspaceCatalog) {
	if rs == nil {
		return
	}
	rs.domainsOrDefault().Catalog = catalog
}

// SetDomainsInMemory replaces all cached domains without persistence. It is the
// composition-root test seam for a domain-spanning mutation on a runtime with
// no writable workspace directory.
func (rs *RuntimeState) SetDomainsInMemory(domains *Domains) {
	if rs == nil || domains == nil {
		return
	}
	rs.domains = domains
}

// commitThenRefresh runs a domain-scoped checked commit and, on success, advances
// the in-memory snapshot to the just-committed state so accessors observe the
// mutation without any render-time disk load. A commit failure leaves the cached
// snapshot unchanged.
func (rs *RuntimeState) commitThenRefresh(commit func() error) error {
	if rs == nil {
		return fmt.Errorf("runtime state is unavailable")
	}
	if err := commit(); err != nil {
		return err
	}
	return rs.RefreshAfterCommit()
}

// AvailableProjects returns every non-archived cataloged project regardless of
// its tags, sorted with pinned first. It is the population every surface that
// stays reachable for a parked project reads: Notes owners, capture and
// %project resolution, task-move destinations, and project administration.
//
// Archival is the only thing it tests, and that is deliberate: a parked project
// keeps every one of those affordances, so narrowing here would collapse it back
// into TaskWorkspaceProjects.
func (rs *RuntimeState) AvailableProjects() []*models.Project {
	var projects []*models.Project
	for _, p := range rs.catalogProjects() {
		if !p.Archived {
			projects = append(projects, p)
		}
	}
	return rs.orderedProjects(projects)
}

// TaskWorkspaceProjects returns the base task workspace: the non-archived
// projects that do not carry the reserved parked tag. It is strictly narrower
// than AvailableProjects, because a parked project keeps its tasks on disk and
// its Notes/capture/move availability while contributing nothing to ordinary
// task projections.
func (rs *RuntimeState) TaskWorkspaceProjects() []*models.Project {
	var projects []*models.Project
	for _, p := range rs.catalogProjects() {
		if p.Archived || models.ProjectTagsParked(p.Tags) {
			continue
		}
		projects = append(projects, p)
	}
	return rs.orderedProjects(projects)
}

// AvailableProject returns the non-archived project matching idOrSlug, or nil.
// It is the single-project counterpart of AvailableProjects, so a destination
// that admits parked projects resolves one the same way it lists them.
func (rs *RuntimeState) AvailableProject(idOrSlug string) *models.Project {
	project := rs.GetProject(idOrSlug)
	if project == nil || project.Archived {
		return nil
	}
	return project
}

// TaskWorkspaceProject returns the base-workspace project matching idOrSlug, or
// nil when it is archived, parked, or absent.
func (rs *RuntimeState) TaskWorkspaceProject(idOrSlug string) *models.Project {
	project := rs.AvailableProject(idOrSlug)
	if project == nil || models.ProjectTagsParked(project.Tags) {
		return nil
	}
	return project
}

// orderedProjects applies the shared presentation order every population owes
// its callers: pinned first, then the protected Unfiled project last.
func (rs *RuntimeState) orderedProjects(projects []*models.Project) []*models.Project {
	sortProjectsWithPinnedFirst(projects)
	moveProjectIDLast(projects, rs.Catalog().protectedProjectID())
	return projects
}

func moveProjectIDLast(projects []*models.Project, projectID string) {
	if projectID == "" {
		return
	}
	for i, project := range projects {
		if project != nil && project.ID == projectID && i != len(projects)-1 {
			copy(projects[i:], projects[i+1:])
			projects[len(projects)-1] = project
			return
		}
	}
}

// AllProjects returns every cataloged project (archived, sandbox, and internal
// included) in catalog order. It is the runtime-snapshot counterpart to
// RuntimeDependencies.AllProjects and replaces secondary-view reads of the raw
// Config.Projects facade slice.
func (rs *RuntimeState) AllProjects() []*models.Project {
	return rs.catalogProjects()
}

// catalogProjects rehydrates every cataloged project into a models.Project with
// its Path derived from the snapshot's data directory.
func (rs *RuntimeState) catalogProjects() []*models.Project {
	catalog := rs.Catalog()
	projects := make([]*models.Project, 0, len(catalog.Projects))
	for _, cp := range catalog.Projects {
		projects = append(projects, cp.ToModel(rs.dataDir))
	}
	return projects
}

// NextOrder returns the persisted workspace-wide Next membership and order
// with every entry owned by a parked project excluded. It is the single
// filtered planning accessor: parking is a read-time predicate rather than a
// destructive edit, so planning.json keeps a parked project's locators and
// unparking restores its plan untouched. Storage-facing callers that must see
// the persisted set whole (archive/restore capture, reconciliation) read
// Planning() directly.
func (rs *RuntimeState) NextOrder() []ProjectTaskLocator {
	next := rs.Planning().GetNext()
	parked := make(map[string]bool)
	for _, project := range rs.Catalog().Projects {
		if models.ProjectTagsParked(project.Tags) {
			parked[project.ID] = true
		}
	}
	if len(parked) == 0 {
		return next
	}
	visible := make([]ProjectTaskLocator, 0, len(next))
	for _, item := range next {
		if !parked[item.ProjectID] {
			visible = append(visible, item)
		}
	}
	return visible
}

// SystemUnfiledProject returns the catalog-owned visible Unfiled project.
func (rs *RuntimeState) SystemUnfiledProject() *models.Project {
	if rs == nil {
		return nil
	}
	return rs.GetProject(rs.Catalog().protectedProjectID())
}

// IsSystemUnfiledProject reports whether id belongs to the catalog-owned Unfiled.
func (rs *RuntimeState) IsSystemUnfiledProject(id string) bool {
	return rs != nil && rs.Catalog().IsSystemUnfiledProject(id)
}

// ArchivedProjects returns archived cataloged projects sorted by name.
func (rs *RuntimeState) ArchivedProjects() []*models.Project {
	var archived []*models.Project
	for _, p := range rs.catalogProjects() {
		if p.Archived {
			archived = append(archived, p)
		}
	}
	sortProjectsByName(archived)
	return archived
}

// ProjectPath derives the on-disk directory for a project slug from the
// snapshot's data directory, mirroring (*Config).GetProjectPath. Absolute
// project paths are never persisted; this is the single derivation point the
// TUI uses instead of reading the facade.
func (rs *RuntimeState) ProjectPath(slug string) string {
	return filepath.Join(rs.dataDir, "projects", slug)
}

// EnsureDataDir creates the workspace data directory, its internal directory,
// and the projects directory, mirroring (*Config).EnsureDataDir but rooted at
// the snapshot's data directory.
func (rs *RuntimeState) EnsureDataDir() error {
	if err := os.MkdirAll(rs.dataDir, DirPermissions); err != nil {
		return err
	}
	if err := os.MkdirAll(InternalDir(rs.dataDir), DirPermissions); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(rs.dataDir, "projects"), DirPermissions)
}

// AllProjectTags returns the sorted, unique project-tag vocabulary across every
// cataloged project. Tags are emergent: a tag exists only because a project
// carries it, so this derived set is the whole namespace and an empty result
// means no project is tagged. It reads the in-memory catalog domain without
// rebuilding the legacy Config facade.
func (rs *RuntimeState) AllProjectTags() []string {
	tagSet := make(map[string]bool)
	for _, p := range rs.Catalog().Projects {
		for _, tag := range p.Tags {
			tagSet[tag] = true
		}
	}
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}
