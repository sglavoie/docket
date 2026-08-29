package config

import (
	"errors"
	"fmt"
)

// The domain committers are the whole-config Save's domain-scoped replacements.
// Each persists only its owning catalog, planning, or machine domain under an
// optimistic-concurrency check rather than blindly rewriting every file.
// A committer never touches the domains it does not own, so concurrent writes
// to separate domains cannot clobber one another through stale snapshots.
//
// Machine-local state stays outside the synced workspace set: a machine commit
// writes machine.json through its own checked save (machine.json lives under the
// local runtime root, not the synced <data>/.tt/ directory), so it never rides a
// synced transaction while still refusing to overwrite a concurrently advanced
// file.
//
// Lens definitions are the one portable domain that keeps that write policy
// while abandoning that location. CommitLensDefinitions writes a synced file,
// <data>/.tt/lenses.json, through the same checked save: no workspace
// transaction is opened and no undo ledger entry is recorded, so `u` never takes
// a lens back even though the definitions now travel between machines.
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

// adoptWorkspaceID reads the on-disk catalog so checked non-catalog commits are
// bound to the workspace's existing identity. These commits must not mint an ID:
// without also creating workspace.json, doing so could establish a second
// identity that a later coordinator would silently abandon.
func adoptWorkspaceID(store *DomainFileStore) error {
	_, rev, err := store.LoadCatalogChecked()
	if err != nil {
		return err
	}
	if rev == "" {
		return fmt.Errorf("%w: checked domain commit requires an existing workspace catalog", ErrWorkspaceIDRequired)
	}
	return nil
}

// CommitCatalog persists only the workspace-catalog domain under an optimistic
// concurrency check, without rewriting the other synced domains.
func (c *DomainCoordinator) CommitCatalog(catalog *WorkspaceCatalog) error {
	if catalog == nil {
		return fmt.Errorf("workspace catalog is required")
	}
	diskCatalog, diskRev, err := c.store.LoadCatalogChecked()
	if err != nil {
		return err
	}
	if diskRev != "" && catalog.WorkspaceID == "" {
		catalog.WorkspaceID = diskCatalog.WorkspaceID
	}
	if _, err := c.store.bindCatalogPayload(catalog); err != nil {
		return err
	}
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
	if err := c.publishOwnedLensDefinitions(machine.NamedLenses); err != nil {
		return err
	}
	return commitChecked(
		func() (string, error) { _, rev, err := c.store.LoadMachineChecked(); return rev, err },
		func(rev string) error { return c.store.SaveMachineChecked(machine, rev) },
	)
}

// CommitLensDefinitions persists only the portable lens-definition domain under
// an optimistic concurrency check. It is the dedicated committer the definitions
// are written through: it opens no workspace transaction and records no undo
// ledger entry, which is what keeps a lens mutation out of `u` now that the
// definitions live in the synced workspace set rather than machine.json.
func (c *DomainCoordinator) CommitLensDefinitions(lenses *WorkspaceLensDefinitions) error {
	if lenses == nil {
		return fmt.Errorf("workspace lens definitions are required")
	}
	if err := adoptWorkspaceID(c.store); err != nil {
		return err
	}
	return commitChecked(
		func() (string, error) { _, rev, err := c.store.LoadLensDefinitionsChecked(); return rev, err },
		func(rev string) error { return c.store.SaveLensDefinitionsChecked(lenses, rev) },
	)
}

// publishOwnedLensDefinitions forwards the lens half of a machine commit to the
// portable home, and only when that home already owns the collection. Ownership
// is decided by the file's existence, which mirrors the read precedence in
// LoadMachineChecked: until a workspace publishes lenses.json, a lens write is
// exactly the machine-local write it has always been, and once it does, both the
// read and the write follow the definitions to the workspace. The machine copy
// is still written beside it, so a binary that has not adopted the portable home
// keeps reading a current fallback.
func (c *DomainCoordinator) publishOwnedLensDefinitions(lenses NamedLensList) error {
	_, rev, err := c.store.LoadLensDefinitionsChecked()
	if err != nil {
		return err
	}
	if rev == "" {
		return nil
	}
	return c.CommitLensDefinitions(&WorkspaceLensDefinitions{Lenses: DefinedLensList(lenses)})
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

// CommitMachine persists only the machine-local state domain with optimistic
// concurrency. It is the domain-scoped replacement for routing a machine-local
// navigation/layout change (current project, view states, histories) through the
// whole-config Save.
func (m *Manager) CommitMachine(machine *MachineState) error {
	return m.DomainCoordinator().CommitMachine(machine)
}
