package transaction

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// baseTestClock matches newTestJournal's fixed clock, so a journal built here and
// one built there agree about what "now" is unless a test moves one of them.
var baseTestClock = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

// newMachineJournal returns a journal over root owned by an explicit machine
// identity, with a fixed clock. Two of these over one root are two machines
// sharing a synced workspace: the only difference between them is the identity
// they stamp and the identity they compare against during recovery.
func newMachineJournal(t *testing.T, root, machineID string, at time.Time) *Journal {
	t.Helper()
	j := NewJournal(filepath.Join(root, ".tt", "transactions"), []string{root}, machineID)
	var seq int
	j.newID = func() string {
		seq++
		return fmt.Sprintf("%s-%03d", machineID, seq)
	}
	j.now = func() time.Time { return at }
	return j
}

func TestPrepare_StampsOwningMachineAndHeartbeat(t *testing.T) {
	root := t.TempDir()
	j := newMachineJournal(t, root, "machine-a", baseTestClock)

	m := prepareWrite(t, j, filepath.Join(root, "projects", "a", "tasks.md"), "one")

	if m.Machine != "machine-a" {
		t.Fatalf("prepared manifest Machine = %q, want %q", m.Machine, "machine-a")
	}
	if m.Heartbeat != baseTestClock.Format(time.RFC3339Nano) {
		t.Fatalf("prepared manifest Heartbeat = %q, want %q", m.Heartbeat, baseTestClock.Format(time.RFC3339Nano))
	}
	loaded, err := LoadManifest(j.ManifestPath(m.ID))
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if loaded.Machine != "machine-a" || loaded.Heartbeat != m.Heartbeat {
		t.Fatalf("persisted ownership = (%q, %q), want (%q, %q)", loaded.Machine, loaded.Heartbeat, m.Machine, m.Heartbeat)
	}
}

// TestRecover_LeavesForeignLiveTransactionAlone is defect 2: the second machine
// opening a synced workspace must not roll the first machine's in-flight
// transaction forward. Both machines see a prepared manifest; only the owner is
// entitled to apply it.
func TestRecover_LeavesForeignLiveTransactionAlone(t *testing.T) {
	root := t.TempDir()
	owner := newMachineJournal(t, root, "machine-a", baseTestClock)
	target := filepath.Join(root, "projects", "a", "tasks.md")
	m := prepareWrite(t, owner, target, "owner bytes")

	other := newMachineJournal(t, root, "machine-b", baseTestClock.Add(time.Minute))
	report, err := other.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}

	if len(report.ForeignLive) != 1 {
		t.Fatalf("ForeignLive = %+v, want one entry", report.ForeignLive)
	}
	foreign := report.ForeignLive[0]
	if foreign.ID != m.ID || foreign.Machine != "machine-a" || foreign.State != StatePrepared {
		t.Fatalf("ForeignLive[0] = %+v, want id %s owned by machine-a in state %s", foreign, m.ID, StatePrepared)
	}
	if len(report.RolledForward) != 0 || len(report.CleanedUp) != 0 {
		t.Fatalf("foreign live transaction was acted on: rolled=%v cleaned=%v", report.RolledForward, report.CleanedUp)
	}
	if report.Blocked() {
		t.Fatalf("foreign live transaction must not block the workspace: %+v", report.Quarantined)
	}
	// The owner's staged bytes and manifest both survive, so the owner can still
	// finish. Sweeping either would destroy a commit that has already returned
	// from Prepare.
	assertMissing(t, target)
	if _, err := os.Stat(m.Ops[0].Staged); err != nil {
		t.Fatalf("foreign staged bytes must survive: %v", err)
	}
	if _, err := os.Stat(other.ManifestPath(m.ID)); err != nil {
		t.Fatalf("foreign manifest must survive: %v", err)
	}

	// The owner still commits normally afterwards: skipping is not a lock.
	if err := owner.Commit(m); err != nil {
		t.Fatalf("owner Commit after foreign recovery pass: %v", err)
	}
	if got := mustReadFile(t, target); got != "owner bytes" {
		t.Fatalf("target content = %q, want %q", got, "owner bytes")
	}
}

// TestRecover_AdoptsForeignTransactionAfterGrace is the other half: a machine
// that dies mid-commit must not wedge the workspace for the machine that is
// still working. Once the heartbeat is stale the transaction is adopted and
// finished.
func TestRecover_AdoptsForeignTransactionAfterGrace(t *testing.T) {
	root := t.TempDir()
	owner := newMachineJournal(t, root, "machine-a", baseTestClock)
	target := filepath.Join(root, "projects", "a", "tasks.md")
	m := prepareWrite(t, owner, target, "abandoned bytes")

	late := newMachineJournal(t, root, "machine-b", baseTestClock.Add(ForeignTransactionGrace+time.Minute))
	report, err := late.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if len(report.ForeignLive) != 0 {
		t.Fatalf("stale foreign transaction must be adopted, not reported: %+v", report.ForeignLive)
	}
	if len(report.RolledForward) != 1 || report.RolledForward[0] != m.ID {
		t.Fatalf("RolledForward = %v, want [%s]", report.RolledForward, m.ID)
	}
	if got := mustReadFile(t, target); got != "abandoned bytes" {
		t.Fatalf("target content = %q, want %q", got, "abandoned bytes")
	}
	assertMissing(t, late.ManifestPath(m.ID))
}

// TestRecover_AdoptsUnownedManifest covers a manifest written by a build that
// predates transaction ownership. It carries no machine, so it is this machine's
// to finish exactly as it always was — refusing it would strand the very
// transactions recovery exists for.
func TestRecover_AdoptsUnownedManifest(t *testing.T) {
	root := t.TempDir()
	owner := newMachineJournal(t, root, "machine-a", baseTestClock)
	target := filepath.Join(root, "projects", "a", "tasks.md")
	m := prepareWrite(t, owner, target, "legacy bytes")
	stripManifestOwnership(t, owner.ManifestPath(m.ID))

	other := newMachineJournal(t, root, "machine-b", baseTestClock.Add(time.Minute))
	report, err := other.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if len(report.ForeignLive) != 0 {
		t.Fatalf("unowned manifest must not read as foreign: %+v", report.ForeignLive)
	}
	if len(report.RolledForward) != 1 {
		t.Fatalf("RolledForward = %v, want one entry", report.RolledForward)
	}
	if got := mustReadFile(t, target); got != "legacy bytes" {
		t.Fatalf("target content = %q, want %q", got, "legacy bytes")
	}
}

// TestRecover_ReclaimsForeignCommittedTransaction confirms ownership gates only
// live transactions. A committed manifest's staged bytes are reclaimable from any
// machine, and leaving them would strand bytes in the synced tree.
func TestRecover_ReclaimsForeignCommittedTransaction(t *testing.T) {
	root := t.TempDir()
	owner := newMachineJournal(t, root, "machine-a", baseTestClock)
	target := filepath.Join(root, "projects", "a", "tasks.md")
	m := prepareWrite(t, owner, target, "committed bytes")
	m.State = StateCommitted
	if err := owner.writeManifest(m); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}

	other := newMachineJournal(t, root, "machine-b", baseTestClock.Add(time.Minute))
	report, err := other.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if len(report.ForeignLive) != 0 {
		t.Fatalf("committed manifest must not read as foreign: %+v", report.ForeignLive)
	}
	if len(report.CleanedUp) != 1 || report.CleanedUp[0] != m.ID {
		t.Fatalf("CleanedUp = %v, want [%s]", report.CleanedUp, m.ID)
	}
}

// TestRecover_QuarantinesStaleForeignTransactionOutsideRoots is the different-path
// sync hazard: one machine keeps the workspace somewhere the other machine's
// approved roots do not cover, so the abandoned transaction it left behind names
// absolute paths this machine must refuse. Ownership does not soften that refusal
// once the owner is gone — the transaction is quarantined, which is the state the
// doctor names and the runbook explains.
func TestRecover_QuarantinesStaleForeignTransactionOutsideRoots(t *testing.T) {
	root := t.TempDir()
	elsewhere := t.TempDir()

	foreignRoot := newMachineJournal(t, elsewhere, "machine-a", baseTestClock)
	foreignRoot.dir = filepath.Join(root, ".tt", "transactions")
	m := prepareWrite(t, foreignRoot, filepath.Join(elsewhere, "projects", "a", "tasks.md"), "other path")

	here := newMachineJournal(t, root, "machine-b", baseTestClock.Add(ForeignTransactionGrace+time.Minute))
	report, err := here.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if !report.Blocked() {
		t.Fatalf("stale foreign transaction naming paths outside the roots must quarantine, got %+v", report)
	}
	if report.Quarantined[0].ID != m.ID {
		t.Fatalf("Quarantined = %+v, want id %s", report.Quarantined, m.ID)
	}
}

// TestHeartbeatFresh covers the readings that decide whether a foreign
// transaction is live, including the future stamp two synced machines with
// skewed clocks produce.
func TestHeartbeatFresh(t *testing.T) {
	j := newMachineJournal(t, t.TempDir(), "machine-a", baseTestClock)
	tests := []struct {
		name      string
		heartbeat string
		want      bool
	}{
		{"absent", "", false},
		{"unparseable", "not-a-timestamp", false},
		{"just now", baseTestClock.Format(time.RFC3339Nano), true},
		{"inside grace", baseTestClock.Add(-ForeignTransactionGrace + time.Second).Format(time.RFC3339Nano), true},
		{"past grace", baseTestClock.Add(-ForeignTransactionGrace - time.Second).Format(time.RFC3339Nano), false},
		{"skewed into the future", baseTestClock.Add(time.Hour).Format(time.RFC3339Nano), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := j.heartbeatFresh(tc.heartbeat); got != tc.want {
				t.Fatalf("heartbeatFresh(%q) = %v, want %v", tc.heartbeat, got, tc.want)
			}
		})
	}
}

func TestMachineIDForHome_DistinguishesHomes(t *testing.T) {
	first := MachineIDForHome("/Users/one")
	second := MachineIDForHome("/Users/two")
	if first == second {
		t.Fatalf("two homes produced one identity %q", first)
	}
	if first != MachineIDForHome("/Users/one/") {
		t.Fatalf("identity is not stable under path cleaning: %q vs %q", first, MachineIDForHome("/Users/one/"))
	}
	if MachineIDForHome("") != DefaultMachineID() {
		t.Fatalf("empty home must resolve this machine")
	}
}

// stripManifestOwnership rewrites a manifest without its ownership fields,
// standing in for one written by a build that predates them. The rewrite is
// deliberately direct rather than through writeManifest, which would stamp the
// fields straight back on.
func stripManifestOwnership(t *testing.T, path string) {
	t.Helper()
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest %q: %v", path, err)
	}
	m.Machine = ""
	m.Heartbeat = ""
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write manifest %q: %v", path, err)
	}
}
