package transaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Inspect is the read-only half of the transactions directory: the doctor has to
// describe what recovery would do without doing it. These tests pin the census
// against the same fixtures the recovery tests drive, so a reading that drifts
// from Recover's judgement is a failure here rather than a wrong diagnosis in
// the CLI.

// writeRawManifest drops bytes at a manifest path without going through the
// journal, which is how a corrupt or hand-edited manifest is staged.
func writeRawManifest(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile %q: %v", path, err)
	}
}

// inspectAs runs Inspect over a root as one machine at one instant, matching the
// journal newMachineJournal would build for the same arguments.
func inspectAs(t *testing.T, root, machineID string) *Inventory {
	t.Helper()
	inv, err := Inspect(filepath.Join(root, ".docket", "transactions"), []string{root}, machineID)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	return inv
}

// findPending returns the single pending record with the given ID.
func findPending(t *testing.T, inv *Inventory, id string) PendingTransaction {
	t.Helper()
	for _, p := range inv.Pending {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("no pending transaction %q in %+v", id, inv.Pending)
	return PendingTransaction{}
}

func TestInspect_MissingDirectoryIsAnEmptyInventory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".docket", "transactions")

	inv, err := Inspect(dir, []string{root}, "machine-a")
	if err != nil {
		t.Fatalf("Inspect over a missing directory: %v", err)
	}
	if inv.Dir != dir {
		t.Fatalf("Inventory.Dir = %q, want %q", inv.Dir, dir)
	}
	if len(inv.Pending) != 0 || len(inv.Quarantined) != 0 {
		t.Fatalf("missing directory reported contents: %+v", inv)
	}
	if inv.Blocked() {
		t.Fatalf("empty inventory must not read as blocked")
	}
	if inv.QuarantineDir() != filepath.Join(dir, "quarantine") {
		t.Fatalf("QuarantineDir() = %q, want %q", inv.QuarantineDir(), filepath.Join(dir, "quarantine"))
	}
}

func TestInspect_DescribesOwnershipWithoutTouchingAnything(t *testing.T) {
	root := t.TempDir()
	owner := newMachineJournal(t, root, "machine-a", baseTestClock)
	target := filepath.Join(root, "projects", "a", "tasks.md")
	m := prepareWrite(t, owner, target, "owner bytes")
	// Inspect reads the wall clock rather than an injected one, so a fixture that
	// wants to be seen as live has to carry a heartbeat from the real present.
	stampManifestHeartbeat(t, owner.ManifestPath(m.ID), time.Now())

	// The owning machine reads its own transaction as owned and live.
	own := findPending(t, inspectAs(t, root, "machine-a"), m.ID)
	if !own.OwnedByThisMachine || own.Machine != "machine-a" {
		t.Fatalf("owner reading its own manifest: %+v", own)
	}
	if !own.Live() || own.State != StatePrepared {
		t.Fatalf("prepared transaction must read as live: %+v", own)
	}
	if own.ForeignLive() {
		t.Fatalf("a machine's own transaction is never foreign: %+v", own)
	}
	if own.EscapesRoots {
		t.Fatalf("a transaction prepared under these roots cannot escape them: %+v", own)
	}
	if own.LoadError != "" {
		t.Fatalf("unexpected LoadError %q", own.LoadError)
	}
	if own.CreatedAt == "" || own.Heartbeat == "" {
		t.Fatalf("ownership stamps missing: %+v", own)
	}

	// The other machine reads the same manifest as foreign and live, which is
	// exactly what Recover reports as ForeignLive rather than rolling forward.
	other := findPending(t, inspectAs(t, root, "machine-b"), m.ID)
	if other.OwnedByThisMachine {
		t.Fatalf("machine-b must not own machine-a's manifest: %+v", other)
	}
	if !other.HeartbeatFresh || !other.ForeignLive() {
		t.Fatalf("fresh foreign transaction must read as foreign live: %+v", other)
	}

	// Nothing moved: inspection is a read.
	assertMissing(t, target)
	if _, err := os.Stat(owner.ManifestPath(m.ID)); err != nil {
		t.Fatalf("manifest must survive inspection: %v", err)
	}
	if _, err := os.Stat(m.Ops[0].Staged); err != nil {
		t.Fatalf("staged bytes must survive inspection: %v", err)
	}
}

// TestInspect_StaleForeignTransactionIsNotForeignLive is the reading that lets
// the doctor stay quiet about a transaction recovery will adopt on next launch.
func TestInspect_StaleForeignTransactionIsNotForeignLive(t *testing.T) {
	root := t.TempDir()
	owner := newMachineJournal(t, root, "machine-a", baseTestClock)
	m := prepareWrite(t, owner, filepath.Join(root, "projects", "a", "tasks.md"), "abandoned")

	stale := stampManifestHeartbeat(t, owner.ManifestPath(m.ID), time.Now().Add(-ForeignTransactionGrace-time.Minute))
	if stale == "" {
		t.Fatalf("heartbeat rewrite produced no stamp")
	}

	pending := findPending(t, inspectAs(t, root, "machine-b"), m.ID)
	if pending.HeartbeatFresh {
		t.Fatalf("heartbeat past the grace window must not read as fresh: %+v", pending)
	}
	if pending.ForeignLive() {
		t.Fatalf("stale foreign transaction must not read as foreign live: %+v", pending)
	}
	if !pending.Live() {
		t.Fatalf("state is still prepared, so the transaction is still live: %+v", pending)
	}
}

// TestInspect_UnownedManifestReportsNoOwner covers a manifest written before
// transactions recorded ownership: it belongs to whoever opens the workspace, so
// it must not read as another machine's.
func TestInspect_UnownedManifestReportsNoOwner(t *testing.T) {
	root := t.TempDir()
	owner := newMachineJournal(t, root, "machine-a", baseTestClock)
	m := prepareWrite(t, owner, filepath.Join(root, "projects", "a", "tasks.md"), "legacy")
	stripManifestOwnership(t, owner.ManifestPath(m.ID))

	pending := findPending(t, inspectAs(t, root, "machine-b"), m.ID)
	if pending.Machine != "" || pending.OwnedByThisMachine {
		t.Fatalf("unowned manifest must report no owner: %+v", pending)
	}
	if pending.HeartbeatFresh || pending.ForeignLive() {
		t.Fatalf("unowned manifest must not read as foreign live: %+v", pending)
	}
}

// TestInspect_ReportsPathsOutsideTheRoots is the different-path sync hazard as
// the doctor sees it: the same workspace synced to another location leaves
// manifests naming absolute paths this machine must refuse.
func TestInspect_ReportsPathsOutsideTheRoots(t *testing.T) {
	root := t.TempDir()
	elsewhere := t.TempDir()

	foreign := newMachineJournal(t, elsewhere, "machine-a", baseTestClock)
	foreign.dir = filepath.Join(root, ".docket", "transactions")
	write := prepareWrite(t, foreign, filepath.Join(elsewhere, "projects", "a", "tasks.md"), "other path")

	pending := findPending(t, inspectAs(t, root, "machine-b"), write.ID)
	if !pending.EscapesRoots {
		t.Fatalf("a manifest naming paths outside the roots must report EscapesRoots: %+v", pending)
	}

	// A move is judged on its source too, not on its target alone. The manifest is
	// written directly because no journal will prepare a move that straddles its
	// own roots; only a manifest arriving from another machine's path can.
	straddling := &Manifest{
		SchemaVersion: SchemaVersion,
		ID:            "straddling-move",
		State:         StatePrepared,
		CreatedAt:     baseTestClock.Format(time.RFC3339Nano),
		Machine:       "machine-a",
		Ops: []FileOp{{
			Kind:   OpMove,
			Target: filepath.Join(root, "projects", "b"),
			Source: filepath.Join(elsewhere, "projects", "a"),
		}},
	}
	writeRawManifest(t, filepath.Join(foreign.dir, straddling.ID+".json"), mustMarshalManifest(t, straddling))

	if pending := findPending(t, inspectAs(t, root, "machine-b"), straddling.ID); !pending.EscapesRoots {
		t.Fatalf("a move whose source escapes the roots must report EscapesRoots: %+v", pending)
	}
}

// TestInspect_UnreadableManifestReportsLoadError proves a corrupt manifest is
// described rather than fatal: the doctor has to name it, and every other field
// is unknowable.
func TestInspect_UnreadableManifestReportsLoadError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".docket", "transactions")
	writeRawManifest(t, filepath.Join(dir, "corrupt.json"), "{not json")
	writeRawManifest(t, filepath.Join(dir, "forward.json"), `{"schema_version":99,"id":"forward","state":"prepared","ops":[]}`)

	inv := inspectAs(t, root, "machine-a")
	if len(inv.Pending) != 2 {
		t.Fatalf("Pending = %+v, want two records", inv.Pending)
	}
	// Sorted by ID, so corrupt precedes forward regardless of directory order.
	if inv.Pending[0].ID != "corrupt" || inv.Pending[1].ID != "forward" {
		t.Fatalf("Pending is not sorted by ID: %+v", inv.Pending)
	}
	for _, pending := range inv.Pending {
		if pending.LoadError == "" {
			t.Fatalf("%q must report a LoadError: %+v", pending.ID, pending)
		}
		if pending.State != "" || pending.Machine != "" || pending.EscapesRoots {
			t.Fatalf("%q reported fields it cannot know: %+v", pending.ID, pending)
		}
		if pending.Live() || pending.ForeignLive() {
			t.Fatalf("an unreadable manifest is not live: %+v", pending)
		}
	}
}

// TestInspect_IgnoresNonManifestEntries keeps the census to manifests: the
// staging files and the quarantine directory share the transactions directory.
func TestInspect_IgnoresNonManifestEntries(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".docket", "transactions")
	writeRawManifest(t, filepath.Join(dir, "notes.txt"), "not a manifest")
	if err := os.MkdirAll(filepath.Join(dir, "subdir.json"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	inv := inspectAs(t, root, "machine-a")
	if len(inv.Pending) != 0 {
		t.Fatalf("non-manifest entries were counted: %+v", inv.Pending)
	}
}

// TestInspect_ListsQuarantineWithoutClearingIt is the bucket the doctor needs
// most: a quarantine refuses every workspace mutation, so it has to be visible
// from a read-only command.
func TestInspect_ListsQuarantineWithoutClearingIt(t *testing.T) {
	root := t.TempDir()
	elsewhere := t.TempDir()

	foreign := newMachineJournal(t, elsewhere, "machine-a", baseTestClock)
	foreign.dir = filepath.Join(root, ".docket", "transactions")
	m := prepareWrite(t, foreign, filepath.Join(elsewhere, "projects", "a", "tasks.md"), "other path")

	here := newMachineJournal(t, root, "machine-b", baseTestClock.Add(ForeignTransactionGrace+time.Minute))
	report, err := here.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if !report.Blocked() {
		t.Fatalf("fixture did not quarantine: %+v", report)
	}

	// A second, unreadable quarantined manifest: being unreadable is one of the
	// reasons a manifest is quarantined in the first place.
	qdir := QuarantineDirFor(here.dir)
	writeRawManifest(t, filepath.Join(qdir, "aaa-corrupt.json"), "{not json")
	writeRawManifest(t, filepath.Join(qdir, "staging-leftover.txt"), "not a manifest")

	inv := inspectAs(t, root, "machine-b")
	if !inv.Blocked() {
		t.Fatalf("a quarantine must read as blocked: %+v", inv)
	}
	if inv.QuarantineDir() != qdir {
		t.Fatalf("QuarantineDir() = %q, want %q", inv.QuarantineDir(), qdir)
	}
	if len(inv.Quarantined) != 2 {
		t.Fatalf("Quarantined = %+v, want two records", inv.Quarantined)
	}
	if inv.Quarantined[0].ID != "aaa-corrupt" {
		t.Fatalf("Quarantined is not sorted by ID: %+v", inv.Quarantined)
	}
	if inv.Quarantined[0].LoadError == "" {
		t.Fatalf("unreadable quarantined manifest must report a LoadError: %+v", inv.Quarantined[0])
	}
	real := inv.Quarantined[1]
	if real.ID != m.ID || real.Machine != "machine-a" || real.State != StatePrepared {
		t.Fatalf("Quarantined[1] = %+v, want id %s owned by machine-a in state %s", real, m.ID, StatePrepared)
	}
	if real.Path != filepath.Join(qdir, m.ID+".json") {
		t.Fatalf("Quarantined[1].Path = %q, want %q", real.Path, filepath.Join(qdir, m.ID+".json"))
	}
	if _, err := os.Stat(real.Path); err != nil {
		t.Fatalf("inspection must not clear the quarantine: %v", err)
	}
}

// TestInspect_UnreadableDirectoriesAreErrors separates "nothing has ever
// committed" from "this directory cannot be read": the first is an empty
// inventory, the second must not be reported as a clean workspace.
func TestInspect_UnreadableDirectoriesAreErrors(t *testing.T) {
	t.Run("transactions directory is a file", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, ".docket", "transactions")
		writeRawManifest(t, dir, "not a directory")

		if _, err := Inspect(dir, []string{root}, "machine-a"); err == nil {
			t.Fatalf("Inspect over a non-directory must fail")
		}
	})

	t.Run("quarantine directory is a file", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, ".docket", "transactions")
		writeRawManifest(t, QuarantineDirFor(dir), "not a directory")

		if _, err := Inspect(dir, []string{root}, "machine-a"); err == nil {
			t.Fatalf("Inspect over a non-directory quarantine must fail")
		}
	})
}

func TestPendingTransaction_LiveStates(t *testing.T) {
	tests := []struct {
		state State
		want  bool
	}{
		{StatePreparing, true},
		{StatePrepared, true},
		{StateCommitting, true},
		{StateCommitted, false},
		{State(""), false},
	}
	for _, tc := range tests {
		if got := (PendingTransaction{State: tc.state}).Live(); got != tc.want {
			t.Fatalf("PendingTransaction{State: %q}.Live() = %v, want %v", tc.state, got, tc.want)
		}
	}
}

// TestInventory_NilIsNotBlocked keeps the nil guard honest: a diagnostic that
// failed to inspect must not report a blocked workspace.
func TestInventory_NilIsNotBlocked(t *testing.T) {
	var inv *Inventory
	if inv.Blocked() {
		t.Fatalf("a nil inventory must not read as blocked")
	}
}

// mustMarshalManifest renders a manifest the way writeManifest does, so a
// hand-built fixture is byte-shaped like a real one.
func mustMarshalManifest(t *testing.T, m *Manifest) string {
	t.Helper()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return string(data)
}

// stampManifestHeartbeat rewrites a manifest's heartbeat directly, standing in
// for a machine that stopped touching it. Going through writeManifest would
// refresh the stamp to the journal's own clock instead.
func stampManifestHeartbeat(t *testing.T, path string, at time.Time) string {
	t.Helper()
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest %q: %v", path, err)
	}
	m.Heartbeat = at.UTC().Format(time.RFC3339Nano)
	writeRawManifest(t, path, mustMarshalManifest(t, m))
	return m.Heartbeat
}
