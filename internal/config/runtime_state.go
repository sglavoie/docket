package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// RuntimeState is the startup-loaded, in-memory snapshot of the five owned
// configuration domains, the raw revision observed for each at load time, and
// the workspace data directory. It is the composition-root state a long-lived
// runtime (the TUI shell) holds instead of the config.Config facade: views read
// each domain's data through the narrow accessors here (no render-time disk
// loads), and a mutation refreshes the snapshot only through the injected
// DomainCoordinator after a checked commit.
//
// It wraps a DomainCoordinator so the same optimistic-concurrency machinery the
// CLI uses is available to the TUI runtime: the CommitCatalog/CommitPlanning/
// CommitSettings/CommitAutomation/CommitMachine seams persist only the changed
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
	rs := &RuntimeState{
		coord:   NewDomainCoordinator(dataDir),
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
	return &RuntimeState{
		coord:   NewDomainCoordinator(dataDir),
		domains: NewDomains(),
		revs:    make(map[domainKind]string, len(syncedDomainKinds)),
		dataDir: dataDir,
	}
}

// NewRuntimeState builds an in-memory runtime snapshot from explicit domains
// without reading disk. It is primarily useful for tests and composition paths
// that already own a domain bundle.
func NewRuntimeState(dataDir string, domains *Domains) *RuntimeState {
	if domains == nil {
		domains = NewDomains()
	}
	return &RuntimeState{
		coord:   NewDomainCoordinator(dataDir),
		domains: domains,
		revs:    make(map[domainKind]string, len(syncedDomainKinds)),
		dataDir: dataDir,
	}
}

// LoadRuntimeState builds the runtime snapshot for this manager's workspace
// from the canonical split-domain layout.
func (m *Manager) LoadRuntimeState() (*RuntimeState, error) {
	return LoadRuntimeState(m.workspaceDataDir())
}

// Reload re-reads every domain from disk through the coordinator, refreshing both
// the in-memory domain snapshot the accessors return and the raw-revision
// baseline CheckedArtifacts stamps. It is the only place the snapshot touches
// disk, so render/update helpers stay disk-free by reading the cached accessors
// instead. A retired-action migration may perform one additional load after its
// checked automation write so every consumer receives the persisted snapshot.
func (rs *RuntimeState) Reload() error {
	domains, err := rs.coord.LoadDomains()
	if err != nil {
		return err
	}
	if domains.Automation.SanitizeRetiredWorkflowActions() {
		// The domain coordinator retained the revision that produced this
		// snapshot. Saving with that exact baseline refuses a concurrent writer
		// instead of silently overwriting automation changes made after load.
		loadedRevision := rs.coord.revs[domainAutomation]
		if err := rs.coord.store.SaveAutomationChecked(domains.Automation, loadedRevision); err != nil {
			return err
		}
		return rs.Reload()
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

// Settings returns the loaded settings domain.
func (rs *RuntimeState) Settings() *Settings {
	if rs == nil {
		return NewSettings()
	}
	return rs.domainsOrDefault().Settings
}

// Automation returns the loaded automation domain.
func (rs *RuntimeState) Automation() *AutomationConfig {
	if rs == nil {
		return NewAutomationConfig()
	}
	return rs.domainsOrDefault().Automation
}

// GetFocusQueue and GetProjectScoringProfile let domain-native consumers use
// the same narrow scoring capability as legacy callers without accepting a
// Config facade.
func (rs *RuntimeState) GetFocusQueue() []FocusQueueItem { return rs.Planning().GetFocusQueue() }
func (rs *RuntimeState) GetProjectScoringProfile(projectID string) *scoring.ScoringProfile {
	return rs.Automation().GetProjectScoringProfile(projectID)
}

// Machine returns the loaded machine-local domain.
func (rs *RuntimeState) Machine() *MachineState {
	if rs == nil {
		return NewMachineState()
	}
	return rs.domainsOrDefault().Machine
}

// CloneDomains returns a deep, independently mutable copy of the runtime's five
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
		Catalog:    rs.Catalog(),
		Planning:   rs.Planning(),
		Settings:   rs.Settings(),
		Automation: rs.Automation(),
		Machine:    rs.Machine(),
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
		domains.Settings,
		domains.Automation,
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
// revision this runtime observed at startup, leaving catalog, settings, and
// automation untouched. Task transactions use it to make planning-reference
// cleanup atomic without widening the config write set.
func (rs *RuntimeState) CheckedPlanningArtifact(planning *PlanningState) (DomainArtifact, error) {
	if rs == nil || rs.coord == nil {
		return DomainArtifact{}, fmt.Errorf("runtime state is unavailable")
	}
	return rs.coord.CheckedPlanningArtifact(planning)
}

// CheckedAutomationArtifact renders an automation-only mutation against the
// revision this runtime observed at startup, leaving catalog, planning, and
// settings untouched. It is used when automation state must join a task commit.
func (rs *RuntimeState) CheckedAutomationArtifact(automation *AutomationConfig) (DomainArtifact, error) {
	if rs == nil || rs.coord == nil {
		return DomainArtifact{}, fmt.Errorf("runtime state is unavailable")
	}
	return rs.coord.CheckedAutomationArtifact(automation)
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

// CommitPlanning persists a mutated planning domain (focus queue, daily plan,
// saved views/filters, review completion) under an optimistic concurrency check
// and refreshes the snapshot. Only the planning domain is written.
func (rs *RuntimeState) CommitPlanning(planning *PlanningState) error {
	return rs.commitThenRefresh(func() error { return rs.coord.CommitPlanning(planning) })
}

// CommitSettings persists a mutated portable-settings domain plus the
// machine-local domain under optimistic concurrency and refreshes the snapshot.
// It mirrors the settings-editor commit: settings.json (synced) and machine.json
// (machine-local) are each written under a checked revision without rewriting the
// planning, catalog, or automation domains.
func (rs *RuntimeState) CommitSettings(settings *Settings, machine *MachineState) error {
	if settings == nil {
		return fmt.Errorf("settings are required")
	}
	if machine == nil {
		return fmt.Errorf("machine state is required")
	}
	return rs.commitThenRefresh(func() error {
		if err := rs.coord.CommitSettings(settings); err != nil {
			return err
		}
		return rs.coord.CommitMachine(machine)
	})
}

// CommitPortableSettings persists only settings.json and refreshes the runtime
// snapshot. CLI mutations such as global pinned-tag changes use this seam when
// they do not also change machine-local state.
func (rs *RuntimeState) CommitPortableSettings(settings *Settings) error {
	if settings == nil {
		return fmt.Errorf("settings are required")
	}
	return rs.commitThenRefresh(func() error { return rs.coord.CommitSettings(settings) })
}

// CommitAutomation persists a mutated automation domain (workflow rules,
// templates, scoring) under an optimistic concurrency check and refreshes the
// snapshot. Only the automation domain is written.
func (rs *RuntimeState) CommitAutomation(automation *AutomationConfig) error {
	return rs.commitThenRefresh(func() error { return rs.coord.CommitAutomation(automation) })
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

// SetSettingsInMemory replaces the cached portable-settings domain without
// persisting it. Isolated shell tests use this path when no workspace data
// directory exists, avoiding writes relative to the package working directory.
func (rs *RuntimeState) SetSettingsInMemory(settings *Settings) {
	if rs == nil {
		return
	}
	rs.domainsOrDefault().Settings = settings
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

// IsActiveContextMember is the single eligibility rule for the projects a
// context scope may resolve to. It is deliberately the same rule ActiveProjects
// applies, because the cross-project surfaces that consume a scope load
// active-only: a scope admitting a project outside this set would count and
// preview a project no surface ever shows. Everything it rejects — archived,
// sandbox, internal, or absent — is a hidden context member, preserved on save
// rather than resolved.
func IsActiveContextMember(p *models.Project) bool {
	return p != nil && !p.Archived && !p.Sandbox && !p.Internal
}

// ActiveProjects returns the non-archived, non-sandbox, non-internal cataloged
// projects sorted with pinned first, rehydrated from the loaded catalog snapshot
// with paths derived from the data directory. It mirrors
// config.Config.GetActiveProjects but reads the in-memory catalog domain instead
// of the facade.
func (rs *RuntimeState) ActiveProjects() []*models.Project {
	var active []*models.Project
	for _, p := range rs.catalogProjects() {
		if IsActiveContextMember(p) {
			active = append(active, p)
		}
	}
	sortProjectsWithPinnedFirst(active)
	moveProjectIDLast(active, rs.Catalog().SystemInboxProjectID)
	return active
}

// AllLoadableProjects returns all non-archived cataloged projects (internal
// included) sorted with pinned first, mirroring
// config.Config.GetAllLoadableProjects from the in-memory catalog snapshot.
func (rs *RuntimeState) AllLoadableProjects() []*models.Project {
	var projects []*models.Project
	for _, p := range rs.catalogProjects() {
		if !p.Archived {
			projects = append(projects, p)
		}
	}
	sortProjectsWithPinnedFirst(projects)
	moveProjectIDLast(projects, rs.Catalog().SystemInboxProjectID)
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

// FocusQueue returns the persisted Focus Queue from the loaded planning domain.
func (rs *RuntimeState) FocusQueue() []FocusQueueItem {
	return rs.Planning().FocusQueue
}

// GetSandboxProject returns the cataloged sandbox project (rehydrated with its
// derived path), or nil when none exists. It mirrors
// (*Config).GetSandboxProject from the in-memory catalog snapshot.
func (rs *RuntimeState) GetSandboxProject() *models.Project {
	for _, p := range rs.catalogProjects() {
		if p.Sandbox {
			return p
		}
	}
	return nil
}

// SystemInboxProject returns the catalog-owned visible Inbox project, or nil
// when an older workspace has not been upgraded yet.
func (rs *RuntimeState) SystemInboxProject() *models.Project {
	if rs == nil {
		return nil
	}
	return rs.GetProject(rs.Catalog().SystemInboxProjectID)
}

// IsSystemInboxProject reports whether id belongs to the catalog-owned Inbox.
func (rs *RuntimeState) IsSystemInboxProject(id string) bool {
	return rs != nil && rs.Catalog().IsSystemInboxProject(id)
}

// ArchivedProjects returns the archived, non-sandbox cataloged projects sorted
// by name, mirroring (*Config).GetArchivedProjects from the in-memory catalog
// snapshot.
func (rs *RuntimeState) ArchivedProjects() []*models.Project {
	var archived []*models.Project
	for _, p := range rs.catalogProjects() {
		if p.Archived && !p.Sandbox {
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

// DefaultContext returns the portable default project context from the settings
// domain, mirroring the former config.Config.DefaultContext field.
func (rs *RuntimeState) DefaultContext() string {
	return rs.Settings().DefaultContext
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

// AllContexts returns the sorted, unique context values across every cataloged
// project. It mirrors (*Config).GetAllContexts but reads the in-memory catalog
// domain instead of the facade, so the settings context selector no longer needs
// config.Config.
func (rs *RuntimeState) AllContexts() []string {
	contextSet := make(map[string]bool)
	for _, p := range rs.Catalog().Projects {
		if p.Context != "" {
			contextSet[p.Context] = true
		}
	}
	contexts := make([]string, 0, len(contextSet))
	for ctx := range contextSet {
		contexts = append(contexts, ctx)
	}
	sort.Strings(contexts)
	return contexts
}
