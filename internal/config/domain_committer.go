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

// commitBaselined performs a revision-checked domain save against the revision
// this session observed, then advances that baseline to the bytes it wrote.
//
// The expectation deliberately comes from the session, not from a fresh read
// taken at write time. Re-reading the revision per attempt only defeats a writer
// racing this exact call; it cannot see a writer that landed between this
// session's load and its commit, so re-reading and retrying is precisely how a
// remote write gets silently overwritten. Checking against what the session
// loaded turns that lost update into ErrStaleObject, which the caller may then
// refuse or reconcile. A coordinator that never loaded the domain observes it
// here instead, which is the same-call check the old loop opened with.
func (c *DomainCoordinator) commitBaselined(
	kind domainKind,
	loadRev func() (string, error),
	save func(rev string) (string, error),
) error {
	rev, err := c.baselineFor(kind, loadRev)
	if err != nil {
		return err
	}
	written, err := save(rev)
	if err != nil {
		return err
	}
	c.recordBaseline(kind, written)
	return nil
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
	return c.commitBaselined(
		domainCatalog,
		func() (string, error) { _, rev, err := c.store.LoadCatalogChecked(); return rev, err },
		func(rev string) (string, error) { return c.store.saveCatalogObserved(catalog, rev, true) },
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
	return c.commitBaselined(
		domainPlanning,
		func() (string, error) { _, rev, err := c.store.LoadPlanningChecked(); return rev, err },
		func(rev string) (string, error) { return c.store.savePlanningObserved(planning, rev, true) },
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
	if err := c.publishOwnedLensDefinitions(machine); err != nil {
		return err
	}
	// machine.json is the one domain with no session baseline, deliberately: it is
	// machine-local, so the only writer that can race this one is another process
	// on this same machine, and there is no remote update to lose. The revision is
	// therefore observed here and never retained, which keeps a long-lived shell
	// from refusing its own theme or active-lens write against a baseline taken
	// many mutations ago.
	_, rev, err := c.store.LoadMachineChecked()
	if err != nil {
		return err
	}
	return c.store.saveMachineObserved(machine, rev, true)
}

// CommitLensDefinitions persists only the portable lens-definition domain under
// an optimistic concurrency check. It is the dedicated committer the definitions
// are written through: it opens no workspace transaction and records no undo
// ledger entry, which is what keeps a lens mutation out of `u` now that the
// definitions live in the synced workspace set rather than machine.json.
// A stale baseline here is reconciled rather than refused, which is the one
// place this package's write policy differs per domain. A lens mutation opens no
// workspace transaction and records no undo ledger entry, so a refusal would be
// unrecoverable from the user's side: there is nothing to replay. Merging is
// also already this domain's defined reconciliation — MergeNamedLenses is the
// order-stable, idempotent join adoption uses — so honoring it on a conflicting
// write adds no second policy. The cost is that a two-way merge cannot express a
// deletion: a lens deleted here while another machine writes the same file comes
// back, because the merge cannot tell "deleted by me" from "added by them".
func (c *DomainCoordinator) CommitLensDefinitions(lenses *WorkspaceLensDefinitions) error {
	_, err := c.commitLensDefinitions(lenses)
	return err
}

// commitLensDefinitions is CommitLensDefinitions returning the collection that
// actually reached disk, which differs from the caller's when a concurrent write
// was merged in. The machine commit needs that list so machine.json and
// lenses.json do not disagree about the workspace's lenses.
func (c *DomainCoordinator) commitLensDefinitions(lenses *WorkspaceLensDefinitions) (NamedLensList, error) {
	if lenses == nil {
		return nil, fmt.Errorf("workspace lens definitions are required")
	}
	if err := adoptWorkspaceID(c.store); err != nil {
		return nil, err
	}
	rev, err := c.baselineFor(domainLenses, func() (string, error) {
		_, r, loadErr := c.store.LoadLensDefinitionsChecked()
		return r, loadErr
	})
	if err != nil {
		return nil, err
	}
	// An undefined collection is still refused by the store on the first attempt,
	// so a merge can never be reached with a nil list standing for "delete all".
	published := lenses.Lenses
	for attempt := 0; attempt < lensCommitMergeAttempts; attempt++ {
		written, saveErr := c.store.saveLensDefinitionsObserved(&WorkspaceLensDefinitions{Lenses: published}, rev, true)
		if saveErr == nil {
			c.recordBaseline(domainLenses, written)
			return published, nil
		}
		if !errors.Is(saveErr, ErrStaleObject) {
			return nil, saveErr
		}
		remote, remoteRev, loadErr := c.store.LoadLensDefinitionsChecked()
		if loadErr != nil {
			return nil, loadErr
		}
		// The on-disk collection is the merge base so its order leads and stays
		// stable across machines; this session's list is the incoming side.
		published = MergeNamedLenses(remote.Lenses, published)
		rev = remoteRev
	}
	return nil, fmt.Errorf("%w: workspace lens definitions changed on disk during %d merge attempts", ErrStaleObject, lensCommitMergeAttempts)
}

// lensCommitMergeAttempts bounds the merge-and-retry loop. Each pass reconciles
// against a strictly newer on-disk revision, so exhausting it means the file is
// being rewritten continuously and refusing is the honest outcome.
const lensCommitMergeAttempts = 3

// publishOwnedLensDefinitions forwards the lens half of a machine commit to the
// portable home, and only when that home owns the collection. Ownership is the
// file's existence or this machine's adoption marker, whichever is true first.
//
// Existence mirrors the read precedence in LoadMachineChecked: until a workspace
// publishes lenses.json, a lens write is exactly the machine-local write it has
// always been, and once it does, both the read and the write follow the
// definitions to the workspace. The marker is what carries the very first
// publication, since the file an adopting machine is about to create cannot
// already exist; afterwards it is also the reason a hand-deleted lenses.json is
// recreated by the next lens write rather than silently reverting an adopted
// machine to machine-local definitions. The machine copy is still written
// beside it, so a binary that has not adopted keeps reading a current fallback.
func (c *DomainCoordinator) publishOwnedLensDefinitions(machine *MachineState) error {
	_, rev, err := c.store.LoadLensDefinitionsChecked()
	if err != nil {
		return err
	}
	if rev == "" && !machine.LensHomeAdopted {
		return nil
	}
	published, err := c.commitLensDefinitions(&WorkspaceLensDefinitions{Lenses: DefinedLensList(machine.NamedLenses)})
	if err != nil {
		return err
	}
	// The machine copy has to carry whatever the merge published, or the fallback
	// written beside lenses.json would claim a collection the workspace no longer
	// holds, and this session would go on editing a list the merge already
	// superseded.
	machine.NamedLenses = published
	return nil
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
