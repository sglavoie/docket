package config

import (
	"errors"
	"fmt"
)

// The domain committers are the whole-config Save's domain-scoped replacements:
// a settings-editor change persists only the settings + machine-local domains and
// a workflow/scoring change persists only the automation domain, each under an
// optimistic-concurrency check rather than blindly rewriting every synced file.
// Unlike Manager.Save (which decomposes and last-writer-wins every domain), a
// committer never touches the domains it does not own, so a concurrent planning
// or catalog write is not clobbered by a settings save that carried a stale copy.
//
// Machine-local state stays outside the synced workspace set: a machine commit
// writes machine.json through its own checked save (machine.json lives under the
// local runtime root, not the synced <data>/.tt/ directory), so it never rides a
// synced transaction while still refusing to overwrite a concurrently advanced
// file.
//
// The single-domain checked commits live on DomainCoordinator (it owns the store
// and is the optimistic-concurrency seam); Manager and RuntimeState expose them
// so the CLI and the long-lived TUI shell drive the same domain-scoped machinery
// without rebuilding or saving config.Config.

// commitChecked performs a revision-checked domain save. It reads the current
// on-disk revision, writes under that expectation, and retries a bounded number
// of times when a concurrent writer advances the file between the load and the
// locked write (ErrStaleObject). A non-stale error (or exhausted retries) is
// returned to the caller.
func commitChecked(loadRev func() (string, error), save func(rev string) error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		var rev string
		if rev, err = loadRev(); err != nil {
			return err
		}
		if err = save(rev); !errors.Is(err, ErrStaleObject) {
			return err
		}
	}
	return err
}

// adoptWorkspaceID reads the on-disk catalog (or mints an ID) so the domain
// artifacts a committer writes carry the same workspace identity Manager.Save
// stamps, keeping a later reload's identity consistent across the domain files.
func adoptWorkspaceID(store *DomainFileStore) error {
	if _, err := store.LoadCatalog(); err != nil {
		return err
	}
	store.EnsureWorkspaceID()
	return nil
}

// CommitCatalog persists only the workspace-catalog domain under an optimistic
// concurrency check, without rewriting the other synced domains.
func (c *DomainCoordinator) CommitCatalog(catalog *WorkspaceCatalog) error {
	if catalog == nil {
		return fmt.Errorf("workspace catalog is required")
	}
	if err := adoptWorkspaceID(c.store); err != nil {
		return err
	}
	catalog.WorkspaceID = c.store.EnsureWorkspaceID()
	return commitChecked(
		func() (string, error) { _, rev, err := c.store.LoadCatalogChecked(); return rev, err },
		func(rev string) error { return c.store.SaveCatalogChecked(catalog, rev) },
	)
}

// CommitPlanning persists only the planning domain (focus queue, daily plan,
// saved views/filters, review completion) under an optimistic concurrency check,
// without rewriting the other synced domains.
func (c *DomainCoordinator) CommitPlanning(planning *PlanningState) error {
	if planning == nil {
		return fmt.Errorf("planning state is required")
	}
	if err := adoptWorkspaceID(c.store); err != nil {
		return err
	}
	return commitChecked(
		func() (string, error) { _, rev, err := c.store.LoadPlanningChecked(); return rev, err },
		func(rev string) error { return c.store.SavePlanningChecked(planning, rev) },
	)
}

// CommitSettings persists only the portable settings domain under an optimistic
// concurrency check. It does not touch machine.json; a settings-editor change
// that also mutates machine-local state commits the machine domain separately
// through CommitMachine (see Manager.CommitSettings / RuntimeState.CommitSettings).
func (c *DomainCoordinator) CommitSettings(settings *Settings) error {
	if settings == nil {
		return fmt.Errorf("settings are required")
	}
	if err := adoptWorkspaceID(c.store); err != nil {
		return err
	}
	return commitChecked(
		func() (string, error) { _, rev, err := c.store.LoadSettingsChecked(); return rev, err },
		func(rev string) error { return c.store.SaveSettingsChecked(settings, rev) },
	)
}

// CommitAutomation persists only the automation domain (workflow rules,
// templates, scoring configuration) under an optimistic concurrency check.
func (c *DomainCoordinator) CommitAutomation(automation *AutomationConfig) error {
	if automation == nil {
		return fmt.Errorf("automation config is required")
	}
	if err := adoptWorkspaceID(c.store); err != nil {
		return err
	}
	return commitChecked(
		func() (string, error) { _, rev, err := c.store.LoadAutomationChecked(); return rev, err },
		func(rev string) error { return c.store.SaveAutomationChecked(automation, rev) },
	)
}

// CommitMachine persists only the machine-local domain under an optimistic
// concurrency check. machine.json lives under the local runtime root, outside the
// synced <data>/.tt/ set, so it never rides a synced transaction.
func (c *DomainCoordinator) CommitMachine(machine *MachineState) error {
	if machine == nil {
		return fmt.Errorf("machine state is required")
	}
	if err := adoptWorkspaceID(c.store); err != nil {
		return err
	}
	return commitChecked(
		func() (string, error) { _, rev, err := c.store.LoadMachineChecked(); return rev, err },
		func(rev string) error { return c.store.SaveMachineChecked(machine, rev) },
	)
}

// CommitCatalog persists only the workspace-catalog domain for this manager's
// workspace. It is the domain-scoped replacement for routing a project-catalog
// change through the whole-config Save.
func (m *Manager) CommitCatalog(catalog *WorkspaceCatalog) error {
	return m.DomainCoordinator().CommitCatalog(catalog)
}

// CommitPlanning persists only the planning domain for this manager's workspace.
// It is the domain-scoped replacement for routing a focus-queue/daily-plan or
// saved-view/filter change through the whole-config Save.
func (m *Manager) CommitPlanning(planning *PlanningState) error {
	return m.DomainCoordinator().CommitPlanning(planning)
}

// CommitSettings persists the portable settings domain and the machine-local
// state domain with optimistic concurrency, without rewriting the other synced
// domains. It is the domain-scoped replacement for routing a settings-editor
// change through the whole-config Save: settings.json (synced) and machine.json
// (machine-local) are each written under a checked revision so a concurrent
// writer is detected, and the machine-local file stays outside the synced set.
func (m *Manager) CommitSettings(settings *Settings, machine *MachineState) error {
	if settings == nil {
		return fmt.Errorf("settings are required")
	}
	if machine == nil {
		return fmt.Errorf("machine state is required")
	}
	coord := m.DomainCoordinator()
	if err := coord.CommitSettings(settings); err != nil {
		return err
	}
	return coord.CommitMachine(machine)
}

// CommitAutomation persists the automation domain (workflow rules, templates,
// scoring configuration) with optimistic concurrency, without rewriting the other
// synced domains. It is the domain-scoped replacement for routing a workflow-rule
// or scoring change through the whole-config Save.
func (m *Manager) CommitAutomation(automation *AutomationConfig) error {
	return m.DomainCoordinator().CommitAutomation(automation)
}

// CommitMachine persists only the machine-local state domain with optimistic
// concurrency. It is the domain-scoped replacement for routing a machine-local
// navigation/layout change (current project, view states, histories) through the
// whole-config Save.
func (m *Manager) CommitMachine(machine *MachineState) error {
	return m.DomainCoordinator().CommitMachine(machine)
}
