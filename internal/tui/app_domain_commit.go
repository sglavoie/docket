package tui

import (
	"maps"
	"slices"

	"github.com/sglavoie/tt/internal/config"
)

// This file holds the shell-level helpers that commit one owning configuration
// domain through RuntimeState. Each helper mutates a defensive copy and commits
// it under the runtime's optimistic-concurrency check.

// commitPlanningMutation applies a mutation to a defensively-copied planning
// domain and commits only the planning domain. Planning owns review-completion
// timestamps and alert dismissals, so these mutations never fold into the
// settings domain or a whole-config save.
func (m *AppModel) commitPlanningMutation(mutate func(*config.PlanningState)) error {
	// A runtime without a writable data directory (some isolated unit tests) can
	// never persist a domain file; skip the commit so the in-memory shell state,
	// which callers mutate before persisting, remains the source of truth and no
	// stray planning.json is written to the process working directory.
	if m.runtime == nil || m.runtime.DataDir() == "" {
		return nil
	}
	planning := clonePlanningState(m.runtime.Planning())
	mutate(planning)
	if err := m.runtime.CommitPlanning(planning); err != nil {
		return err
	}
	return nil
}

// clonePlanningState returns a defensive copy of a planning snapshot whose
// mutable slices are freshly allocated, so a mutation applied before a checked
// commit can never alias and corrupt the cached RuntimeState snapshot.
func clonePlanningState(p *config.PlanningState) *config.PlanningState {
	cp := *p
	cp.FocusQueue = append([]config.FocusQueueItem(nil), p.FocusQueue...)
	cp.DailyPlan = append([]config.DailyPlanItem(nil), p.DailyPlan...)
	cp.FocusDismissedIDs = append([]string(nil), p.FocusDismissedIDs...)
	cp.AlertDismissals = append([]config.AlertDismissal(nil), p.AlertDismissals...)
	return &cp
}

// commitPlanningToRuntime commits a planning mutation through a runtime snapshot
// without touching the legacy config.Config facade. It is the view-level
// counterpart to AppModel.commitPlanningMutation for the focus views, which hold
// their own runtime reference and drive planning-owned reads/writes off it. A nil
// or non-writable runtime is a silent no-op so tests that never root a writable
// workspace do not attempt (or fail) a disk commit; the caller's in-memory shell
// state remains authoritative.
func commitPlanningToRuntime(runtime *config.RuntimeState, mutate func(*config.PlanningState)) error {
	if runtime == nil || runtime.DataDir() == "" {
		return nil
	}
	planning := clonePlanningState(runtime.Planning())
	mutate(planning)
	return runtime.CommitPlanning(planning)
}

// commitMachineMutation applies a mutation to a defensively-copied machine-local
// domain and commits only the machine domain through the runtime's checked
// commit, then syncs the legacy facade. Machine owns project-view state, the
// outliner/tree/cross-outliner layout, dashboard split/info-panel state, tab
// order, and task-form collapse, so these layout mutations never fold into a
// synced domain or a whole-config save. A runtime without a writable data
// directory (isolated unit tests) is a silent no-op so the caller's in-memory
// view state remains authoritative and no stray machine.json is written.
func (m *AppModel) commitMachineMutation(mutate func(*config.MachineState)) error {
	if m.runtime == nil {
		return nil
	}
	machine := cloneMachineState(m.runtime.Machine())
	mutate(machine)
	if m.runtime.DataDir() == "" {
		// No writable workspace (isolated tests): apply in memory so reads via
		// runtime.Machine() observe the mutation, without touching disk or
		// reloading the facade (which would discard the test's in-memory state).
		m.runtime.SetMachineInMemory(machine)
		return nil
	}
	if err := m.runtime.CommitMachine(machine); err != nil {
		return err
	}
	return nil
}

// commitMachineToRuntime commits a machine-local mutation through a runtime
// snapshot without touching the legacy config.Config facade. It is the
// view-level counterpart to AppModel.commitMachineMutation for views that hold
// their own runtime reference (e.g. the all-tasks tab-order handlers). A nil or
// non-writable runtime is a silent no-op so isolated tests do not attempt a disk
// commit; the caller's in-memory state remains authoritative.
func commitMachineToRuntime(runtime *config.RuntimeState, mutate func(*config.MachineState)) error {
	if runtime == nil {
		return nil
	}
	machine := cloneMachineState(runtime.Machine())
	mutate(machine)
	if runtime.DataDir() == "" {
		runtime.SetMachineInMemory(machine)
		return nil
	}
	return runtime.CommitMachine(machine)
}

// commitAutomationToRuntime applies an automation-only mutation without
// rebuilding or saving the Config facade. Isolated no-data-dir views retain the
// historical no-persistence behavior.
func commitAutomationToRuntime(runtime *config.RuntimeState, mutate func(*config.AutomationConfig)) error {
	if runtime == nil || runtime.DataDir() == "" {
		return nil
	}
	automation := cloneAutomationConfig(runtime.Automation())
	mutate(automation)
	return runtime.CommitAutomation(automation)
}

// commitSettingsToRuntime commits a portable-settings mutation through a runtime
// snapshot without touching the legacy config.Config facade. It is the
// view-level counterpart used by the kanban view, whose axis/layout preferences
// live in the Settings domain (KanbanSettings). A nil or non-writable runtime is
// a silent no-op.
func commitSettingsToRuntime(runtime *config.RuntimeState, mutate func(*config.Settings)) error {
	if runtime == nil || runtime.DataDir() == "" {
		return nil
	}
	domains, err := runtime.CloneDomains()
	if err != nil {
		return err
	}
	mutate(domains.Settings)
	return runtime.CommitSettings(domains.Settings, domains.Machine)
}

// commitCatalogMutation applies a mutation to a defensively-copied
// workspace-catalog domain and commits only the catalog domain through the
// runtime's checked commit, then syncs the legacy facade. Catalog owns the
// project list, so project create/archive/unarchive/pin/rename/delete write
// their durable effect here — one checked catalog rewrite through the journaled
// transaction boundary — instead of a whole-config Manager.Save. A runtime
// without a writable data directory (isolated unit tests) applies the mutation
// in memory so runtime.Catalog() observes it, without touching disk.
func (m *AppModel) commitCatalogMutation(mutate func(*config.WorkspaceCatalog)) error {
	if m.runtime == nil {
		return nil
	}
	catalog := cloneWorkspaceCatalog(m.runtime.Catalog())
	mutate(catalog)
	if m.runtime.DataDir() == "" {
		m.runtime.SetCatalogInMemory(catalog)
		m.syncProjectCache()
		return nil
	}
	if err := m.runtime.CommitCatalog(catalog); err != nil {
		return err
	}
	m.syncProjectCache()
	return nil
}

// commitCatalogToRuntime commits a catalog mutation through a runtime snapshot
// without touching the legacy config.Config facade. It is the view-level
// counterpart to AppModel.commitCatalogMutation for views (project creator/
// editor/cycle manager) that hold their own runtime reference and drive
// catalog-owned writes off it. A nil or non-writable runtime applies the
// mutation in memory so runtime.Catalog() observes it; the caller's in-memory
// shell state remains authoritative.
func commitCatalogToRuntime(runtime *config.RuntimeState, mutate func(*config.WorkspaceCatalog)) error {
	if runtime == nil {
		return nil
	}
	catalog := cloneWorkspaceCatalog(runtime.Catalog())
	mutate(catalog)
	if runtime.DataDir() == "" {
		runtime.SetCatalogInMemory(catalog)
		return nil
	}
	return runtime.CommitCatalog(catalog)
}

// cloneWorkspaceCatalog returns a defensive copy of a catalog snapshot whose
// project slice is freshly allocated, so a mutation applied before a checked
// commit can never alias and corrupt the cached RuntimeState snapshot.
// CatalogProject entries are value types, so a shallow slice clone fully
// isolates the copy.
func cloneWorkspaceCatalog(c *config.WorkspaceCatalog) *config.WorkspaceCatalog {
	cp := *c
	cp.Projects = slices.Clone(c.Projects)
	return &cp
}

// cloneAutomationConfig returns a defensive copy of an automation snapshot whose
// mutable slices and maps are freshly allocated, so a rule/template mutation
// applied before a checked commit can never alias and corrupt the cached
// RuntimeState snapshot. Rule/template entries are shallow-copied because every
// mutation replaces a whole entry or sets a scalar field on the cloned backing
// array; nested slices/pointers are never mutated in place.
func cloneAutomationConfig(a *config.AutomationConfig) *config.AutomationConfig {
	cp := *a
	cp.WorkflowRules = slices.Clone(a.WorkflowRules)
	cp.TaskTemplates = slices.Clone(a.TaskTemplates)
	cp.ScoringProfiles = slices.Clone(a.ScoringProfiles)
	cp.WorkflowEvalStates = maps.Clone(a.WorkflowEvalStates)
	cp.ProjectScoringProfile = maps.Clone(a.ProjectScoringProfile)
	return &cp
}

func cloneRuntimeDomains(runtime *config.RuntimeState) *config.Domains {
	if runtime == nil {
		return nil
	}
	settings := *runtime.Settings()
	return &config.Domains{
		Catalog:    cloneWorkspaceCatalog(runtime.Catalog()),
		Planning:   clonePlanningState(runtime.Planning()),
		Settings:   &settings,
		Automation: cloneAutomationConfig(runtime.Automation()),
		Machine:    cloneMachineState(runtime.Machine()),
	}
}

// commitRuntimeDomains commits a domain-spanning lifecycle mutation. The four
// synced artifacts land as one application-layer transaction; machine-local
// state is checked and committed separately because it lives outside the
// workspace transaction root.
func (m *AppModel) commitRuntimeDomains(domains *config.Domains) error {
	if m == nil || m.runtime == nil || domains == nil {
		return nil
	}
	if m.runtime.DataDir() == "" || m.appService == nil || m.configManager == nil {
		m.runtime.SetDomainsInMemory(domains)
		m.syncProjectCache()
		return nil
	}
	arts, err := syncedDomainArtifacts(
		m.configManager,
		domains.Catalog,
		domains.Planning,
		domains.Settings,
		domains.Automation,
	)
	if err != nil {
		return err
	}
	if err := m.appService.CommitConfig(arts...); err != nil {
		return err
	}
	if err := m.runtime.CommitMachine(domains.Machine); err != nil {
		return err
	}
	m.syncProjectCache()
	return nil
}

// cloneMachineState returns a defensive copy of a machine snapshot whose mutable
// maps and slices are freshly allocated, so a mutation applied before a checked
// commit can never alias and corrupt the cached RuntimeState snapshot. Pointer
// fields are shallow-copied because every mutation path replaces the pointer
// (via the Set* accessors) rather than mutating the pointed-to value in place.
func cloneMachineState(m *config.MachineState) *config.MachineState {
	cp := *m
	cp.CycleProjects = slices.Clone(m.CycleProjects)
	cp.RecentTaskVisits = slices.Clone(m.RecentTaskVisits)
	cp.RecentPaletteItems = slices.Clone(m.RecentPaletteItems)
	cp.PaletteSearchHistory = slices.Clone(m.PaletteSearchHistory)
	cp.GlobalSearchHistory = slices.Clone(m.GlobalSearchHistory)
	cp.CommandHistory = slices.Clone(m.CommandHistory)
	cp.PinnedCommands = slices.Clone(m.PinnedCommands)
	cp.CrossOutlinerFilterTags = slices.Clone(m.CrossOutlinerFilterTags)
	cp.CrossOutlinerFilterSections = slices.Clone(m.CrossOutlinerFilterSections)
	cp.AllTasksTabOrder = slices.Clone(m.AllTasksTabOrder)

	cp.ContextCycleProjects = cloneStringSliceMap(m.ContextCycleProjects)
	cp.CustomContexts = cloneStringSliceMap(m.CustomContexts)
	cp.SavedSearches = maps.Clone(m.SavedSearches)
	cp.ProjectViewStates = maps.Clone(m.ProjectViewStates)
	cp.OutlinerCollapsedSections = maps.Clone(m.OutlinerCollapsedSections)
	cp.TreeDefaultColumns = maps.Clone(m.TreeDefaultColumns)
	cp.OutlinerDefaultColumns = maps.Clone(m.OutlinerDefaultColumns)
	cp.CrossOutlinerCollapsedSections = maps.Clone(m.CrossOutlinerCollapsedSections)
	cp.CrossOutlinerFoldState = maps.Clone(m.CrossOutlinerFoldState)
	cp.CrossOutlinerColumnToggles = maps.Clone(m.CrossOutlinerColumnToggles)
	cp.CrossOutlinerFilterAttrs = maps.Clone(m.CrossOutlinerFilterAttrs)
	return &cp
}

// cloneStringSliceMap deep-copies a map whose values are slices, so neither the
// map nor any value slice aliases the cached snapshot.
func cloneStringSliceMap(src map[string][]string) map[string][]string {
	if src == nil {
		return nil
	}
	dst := make(map[string][]string, len(src))
	for k, v := range src {
		dst[k] = slices.Clone(v)
	}
	return dst
}

// commitSettingsMutation applies a mutation to a copy of the settings domain and
// commits the settings (+ machine-local) domains through the runtime's checked
// commit, then syncs the legacy facade. It is used for portable behavior
// preferences mutated outside the settings editor (e.g. tutor dismissal).
func (m *AppModel) commitSettingsMutation(mutate func(*config.Settings)) error {
	if m.runtime == nil {
		return nil
	}
	domains, err := m.runtime.CloneDomains()
	if err != nil {
		return err
	}
	mutate(domains.Settings)
	if m.runtime.DataDir() == "" {
		m.runtime.SetSettingsInMemory(domains.Settings)
		return nil
	}
	if err := m.runtime.CommitSettings(domains.Settings, domains.Machine); err != nil {
		return err
	}
	return nil
}

// commitAutomationMutation applies a mutation to a defensively-copied automation
// domain and commits only the automation domain through the runtime's checked
// commit, then syncs the legacy facade. Automation owns workflow rules,
// templates, and scoring configuration, so these mutations never fold into a
// synced behavior/appearance domain or a whole-config save.
func (m *AppModel) commitAutomationMutation(mutate func(*config.AutomationConfig)) error {
	if m.runtime == nil || m.runtime.DataDir() == "" {
		return nil
	}
	automation := cloneAutomationConfig(m.runtime.Automation())
	mutate(automation)
	if err := m.runtime.CommitAutomation(automation); err != nil {
		return err
	}
	return nil
}
