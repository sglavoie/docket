package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSwitchMachineByNameReturnsToThatMachineHome is the harness contract behind
// the scenario action's `machine` field. An unnamed switch must keep landing
// somewhere new; a named one must be durable, or "machine A, then B, then back
// to A" would silently read as a third machine that has never seen the
// workspace and every assertion after the round trip would be vacuous.
func TestSwitchMachineByNameReturnsToThatMachineHome(t *testing.T) {
	_, workspace, err := NewFixtureDriver(FixtureMultiProject, 100, 35)
	if err != nil {
		t.Fatalf("build fixture driver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	home := func(machine string) string {
		t.Helper()
		if err := workspace.switchMachineHome(machine); err != nil {
			t.Fatalf("switch to machine %q: %v", machine, err)
		}
		return os.Getenv("HOME")
	}

	laptop := home("laptop")
	desktop := home("desktop")
	if laptop == desktop {
		t.Fatalf("two named machines share the home %q", laptop)
	}
	if again := home("laptop"); again != laptop {
		t.Fatalf("returning to laptop landed on %q, want %q", again, laptop)
	}

	firstAnonymous := home("")
	secondAnonymous := home("")
	if firstAnonymous == secondAnonymous {
		t.Fatalf("an unnamed switch reused the home %q; it must always be a machine that has never seen this workspace", firstAnonymous)
	}
	for _, named := range []string{laptop, desktop} {
		if firstAnonymous == named || secondAnonymous == named {
			t.Fatalf("an unnamed switch landed on a named machine's home %q", named)
		}
	}
}

// TestSecondMachineDriverReadsTheSyncedWorkspace opens one workspace as a second
// machine and asserts it sees the synced half. It is the driver-level foundation
// two-machine hazard tests are written against: the data directory is shared and
// only machine identity moves.
func TestSecondMachineDriverReadsTheSyncedWorkspace(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureMultiProject, 100, 35)
	if err != nil {
		t.Fatalf("build fixture driver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	first := driver.Render()
	if !strings.Contains(first, "Active Projects") {
		t.Fatalf("first machine screen missing the root header:\n%s", first)
	}

	second, err := workspace.secondMachineDriver("laptop", 100, 35)
	if err != nil {
		t.Fatalf("open the workspace as a second machine: %v", err)
	}
	screen := second.Render()
	if !strings.Contains(screen, "Active Projects") {
		t.Fatalf("second machine did not open on the synced workspace:\n%s", screen)
	}
	// The synced catalog traveled; only the machine-local home moved.
	if got := workspace.machineHomes["laptop"]; !strings.HasPrefix(got, workspace.Root) {
		t.Fatalf("laptop home = %q, want it inside the workspace temp root %q", got, workspace.Root)
	}
	if _, err := os.Stat(filepath.Join(workspace.DataDir, ".docket", "workspace.json")); err != nil {
		t.Fatalf("the synced catalog should be untouched by a machine switch: %v", err)
	}
}

// TestSyncSnapshotAndApplyCarryTheWorkspace covers the copy helpers a sync
// service is modeled with: a captured tree replayed into an empty directory has
// to be the same workspace, and a partially-transferred one has to be the same
// workspace minus exactly the transaction files that never arrived.
func TestSyncSnapshotAndApplyCarryTheWorkspace(t *testing.T) {
	_, workspace, err := NewFixtureDriver(FixtureMultiProject, 100, 35)
	if err != nil {
		t.Fatalf("build fixture driver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	full, err := syncSnapshot(workspace.DataDir)
	if err != nil {
		t.Fatalf("snapshot workspace: %v", err)
	}
	if len(full) == 0 {
		t.Fatal("snapshot captured no files")
	}
	if _, ok := full[".docket/workspace.json"]; !ok {
		t.Fatalf("snapshot missing the catalog; captured %d files", len(full))
	}

	destination := t.TempDir()
	if applyErr := syncApply(destination, full); applyErr != nil {
		t.Fatalf("apply snapshot: %v", applyErr)
	}
	replayed, err := syncSnapshot(destination)
	if err != nil {
		t.Fatalf("snapshot the applied copy: %v", err)
	}
	if len(replayed) != len(full) {
		t.Fatalf("applied copy holds %d files, want %d", len(replayed), len(full))
	}
	for rel, data := range full {
		got, ok := replayed[rel]
		if !ok {
			t.Fatalf("applied copy is missing %s", rel)
		}
		if string(got) != string(data) {
			t.Fatalf("applied copy differs at %s", rel)
		}
	}

	// A transaction directory exists only once something has been committed
	// through the commit service, which the fixture's own writes do.
	transactions := 0
	for rel := range full {
		if strings.HasPrefix(rel, syncTransactionsPrefix) {
			transactions++
		}
	}
	partial, err := syncSnapshotPartialTransactions(workspace.DataDir)
	if err != nil {
		t.Fatalf("partial snapshot: %v", err)
	}
	switch transactions {
	case 0:
		if len(partial) != len(full) {
			t.Fatalf("with no transaction files a partial snapshot must equal a full one: %d vs %d", len(partial), len(full))
		}
	default:
		if len(partial) != len(full)-1 {
			t.Fatalf("partial snapshot holds %d files, want %d", len(partial), len(full)-1)
		}
		for rel := range full {
			if _, ok := partial[rel]; !ok && !strings.HasPrefix(rel, syncTransactionsPrefix) {
				t.Fatalf("a partial transaction transfer dropped %s, which is not a transaction file", rel)
			}
		}
	}
}
